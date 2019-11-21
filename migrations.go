package integram

import (
	"time"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

func migrations(db *mgo.Database, serviceName string) error {
	err := migrateMissingOAuthStores(db, serviceName)
	if err != nil {
		return err
	}

	return nil
}

func migrateMissingOAuthStores(db *mgo.Database, serviceName string) error {
	name := "MissingOAuthStores"
	n, _ := db.C("migrations").FindId(serviceName + "_" + name).Count()
	if n > 0 {
		return nil
	}

	info, err := db.C("users").UpdateAll(bson.M{
		"protected." + serviceName + ".oauthtoken": bson.M{"$exists": true, "$ne": ""},
		"$or": []bson.M{
			{"protected." + serviceName + ".oauthstore": bson.M{"$exists": false}},
			{"protected." + serviceName + ".oauthstore": ""},
		},
	},
		bson.M{"$set": bson.M{
			"protected." + serviceName + ".oauthstore": "default",
			"protected." + serviceName + ".oauthvalid": true,

		}})

	if err != nil && err != mgo.ErrNotFound {
		return err
	}

	return db.C("migrations").Insert(bson.M{"_id": serviceName + "_" + name, "date": time.Now(), "migrated": info.Updated})
}
