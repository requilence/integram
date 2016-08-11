package trello

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	t "github.com/hackerlist/trello"
	"github.com/requilence/integram"
	"github.com/requilence/integram/decent"
	iurl "github.com/requilence/integram/url"
	"gopkg.in/mgo.v2/bson"
	tg "gopkg.in/telegram-bot-api.v3"
)

// TimeToJustUpdateMessage used when received to fade the message and just update the message with just posted card
const TimeToJustUpdateMessage = time.Minute * 1

type action struct {
	ID              string
	IDMemberCreator string
	Data            actionData
	Type            string
	Date            time.Time
	MemberCreator   t.Member
	Member          *t.Member
}

type attachment struct {
	PreviewURL2x string
	PreviewURL   string
	URL          string
	Name         string
	ID           string
}

type actionData struct {
	Text     string
	Value    string
	IDMember string
	Voted    bool

	Attachment *attachment

	Label *struct {
		Color string
		Name  string
		ID    string
	}

	Checklist t.Checklist

	CheckItem t.CheckItem

	Card       t.Card
	CardSource *t.Card

	List struct {
		Name string
		ID   string
	}
	ListAfter *struct {
		Name string
		ID   string
	}
	ListBefore *struct {
		Name string
		ID   string
	}
	Board       t.Board
	BoardSource *t.Board

	Old *struct {
		Name   string
		ID     string
		text   string
		IDList string
		Closed bool
		Due    *time.Time
		Desc   string
	}

	Organization *struct {
		Name string
		ID   string
	}
}

type member struct {
	ID         string
	AvatarHash string
	FullName   string
	Initials   string
	Username   string
}

type board struct {
	ShortLink string
	Name      string
	ID        string
}

func (b *board) URL() string {
	return "https://trello.com/b/" + b.ShortLink
}

type boardExtended struct {
	ID             string
	Name           string
	Desc           string
	Closed         bool
	IDOrganization string
	Pinned         bool
	ShortURL       string
	Prefs          struct {
		Voting      string
		Comments    string
		Invitations string
	}
	LabelNames map[string]string
}

// TimeOrNil custom time to workaround 'null'/non-set time in JSON
type TimeOrNil struct {
	*time.Time
}

// UnmarshalJSON checks field for 'null' string first
func (t *TimeOrNil) UnmarshalJSON(data []byte) (err error) {
	if string(data) == "null" {
		t.Time = &time.Time{}
		return
	}
	t2, err := time.Parse(`"`+time.RFC3339+`"`, string(data))
	t.Time = &t2
	return
}

type card struct {
	ShortLink string
	Name      string
	Closed    bool
	Due       TimeOrNil
	Members   []*member
	Votes     int
	ID        string
	Desc      string
}

func (c *card) url() string {
	return "https://trello.com/c/" + c.ShortLink
}

type webhook struct {
	Action action
	Model  boardExtended
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
		c.EditMessagesWithEventID("card_"+card.Id, "actions", cardText(c, card), cardInlineKeyboard(card, false))
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
func webhookHandler(c *integram.Context, wc *integram.WebhookContext) (err error) {
	u, _ := iurl.Parse("https://trello.com")
	c.ServiceBaseURL = *u

	wh := &webhook{}

	err = wc.JSON(wh)
	if err != nil {
		return
	}


	if wh.Action.ID == "" {
		return
	}
	cs := chatSettings(c)

	if _, ok := cs.Boards[wh.Model.ID]; !ok {
		return
	}

	bs := cs.Boards[wh.Model.ID]
	if !bs.Enabled {
		return
	}

	e := false

	if exists := c.Chat.Cache("action_"+wh.Action.ID, &e); exists && e {
		c.Log().Errorf("duplicate trello webhook %s, request %s, action %s, chat %s", wc.HookID(), wc.RequestID(), wh.Action.ID, c.Chat.ID)
		return
	}

	c.Chat.SetCache("action_"+wh.Action.ID, true, time.Hour)

	// if this action is produced inside the TG itself – ignore webhook (f.e. reply to comment)
	if tm, _ := c.FindMessageByEventID("action_" + wh.Action.ID); tm != nil {
		c.Log().Errorf("duplicate trello webhook %s, request %s, action %s, chat %s", wc.HookID(), wc.RequestID(), wh.Action.ID, c.Chat.ID)
		return
	}

	msg := c.NewMessage().AddEventID("action_"+wh.Action.ID, "wh_"+wc.HookID())

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

	if wh.Action.Data.List.ID != "" {
		card.List = &t.List{Name: wh.Action.Data.List.Name, Id: wh.Action.Data.List.ID}
	}

	if wh.Action.Data.Board.Id != "" {
		card.Board = &wh.Action.Data.Board
	} else {
		card.Board = &t.Board{Id: wh.Model.ID, Name: wh.Model.Name, ShortUrl: wh.Model.ShortURL, Closed: wh.Model.Closed}
	}

	// Maybe we need to update existing message?
	cardMsg, _ := cardMessage(c, card.Id)
	cardMsgJustPosted := false

	if cardMsg != nil && cardMsg.Date.Add(TimeToJustUpdateMessage).After(time.Now()) {
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
			msg.SetText("💬 "+mention(c, byMember)+": "+wh.Action.Data.Text).
				EnableHTML().
				SetReplyAction(cardReplied, card.Id).
				Send()
			return
		}

		wp := c.WebPreview(mention(c, card.MemberCreator), card.Board.Name+" • "+card.List.Name, card.Name, card.URL(), "")

		return msg.SetText("💬 "+mention(c, byMember)+": "+wh.Action.Data.Text+" "+m.URL("↗️", wp)).
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

		updateCardMessages(c, wc, card)
		if cardMsgJustPosted {
			return err
		}

		if !bs.Filter.Checklisted {
			return nil
		}

		msg.SetTextFmt("%s adds the checklist %s", mention(c, byMember), m.Bold(wh.Action.Data.Checklist.Name)).
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
			if err != nil {
				c.Log().WithError(err).Error("failed to add checklist to the card")
			}
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

		if !bs.Filter.Checklisted {
			return
		}

		if wh.Action.Type == "updateCheckItemStateOnCard" {

			if wh.Action.Data.CheckItem.State == "incomplete" {
				msg.SetTextFmt("❌ %s uncomplete %s on %s", mention(c, byMember), m.Bold(wh.Action.Data.CheckItem.Name), m.Bold(wh.Action.Data.Checklist.Name))

			} else {
				msg.SetTextFmt("✅ %s complete %s on %s", mention(c, byMember), m.Bold(wh.Action.Data.CheckItem.Name), m.Bold(wh.Action.Data.Checklist.Name))
			}

		} else {
			msg.SetTextFmt("%s adds the checklist item %s to list %s", mention(c, byMember), m.Bold(wh.Action.Data.CheckItem.Name), m.Bold(wh.Action.Data.Checklist.Name))
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
		msg.SetTextFmt("%s %s %s", mention(c, byMember), a, mention(c, wh.Action.Member))

	case "voteOnCard":

		if wh.Action.Data.Voted == true {
			c.UpdateServiceCache("card_"+card.Id, bson.M{"$addToSet": bson.M{"val.idmembersvoted": wh.Action.IDMemberCreator}}, &card)
		} else {
			c.UpdateServiceCache("card_"+card.Id, bson.M{"$pull": bson.M{"val.idmembersvoted": wh.Action.IDMemberCreator}}, &card)
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
		if strings.Contains(wh.Action.Data.Attachment.URL, "trello-attach") {
			_, err = c.Service().DoJob(downloadAttachment, c, card.Id, replyTo, "by "+mention(c, byMember), wh.Action.Data.Attachment)
			return err
		}
		msg.SetTextFmt("%s attached the link %s", mention(c, byMember), wh.Action.Data.Attachment.URL)
		// todo: reuse fileid in multichat webhooks
	case "updateCard":

		oldCard := wh.Action.Data.Old
		if oldCard == nil {
			return errors.New("updateCard without oldCard")
		}

		if oldCard.IDList != "" {
			// card moved to another list
			err = c.UpdateServiceCache("card_"+card.Id, bson.M{"$set": bson.M{"val.list": wh.Action.Data.ListAfter}}, card)
			//err = c.EditMessageText(cardMsg, cardText(c, card))
			updateCardMessages(c, wc, card)
			if cardMsgJustPosted && err == nil {
				return
			}
			if !bs.Filter.CardMoved {
				return
			}
			msg.EnableHTML()
			msg.Text = fmt.Sprintf("%s moved card to %s", mention(c, byMember), m.Fixed(wh.Action.Data.ListAfter.Name))
		} else if oldCard.Name != "" {
			// card renamed
			err = c.UpdateServiceCache("card_"+card.Id, bson.M{"$set": bson.M{"val.name": card.Name}}, card)
			updateCardMessages(c, wc, card)
			if cardMsgJustPosted && err == nil {
				return
			}
			msg.EnableHTML()
			msg.SetSilent(true)
			msg.Text = fmt.Sprintf("✏️ %s", mention(c, byMember))

		} else if oldCard.Closed != card.Closed {
			err = c.UpdateServiceCache("card_"+card.Id, bson.M{"$set": bson.M{"val.closed": card.Closed}}, card)
			updateCardMessages(c, wc, card)
			if cardMsgJustPosted && err == nil {
				return
			}
			if !bs.Filter.Archived {
				return
			}
			// archived/unarchived
			un := ""
			if card.Closed == false {
				un = "un"
			}
			msg.Text = fmt.Sprintf("%s %sarchived the card", mention(c, byMember), un)
		} else if oldCard.Due != nil {
			// due date set/unset
			err = c.UpdateServiceCache("card_"+card.Id, bson.M{"$set": bson.M{"val.due": card.Due}}, card)
			updateCardMessages(c, wc, card)
			if cardMsgJustPosted && err == nil {
				return
			}

			if !bs.Filter.Due {
				return
			}

			msg.SetSilent(true)

			if card.Due != nil && !card.Due.IsZero() {
				msg.EnableHTML()
				msg.Text = fmt.Sprintf("%s set the due date: `%v`", mention(c, byMember), decent.Relative(card.Due.In(c.User.TzLocation())))
			} else {
				msg.Text = fmt.Sprintf("%s removed the due date", mention(c, byMember))
			}
		} else if oldCard.Desc != card.Desc {
			card.Desc = cleanDesc(card.Desc)
			if card.Desc == "" {
				return
			}
			// description edited
			err = c.UpdateServiceCache("card_"+card.Id, bson.M{"$set": bson.M{"val.desc": card.Desc}}, card)
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
				msg.Text = fmt.Sprintf("✏️ %s", mention(c, byMember))
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
			msg.Text += " " + m.URL("↗️", c.WebPreview(mention(c, card.MemberCreator), card.Board.Name+" • "+card.List.Name, card.Name, card.URL(), ""))
		}
	} else {
		msg.DisableWebPreview()
	}
	msg.SetReplyAction(cardReplied, card.Id).Send()

	return
}
func mention(c *integram.Context, member *t.Member) string {
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
				kb.AddPMSwitchButton(c.Bot(), "👉  Tap me to auth", "auth")
				c.EditPressedInlineKeyboard(kb)
			} else {
				kb := integram.InlineKeyboard{}
				kb.AddPMSwitchButton(c.Bot(), "👉  Tap me to auth", "auth")
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
	}
	return errors.New("Can't find the text or file in the reply message")
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

	var a action
	err = json.Unmarshal(b, &a)

	if err != nil {
		c.Log().WithError(err).Error("attachFileToCard json error")
	}
	return c.Message.UpdateEventsID(c.Db(), "action_"+a.ID)
}

func commentCard(c *integram.Context, cardID string, text string) error {
	c.SendAction(tg.ChatTyping)

	extra := url.Values{"text": {text}}
	b, err := api(c).Request("POST", "cards/"+cardID+"/actions/comments", nil, extra)

	if err != nil {
		return err
	}

	var a action
	err = json.Unmarshal(b, &a)

	if err != nil {
		c.Log().WithError(err).Error("commentCard json error")
	}

	return c.Message.UpdateEventsID(c.Db(), "action_"+a.ID)
}

func downloadAttachment(c *integram.Context, cardID string, replyToMsgID int, text string, attachment attachment) error {
	if attachment.PreviewURL != "" {
		c.SendAction(tg.ChatUploadPhoto)
	} else {
		c.SendAction(tg.ChatUploadDocument)
	}

	var fileLocalPath string
	c.User.Cache("attachment_"+attachment.ID, &fileLocalPath)

	if fileLocalPath != "" {
		if _, err := os.Stat(fileLocalPath); os.IsNotExist(err) {
			fileLocalPath = ""
		}
	}

	if fileLocalPath == "" {
		var err error
		fileLocalPath, err = c.DownloadURL(attachment.URL)
		if err != nil {
			return err
		}
		c.User.SetCache("attachment_"+attachment.ID, fileLocalPath, time.Hour*24)
	}
	if attachment.PreviewURL != "" {
		return c.NewMessage().SetReplyAction(cardReplied, cardID).SetText(text).SetReplyToMsgID(replyToMsgID).SetImage(fileLocalPath, attachment.Name).Send()

	}
	return c.NewMessage().SetReplyAction(cardReplied, cardID).SetText(text).SetReplyToMsgID(replyToMsgID).SetDocument(fileLocalPath, attachment.Name).Send()
}

func cardMessage(c *integram.Context, cardID string) (*integram.Message, error) {
	msg, err := c.FindMessageByEventID("card_" + cardID)
	if err == nil && msg != nil {
		log.Debugf("card message found %v", cardID)

		return msg, nil
	}
	return nil, err
}
