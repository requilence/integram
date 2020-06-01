package integram

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	uurl "net/url"
	"runtime"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kennygrant/sanitize"
	"github.com/mrjones/oauth"
	tg "github.com/requilence/telegram-bot-api"
	"github.com/requilence/url"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"os"
	"path/filepath"
)

// MaxMsgsToUpdateWithEventID set the maximum number of last messages to update with EditMessagesTextWithEventID
var MaxMsgsToUpdateWithEventID = 10

// Context of the current request
type Context struct {
	ServiceName        string              // Actual service's name. Use context's Service() method to receive full service config
	ServiceBaseURL     url.URL             // Useful for self-hosted services. Default set to service's DefaultHost
	db                 *mgo.Database       // Per request MongoDB session. Use context's Db() method to get it from outside
	gin                *gin.Context        // Gin context used to access http's request and generate response
	User               User                // User associated with current webhook or Telegram update.
	Chat               Chat                // Chat associated with current webhook or Telegram update
	Message            *IncomingMessage    // Telegram incoming message if it triggired current request
	MessageEdited      bool                // True if Message is edited message instead of the new one
	InlineQuery        *tg.InlineQuery     // Telegram inline query if it triggired current request
	ChosenInlineResult *chosenInlineResult // Telegram chosen inline result if it triggired current request

	Callback              *callback  // Telegram inline buttons callback if it it triggired current request
	inlineQueryAnsweredAt *time.Time // used to log slow inline responses
	messageAnsweredAt     *time.Time // used to log slow messages responses

}

type chosenInlineResult struct {
	tg.ChosenInlineResult
	Message *OutgoingMessage // generated message saved in DB
}

type callback struct {
	ID         string
	Message    *OutgoingMessage // Where button was pressed
	Data       string
	AnsweredAt *time.Time
	State      int // state is used for checkbox buttons or for other switches
}

func (c *Context) SetDb(database *mgo.Database) {
	c.db = database
}

// SetServiceBaseURL set the baseURL for the current request. Useful when service can be self-hosted. The actual service URL can be found in the incoming webhook
func (c *Context) SetServiceBaseURL(domainOrURL string) {
	u, _ := getBaseURL(domainOrURL)

	if u != nil && u.Host != "" {
		c.ServiceBaseURL = *u
	} else if domainOrURL != "" {
		c.ServiceBaseURL = url.URL{Scheme: "https", Host: domainOrURL}
	} else {
		c.Log().Error("Can't use SetServiceHostFromURL with empty arg")
	}
}

// SaveOAuthProvider add the OAuth client to DB. Useful when the new OAuth provider registred for self-hosted services
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

// OAuthProvider details. Useful for services that can be installed on your own side
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

func (c *Context) getAndStoreOAuth2TokenFromAuthCode(code string) error {
	var expiresAt *time.Time
	var accessToken string
	var refreshToken string

	otoken, err := c.OAuthProvider().OAuth2Client(c).Exchange(oauth2.NoContext, code)
	if err != nil {
		return fmt.Errorf("failed to get token: %s", err.Error())
	}

	if otoken != nil {
		accessToken = otoken.AccessToken
		refreshToken = otoken.RefreshToken
		expiresAt = &otoken.Expiry
	}

	err = oauthTokenStore.SetOAuthAccessToken(&c.User, accessToken, expiresAt)
	if err != nil {
		log.WithError(err).Error("Can't save OAuth token to store")
		return fmt.Errorf("failed to store oauth token")
	}

	if refreshToken != "" {
		oauthTokenStore.SetOAuthRefreshToken(&c.User, refreshToken)
	}

	return nil
}

// OAuthFinishFromCommand should be called in the OAuth2 flow after bot gets '/start oauth_<codeKey>' command
// it extracts the actual Authorization code from the cache, ensures it match the current user ID
func (c *Context) OAuthFinishFromCommand(codeKey string) error {
	s := c.Service()
	if s.DefaultOAuth2 == nil {
		return fmt.Errorf("oauth2 config not set")
	}
	var codeInfo oAuthCallbackCodeInfo
	exists := c.ServiceCache("oauth_code_"+codeKey, &codeInfo)
	if !exists {
		return fmt.Errorf("oauth code not found")
	}

	if codeInfo.State != c.User.AuthTempToken() {
		return fmt.Errorf("oauth state mismatches the one for the current TG user")
	}

	val := oAuthIDCache{}
	err := c.Db().C("users_cache").Find(bson.M{"key": "auth_" + codeInfo.State}).One(&val)
	if err != nil || val.UserID == 0 {
		return fmt.Errorf("failed to find oauth state info")
	}

	if val.UserID != c.User.ID {
		return fmt.Errorf("oauth state has mismatching user ID")
	}

	err = c.getAndStoreOAuth2TokenFromAuthCode(codeInfo.Code)
	if err != nil {
		return err
	}

	c.StatIncUser(StatOAuthSuccess)

	if s.OAuthSuccessful != nil {
		s.DoJob(s.OAuthSuccessful, c)
	}

	return nil
}

func replaceBaseURL(oldURL string, baseURL url.URL) string {
	u, err := url.Parse(oldURL)
	if err != nil {
		return oldURL
	}

	u.Host = baseURL.Host
	u.Scheme = baseURL.Scheme
	if baseURL.Path != "" && baseURL.Path != "/" {
		u.Path = strings.TrimRight(baseURL.Path, "/") + u.Path
		u.RawPath = "" //remove RawPath to avoid differences with Path
	}
	return u.String()
}

// OAuth1Client returns oauth.Consumer using OAuthProvider details
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
	config.AccessTokenURL = replaceBaseURL(config.AccessTokenURL, o.BaseURL)
	config.AuthorizeTokenURL = replaceBaseURL(config.AuthorizeTokenURL, o.BaseURL)
	config.RequestTokenURL = replaceBaseURL(config.RequestTokenURL, o.BaseURL)

	consumer := oauth.NewConsumer(
		o.ID,
		o.Secret,
		oauth.ServiceProvider{
			RequestTokenUrl:   config.RequestTokenURL,
			AuthorizeTokenUrl: config.AuthorizeTokenURL,
			AccessTokenUrl:    config.AccessTokenURL,
		},
	)
	consumer.AdditionalAuthorizationUrlParams = service.DefaultOAuth1.AdditionalAuthorizationURLParams
	return consumer
}

// OAuth2Client returns oauth2.Config using OAuthProvider details
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

// WebhookContext is passed to WebhookHandler of service
type WebhookContext struct {
	gin        *gin.Context
	body       []byte
	firstParse bool

	requestID string
}

// FirstParse indicates that the request body is not yet readed
func (wc *WebhookContext) FirstParse() bool {
	return wc.firstParse
}

// Param return param from url
func (wc *WebhookContext) Param(s string) string {
	return wc.gin.Param(s)
}

// File serves local file
func (wc *WebhookContext) File(path string) {
	wc.gin.File(path)
}

func (wc *WebhookContext) Writer() http.ResponseWriter {
	return wc.gin.Writer
}

func (wc *WebhookContext) Request() *http.Request {
	return wc.gin.Request
}

func (wc *WebhookContext) Store(key string, b interface{}) {
	wc.gin.Set(key, b)
}

func (wc *WebhookContext) Get(key string) (interface{}, bool) {
	return wc.gin.Get(key)
}

// Headers returns the headers of request
func (wc *WebhookContext) Headers() map[string][]string {
	return map[string][]string(wc.gin.Request.Header)
}

// Header returns the request header with the name
func (wc *WebhookContext) Header(key string) string {
	return wc.gin.Request.Header.Get(key)
}

// Header returns the request header with the name
func (wc *WebhookContext) Response(code int, s string) {
	wc.gin.String(code, s)
}

// Redirect send the Location header
func (wc *WebhookContext) Redirect(code int, s string) {
	wc.gin.Redirect(code, s)
}

// KeyboardAnswer retrieve the data related to pressed button
// buttonText will be returned only in case this button relates to the one in db for this chat
func (c *Context) KeyboardAnswer() (data string, buttonText string) {
	keyboard, err := c.keyboard()

	if err != nil || keyboard.ChatID == 0 {
		log.WithError(err).Error("Can't get stored keyboard")
		return
	}

	// In group chat keyboard answer always include msg_id of original message that generate this keyboard
	if c.Chat.ID < 0 && c.Message.ReplyToMsgID != keyboard.MsgID {
		return
	}

	if c.Message.Text == "" {
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
			Keyboard: m.KeyboardMarkup.db(),
		}
	OUTER:
		if m.Selective && m.ChatID < 0 {
			// For groups save keyboard for all mentioned users to know who exactly can press the button
			usersID := detectTargetUsersID(db, &m.Message)
			if len(usersID) == 0 {
				m.Selective = false
				goto OUTER
			}

			_, err = db.C("users").UpdateAll(bson.M{"_id": bson.M{"$in": usersID}}, bson.M{"$pull": bson.M{"keyboardperchat": bson.M{"chatid": m.ChatID}}})
			_, err = db.C("users").UpdateAll(bson.M{"_id": bson.M{"$in": usersID}}, bson.M{"$push": bson.M{"keyboardperchat": chatKB}})

		} else {
			var info *mgo.ChangeInfo
			if m.ChatID < 0 {
				// If we send keyboard in Telegram's group chat without Selective param we need to erase all other keyboards. Even for other bots, because they will be overridden
				//	info, err = db.C("chats").UpdateAll(bson.M{}, bson.M{"$pull": bson.M{"keyboardperbot": bson.M{"chatid": m.ChatID}}})
				info, err = db.C("users").UpdateAll(bson.M{}, bson.M{"$pull": bson.M{"keyboardperchat": bson.M{"chatid": m.ChatID}}})

				kbAr := []chatKeyboard{chatKB}
				info, err = db.C("chats").UpsertId(m.ChatID, bson.M{"$set": bson.M{"keyboardperbot": kbAr}})
			} else {
				info, err = db.C("chats").UpdateAll(bson.M{"_id": m.ChatID}, bson.M{"$pull": bson.M{"keyboardperbot": bson.M{"botid": m.BotID}}})
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

// Keyboard retrieve keyboard for the current chat if set otherwise empty keyboard is returned
func (c *Context) keyboard() (chatKeyboard, error) {

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

// Log creates the logrus entry and attach corresponding info from the context
func (c *Context) Log() *log.Entry {
	fields := log.Fields{"service": c.ServiceName}

	if Config.Debug {
		pc := make([]uintptr, 10)
		runtime.Callers(2, pc)
		f := runtime.FuncForPC(pc[0])
		fields["file"], fields["line"] = f.FileLine(pc[0])
		fields["func"] = f.Name()
	}

	if c.User.ID > 0 {
		fields["user"] = c.User.ID
	}
	if c.Chat.ID != 0 {
		fields["chat"] = c.Chat.ID
	}
	if c.Message != nil {
		fields["msg"] = c.Message.GetTextHash()
	}

	if c.ChosenInlineResult != nil {
		fields["chosenresult"] = c.ChosenInlineResult
	}

	if c.InlineQuery != nil {
		fields["inlinequery"] = c.InlineQuery
	}

	if c.Callback != nil {
		fields["callback"] = c.Callback.Data
		fields["callback_id"] = c.Callback.ID

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

// Db returns the MongoDB *mgo.Database instance
func (c *Context) Db() *mgo.Database {
	return c.db
}

// Service related to the current context
func (c *Context) Service() *Service {
	s, _ := serviceByName(c.ServiceName)
	return s
}

// Bot related to the service of current request
func (c *Context) Bot() *Bot {
	return c.Service().Bot()
}

// EditPressedMessageText edit the text in the msg where user taped it in case this request is triggered by inlineButton callback
func (c *Context) EditPressedMessageText(text string) error {
	if c.Callback == nil {
		return errors.New("EditPressedMessageText: Callback is not presented")
	}

	return c.EditMessageText(c.Callback.Message, text)
}

// EditPressedMessageTextAndInlineKeyboard edit the text and inline keyboard in the msg where user taped it in case this request is triggered by inlineButton callback
func (c *Context) EditPressedMessageTextAndInlineKeyboard(text string, kb InlineKeyboard) error {
	if c.Callback == nil {
		return errors.New("EditPressedMessageTextAndInlineKeyboard: Callback is not presented")
	}

	return c.EditMessageTextAndInlineKeyboard(c.Callback.Message, c.Callback.Message.InlineKeyboardMarkup.State, text, kb)
}

// EditPressedInlineKeyboard edit the inline keyboard in the msg where user taped it in case this request is triggered by inlineButton callback
func (c *Context) EditPressedInlineKeyboard(kb InlineKeyboard) error {
	if c.Callback == nil {
		return errors.New("EditPressedInlineKeyboard: Callback is not presented")
	}

	return c.EditInlineKeyboard(c.Callback.Message, c.Callback.Message.InlineKeyboardMarkup.State, kb)
}

// EditPressedInlineButton edit the text and state of pressed inline button in case this request is triggered by inlineButton callback
func (c *Context) EditPressedInlineButton(newState int, newText string) error {
	log.WithField("newText", newText).WithField("newState", newState).Info("EditPressedInlineButton")
	if c.Callback == nil {
		return errors.New("EditPressedInlineButton: Callback is not presented")
	}

	return c.EditInlineStateButton(c.Callback.Message, c.Callback.Message.InlineKeyboardMarkup.State, c.Callback.State, c.Callback.Data, newState, newText)
}

// EditMessageText edit the text of message previously sent by the bot
func (c *Context) EditMessageText(om *OutgoingMessage, text string) error {
	if om == nil {
		return errors.New("Empty message provided")
	}

	bot := c.Bot()
	if om.ParseMode == "HTML" {
		textCleared, err := sanitize.HTMLAllowing(text, []string{"a", "b", "strong", "i", "em", "a", "code", "pre"}, []string{"href"})

		if err == nil && textCleared != "" {
			text = textCleared
		}
	}
	om.Text = text
	prevTextHash := om.TextHash
	om.TextHash = om.GetTextHash()

	if om.TextHash == prevTextHash {
		c.Log().Debugf("EditMessageText – message (_id=%s botid=%v id=%v) not updated text have not changed", om.ID.Hex(), bot.ID, om.MsgID)
		return nil
	}

	_, err := bot.API.Send(tg.EditMessageTextConfig{
		BaseEdit: tg.BaseEdit{
			ChatID:      om.ChatID,
			MessageID:   om.MsgID,
			ReplyMarkup: &tg.InlineKeyboardMarkup{InlineKeyboard: om.InlineKeyboardMarkup.tg()},
		},
		ParseMode:             om.ParseMode,
		DisableWebPagePreview: !om.WebPreview,
		Text:                  text,
	})
	if err != nil {
		if err.(tg.Error).IsCantAccessChat() || err.(tg.Error).ChatMigrated() {
			if c.Callback != nil && c.Callback.AnsweredAt == nil {
				c.AnswerCallbackQuery("Sorry, message can be outdated. Bot can't edit messages created before converting to the Super Group", false)
			}
		} else if err.(tg.Error).IsAntiFlood() {
			c.Log().WithError(err).Warn("TG Anti flood activated")
		}
	} else {
		err = c.db.C("messages").UpdateId(om.ID, bson.M{"$set": bson.M{"texthash": om.TextHash}})
	}
	return err
}

// EditMessagesTextWithEventID edit the last MaxMsgsToUpdateWithEventID messages' text with the corresponding eventID  in ALL chats
func (c *Context) EditMessagesTextWithEventID(eventID string, text string) (edited int, err error) {
	var messages []OutgoingMessage
	//update MAX_MSGS_TO_UPDATE_WITH_EVENTID last bot messages
	c.db.C("messages").Find(bson.M{"botid": c.Bot().ID, "eventid": eventID}).Sort("-_id").Limit(MaxMsgsToUpdateWithEventID).All(&messages)
	for _, message := range messages {
		err = c.EditMessageText(&message, text)
		if err != nil {
			c.Log().WithError(err).WithField("eventid", eventID).Error("EditMessagesTextWithEventID")
		} else {
			edited++
		}
	}
	return edited, err
}

// EditMessagesTextWithMessageID edit the one message text with by message BSON ID
func (c *Context) EditMessageTextWithMessageID(msgID bson.ObjectId, text string) (edited int, err error) {
	var message OutgoingMessage

	c.db.C("messages").Find(bson.M{"_id": msgID, "botid": c.Bot().ID}).One(&message)
	err = c.EditMessageText(&message, text)
	if err != nil {
		c.Log().WithError(err).WithField("msgid", msgID).Error("EditMessageTextWithMessageID")
	} else {
		edited++
	}

	return edited, err
}

// EditMessagesWithEventID edit the last MaxMsgsToUpdateWithEventID messages' text and inline keyboard with the corresponding eventID in ALL chats
func (c *Context) EditMessagesWithEventID(eventID string, fromState string, text string, kb InlineKeyboard) (edited int, err error) {
	var messages []OutgoingMessage
	f := bson.M{"botid": c.Bot().ID, "eventid": eventID}
	if fromState != "" {
		f["inlinekeyboardmarkup.state"] = fromState
	}

	//update MAX_MSGS_TO_UPDATE_WITH_EVENTID last bot messages
	c.db.C("messages").Find(f).Sort("-_id").Limit(MaxMsgsToUpdateWithEventID).All(&messages)
	for _, message := range messages {
		err = c.EditMessageTextAndInlineKeyboard(&message, fromState, text, kb)
		if err != nil {
			c.Log().WithError(err).WithField("eventid", eventID).Error("EditMessagesWithEventID")
		} else {
			edited++
		}
	}
	return edited, err
}

// DeleteMessagesWithEventID deletes the last MaxMsgsToUpdateWithEventID messages' text and inline keyboard with the corresponding eventID in ALL chats
func (c *Context) DeleteMessagesWithEventID(eventID string) (deleted int, err error) {
	var messages []OutgoingMessage
	f := bson.M{"botid": c.Bot().ID, "eventid": eventID}

	//update MAX_MSGS_TO_UPDATE_WITH_EVENTID last bot messages
	c.db.C("messages").Find(f).Sort("-_id").Limit(MaxMsgsToUpdateWithEventID).All(&messages)
	for _, message := range messages {
		err = c.DeleteMessage(&message)
		if err != nil {
			c.Log().WithError(err).WithField("eventid", eventID).Error("DeleteMessagesWithEventID")
		} else {
			deleted++
		}
	}
	return deleted, err
}

// DeleteMessage deletes the outgoing message's text and inline keyboard
func (c *Context) DeleteMessage(om *OutgoingMessage) error {
	bot := c.Bot()
	if om.MsgID != 0 {
		log.WithField("msgID", om.MsgID).Debug("DeleteMessage")
	} else {
		om.ChatID = 0
		log.WithField("inlineMsgID", om.InlineMsgID).Debug("DeleteMessage")
	}

	var msg OutgoingMessage
	var ci *mgo.ChangeInfo
	var err error

	ci, err = c.db.C("messages").Find(bson.M{"_id": om.ID}).Apply(mgo.Change{Remove: true}, &msg)

	if err != nil {
		c.Log().WithError(err).Error("DeleteMessage messages remove error")
	}

	if msg.BotID == 0 {
		c.Log().Warn(fmt.Sprintf("DeleteMessage – message (_id=%s botid=%v id=%v) not found", om.ID, bot.ID, om.MsgID))
		return nil

	}
	if ci.Removed == 0 {
		c.Log().Warn(fmt.Sprintf("DeleteMessage – message (_id=%s botid=%v id=%v) not removed ", om.ID, bot.ID, om.MsgID))

		return nil
	}

	_, err = bot.API.Send(tg.DeleteMessageConfig{
		ChatID:    om.ChatID,
		MessageID: om.MsgID,
	})

	if err != nil {
		if err.(tg.Error).IsCantAccessChat() || err.(tg.Error).ChatMigrated() {
			if c.Callback != nil {
				c.AnswerCallbackQuery("Message can be outdated. Bot can't edit messages created before converting to the Super Group", false)
			}
		} else if err.(tg.Error).IsAntiFlood() {
			c.Log().WithError(err).Warn("TG Anti flood activated")
		}
		// Oops. error is occurred – revert the original message
		c.db.C("messages").Insert(om)
		return err
	}

	return nil
}

// EditMessageTextAndInlineKeyboard edit the outgoing message's text and inline keyboard
func (c *Context) EditMessageTextAndInlineKeyboard(om *OutgoingMessage, fromState string, text string, kb InlineKeyboard) error {
	bot := c.Bot()
	if om.MsgID != 0 {
		log.WithField("msgID", om.MsgID).Debug("EditMessageTextAndInlineKeyboard")
	} else {
		om.ChatID = 0
		log.WithField("inlineMsgID", om.InlineMsgID).Debug("EditMessageTextAndInlineKeyboard")
	}

	var msg OutgoingMessage

	var err error

	if om.ParseMode == "HTML" {
		textCleared, err := sanitize.HTMLAllowing(text, []string{"a", "b", "strong", "i", "em", "a", "code", "pre"}, []string{"href"})

		if err == nil && textCleared != "" {
			text = textCleared
		}
	}

	om.Text = text
	prevTextHash := om.TextHash
	om.TextHash = om.GetTextHash()

	if fromState != "" {
		_, err = c.db.C("messages").Find(bson.M{"_id": om.ID, "$or": []bson.M{{"inlinekeyboardmarkup.state": fromState}, {"inlinekeyboardmarkup": bson.M{"$exists": false}}}}).Apply(mgo.Change{Update: bson.M{"$set": bson.M{"inlinekeyboardmarkup": kb, "texthash": om.TextHash}}}, &msg)
	} else {
		_, err = c.db.C("messages").Find(bson.M{"_id": om.ID}).Apply(mgo.Change{Update: bson.M{"$set": bson.M{"inlinekeyboardmarkup": kb, "texthash": om.TextHash}}}, &msg)
	}

	if err != nil {
		c.Log().WithError(err).Error("EditMessageTextAndInlineKeyboard messages update error")
	}

	if msg.BotID == 0 {
		c.Log().Warn(fmt.Sprintf("EditMessageTextAndInlineKeyboard – message (_id=%s botid=%v id=%v state %s) not found", om.ID.Hex(), bot.ID, om.MsgID, fromState))
		return nil

	}

	tgKeyboard := kb.tg()

	if prevTextHash == om.TextHash {
		prevTGKeyboard := om.InlineKeyboardMarkup.tg()
		if whetherTGInlineKeyboardsAreEqual(prevTGKeyboard, tgKeyboard) {
			c.Log().Debugf("EditMessageTextAndInlineKeyboard – message (_id=%s botid=%v id=%v state %s) not updated both text and kb have not changed", om.ID.Hex(), bot.ID, om.MsgID, fromState)
			return nil
		}
	}

	_, err = bot.API.Send(tg.EditMessageTextConfig{
		BaseEdit: tg.BaseEdit{
			ChatID:          om.ChatID,
			InlineMessageID: om.InlineMsgID,
			MessageID:       om.MsgID,
			ReplyMarkup:     &tg.InlineKeyboardMarkup{InlineKeyboard: tgKeyboard},
		},
		ParseMode:             om.ParseMode,
		Text:                  text,
		DisableWebPagePreview: !om.WebPreview,
	})

	if err != nil {
		if err.(tg.Error).IsCantAccessChat() || err.(tg.Error).ChatMigrated() {
			if c.Callback != nil {
				c.AnswerCallbackQuery("Message can be outdated. Bot can't edit messages created before converting to the Super Group", false)
			}
		} else if err.(tg.Error).IsAntiFlood() {
			c.Log().WithError(err).Warn("TG Anti flood activated")
		}
		// Oops. error is occurred – revert the original keyboard
		c.db.C("messages").Update(bson.M{"_id": msg.ID}, bson.M{"$set": bson.M{"texthash": prevTextHash, "inlinekeyboardmarkup": msg.InlineKeyboardMarkup}})
		return err
	}

	return nil
}

// EditInlineKeyboard edit the outgoing message's inline keyboard
func (c *Context) EditInlineKeyboard(om *OutgoingMessage, fromState string, kb InlineKeyboard) error {

	bot := c.Bot()
	if om.MsgID != 0 {
		log.WithField("msgID", om.MsgID).Debug("EditMessageTextAndInlineKeyboard")
	} else {
		om.ChatID = 0
		log.WithField("inlineMsgID", om.InlineMsgID).Debug("EditMessageTextAndInlineKeyboard")
	}
	var msg OutgoingMessage

	_, err := c.db.C("messages").Find(bson.M{"_id": om.ID, "$or": []bson.M{{"inlinekeyboardmarkup.state": fromState}, {"inlinekeyboardmarkup": bson.M{"$exists": false}}}}).Apply(mgo.Change{Update: bson.M{"$set": bson.M{"inlinekeyboardmarkup": kb}}}, &msg)

	if msg.BotID == 0 {
		return fmt.Errorf("EditInlineKeyboard – message (botid=%v id=%v state %s) not found", bot.ID, om.MsgID, fromState)
	}

	tgKeyboard := kb.tg()
	prevTGKeyboard := om.InlineKeyboardMarkup.tg()
	if whetherTGInlineKeyboardsAreEqual(prevTGKeyboard, tgKeyboard) {
		c.Log().Debugf("EditMessageTextAndInlineKeyboard – message (_id=%s botid=%v id=%v state %s) not updated both text and kb have not changed", om.ID, bot.ID, om.MsgID, fromState)
		return nil
	}

	_, err = bot.API.Send(tg.EditMessageReplyMarkupConfig{
		BaseEdit: tg.BaseEdit{
			ChatID:          om.ChatID,
			MessageID:       om.MsgID,
			InlineMessageID: om.InlineMsgID,
			ReplyMarkup:     &tg.InlineKeyboardMarkup{InlineKeyboard: tgKeyboard},
		},
	})

	if err != nil {
		if err.(tg.Error).IsCantAccessChat() || err.(tg.Error).ChatMigrated() {
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

// EditInlineButton edit the outgoing message's inline button
func (c *Context) EditInlineButton(om *OutgoingMessage, kbState string, buttonData string, newButtonText string) error {
	return c.EditInlineStateButton(om, kbState, 0, buttonData, 0, newButtonText)

}

// EditInlineStateButton edit the outgoing message's inline button with a state
func (c *Context) EditInlineStateButton(om *OutgoingMessage, kbState string, oldButtonState int, buttonData string, newButtonState int, newButtonText string) error {
	if oldButtonState > 9 || oldButtonState < 0 {
		c.Log().WithField("data", buttonData).WithField("text", newButtonText).Errorf("EditInlineStateButton – oldButtonState must be [0-9], %d recived", oldButtonState)
	}

	if newButtonState > 9 || newButtonState < 0 {
		c.Log().WithField("data", buttonData).WithField("text", newButtonText).Errorf("EditInlineStateButton – newButtonState must be [0-9], %d recived", newButtonState)
	}

	bot := c.Bot()

	var msg OutgoingMessage
	c.db.C("messages").Find(bson.M{"_id": om.ID, "inlinekeyboardmarkup.state": kbState}).One(&msg)
	// need a more thread safe solution to switch stored keyboard
	if msg.BotID == 0 {
		return fmt.Errorf("EditInlineButton – message (botid=%v id=%v(%v) state %s) not found", bot.ID, om.MsgID, om.InlineMsgID, kbState)
	}
	i, j, _ := msg.InlineKeyboardMarkup.Find(buttonData)

	if i < 0 {
		return fmt.Errorf("EditInlineButton – button %v not found in message (botid=%v id=%v(%v) state %s) not found", buttonData, bot.ID, om.MsgID, om.InlineMsgID, kbState)
	}

	//first of all – change stored keyboard to avoid simultaneously changing requests
	set := bson.M{fmt.Sprintf("inlinekeyboardmarkup.buttons.%d.%d.text", i, j): newButtonText}

	if newButtonState != oldButtonState {
		set = bson.M{fmt.Sprintf("inlinekeyboardmarkup.buttons.%d.%d.text", i, j): newButtonText, fmt.Sprintf("inlinekeyboardmarkup.buttons.%d.%d.state", i, j): newButtonState}
	}

	info, err := c.db.C("messages").UpdateAll(bson.M{"_id": msg.ID, "inlinekeyboardmarkup.state": kbState, fmt.Sprintf("inlinekeyboardmarkup.buttons.%d.%d.data", i, j): buttonData}, bson.M{"$set": set})

	if info.Updated == 0 {
		// another one thread safe check
		return fmt.Errorf("EditInlineButton – button[%d][%d] %v not found in message (botid=%v id=%v(%v) state %s) not found", i, j, buttonData, bot.ID, om.MsgID, om.InlineMsgID, kbState)
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
			ReplyMarkup:     &tg.InlineKeyboardMarkup{InlineKeyboard: kb.tg()},
		},
	})
	if err != nil {
		// Oops. error is occurred – revert the original keyboard
		err := c.db.C("messages").UpdateId(msg.ID, bson.M{"$set": bson.M{"inlinekeyboardmarkup": msg.InlineKeyboardMarkup}})
		return err
	}

	return nil
}

// AnswerInlineQueryWithResults answer the inline query that triggered this request
func (c *Context) AnswerInlineQueryWithResults(res []interface{}, cacheTime int, isPersonal bool, nextOffset string) error {
	bot := c.Bot()
	_, err := bot.API.AnswerInlineQuery(tg.InlineConfig{IsPersonal: isPersonal, CacheTime: cacheTime, InlineQueryID: c.InlineQuery.ID, Results: res, NextOffset: nextOffset})
	n := time.Now()
	c.inlineQueryAnsweredAt = &n
	return err
}

// AnswerInlineQueryWithResults answer the inline query that triggered this request
func (c *Context) AnswerInlineQueryWithResultsAndPM(res []interface{}, cacheTime int, isPersonal bool, nextOffset string, PMText string, PMParameter string) error {
	bot := c.Bot()
	_, err := bot.API.AnswerInlineQuery(tg.InlineConfig{IsPersonal: true, InlineQueryID: c.InlineQuery.ID, Results: res, NextOffset: nextOffset, SwitchPMText: PMText, SwitchPMParameter: PMParameter})
	n := time.Now()
	c.inlineQueryAnsweredAt = &n
	return err
}

// AnswerInlineQueryWithPM answer the inline query that triggered this request with Private Message redirect tip
func (c *Context) AnswerInlineQueryWithPM(text string, parameter string) error {
	bot := c.Bot()
	_, err := bot.API.AnswerInlineQuery(tg.InlineConfig{IsPersonal: true, InlineQueryID: c.InlineQuery.ID, SwitchPMText: text, SwitchPMParameter: parameter})
	n := time.Now()
	c.inlineQueryAnsweredAt = &n
	return err
}

func (c *Context) AnswerCallbackQueryWithURL(url string) error {
	bot := c.Bot()
	_, err := bot.API.AnswerCallbackQuery(tg.CallbackConfig{CallbackQueryID: c.Callback.ID, URL: url})
	return err
}

// AnswerCallbackQuery answer the inline keyboard callback query that triggered this request with toast or alert
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

// NewMessage creates the message targeted to the current chat
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
	msg.ctx = c
	return msg
}

// SendAction send the one of "typing", "upload_photo", "record_video", "upload_video", "record_audio", "upload_audio", "upload_document", "find_location"
func (c *Context) SendAction(s string) error {
	_, err := c.Bot().API.Send(tg.NewChatAction(c.Chat.ID, s))
	return err
}

// DownloadURL downloads the remote URL and returns the local file path
func (c *Context) DownloadURL(url string) (filePath string, err error) {

	ext := filepath.Ext(url)
	out, err := ioutil.TempFile("", fmt.Sprintf("%d_%d", c.Bot().ID, c.Chat.ID))

	if err != nil {
		return "", err
	}

	out.Close()
	os.Rename(out.Name(), out.Name()+ext)

	out, err = os.OpenFile(out.Name()+ext, os.O_RDWR, 0666)
	if err != nil {
		return "", err
	}
	defer out.Close()

	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", errors.New("non 2xx resp status")
	}
	defer resp.Body.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return "", err
	}

	return out.Name(), nil
}

// RAW returns request's body
func (wc *WebhookContext) RAW() (*[]byte, error) {
	var err error
	if wc.body == nil {
		wc.firstParse = true
		wc.body, err = ioutil.ReadAll(wc.gin.Request.Body)
		if err != nil {
			return nil, err
		}
	}
	return &wc.body, nil
}

// JSON decodes the JSON in the request's body to the out interface
func (wc *WebhookContext) JSON(out interface{}) error {
	var err error
	if wc.body == nil {
		wc.firstParse = true
		wc.body, err = ioutil.ReadAll(wc.gin.Request.Body)

		if err != nil {
			return err
		}
	}
	err = json.Unmarshal(wc.body, out)

	if err != nil && strings.HasPrefix(string(wc.body), "payload=") {
		s := string(wc.body)
		s, err = uurl.QueryUnescape(s[8:])
		if err != nil {
			return err
		}
		err = json.Unmarshal([]byte(s), out)

	}
	return err
}

// Form decodes the POST form in the request's body to the out interface
func (wc *WebhookContext) Form() uurl.Values {
	//todo: bug, RAW() unavailable after ParseForm()
	wc.gin.Request.ParseForm()
	return wc.gin.Request.PostForm
}

// FormValue return form data with specific key
func (wc *WebhookContext) FormValue(key string) string {
	err := wc.gin.Request.ParseForm()
	if err != nil {
		log.Error(err)
	}
	return wc.gin.Request.PostForm.Get(key)
}

// FormValue return form data with specific key
func (wc *WebhookContext) QueryValue(key string) string {
	err := wc.gin.Request.ParseForm()
	if err != nil {
		log.Error(err)
	}

	return wc.gin.Request.Form.Get(key)
}

// HookID returns the HookID from the URL
func (wc *WebhookContext) HookID() string {
	return wc.gin.Param("param")
}

// RequestID returns the unique generated request ID
func (wc *WebhookContext) RequestID() string {
	return wc.requestID
}
