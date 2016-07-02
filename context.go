package integram

import (
	"encoding/json"
	"io/ioutil"

	"golang.org/x/oauth2"

	"errors"
	"fmt"
	"time"

	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"github.com/mrjones/oauth"
	"github.com/requilence/integram/url"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	tg "gopkg.in/telegram-bot-api.v3"
	"io"
	"net/http"
	uurl "net/url"
	"runtime"
)

const MAX_MSGS_TO_UPDATE_WITH_EVENTID = 10

// Per request context
type Context struct {
	ServiceName        string              // Actual service's name. Use context's Service() method to receive full service config
	ServiceBaseURL     url.URL             // Useful for self-hosted services. Default set to service's DefaultHost
	db                 *mgo.Database       // Per request MongoDB session. Use context's Db() method to get it from outside
	gin                *gin.Context        // Gin context used to access http's request and generate response
	User               User                // User associated with current webhook or Telegram update.
	Chat               Chat                // Chat associated with current webhook or Telegram update
	Message            *IncomingMessage    // Telegram incoming message if it triggired current request
	InlineQuery        *tg.InlineQuery     // Telegram inline query if it triggired current request
	ChosenInlineResult *ChosenInlineResult // Telegram chosen inline result if it triggired current request

	Callback              *Callback  // Telegram inline buttons callback if it it triggired current request
	inlineQueryAnsweredAt *time.Time // used to log slow inline responses
}

type ChosenInlineResult struct {
	tg.ChosenInlineResult
	Message *OutgoingMessage // generated message saved in DB
}

type Callback struct {
	ID         string
	Message    *OutgoingMessage // Where button was pressed
	Data       string
	AnsweredAt *time.Time
	State      int // state is used for checkbox buttons or for other switches
}

func (c *Context) SetServiceBaseURL(domainOrURL string) {
	u, _ := getBaseURL(domainOrURL)

	if u != nil {
		c.ServiceBaseURL = *u
	} else if domainOrURL != "" {
		c.ServiceBaseURL = url.URL{Scheme: "https", Host: domainOrURL}
	} else {
		c.Log().Error("Can't use SetServiceHostFromURL with empty arg")
	}
}
func (c *Context) SaveOAuthProvider(baseURL url.URL, id string, secret string) (*OAuthProvider, error) {
	if id == "" || secret == "" {
		return nil, errors.New("id and secret must not be empty")
	}

	baseURL.Host = strings.ToLower(baseURL.Host)

	provider := OAuthProvider{BaseURL: baseURL, ID: id, Secret: secret, Service: c.ServiceName}
	//TODO: multiply installations on one host are not available
	c.db.C("oauth_providers").UpsertId(provider.internalID(), provider.toBson())

	return &provider, nil
}

// return OAuthProvider details. Useful for services that can be installed on your own side
func (c *Context) OAuthProvider() *OAuthProvider {

	service := c.Service()
	if c.ServiceBaseURL.Host == "" || c.ServiceBaseURL.Host == service.DefaultBaseURL.Host {
		return service.DefaultOAuthProvider()
	} else if c.ServiceBaseURL.Host != "" {

		p, _ := findOauthProviderByHost(c.db, c.ServiceBaseURL.Host)
		if p == nil {
			p = &OAuthProvider{BaseURL: c.ServiceBaseURL, Service: c.ServiceName}
		}
		/*if err != nil {
			c.Log().WithError(err).WithField("host", c.ServiceBaseURL.Host).Error("Can't get OAuthProvider")
		}*/

		return p
	}
	c.Log().Error("Can't get OAuthProvider – empty ServiceBaseURL")

	return nil
}

func replaceBaseURL(oldUrl string, baseURL url.URL) string {
	u, err := url.Parse(oldUrl)
	if err != nil {
		return oldUrl
	}

	u.Host = baseURL.Host
	u.Scheme = baseURL.Scheme
	if baseURL.Path != "" && baseURL.Path != "/" {
		u.Path = strings.TrimRight(baseURL.Path, "/") + u.Path
		u.RawPath = "" //remove RawPath to avoid differences with Path
	}
	return u.String()
}

func (o *OAuthProvider) OAuth1Client(c *Context) *oauth.Consumer {

	if o.ID == "" {
		log.Error(errors.New("Can't get OAuth1Client – ID not set"))
		return nil
	}

	service := c.Service()
	config := service.DefaultOAuth1

	if config.AccessTokenReceiver == nil {
		log.Error(errors.New("Can't get OAuth1Client – AccessTokenReceiver not set"))

		return nil
	}

	config.Key = o.ID
	config.Secret = o.Secret
	config.AccessTokenUrl = replaceBaseURL(config.AccessTokenUrl, o.BaseURL)
	config.AuthorizeTokenUrl = replaceBaseURL(config.AuthorizeTokenUrl, o.BaseURL)
	config.RequestTokenUrl = replaceBaseURL(config.RequestTokenUrl, o.BaseURL)

	consumer := oauth.NewConsumer(
		o.ID,
		o.Secret,
		oauth.ServiceProvider{
			RequestTokenUrl:   config.RequestTokenUrl,
			AuthorizeTokenUrl: config.AuthorizeTokenUrl,
			AccessTokenUrl:    config.AccessTokenUrl,
		},
	)
	consumer.AdditionalAuthorizationUrlParams = service.DefaultOAuth1.AdditionalAuthorizationUrlParams
	return consumer
}

func (o *OAuthProvider) OAuth2Client(c *Context) *oauth2.Config {

	if o.ID == "" {
		return nil
	}

	service := c.Service()

	if service.DefaultOAuth2 == nil {
		return nil
	}

	config := service.DefaultOAuth2.Config

	config.ClientID = o.ID
	config.ClientSecret = o.Secret
	if c.User.ID != 0 {
		config.RedirectURL = c.User.OauthRedirectURL()
	}

	config.Endpoint = oauth2.Endpoint{
		AuthURL:  replaceBaseURL(config.Endpoint.AuthURL, o.BaseURL),
		TokenURL: replaceBaseURL(config.Endpoint.TokenURL, o.BaseURL),
	}

	return &config
}

/*func getTokenForApplication(c *integram.Context, app *Application, authID string, code string) (clientExists bool, token string, err error) {

	oauth2.SetAuthURLParam()
	data := url.Values{}
	data.Set("client_id", app.Key)
	data.Set("client_secret", app.Secret)
	data.Set("code", code)
	data.Add("grant_type", "authorization_code")
	data.Add("redirect_uri", integram.BaseURL+"/auth?id="+authID)
	req, _ := http.NewRequest("POST", app.BaseURL+"/oauth/token", bytes.NewBufferString(data.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	b, err := httputil.DumpRequestOut(req, true)

	resp, err := http.DefaultClient.Do(req)
	fmt.Printf("req err: %v\n%v", err, string(b))

	if err != nil {
		return
		//c.Log().WithError(err).Error("Error checking application id and secret")

	}
	defer resp.Body.Close()

	if err != nil {
		return

		//c.Log().WithError(err).Error("Error checking application id and secret")
		//return false
	}

	//b, err = ioutil.ReadAll(resp.Body)
	//fmt.Printf("resp err: %v\n%v", err, string(b))

	d := json.NewDecoder(resp.Body)
	res := struct {
		Error             string
		Error_description string
		Access_token      string
	}{}

	err = d.Decode(&res)

	if err != nil {
		return
	}
	fmt.Printf("%+v\n", res)

	if res.Error == "invalid_client" {
		// invalid_client error means Application with this Id and Secret doesn't exists
		return false, "", errors.New(res.Error + ": " + res.Error_description)
	}

	if res.Error != "" {
		return true, "", errors.New(res.Error + ": " + res.Error_description)
	}

	return true, res.Access_token, nil
}*/

type WebhookContext struct {
	gin        *gin.Context
	headers    map[string]string
	body       []byte
	firstParse bool
	requestID  string
}

func (wc *WebhookContext) FirstParse() bool {
	return wc.firstParse
}
func (wc *WebhookContext) Headers() map[string][]string {
	return map[string][]string(wc.gin.Request.Header)
}
func (wc *WebhookContext) Header(key string) string {
	return wc.gin.Request.Header.Get(key)
}
func (c *Context) KeyboardAnswer() (data string, buttonText string) {
	keyboard, err := c.Keyboard()

	if err != nil || keyboard.ChatID == 0 {
		log.WithError(err).Error("Can't get stored keyboard")
		return
	}

	// In group chat keyboard answer always include msg_id of original message that generate this keyboard
	if c.Chat.ID < 0 && c.Message.ReplyToMsgID != keyboard.MsgID {
		return
	}

	var ok bool

	if data, ok = keyboard.Keyboard[checksumString(c.Message.Text)]; ok {
		buttonText = c.Message.Text
		log.Debugf("button pressed [%v], %v\n", data, c.Message.Text)
	}

	return

}

func saveKeyboard(m *OutgoingMessage, db *mgo.Database) error {
	var err error
	if m.KeyboardMarkup != nil {
		chatKB := chatKeyboard{
			MsgID:    m.MsgID,
			BotID:    m.BotID,
			ChatID:   m.ChatID,
			Date:     time.Now(),
			Keyboard: m.KeyboardMarkup.DB(),
		}

		if m.Selective && m.ChatID < 0 {
			// For groups save keyboard for all mentioned users to know who exactly can press the button
			usersID := detectTargetUsersID(db, &m.Message)
			if len(usersID) == 0 {
				//TODO: There is workaround for this by ignoring selective
				return errors.New("You don't specify any valid users via @mentions or via reply to msg_id")
			}
			var info *mgo.ChangeInfo

			info, err = db.C("users").UpdateAll(bson.M{"_id": bson.M{"$in": usersID}}, bson.M{"$pull": bson.M{"keyboardperchat": bson.M{"chatid": m.ChatID}}})
			log.WithField("changes", *info).WithError(err).Debug("pulling exist user's keybpards")

			info, err = db.C("users").UpdateAll(bson.M{"_id": bson.M{"$in": usersID}}, bson.M{"$push": bson.M{"keyboardperchat": chatKB}})
			log.WithField("changes", *info).WithError(err).Debug("setting keyboards")

		} else {
			var info *mgo.ChangeInfo
			if m.ChatID < 0 {
				// If we send keyboard in Telegram's group chat without Selective param we need to erase all other keyboards. Even for other bots, because they will be overridden
				//	info, err = db.C("chats").UpdateAll(bson.M{}, bson.M{"$pull": bson.M{"keyboardperbot": bson.M{"chatid": m.ChatID}}})
				info, err = db.C("users").UpdateAll(bson.M{}, bson.M{"$pull": bson.M{"keyboardperchat": bson.M{"chatid": m.ChatID}}})

				log.WithField("changes", info).WithError(err).WithField("chatID", m.ChatID).Info("unsetting all user's keyboards for chat")

				log.WithField("changes", *info).WithError(err).Debug("pulling exist user's keybpards")
				kbAr := []chatKeyboard{chatKB}
				info, err = db.C("chats").UpsertId(m.ChatID, bson.M{"$set": bson.M{"keyboardperbot": kbAr}})
			} else {
				info, err = db.C("chats").UpdateAll(bson.M{"_id": m.ChatID}, bson.M{"$pull": bson.M{"keyboardperbot": bson.M{"botid": m.BotID}}})
				log.WithField("changes", info).WithError(err).WithField("chatID", m.ChatID).Info("unsetting all user's keyboards for chat")

				info, err = db.C("chats").UpsertId(m.ChatID, bson.M{"$push": bson.M{"keyboardperbot": chatKB}})
			}

			if err != nil {
				log.WithField("changes", info).WithError(err).WithField("chatid", m.ChatID).Error("Error setting keyboard for chat")
			}

		}
	} else if m.KeyboardHide {

		if m.Selective && m.ChatID < 0 {
			var info *mgo.ChangeInfo

			usersID := detectTargetUsersID(db, &m.Message)
			info, err := db.C("users").UpdateAll(bson.M{"_id": bson.M{"$in": usersID}, fmt.Sprintf("keyboardperchat.%d.botid", m.ChatID): m.BotID}, bson.M{"$unset": bson.M{fmt.Sprintf("keyboardperchat.%d", m.ChatID): true}})
			log.WithField("changes", info).WithError(err).Info("unsetting keyboards")

		} else {

			_, err = db.C("chats").UpdateAll(bson.M{"_id": m.ChatID}, bson.M{"$pull": bson.M{"keyboardperbot": bson.M{"botid": m.BotID}}})

			if err != nil {
				log.WithError(err).WithField("chatid", m.ChatID).Error("Error while unsetting keyboards")
			}
		}
	}
	return err
}

func (c *Context) Keyboard() (chatKeyboard, error) {

	udata, _ := c.User.getData()
	chatID := c.Chat.ID

	for _, kb := range udata.KeyboardPerChat {
		if kb.ChatID == chatID && kb.BotID == c.Bot().ID {
			return kb, nil
		}

	}

	cdata, _ := c.Chat.getData()

	for _, kb := range cdata.KeyboardPerBot {
		if kb.ChatID == chatID && kb.BotID == c.Bot().ID {
			return kb, nil
		}
	}

	return chatKeyboard{}, nil
}

func (c *Context) Log() *log.Entry {
	fields := log.Fields{"service": c.ServiceName}

	pc := make([]uintptr, 10)
	runtime.Callers(2, pc)
	f := runtime.FuncForPC(pc[0])
	fields["file"], fields["line"] = f.FileLine(pc[0])
	fields["func"] = f.Name()

	if c.User.ID > 0 {
		fields["user"] = c.User.ID
	}
	if c.Chat.ID > 0 {
		fields["chat"] = c.Chat.ID
	}
	if c.Message != nil {
		fields["bot"] = c.Message.BotID
		fields["msg"] = c.Message.Text
	}

	if c.ChosenInlineResult != nil {
		fields["chosenresult"] = c.ChosenInlineResult
	}

	if c.InlineQuery != nil {
		fields["inlinequery"] = c.InlineQuery
	}

	if c.Callback != nil {
		fields["callback"] = c.Callback.Data
		if c.Callback.Message.MsgID > 0 {
			fields["callback_msgid"] = c.Callback.Message.MsgID
		} else {
			fields["callback_inlinemsgid"] = c.Callback.Message.InlineMsgID
		}

	}

	if c.gin != nil {
		fields["url"] = c.gin.Request.Method + " " + c.gin.Request.URL.String()
		fields["ip"] = c.gin.Request.RemoteAddr
	}

	fields["domain"] = c.ServiceBaseURL.Host

	return log.WithFields(fields)
}

func (c *Context) Db() *mgo.Database {
	return c.db
}

func (c *Context) Service() *Service {
	s, _ := serviceByName(c.ServiceName)
	if s == nil {
		//c.Log().WithField("service", c.ServiceName).Error("No service found")
	}
	return s
}

func (c *Context) Bot() *Bot {
	return c.Service().Bot()
}

func (c *Context) EditPressedMessageText(text string) error {
	if c.Callback == nil {
		return errors.New("Callback to answer is not presented")
	}

	return c.EditMessageText(c.Callback.Message, text)
}

func (c *Context) EditPressedMessageTextAndInlineKeyboard(text string, kb InlineKeyboard) error {
	if c.Callback == nil {
		return errors.New("Callback to answer is not presented")
	}

	return c.EditMessageTextAndInlineKeyboard(c.Callback.Message, c.Callback.Message.InlineKeyboardMarkup.State, text, kb)
}

func (c *Context) EditMessageText(om *OutgoingMessage, text string) error {
	if om == nil {
		return errors.New("Empty message provided")
	}
	bot := c.Bot()
	if om.Text == text {
		return errors.New("EditMessageText: text not mofified")
	}

	_, err := bot.API.Send(tg.EditMessageTextConfig{
		BaseEdit: tg.BaseEdit{
			ChatID:      om.ChatID,
			MessageID:   om.MsgID,
			ReplyMarkup: &tg.InlineKeyboardMarkup{InlineKeyboard: om.InlineKeyboardMarkup.TG()},
		},
		ParseMode: om.ParseMode,
		Text:      text,
	})
	if err != nil {
		if err.(tg.Error).IsCantAccessChat() || err.(tg.Error).ChatMigratedToChatID() != 0 {
			if c.Callback != nil && c.Callback.AnsweredAt == nil {
				c.AnswerCallbackQuery("Sorry, message can be outdated. Bot can't edit messages created before converting to the Super Group", false)
			}
		} else if err.(tg.Error).IsAntiFlood() {
			c.Log().WithError(err).Warn("TG Anti flood activated")
		}
	}
	return err
}

func (c *Context) EditMessagesTextWithEventID(botID int64, eventID string, text string) error {
	var messages []OutgoingMessage
	//update MAX_MSGS_TO_UPDATE_WITH_EVENTID last bot messages
	c.db.C("messages").Find(bson.M{"botid": botID, "eventid": eventID}).Sort("-_id").Limit(MAX_MSGS_TO_UPDATE_WITH_EVENTID).All(&messages)
	for _, message := range messages {
		err := c.EditMessageText(&message, text)
		if err != nil {
			c.Log().WithError(err).WithField("eventid", eventID).Error("EditMessagesTextWithEventID")
		}
	}
	return nil
}

func (c *Context) EditMessagesWithEventID(botID int64, eventID string, fromState string, text string, kb InlineKeyboard) error {
	var messages []OutgoingMessage
	//update MAX_MSGS_TO_UPDATE_WITH_EVENTID last bot messages
	c.db.C("messages").Find(bson.M{"botid": botID, "eventid": eventID}).Sort("-_id").Limit(MAX_MSGS_TO_UPDATE_WITH_EVENTID).All(&messages)
	for _, message := range messages {
		err := c.EditMessageTextAndInlineKeyboard(&message, fromState, text, kb)
		if err != nil {
			c.Log().WithError(err).WithField("eventid", eventID).Error("EditMessagesWithEventID")
		}
	}
	return nil
}

func (c *Context) EditMessageTextAndInlineKeyboard(om *OutgoingMessage, fromState string, text string, kb InlineKeyboard) error {
	bot := c.Bot()
	if om.MsgID != 0 {
		log.WithField("msgID", om.MsgID).Debug("EditMessageTextAndInlineKeyboard")
	} else {
		om.ChatID = 0
		log.WithField("inlineMsgID", om.InlineMsgID).Debug("EditMessageTextAndInlineKeyboard")
	}

	var msg OutgoingMessage
	var ci *mgo.ChangeInfo
	if fromState != "" {
		ci, _ = c.db.C("messages").Find(bson.M{"_id": om.ID, "inlinekeyboardmarkup.state": fromState}).Apply(mgo.Change{Update: bson.M{"$set": bson.M{"inlinekeyboardmarkup": kb, "text": text}}}, &msg)
	} else {
		ci, _ = c.db.C("messages").Find(bson.M{"_id": om.ID}).Apply(mgo.Change{Update: bson.M{"$set": bson.M{"inlinekeyboardmarkup": kb, "text": text}}}, &msg)

	}

	if msg.BotID == 0 {
		c.Log().Warn(fmt.Sprintf("EditMessageTextAndInlineKeyboard – message (_id=%s botid=%v id=%v state %s) not found", om.ID, bot.ID, om.MsgID, fromState))
		return nil

	}
	if ci.Updated == 0 {
		return nil
	}

	_, err := bot.API.Send(tg.EditMessageTextConfig{
		BaseEdit: tg.BaseEdit{
			ChatID:          om.ChatID,
			InlineMessageID: om.InlineMsgID,
			MessageID:       om.MsgID,
			ReplyMarkup:     &tg.InlineKeyboardMarkup{InlineKeyboard: kb.TG()},
		},
		ParseMode: om.ParseMode,
		Text:      text,
		DisableWebPagePreview: !om.WebPreview,
	})

	if err != nil {
		if err.(tg.Error).IsCantAccessChat() || err.(tg.Error).ChatMigratedToChatID() != 0 {
			if c.Callback != nil {
				c.AnswerCallbackQuery("Message can be outdated. Bot can't edit messages created before converting to the Super Group", false)
			}
		} else if err.(tg.Error).IsAntiFlood() {
			c.Log().WithError(err).Warn("TG Anti flood activated")
		}
		// Oops. error is occurred – revert the original keyboard
		err := c.db.C("messages").Update(bson.M{"_id": msg.ID}, bson.M{"$set": bson.M{"inlinekeyboardmarkup": msg.InlineKeyboardMarkup}})
		return err
	}

	return nil
}

func (c *Context) EditPressedInlineKeyboard(kb InlineKeyboard) error {
	if c.Callback == nil {
		return errors.New("Callback to answer is not presented")
	}

	return c.EditInlineKeyboard(c.Callback.Message, c.Callback.Message.InlineKeyboardMarkup.State, kb)
}

func (c *Context) EditPressedInlineButton(newState int, newText string) error {
	log.WithField("newText", newText).WithField("newState", newState).Info("EditPressedInlineButton")
	if c.Callback == nil {
		return errors.New("Callback to answer is not presented")
	}

	return c.EditInlineStateButton(c.Callback.Message, c.Callback.Message.InlineKeyboardMarkup.State, c.Callback.State, c.Callback.Data, newState, newText)
}

// fromState is to avoid simultaneously changing request
func (c *Context) EditInlineKeyboard(om *OutgoingMessage, fromState string, kb InlineKeyboard) error {
	log.WithField("msgID", om.MsgID).Info("EditInlineKeyboard")

	bot := c.Bot()
	if om.MsgID != 0 {
		log.WithField("msgID", om.MsgID).Debug("EditMessageTextAndInlineKeyboard")
	} else {
		om.ChatID = 0
		log.WithField("inlineMsgID", om.InlineMsgID).Debug("EditMessageTextAndInlineKeyboard")
	}
	var msg OutgoingMessage
	ci, err := c.db.C("messages").Find(bson.M{"_id": om.ID, "inlinekeyboardmarkup.state": fromState}).Apply(mgo.Change{Update: bson.M{"$set": bson.M{"inlinekeyboardmarkup": kb}}}, &msg)

	if msg.BotID == 0 {
		return errors.New(fmt.Sprintf("EditInlineKeyboard – message (botid=%v id=%v state %s) not found", bot.ID, om.MsgID, fromState))

	}

	if ci.Updated == 0 {
		return nil
	}

	_, err = bot.API.Send(tg.EditMessageReplyMarkupConfig{
		BaseEdit: tg.BaseEdit{
			ChatID:          om.ChatID,
			MessageID:       om.MsgID,
			InlineMessageID: om.InlineMsgID,
			ReplyMarkup:     &tg.InlineKeyboardMarkup{InlineKeyboard: kb.TG()},
		},
	})

	if err != nil {
		if err.(tg.Error).IsCantAccessChat() || err.(tg.Error).ChatMigratedToChatID() != 0 {
			if c.Callback != nil {
				c.AnswerCallbackQuery("Message can be outdated. Bot can't edit messages created before converting to the Super Group", false)
			}
		} else if err.(tg.Error).IsAntiFlood() {
			c.Log().WithError(err).Warn("TG Anti flood activated")
		}
		// Oops. error is occurred – revert the original keyboard
		err := c.db.C("messages").Update(bson.M{"_id": msg.ID}, bson.M{"$set": bson.M{"inlinekeyboardmarkup": msg.InlineKeyboardMarkup}})
		return err
	}

	return nil

}
func (c *Context) EditInlineButton(om *OutgoingMessage, kbState string, buttonData string, newButtonText string) error {
	return c.EditInlineStateButton(om, kbState, 0, buttonData, 0, newButtonText)

}
func (c *Context) EditInlineStateButton(om *OutgoingMessage, kbState string, oldButtonState int, buttonData string, newButtonState int, newButtonText string) error {
	log.WithField("newText", newButtonText).Info("EditInlineButton")
	if oldButtonState > 9 || oldButtonState < 0 {
		log.WithField("data", buttonData).WithField("text", newButtonText).Errorf("EditInlineStateButton – oldButtonState must be [0-9], %s recived", oldButtonState)
	}

	if newButtonState > 9 || newButtonState < 0 {
		log.WithField("data", buttonData).WithField("text", newButtonText).Errorf("EditInlineStateButton – newButtonState must be [0-9], %s recived", newButtonState)
	}

	bot := c.Bot()

	var msg OutgoingMessage
	c.db.C("messages").Find(bson.M{"_id": om.ID, "inlinekeyboardmarkup.state": kbState}).One(&msg)
	//spew.Dump(msg)
	// need a more thread safe solution to switch stored keyboard
	if msg.BotID == 0 {
		return errors.New(fmt.Sprintf("EditInlineButton – message (botid=%v id=%v(%v) state %s) not found", bot.ID, om.MsgID, om.InlineMsgID, kbState))
	}
	i, j, _ := msg.InlineKeyboardMarkup.Find(buttonData)
	//spew.Dump(i, j)

	if i < 0 {
		return errors.New(fmt.Sprintf("EditInlineButton – button %v not found in message (botid=%v id=%v(%v) state %s) not found", buttonData, bot.ID, om.MsgID, om.InlineMsgID, kbState))
	}

	//first of all – change stored keyboard to avoid simultaneously changing requests
	set := bson.M{fmt.Sprintf("inlinekeyboardmarkup.buttons.%d.%d.text", i, j): newButtonText}

	if newButtonState != oldButtonState {
		set = bson.M{fmt.Sprintf("inlinekeyboardmarkup.buttons.%d.%d.text", i, j): newButtonText, fmt.Sprintf("inlinekeyboardmarkup.buttons.%d.%d.state", i, j): newButtonState}
	}

	info, err := c.db.C("messages").UpdateAll(bson.M{"_id": msg.ID, "inlinekeyboardmarkup.state": kbState, fmt.Sprintf("inlinekeyboardmarkup.buttons.%d.%d.data", i, j): buttonData}, bson.M{"$set": set})

	//spew.Dump(info)
	if info.Updated == 0 {
		// another one thread safe check
		return errors.New(fmt.Sprintf("EditInlineButton – button[%d][%d] %v not found in message (botid=%v id=%v(%v) state %s) not found", i, j, buttonData, bot.ID, om.MsgID, om.InlineMsgID, kbState))
	}

	kb := msg.InlineKeyboardMarkup
	kb.Buttons[i][j].Text = newButtonText
	kb.Buttons[i][j].State = newButtonState

	// todo: the stored keyboard can differ from actual because we update the whole keyboard in TG but update only target button locally
	// But maybe it's ok...
	_, err = bot.API.Send(tg.EditMessageReplyMarkupConfig{
		BaseEdit: tg.BaseEdit{
			ChatID:          om.ChatID,
			MessageID:       om.MsgID,
			InlineMessageID: om.InlineMsgID,
			ReplyMarkup:     &tg.InlineKeyboardMarkup{InlineKeyboard: kb.TG()},
		},
	})
	if err != nil {
		// Oops. error is occurred – revert the original keyboard
		err := c.db.C("messages").UpdateId(msg.ID, bson.M{"$set": bson.M{"inlinekeyboardmarkup": msg.InlineKeyboardMarkup}})
		return err
	}

	return nil
}

func (c *Context) AnswerInlineQueryWithResults(res []interface{}, cacheTime int, nextOffset string) error {
	bot := c.Bot()
	_, err := bot.API.AnswerInlineQuery(tg.InlineConfig{IsPersonal: true, InlineQueryID: c.InlineQuery.ID, Results: res, NextOffset: nextOffset})
	n := time.Now()
	c.inlineQueryAnsweredAt = &n
	return err
}

func (c *Context) AnswerInlineQueryWithPM(text string, parameter string) error {
	bot := c.Bot()
	_, err := bot.API.AnswerInlineQuery(tg.InlineConfig{IsPersonal: true, InlineQueryID: c.InlineQuery.ID, SwitchPMText: text, SwitchPMParameter: parameter})
	n := time.Now()
	c.inlineQueryAnsweredAt = &n
	return err
}

func (c *Context) AnswerCallbackQuery(text string, showAlert bool) error {
	if c.Callback == nil {
		return errors.New("Callback to answer is not presented")
	}

	if c.Callback.AnsweredAt != nil {
		return errors.New("Callback already answered")
	}

	bot := c.Bot()

	_, err := bot.API.AnswerCallbackQuery(tg.CallbackConfig{CallbackQueryID: c.Callback.ID, Text: text, ShowAlert: showAlert})
	if err == nil {
		n := time.Now()
		c.Callback.AnsweredAt = &n
	}
	return err
}

func (c *Context) NewMessage() *OutgoingMessage {
	bot := c.Bot()
	msg := &OutgoingMessage{}
	msg.BotID = bot.ID
	msg.FromID = bot.ID
	msg.WebPreview = true
	if c.Chat.ID != 0 {
		msg.ChatID = c.Chat.ID
	} else {
		msg.ChatID = c.User.ID
	}
	return msg
}

func (c *Context) SendAction(s string) error {
	_, err := c.Bot().API.Send(tg.NewChatAction(c.Chat.ID, s))
	return err
}

func (c *Context) DownloadURL(url string) (filePath string, err error) {
	out, err := ioutil.TempFile("", fmt.Sprintf("%d_%d", c.Bot().ID, c.Chat.ID))

	if err != nil {
		return "", err
	}
	defer out.Close()

	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return "", err
	}

	return out.Name(), nil
}

func (c *WebhookContext) RAW() (*[]byte, error) {
	var err error
	if c.body == nil {
		c.firstParse = true
		c.body, err = ioutil.ReadAll(c.gin.Request.Body)
		if err != nil {
			return nil, err
		}
	}
	return &c.body, nil
}

func (c *WebhookContext) JSON(out interface{}) error {
	var err error
	if c.body == nil {
		c.firstParse = true
		c.body, err = ioutil.ReadAll(c.gin.Request.Body)

		if err != nil {
			return err
		}
	}
	err = json.Unmarshal(c.body, out)

	if err != nil && strings.HasPrefix(string(c.body), "payload=") {
		s := string(c.body)
		s, err = uurl.QueryUnescape(s[8:])
		if err != nil {
			return err
		}
		err = json.Unmarshal([]byte(s), out)

	}
	return err
}

func (c *WebhookContext) Form() uurl.Values {
	return c.gin.Request.PostForm

}
func (c *WebhookContext) FormValue(key string) string {
	err := c.gin.Request.ParseForm()
	if err != nil {
		log.Error(err)
	}
	return c.gin.Request.PostForm.Get(key)

}

func (c *WebhookContext) HookID() string {
	return c.gin.Param("param")
}

func (c *WebhookContext) RequestID() string {
	return c.requestID
}
