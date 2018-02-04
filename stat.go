package integram

import (
	"fmt"
	"gopkg.in/mgo.v2/bson"
	"time"
)

type StatKey string

const (
	StatInlineQueryAnswered        StatKey = "iq_answered"
	StatInlineQueryNotAnswered     StatKey = "iq_not_answered"
	StatInlineQueryTimeouted       StatKey = "iq_timeouted"
	StatInlineQueryCanceled        StatKey = "iq_canceled"
	StatInlineQueryChosen          StatKey = "iq_chosen"
	StatInlineQueryProcessingError StatKey = "iq_error"


	StatWebhookHandled               StatKey = "wh_handled"
	StatWebhookProducedMessageToChat StatKey = "wh_message"
	StatWebhookProcessingError       StatKey = "wh_error"

	StatIncomingMessageAnswered    StatKey = "im_replied"
	StatIncomingMessageNotAnswered StatKey = "im_not_replied"

	StatOAuthSuccess StatKey = "oauth_success"
)

type stat struct {
	//ID            string  `bson:"_id"` // service_key_day
	Service       string  `bson:"s"`
	DayN          uint16  `bson:"d"` // Days since unix epoch (Unix TS)/(24*60*60)
	Key           StatKey `bson:"k"`
	Counter       uint32  `bson:"v"`
	UniqueCounter uint32  `bson:"uv"`

	Series5m       map[string]uint32 `bson:"v5m"`
	UniqueSeries5m map[string]uint32 `bson:"uv5m"`
}

// count only once per user/chat per period
type uniqueStat struct {
	ID      string  `bson:"_id"` // service_day
	Service string  `bson:"s"`
	DayN    uint16  // Days since unix epoch (Unix TS)/(24*60*60)
	Key     StatKey `bson:"k"`
	PeriodN uint16  `bson:"p"` // -1 for the whole day period
	IDS     []int64 `bson:"u"`

	ExpiresAt time.Time `bson:"exp"`
}

func (c *Context) StatIncBy(key StatKey, uniqueID int64, inc int) error {

	if !Config.MongoStatistic {
		return nil
	}

	n := time.Now()
	unixDay := uint16(n.Unix() / (24 * 60 * 60))

	//id := generateStatID(c.Bot().ID, key, unixDay)

	requesterID := c.User.ID
	if requesterID == 0 {
		requesterID = c.Chat.ID
	}

	startOfTheDayTS := int64(unixDay) * 24 * 60 * 60

	periodN := int((n.Unix() - startOfTheDayTS) / (60 * 5)) // number of 5min period within a day

	updateInc := bson.M{
		"v": inc,
		fmt.Sprintf("v5m.%d", periodN): inc,
	}

	if uniqueID != 0 {
		// check the ID uniqueness for the current day

		ci, err := c.Db().C("stats_unique").Upsert(
			bson.M{"s": c.ServiceName, "k": key, "d": unixDay, "p": -1, "u": bson.M{"$ne": uniqueID}},
			bson.M{
				"$push": bson.M{
					"u": uniqueID,
				},

				"$setOnInsert": bson.M{"expiresat": time.Unix(startOfTheDayTS+24*60*60, 0)},
			})

		if err == nil && (ci.Matched > 0 || ci.UpsertedId != nil) {
			// this ID wasn't found for the current day. So we can add to both overall and 5min period
			updateInc["uv"] = 1
		}

		// check the ID uniqueness for the current 5min period
		ci, err = c.Db().C("stats_unique").Upsert(
			bson.M{"s": c.ServiceName, "k": key, "d": unixDay, "p": periodN, "u": bson.M{"$ne": uniqueID}},
			bson.M{
				"$push": bson.M{
					"u": requesterID,
				},

				"$setOnInsert": bson.M{"exp": time.Unix(startOfTheDayTS+24*60*60, 0)},
			})

		if err == nil && (ci.Matched > 0 || ci.UpsertedId != nil) {
			// this ID wasn't found for the current day. So we can add to both overall and 5min period
			updateInc[fmt.Sprintf("uv5m.%d", periodN)] = 1
		}
	}

	_, err := c.Db().C("stats").Upsert(bson.M{"s": c.ServiceName, "k": key, "d": unixDay}, bson.M{
		"$inc":         updateInc,
		"$setOnInsert": bson.M{"s": c.ServiceName, "d": unixDay, "k": key},
	})
	return err
}

func (c *Context) StatInc(key StatKey) error {
	return c.StatIncBy(key, 0, 1)
}

func (c *Context) StatIncChat(key StatKey) error {
	return c.StatIncBy(key, c.Chat.ID, 1)
}

func (c *Context) StatIncUser(key StatKey) error {
	return c.StatIncBy(key, c.User.ID, 1)
}
