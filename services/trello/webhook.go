package trello

import (
	"encoding/json"
	"fmt"
	"time"

	iurl "github.com/requilence/integram/url"

	"errors"

	"net/url"

	log "github.com/Sirupsen/logrus"
	t "github.com/hackerlist/trello"
	"github.com/requilence/integram"
	"github.com/requilence/integram/decent"
	m "github.com/requilence/integram/html"

	"gopkg.in/mgo.v2/bson"
	tg "gopkg.in/telegram-bot-api.v3"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const TIME_TO_UPDATE_EXISTING_CARD_MESSAGE_IN_CHAT = time.Minute * 1

type Action struct {
	// Action ID
	Id              string
	IdMemberCreator string
	Data            ActionData
	Type            string
	Date            time.Time
	MemberCreator   t.Member
	Member          *t.Member
}

type Attachment struct {
	PreviewUrl2x string
	PreviewUrl   string
	Url          string
	Name         string
	Id           string
}

type ActionData struct {
	Text     string
	Value    string
	IdMember string
	Voted    bool

	Attachment *Attachment

	Label *struct {
		Color string
		Name  string
		Id    string
	}

	Checklist t.Checklist

	CheckItem t.CheckItem

	Card       t.Card
	CardSource *t.Card

	List struct {
		Name string
		Id   string
	}
	ListAfter *struct {
		Name string
		Id   string
	}
	ListBefore *struct {
		Name string
		Id   string
	}
	Board       t.Board
	BoardSource *t.Board

	Old *struct {
		Name   string
		Id     string
		text   string
		IdList string
		Closed bool
		Due    *time.Time
		Desc   string
	}

	Organization *struct {
		Name string
		Id   string
	}
}

type Member struct {
	Id         string
	AvatarHash string
	FullName   string
	Initials   string
	Username   string
}

type Board struct {
	ShortLink string
	Name      string
	Id        string
}

func (b *Board) url() string {
	return "https://trello.com/b/" + b.ShortLink
}

type BoardExtended struct {
	Id             string
	Name           string
	Desc           string
	Closed         bool
	IdOrganization string
	Pinned         bool
	ShortUrl       string
	Prefs          struct {
		Voting      string
		Comments    string
		Invitations string
	}
	LabelNames map[string]string
}

type TimeOrNil struct {
	*time.Time
}

func (t *TimeOrNil) UnmarshalJSON(data []byte) (err error) {
	if string(data) == "null" {
		t.Time = &time.Time{}
		return
	}
	t2, err := time.Parse(`"`+time.RFC3339+`"`, string(data))
	t.Time = &t2
	return
}

type Card struct {
	ShortLink string
	Name      string
	Closed    bool
	Due       TimeOrNil
	Members   []*Member
	Votes     int
	Id        string
	Desc      string
}

func (c *Card) url() string {
	return "https://trello.com/c/" + c.ShortLink
}

type Webhook struct {
	Action Action
	Model  BoardExtended
}

func cardPath(card *t.Card) (path string) {

	if card.Board != nil {
		path += card.Board.Name
	}

	if card.List != nil {
		if path != "" {
			path = " • " + path
		}
		path = card.List.Name + path
	}
	return
}

func updateCardMessages(c *integram.Context, request *integram.WebhookContext, card *t.Card) {
	if request.FirstParse() {
		c.EditMessagesWithEventID(c.Bot().ID, "card_"+card.Id, "actions", cardText(c, card), cardInlineKeyboard(card, false))
	}
}

var mdURLRe = regexp.MustCompile(`\[.*?\]\(.*?\)`)

func cleanMarkdown(desc string) string {
	return strings.Trim(mdURLRe.ReplaceAllString(desc, ""), "\n\t\r ")
}
func cleanDesc(desc string) string {
	if desc == "" {
		return ""
	}
	a := strings.Split(desc, "---\n")

	return strings.Trim(a[0], "\n\t\r ")
}
func WebhookHandler(c *integram.Context, wc *integram.WebhookContext) (err error) {
	u, _ := iurl.Parse("https://trello.com")
	c.ServiceBaseURL = *u

	wh := &Webhook{}
	err = wc.JSON(wh)
	if err != nil {
		return
	}
	//b, _ := wc.RAW()
	//log.Printf("Received trello webhook type %s for board %v chat %d: %+v", wh.Action.Type, wh.Model.Id, c.Chat.ID, wh)

	if wh.Action.Id == "" {
		return
	}
	cs := chatSettings(c)

	if _, ok := cs.Boards[wh.Model.Id]; !ok {
		return
	}

	bs := cs.Boards[wh.Model.Id]
	if !bs.Enabled {
		return
	}

	e := false

	if exists := c.Chat.Cache("action_"+wh.Action.Id, &e); exists && e {
		c.Log().Errorf("duplicate trello webhook %s, request %s, %s, action %s, chat %s", wc.HookID(), wc.RequestID(), wh.Action.Id, c.Chat.ID)
		return
	}

	c.Chat.SetCache("action_"+wh.Action.Id, true, time.Hour)

	// if this action is produced inside the TG itself – ignore webhook (f.e. reply to comment)
	if tm, _ := c.FindMessageByEventID("action_" + wh.Action.Id); tm != nil {
		c.Log().Errorf("duplicate trello webhook %s, request %s, action %s, chat %s", wc.HookID(), wc.RequestID(), wh.Action.Id, c.Chat.ID)
		return
	}

	msg := c.NewMessage().AddEventID("action_"+wh.Action.Id, "wh_"+wc.HookID())

	card := &wh.Action.Data.Card

	if wh.Action.Type != "createCard" && card != nil && card.Id != "" {
		dbCard := &t.Card{}
		// fill data from DB
		// todo:double cache fetching here (inside getCard)
		exists := c.ServiceCache("card_"+card.Id, dbCard)
		if !exists {
			api := api(c)
			if api != nil {
				dbCard, err = getCard(c, api, card.Id)
				if err != nil {
					c.Log().WithError(err).Error("error getting trello card")
				}
			}
		}

		if dbCard != nil && dbCard.Id != "" {
			card.IdMembersVoted = dbCard.IdMembersVoted
			card.List = dbCard.List
			card.Board = dbCard.Board
			card.MemberCreator = dbCard.MemberCreator
			card.Checklists = dbCard.Checklists
		} else {
			storeCard(c, card)
		}
	}

	byMember := &wh.Action.MemberCreator

	if wh.Action.Data.ListAfter != nil {
		wh.Action.Data.List = *wh.Action.Data.ListAfter
	}

	if wh.Action.Data.List.Id != "" {
		card.List = &t.List{Name: wh.Action.Data.List.Name, Id: wh.Action.Data.List.Id}
	}

	if wh.Action.Data.Board.Id != "" {
		card.Board = &wh.Action.Data.Board
	} else {
		card.Board = &t.Board{Id: wh.Model.Id, Name: wh.Model.Name, ShortUrl: wh.Model.ShortUrl, Closed: wh.Model.Closed}
	}

	// Maybe we need to update existing message?
	cardMsg, _ := cardMessage(c, card.Id)
	cardMsgJustPosted := false

	if cardMsg != nil && cardMsg.Date.Add(TIME_TO_UPDATE_EXISTING_CARD_MESSAGE_IN_CHAT).After(time.Now()) {
		cardMsgJustPosted = true
	}

	if cardMsg != nil {
		msg.SetReplyToMsgID(cardMsg.MsgID)
	}

	switch wh.Action.Type {
	case "createCard":

		if !bs.Filter.CardCreated {
			return
		}

		if cardMsgJustPosted {
			return
		}
		card.MemberCreator = byMember
		card.Pos = float64(wh.Action.Date.Unix())

		storeCard(c, card)

		return msg.SetText(cardText(c, card)).
			AddEventID("card_"+card.Id). // save initial card message to reply them in case of card-related actions
			EnableHTML().
			SetReplyAction(cardReplied, card.Id).
			SetInlineKeyboard(cardInlineKeyboard(card, false)).
			SetCallbackAction(inlineCardButtonPressed, card.Id).
			Send()

	case "commentCard":
		if !bs.Filter.CardCommented {
			return
		}

		// make a comment to reply original card message
		if cardMsg != nil {
			msg.SetText("💬 "+Mention(c, byMember)+": "+wh.Action.Data.Text).
				EnableHTML().
				SetReplyAction(cardReplied, card.Id).
				Send()
			return
		}

		wp := c.WebPreview(Mention(c, card.MemberCreator), card.Board.Name+" • "+card.List.Name, card.Name, card.URL(), "")

		return msg.SetText("💬 "+Mention(c, byMember)+": "+wh.Action.Data.Text+" "+m.URL("↗️", wp)).
			EnableHTML().
			SetReplyAction(cardReplied, card.Id).
			Send()

	case "addChecklistToCard":
		api := api(c)
		msg.SetSilent(true)

		checklist, err := api.Checklist(wh.Action.Data.Checklist.Id)
		if err != nil {
			return err
		}
		err = c.UpdateServiceCache("card_"+card.Id, bson.M{"$addToSet": bson.M{"val.checklists": checklist}}, card)

		//c.EditMessageText(cardMsg, cardText(c, card))
		updateCardMessages(c, wc, card)
		if cardMsgJustPosted {
			return err
		}

		msg.SetTextFmt("%s adds the checklist %s", Mention(c, byMember), m.Bold(wh.Action.Data.Checklist.Name)).
			EnableHTML().
			SetReplyAction(cardReplied, card.Id)

	case "createCheckItem", "updateCheckItemStateOnCard", "updateCheckItem":
		api := api(c)
		checklistIndex := -1
		msg.SetSilent(true)

		for i, checklist := range card.Checklists {
			if checklist.Id == wh.Action.Data.Checklist.Id {
				checklistIndex = i
				break
			}
		}
		if checklistIndex == -1 {
			checklist, err := api.Checklist(wh.Action.Data.Checklist.Id)
			if err != nil {
				return err
			}
			err = c.UpdateServiceCache("card_"+card.Id, bson.M{"$addToSet": bson.M{"val.checklists": checklist}}, card)
			for i, checklist := range card.Checklists {
				if checklist.Id == wh.Action.Data.Checklist.Id {
					checklistIndex = i
					break
				}
			}
		}
		if checklistIndex == -1 {
			c.Log().WithFields(log.Fields{"card": card.Id, "checklist": wh.Action.Data.Checklist.Id}).Error("Can't find checklist")
		}

		if wh.Action.Type == "createCheckItem" {
			err = c.UpdateServiceCache("card_"+card.Id, bson.M{"$addToSet": bson.M{fmt.Sprintf("val.checklists.%d.checkitems", checklistIndex): wh.Action.Data.CheckItem}}, card)
		} else if wh.Action.Type == "updateCheckItemStateOnCard" || wh.Action.Type == "updateCheckItem" {

			checkItemIndex := -1

			if checklistIndex > -1 {
				for i, CheckItem := range card.Checklists[checklistIndex].CheckItems {
					if CheckItem.Id == wh.Action.Data.CheckItem.Id {
						checkItemIndex = i
						break
					}
				}

				err = c.UpdateServiceCache("card_"+card.Id, bson.M{"$set": bson.M{fmt.Sprintf("val.checklists.%d.checkitems.%d", checklistIndex, checkItemIndex): wh.Action.Data.CheckItem}}, card)
			}
		}
		updateCardMessages(c, wc, card)

		if cardMsgJustPosted {
			return
		}

		if wh.Action.Type == "updateCheckItemStateOnCard" {

			if wh.Action.Data.CheckItem.State == "incomplete" {
				msg.SetTextFmt("❌ %s uncomplete %s on %s", Mention(c, byMember), m.Bold(wh.Action.Data.CheckItem.Name), m.Bold(wh.Action.Data.Checklist.Name))

			} else {
				msg.SetTextFmt("✅ %s complete %s on %s", Mention(c, byMember), m.Bold(wh.Action.Data.CheckItem.Name), m.Bold(wh.Action.Data.Checklist.Name))
			}

		} else {
			msg.SetTextFmt("%s adds the checklist item %s to list %s", Mention(c, byMember), m.Bold(wh.Action.Data.CheckItem.Name), m.Bold(wh.Action.Data.Checklist.Name))
		}

	case "addMemberToCard", "removeMemberFromCard":
		//todo:fix thix workaround by checking max card id
		time.Sleep(time.Millisecond * 500)
		var a string
		if wh.Action.Type == "removeMemberFromCard" {
			a = "removes"
			err = c.UpdateServiceCache("card_"+card.Id, bson.M{"$pull": bson.M{"val.members": wh.Action.Member}}, card)
			if err != nil {
				log.WithError(err).Error("Error when trying to UpdateServiceCache")
			}
		} else {
			a = "adds"

			err = c.UpdateServiceCache("card_"+card.Id, bson.M{"$addToSet": bson.M{"val.members": wh.Action.Member}}, card)
			if err != nil {
				log.WithError(err).Error("Error when trying to UpdateServiceCache")
			}
		}

		if cardMsg != nil {
			updateCardMessages(c, wc, card)

			if cardMsgJustPosted {
				return
			}

			if !bs.Filter.PersonAssigned {
				return
			}
		}
		msg.SetTextFmt("%s %s %s", Mention(c, byMember), a, Mention(c, wh.Action.Member))

	case "voteOnCard":

		if wh.Action.Data.Voted == true {
			c.UpdateServiceCache("card_"+card.Id, bson.M{"$addToSet": bson.M{"val.idmembersvoted": wh.Action.IdMemberCreator}}, &card)
		} else {
			c.UpdateServiceCache("card_"+card.Id, bson.M{"$pull": bson.M{"val.idmembersvoted": wh.Action.IdMemberCreator}}, &card)
		}

		if cardMsg != nil {
			updateCardMessages(c, wc, card)
			/*if membersVotedCount > 0 {
				err = c.EditInlineButton(cardMsg.ChatID, cardMsg.MsgID, cardMsg.InlineKeyboardMarkup.State, "vote", fmt.Sprintf("👍 %d", membersVotedCount))
			} else {
				err = c.EditInlineButton(cardMsg.ChatID, cardMsg.MsgID, cardMsg.InlineKeyboardMarkup.State, "vote", "👍")
			}*/

		}
		if err != nil {
			return err
		}
	case "addAttachmentToCard":
		replyTo := 0
		if cardMsg != nil {
			replyTo = cardMsg.MsgID
		}
		var err error
		if strings.Contains(wh.Action.Data.Attachment.Url, "trello-attach") {
			_, err = c.Service().DoJob(downloadAttachment, c, card.Id, replyTo, "by "+Mention(c, byMember), wh.Action.Data.Attachment)
			return err
		} else {
			msg.SetTextFmt("%s attached the link %s", Mention(c, byMember), wh.Action.Data.Attachment.Url)
		}
		// todo: reuse fileid in multichat webhooks
	case "updateCard":

		oldCard := wh.Action.Data.Old
		if oldCard == nil {
			return errors.New("updateCard without oldCard")
		}
		//fmt.Printf("updateCard\nold: %+v\n\nnew:%+v\n",oldCard,&wh.Action.Data.Card)

		if oldCard.IdList != "" {
			// card moved to another list
			err = c.UpdateServiceCache("card_"+card.Id, bson.M{"$set": bson.M{"val.list": wh.Action.Data.ListAfter}}, card)
			//err = c.EditMessageText(cardMsg, cardText(c, card))
			updateCardMessages(c, wc, card)
			if cardMsgJustPosted && err == nil {
				return
			}
			msg.EnableHTML()
			msg.Text = fmt.Sprintf("%s moved card to %s", Mention(c, byMember), m.Fixed(wh.Action.Data.ListAfter.Name))
		} else if oldCard.Name != "" {
			// card renamed

			err = c.UpdateServiceCache("card_"+card.Id, bson.M{"$set": bson.M{"val.name": card.Name}}, card)
			//err = c.EditMessageText(cardMsg, cardText(c, card))
			updateCardMessages(c, wc, card)
			if cardMsgJustPosted && err == nil {
				return
			}
			msg.EnableHTML()
			msg.SetSilent(true)
			msg.Text = fmt.Sprintf("✏️ %s", Mention(c, byMember))

		} else if oldCard.Closed != card.Closed {
			err = c.UpdateServiceCache("card_"+card.Id, bson.M{"$set": bson.M{"val.closed": card.Closed}}, card)

			//err = c.EditMessageText(cardMsg, cardText(c, card))
			updateCardMessages(c, wc, card)
			if cardMsgJustPosted && err == nil {
				return
			}
			// archived/unarchived
			un := ""
			if card.Closed == false {
				un = "un"
			}
			msg.Text = fmt.Sprintf("%s %sarchived the card", Mention(c, byMember), un)
		} else if oldCard.Due != nil {
			// due date set/unset
			//spew.Dump("oldCard.Due", oldCard.Due, card.Due)
			err = c.UpdateServiceCache("card_"+card.Id, bson.M{"$set": bson.M{"val.due": card.Due}}, card)
			//err = c.EditMessageText(cardMsg, cardText(c, card))
			updateCardMessages(c, wc, card)
			if cardMsgJustPosted && err == nil {
				return
			}
			msg.SetSilent(true)

			if card.Due != nil && !card.Due.IsZero() {
				msg.EnableHTML()
				msg.Text = fmt.Sprintf("%s set the due date: `%v`", Mention(c, byMember), decent.Relative(card.Due.In(c.User.TzLocation())))
			} else {
				msg.Text = fmt.Sprintf("%s removed the due date", Mention(c, byMember))
			}
		} else if oldCard.Desc != card.Desc {
			card.Desc = cleanDesc(card.Desc)
			if card.Desc == "" {
				return
			}
			// description edited
			err = c.UpdateServiceCache("card_"+card.Id, bson.M{"$set": bson.M{"val.desc": card.Desc}}, card)
			//err = c.EditMessageText(cardMsg, cardText(c, card))
			updateCardMessages(c, wc, card)

			if cardMsgJustPosted && err == nil {
				return
			}

			if cardMsg == nil {
				//hide notification because desc is not presented in webpreview
				//todo: handle this case
				return
			}

			msg.SetSilent(true)

			if card.Desc != "" {
				msg.Text = fmt.Sprintf("✏️ %s", Mention(c, byMember))
			}
		} else {
			return
		}

	default:
		return
	}

	msg.EnableHTML()
	if msg.ReplyToMsgID == 0 {
		if card == nil || card.List == nil || card.Board == nil {
			c.Log().WithField("card", card).Error("Cant create webpreview")
		} else {
			msg.Text += " " + m.URL("↗️", c.WebPreview(Mention(c, card.MemberCreator), card.Board.Name+" • "+card.List.Name, card.Name, card.URL(), ""))
		}
	} else {
		msg.DisableWebPreview()
	}
	msg.SetReplyAction(cardReplied, card.Id).Send()

	return
}
func Mention(c *integram.Context, member *t.Member) string {
	if member == nil {
		return ""
	}
	userName := ""
	c.ServiceCache("nick_map_"+member.Username, &userName)
	if userName == "" {
		return m.Bold(member.FullName)
	}
	return "@" + userName
}

func cardReplied(c *integram.Context, cardID string) error {
	//	msg,_:=c.Message.SetReplyAction()
	c.Message.SetReplyAction(cardReplied, cardID)

	if !c.User.OAuthValid() {
		c.User.SetAfterAuthAction(cardReplied, cardID)

		if c.User.IsPrivateStarted() {
			c.AnswerCallbackQuery("Open the private chat with Trello", false)
			c.User.SetCache("auth_redirect", true, time.Hour*24)
			c.NewMessage().EnableAntiFlood().SetTextFmt("You need to authorize me in order to comment cards with replies and use buttons: %s", c.User.OauthInitURL()).SetChat(c.User.ID).Send()
		} else {

			if c.Callback != nil {
				kb := c.Callback.Message.InlineKeyboardMarkup
				c.AnswerCallbackQuery("You need to authorize me\nUse the \"Tap me to auth\" button", true)
				kb.AddPMSwitchButton(c, "👉  Tap me to auth", "auth")
				c.EditPressedInlineKeyboard(kb)
			} else {
				kb := integram.InlineKeyboard{}
				kb.AddPMSwitchButton(c, "👉  Tap me to auth", "auth")
				c.NewMessage().EnableAntiFlood().SetText("You need to authorize me in order to comment cards with replies and use buttons").SetReplyToMsgID(c.Message.MsgID).SetInlineKeyboard(kb).Send()
			}

		}
		return nil
	}

	if c.Message.Document != nil {
		if c.Message.Document.FileSize > 10*1024*1024 {
			return c.NewMessage().SetReplyToMsgID(c.Message.MsgID).SetText("Sorry, Max file size for Trello is limited to 10MB").Send()
		}
		_, err := c.Service().DoJob(attachFileToCard, c, cardID, c.Message.Document)
		return err
	}

	if c.Message.Photo != nil && len(*c.Message.Photo) > 0 {
		maxQuality := 0
		maxSize := 0
		for i, photoSize := range *c.Message.Photo {
			if photoSize.FileSize > maxSize && photoSize.FileSize < 1024*1024*10 {
				maxQuality = i
			}
		}
		fileName := ""

		if c.User.UserName != "" {
			fileName += c.User.UserName
		} else if c.User.FirstName != "" {
			fileName += filepath.Clean(c.User.FirstName)
		}
		if c.Message.Caption != "" {
			fileName += "_" + filepath.Clean(c.Message.Caption)
		} else {
			fileName += fmt.Sprintf("_%d", c.Message.MsgID)
		}
		fileName += ".jpg"

		_, err := c.Service().DoJob(attachFileToCard, c, cardID, tg.Document{MimeType: "image/jpeg", FileID: (*c.Message.Photo)[maxQuality].FileID, FileName: fileName})
		return err
	}

	if c.Message.Text != "" {
		_, err := c.Service().DoJob(commentCard, c, cardID, c.Message.Text)
		return err
	} else {
		return errors.New("Can't find the text or file in the reply message")
	}

}

func attachFileToCard(c *integram.Context, cardID string, doc tg.Document) error {
	if doc.MimeType == "image/jpeg" {
		c.SendAction(tg.ChatUploadPhoto)
	} else {
		c.SendAction(tg.ChatUploadDocument)
	}

	var fileLocalPath string
	c.User.Cache("file_"+doc.FileID, &fileLocalPath)

	if fileLocalPath != "" {
		if _, err := os.Stat(fileLocalPath); os.IsNotExist(err) {
			fileLocalPath = ""
		}
	}

	if fileLocalPath == "" {
		url, err := c.Bot().API.GetFileDirectURL(doc.FileID)
		if err != nil {
			return err
		}
		fileLocalPath, err = c.DownloadURL(url)
		if err != nil {
			return err
		}
		c.User.SetCache("file_"+doc.FileID, fileLocalPath, time.Hour*24)
	}

	extra := url.Values{"mimeType": {doc.MimeType}, "name": {doc.FileName}, "url": {"null"}}

	body, contentType, err := multipartBody(extra, "file", fileLocalPath)

	if err != nil {
		return err
	}

	b, err := api(c).RequestWithHeaders("POST", "cards/"+cardID+"/attachments", body, map[string]string{"Content-Type": contentType}, nil)

	if err != nil {
		return err
	}

	var a Action
	err = json.Unmarshal(b, &a)

	return c.Message.UpdateEventsID(c.Db(), "action_"+a.Id)
}

func commentCard(c *integram.Context, cardID string, text string) error {
	c.SendAction(tg.ChatTyping)

	extra := url.Values{"text": {text}}
	b, err := api(c).Request("POST", "cards/"+cardID+"/actions/comments", nil, extra)

	if err != nil {
		return err
	}

	var a Action
	err = json.Unmarshal(b, &a)

	return c.Message.UpdateEventsID(c.Db(), "action_"+a.Id)
}

func downloadAttachment(c *integram.Context, cardID string, replyToMsgID int, text string, attachment Attachment) error {
	if attachment.PreviewUrl != "" {
		c.SendAction(tg.ChatUploadPhoto)
	} else {
		c.SendAction(tg.ChatUploadDocument)
	}

	var fileLocalPath string
	c.User.Cache("attachment_"+attachment.Id, &fileLocalPath)

	if fileLocalPath != "" {
		if _, err := os.Stat(fileLocalPath); os.IsNotExist(err) {
			fileLocalPath = ""
		}
	}

	if fileLocalPath == "" {
		var err error
		fileLocalPath, err = c.DownloadURL(attachment.Url)
		if err != nil {
			return err
		}
		c.User.SetCache("attachment_"+attachment.Id, fileLocalPath, time.Hour*24)
	}
	if attachment.PreviewUrl != "" {
		return c.NewMessage().SetReplyAction(cardReplied, cardID).SetText(text).SetReplyToMsgID(replyToMsgID).SetImage(fileLocalPath, attachment.Name).Send()

	} else {
		return c.NewMessage().SetReplyAction(cardReplied, cardID).SetText(text).SetReplyToMsgID(replyToMsgID).SetDocument(fileLocalPath, attachment.Name).Send()
	}

}

/*func cardMessageID(c *integram.Context, cardID string) int {
	var cardMessageBsonID bson.ObjectId
	c.Chat.Cache("card_"+cardID, &cardMessageBsonID)

	cardMessage, err := c.FindMessageByBsonID(cardMessageBsonID)
	if err == nil && cardMessage != nil {
		return cardMessage.MsgID
	}
	return 0
}*/

func cardMessage(c *integram.Context, cardID string) (*integram.OutgoingMessage, error) {
	msg, err := c.FindMessageByEventID("card_" + cardID)
	if err == nil && msg != nil {
		log.Infof("card message found %v", cardID)

		return msg.FindOutgoingMessage(c.Db())
	}
	return nil, err
}
