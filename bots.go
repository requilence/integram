package integram

import (
	"encoding/gob"
	"errors"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/kennygrant/sanitize"
	"github.com/requilence/jobs"
	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	tg "gopkg.in/telegram-bot-api.v3"
)

const inlineButtonStateKeyword = '`'
const antiFloodTimeout = 60

var botPerID = make(map[int64]*Bot)
var botPerService = make(map[string]*Bot)

var botTokenRE = regexp.MustCompile("([0-9]*):([0-9a-zA-Z_-]*)")

// Bot represents parsed auth data & API reference
type Bot struct {
	// Bot Telegram user id
	ID int64

	// Bot Telegram username
	Username string

	// Bot Telegram token
	token string

	// Slice of services that using this bot (len=1 means that bot is dedicated for service – recommended case)
	services []*Service

	// Used to store long-pulling updates channel and survive panics
	updatesChan <-chan tg.Update
	API         *tg.BotAPI
}

// Message represent both outgoing and incoming message data
type Message struct {
	ID               bson.ObjectId `bson:"_id,omitempty"` // Internal unique BSON ID
	EventID          []string      `bson:",omitempty"`
	MsgID            int           `bson:",omitempty"`         // Telegram Message ID. BotID+MsgID is unique
	InlineMsgID      string        `bson:",omitempty"`         // Telegram InlineMessage ID. ChatID+InlineMsgID is unique
	BotID            int64         `bson:",minsize"`           // TG bot's ID on which behalf message is sending or receiving
	FromID           int64         `bson:",minsize"`           // TG User's ID of sender. Equal to BotID in case of outgoinf message from bot
	ChatID           int64         `bson:",omitempty,minsize"` // Telegram chat's ID, equal to FromID in case of private message
	BackupChatID     int64         `bson:",omitempty,minsize"` // This chat will be used if chatid failed (bot not started or stopped or group deactivated)
	ReplyToMsgID     int           `bson:",omitempty"`         // If this message is reply, contains Telegram's Message ID of original message
	Date             time.Time
	Text             string           `bson:",omitempty"`
	AntiFlood        bool             `bson:",omitempty"`
	Deleted          bool             `bson:",omitempty"` // f.e. admin can delete the message in supergroup and we can't longer edit or reply on it
	OnCallbackAction string           `bson:",omitempty"` // Func to call on inline button press
	OnCallbackData   []byte           `bson:",omitempty"` // Args to send to this func
	OnReplyAction    string           `bson:",omitempty"` // Func to call on message reply
	OnReplyData      []byte           `bson:",omitempty"` // Args to send to this func
	om               *OutgoingMessage // Cache when retreiving original replied message
}

// IncomingMessage specifies data that available for incoming message
type IncomingMessage struct {
	Message               `bson:",inline"`
	From                  User
	Chat                  Chat
	ForwardFrom           *User
	ForwardDate           time.Time
	ReplyToMessage        *Message            `bson:"-"`
	ForwardFromChat       *Chat               `json:"forward_from_chat"`       // optional
	EditDate              int                 `json:"edit_date"`               // optional
	Entities              *[]tg.MessageEntity `json:"entities"`                // optional
	Audio                 *tg.Audio           `json:"audio"`                   // optional
	Document              *tg.Document        `json:"document"`                // optional
	Photo                 *[]tg.PhotoSize     `json:"photo"`                   // optional
	Sticker               *tg.Sticker         `json:"sticker"`                 // optional
	Video                 *tg.Video           `json:"video"`                   // optional
	Voice                 *tg.Voice           `json:"voice"`                   // optional
	Caption               string              `json:"caption"`                 // optional
	Contact               *tg.Contact         `json:"contact"`                 // optional
	Location              *tg.Location        `json:"location"`                // optional
	Venue                 *tg.Venue           `json:"venue"`                   // optional
	NewChatMember         *User               `json:"new_chat_member"`         // optional
	LeftChatMember        *User               `json:"left_chat_member"`        // optional
	NewChatTitle          string              `json:"new_chat_title"`          // optional
	NewChatPhoto          *[]tg.PhotoSize     `json:"new_chat_photo"`          // optional
	DeleteChatPhoto       bool                `json:"delete_chat_photo"`       // optional
	GroupChatCreated      bool                `json:"group_chat_created"`      // optional
	SuperGroupChatCreated bool                `json:"supergroup_chat_created"` // optional
	ChannelChatCreated    bool                `json:"channel_chat_created"`    // optional
	MigrateToChatID       int64               `json:"migrate_to_chat_id"`      // optional
	MigrateFromChatID     int64               `json:"migrate_from_chat_id"`    // optional
	PinnedMessage         *Message            `json:"pinned_message"`          // optional

	// Need to update message in DB. Used f.e. when you set the eventID for outgoing message
	needToUpdateDB bool
}

// OutgoingMessage specispecifiesfy data of performing or performed outgoing message
type OutgoingMessage struct {
	Message              `bson:",inline"`
	KeyboardHide         bool           `bson:",omitempty"`
	ResizeKeyboard       bool           `bson:",omitempty"`
	KeyboardMarkup       Keyboard       `bson:"-"`
	InlineKeyboardMarkup InlineKeyboard `bson:",omitempty"`
	Keyboard             bool           `bson:",omitempty"`
	ParseMode            string         `bson:",omitempty"`
	OneTimeKeyboard      bool           `bson:",omitempty"`
	Selective            bool           `bson:",omitempty"`
	ForceReply           bool           `bson:",omitempty"`
	WebPreview           bool           `bson:",omitempty"`
	Silent               bool           `bson:",omitempty"`
	FilePath             string         `bson:",omitempty"`
	FileName             string         `bson:",omitempty"`
	FileType             string         `bson:",omitempty"`
	processed            bool
}

// Keyboard is a Shorthand for [][]Button
type Keyboard []Buttons

// Buttons is a Shorthand for []Button
type Buttons []Button

// InlineKeyboard contains the data to create the Inline keyboard for Telegram and store it in DB
type InlineKeyboard struct {
	Buttons    []InlineButtons // You must specify at least 1 InlineButton in slice
	FixedWidth bool            `bson:",omitempty"` // will add right padding to match all buttons text width
	State      string          // determine the current keyboard's state. Useful to change the behavior for branch cases and make it little thread safe while it is using by several users
	MaxRows    int             `bson:",omitempty"` // Will automatically add next/prev buttons. Zero means no limit
	RowOffset  int             `bson:",omitempty"` // Current offset when using MaxRows
}

// InlineButtons is a Shorthand for []InlineButton
type InlineButtons []InlineButton

// Button contains the data to create Keyboard
type Button struct {
	Data string // data is stored in the DB. May be collisions if button text is not unique per keyboard
	Text string // should be unique per keyboard
}

// InlineButton contains the data to create InlineKeyboard
// One of URL, Data, SwitchInlineQuery must be specified
// If more than one specified the first in order of (URL, Data, SwitchInlineQuery) will be used
type InlineButton struct {
	Text              string
	State             int
	URL               string `bson:",omitempty"`
	Data              string `bson:",omitempty"`          // maximum 64 bytes
	SwitchInlineQuery string `bson:",omitempty"`          //
	OutOfPagination   bool   `bson:",omitempty" json:"-"` // Only for the single button in first or last row. Use together with InlineKeyboard.MaxRows – for buttons outside of pagination list
}

// InlineKeyboardMarkup allow to generate TG and DB data from different states - (InlineButtons, []InlineButtons and InlineKeyboard)
type InlineKeyboardMarkup interface {
	tg() [][]tg.InlineKeyboardButton
	Keyboard() InlineKeyboard
}

// KeyboardMarkup allow to generate TG and DB data from different states - (Buttons and Keyboard)
type KeyboardMarkup interface {
	tg() [][]tg.KeyboardButton
	Keyboard() Keyboard
	db() map[string]string
}

func (c *Bot) tgToken() string {
	return fmt.Sprintf("%d:%s", c.ID, c.token)
}

// PMURL return URL to private messaging with the bot like https://telegram.me/trello_bot?start=param
func (c *Bot) PMURL(param string) string {
	if param == "" {
		return fmt.Sprintf("https://telegram.me/%v", c.Username)
	}

	return fmt.Sprintf("https://telegram.me/%v?start=%v", c.Username, param)
}

func (c *Bot) webhookURL() *url.URL {
	url, _ := url.Parse(fmt.Sprintf("%s/tg/%d/%s", BaseURL, c.ID, compactHash(c.token)))
	return url
}

func (service *Service) registerBot(fullTokenWithID string) error {

	s := botTokenRE.FindStringSubmatch(fullTokenWithID)

	if len(s) < 3 {
		return errors.New("can't parse token")
	}
	id, err := strconv.ParseInt(s[1], 10, 64)
	if err != nil {
		return err
	}
	if _, exists := botPerID[id]; !exists {
		bot := Bot{ID: id, token: s[2], services: []*Service{service}}
		botPerID[id] = &bot

		token := bot.tgToken()
		bot.API, err = tg.NewBotAPI(token)

		if err != nil {
			log.WithError(err).WithField("token", token).Error("NewBotAPI returned error")
			return err
		}

		me, err := bot.API.GetMe()

		if err != nil {
			log.WithError(err).WithField("token", token).Error("GetMe returned error")
			return err
		}

		bot.Username = me.UserName

	} else {
		b := botPerID[id]
		b.services = append(b.services, service)
		botPerID[id] = b
	}
	botPerService[service.Name] = botPerID[id]
	return nil
}

// Find the InlineButton in Keyboard by the Data
func (keyboard *InlineKeyboard) Find(buttonData string) (i, j int, but *InlineButton) {
	for i, buttonsRow := range keyboard.Buttons {
		for j, button := range buttonsRow {
			if button.Data == buttonData {
				return i, j, &button
			}
		}
	}
	return -1, -1, nil
}

// EditText find the InlineButton in Keyboard by the Data and change the text of that button
func (keyboard *InlineKeyboard) EditText(buttonData string, newText string) {
	for i, buttonsRow := range keyboard.Buttons {
		for j, button := range buttonsRow {
			if button.Data == buttonData {
				keyboard.Buttons[i][j].Text = newText
				return
			}
		}
	}
}

// AddPMSwitchButton add the button to switch to PM as a first row in the InlineKeyboard
func (keyboard *InlineKeyboard) AddPMSwitchButton(b *Bot, text string, param string) {
	if len(keyboard.Buttons) > 0 && len(keyboard.Buttons[0]) > 0 && keyboard.Buttons[0][0].Text == text {
		return
	}
	keyboard.PrependRows(InlineButtons{InlineButton{Text: text, URL: b.PMURL(param)}})
}

// AppendRows adds 1 or more InlineButtons (rows) to the end of InlineKeyboard
func (keyboard *InlineKeyboard) AppendRows(buttons ...InlineButtons) {
	keyboard.Buttons = append(keyboard.Buttons, buttons...)
}

// PrependRows adds 1 or more InlineButtons (rows) to the begin of InlineKeyboard
func (keyboard *InlineKeyboard) PrependRows(buttons ...InlineButtons) {
	keyboard.Buttons = append(buttons, keyboard.Buttons...)
}

// Append adds 1 or more InlineButton (column) to the end of InlineButtons(row)
func (buttons *InlineButtons) Append(data string, text string) {
	if len(data) > 64 {
		log.WithField("text", text).Errorf("InlineButton data '%s' extends 64 bytes limit", data)
	}
	*buttons = append(*buttons, InlineButton{Data: data, Text: text})
}

// Prepend adds 1 or more InlineButton (column) to the begin of InlineButtons(row)
func (buttons *InlineButtons) Prepend(data string, text string) {
	if len(data) > 64 {
		log.WithField("text", text).Errorf("InlineButton data '%s' extends 64 bytes limit", data)
	}
	*buttons = append([]InlineButton{{Data: data, Text: text}}, *buttons...)
}

// AppendWithState add the InlineButton with state to the end of InlineButtons(row)
// Useful for checkbox or to revert the action
func (buttons *InlineButtons) AppendWithState(state int, data string, text string) {
	if len(data) > 64 {
		log.WithField("text", text).Errorf("InlineButton data '%s' extends 64 bytes limit", data)
	}
	if state > 9 || state < 0 {
		log.WithField("data", data).WithField("text", text).Errorf("AppendWithState – state must be [0-9], %d received", state)
	}
	*buttons = append(*buttons, InlineButton{Data: data, Text: text, State: state})
}

// PrependWithState add the InlineButton with state to the begin of InlineButtons(row)
// Useful for checkbox or to revert the action
func (buttons *InlineButtons) PrependWithState(state int, data string, text string) {
	if len(data) > 64 {
		log.WithField("text", text).Errorf("InlineButton data '%s' extends 64 bytes limit", data)
	}
	if state > 9 || state < 0 {
		log.WithField("data", data).WithField("text", text).Errorf("PrependWithState – state must be [0-9], %d received", state)
	}
	*buttons = append([]InlineButton{{Data: data, Text: text, State: state}}, *buttons...)
}

// AddURL adds InlineButton with URL to the end of InlineButtons(row)
func (buttons *InlineButtons) AddURL(url string, text string) {
	*buttons = append(*buttons, InlineButton{URL: url, Text: text})
}

// Markup generate InlineKeyboard from InlineButtons ([]Button), chunking buttons by columns number, and specifying current keyboard state
// Keyboard state useful for nested levels to determine current position
func (buttons *InlineButtons) Markup(columns int, state string) InlineKeyboard {
	keyboard := InlineKeyboard{}

	col := 0

	row := InlineButtons{}
	len := len(*buttons)
	for i, button := range *buttons {
		row = append(row, button)

		col++
		if col == columns || i == (len-1) {
			col = 0
			keyboard.AppendRows(row)
			row = InlineButtons{}
		}
	}
	keyboard.State = state
	return keyboard
}

// Keyboard generates inline keyboard from inline keyboard  :-D
func (keyboard InlineKeyboard) Keyboard() InlineKeyboard {
	return keyboard
}

// Keyboard generates inline keyboard with 1 button
func (button InlineButton) Keyboard() InlineKeyboard {
	return InlineKeyboard{Buttons: []InlineButtons{{button}}}
}

// Keyboard generates inline keyboard with 1 column
func (buttons InlineButtons) Keyboard() InlineKeyboard {
	return buttons.Markup(1, "")
}

func (button InlineButton) tg() [][]tg.InlineKeyboardButton {
	return button.Keyboard().tg()
}

func (buttons InlineButtons) tg() [][]tg.InlineKeyboardButton {
	return buttons.Keyboard().tg()
}

func (keyboard InlineKeyboard) tg() [][]tg.InlineKeyboardButton {
	res := make([][]tg.InlineKeyboardButton, len(keyboard.Buttons))

	maxWidth := 0
	if keyboard.FixedWidth {

		for _, columns := range keyboard.Buttons {
			for _, button := range columns {
				if len(button.Text) > maxWidth {
					maxWidth = len(button.Text)
				}
			}
		}
	}
	for r, columns := range keyboard.Buttons {
		res[r] = make([]tg.InlineKeyboardButton, len(keyboard.Buttons[r]))
		c := 0
		for _, button := range columns {
			if keyboard.FixedWidth {
				button.Text = button.Text + strings.Repeat(" ", maxWidth-len(button.Text))
			}

			if button.State != 0 {
				button.Data = fmt.Sprintf("%c%d%s", inlineButtonStateKeyword, button.State, button.Data)
			}

			if button.URL != "" {
				res[r][c] = tg.InlineKeyboardButton{Text: button.Text, URL: button.URL}
			} else if button.Data != "" {
				res[r][c] = tg.InlineKeyboardButton{Text: button.Text, CallbackData: button.Data}
			} else {
				res[r][c] = tg.InlineKeyboardButton{Text: button.Text, SwitchInlineQuery: &button.SwitchInlineQuery}
			}
			c++
		}
	}
	return res
}

// AddRows adds 1 or more Buttons (rows) to the end of InlineKeyboard
func (keyboard *Keyboard) AddRows(buttons ...Buttons) {
	*keyboard = append(*keyboard, buttons...)
}

// Prepend adds InlineButton with URL to the begin of InlineButtons(row)
func (buttons *Buttons) Prepend(data string, text string) {
	*buttons = append([]Button{{Data: data, Text: text}}, *buttons...)
}

// Append adds Button with URL to the end of Buttons(row)
func (buttons *Buttons) Append(data string, text string) {
	*buttons = append(*buttons, Button{Data: data, Text: text})
}

// InlineButtons converts Buttons to InlineButtons
// useful with universal methods that create keyboard (f.e. settigns) for both usual and inline keyboard
func (buttons *Buttons) InlineButtons() InlineButtons {
	row := InlineButtons{}

	for _, button := range *buttons {
		row.Append(button.Data, button.Text)

	}
	return row
}

// Markup generate Keyboard from Buttons ([]Button), chunking buttons by columns number
func (buttons *Buttons) Markup(columns int) Keyboard {
	keyboard := Keyboard{}

	col := 0

	row := Buttons{}
	len := len(*buttons)
	for i, button := range *buttons {
		row.Append(button.Data, button.Text)
		col++
		if col == columns || i == (len-1) {
			col = 0
			keyboard.AddRows(row)
			row = Buttons{}
		}
	}

	return keyboard
}

// Keyboard is generating Keyboard with 1 column
func (buttons Buttons) Keyboard() Keyboard {
	return buttons.Markup(1)
}

func (buttons Buttons) tg() [][]tg.KeyboardButton {
	return buttons.Keyboard().tg()
}

func (buttons Buttons) db() map[string]string {
	res := make(map[string]string)
	for _, button := range buttons {
		res[checksumString(button.Text)] = button.Data
	}
	return res
}

// Keyboard generates keyboard from 1 button
func (button Button) Keyboard() Keyboard {
	btns := Buttons{button}
	return btns.Keyboard()
}

func (button Button) tg() [][]tg.KeyboardButton {
	btns := Buttons{button}
	return btns.Keyboard().tg()
}

func (button Button) db() map[string]string {
	res := make(map[string]string)
	res[checksumString(button.Text)] = button.Data

	return res
}

func (keyboard Keyboard) db() map[string]string {
	res := make(map[string]string)
	for _, columns := range keyboard {
		for _, button := range columns {
			res[checksumString(button.Text)] = button.Data
		}
	}
	return res
}

// Keyboard generate keyboard for keyboard – just to match the KeyboardMarkup interface
func (keyboard Keyboard) Keyboard() Keyboard {
	return keyboard
}

func (keyboard Keyboard) tg() [][]tg.KeyboardButton {
	res := make([][]tg.KeyboardButton, len(keyboard))

	for r, columns := range keyboard {
		res[r] = make([]tg.KeyboardButton, len(keyboard[r]))
		c := 0
		for _, button := range columns {
			res[r][c] = tg.KeyboardButton{Text: button.Text}
			c++
		}
	}
	return res
}

// FindOutgoingMessage retrieve the message that was sent by bot. Useful to change callbacks, eventid and to edit the message
func (m *Message) FindOutgoingMessage(db *mgo.Database) (*OutgoingMessage, error) {
	if m.om != nil {
		return m.om, nil
	}
	if m.BotID != m.FromID {
		return nil, nil
	}

	om := OutgoingMessage{}
	err := db.C("messages").Find(bson.M{"chatid": m.ChatID, "botid": m.BotID, "msgid": m.MsgID}).One(&om)

	if err != nil {
		return nil, err
	}
	m.om = &om
	return &om, nil
}

// FindMessageByBsonID find message by Mongo Object ID
func (c *Context) FindMessageByBsonID(id bson.ObjectId) (*Message, error) {
	return findMessageByBsonID(c.db, id)
}

// FindMessageByEventID find message by event id
func (c *Context) FindMessageByEventID(id string) (*Message, error) {
	return findMessageByEventID(c.db, c.Chat.ID, c.Bot().ID, id)
}

func findMessageByEventID(db *mgo.Database, chatID int64, botID int64, eventID string) (*Message, error) {
	msg := OutgoingMessage{}

	err := db.C("messages").Find(bson.M{"chatid": chatID, "botid": botID, "eventid": eventID}).Sort("-_id").One(&msg)
	if err != nil || msg.BotID == 0 {
		return nil, err
	}
	msg.Message.om = &msg
	return &msg.Message, nil
}

func findMessageByBsonID(db *mgo.Database, id bson.ObjectId) (*Message, error) {
	if !id.Valid() {
		return nil, errors.New("BSON ObjectId is not valid")
	}
	msg := OutgoingMessage{}
	err := db.C("messages").Find(bson.M{"_id": id}).One(&msg)
	if err != nil {
		return nil, err
	}
	msg.Message.om = &msg
	return &msg.Message, nil
}

func findMessage(db *mgo.Database, chatID int64, botID int64, msgID int) (*Message, error) {
	msg := OutgoingMessage{}
	err := db.C("messages").Find(bson.M{"chatid": chatID, "botid": botID, "msgid": msgID}).One(&msg)
	if err != nil {
		return nil, err
	}
	msg.Message.om = &msg
	return &msg.Message, nil
}

func findInlineMessage(db *mgo.Database, botID int64, inlineMsgID string) (*Message, error) {
	msg := OutgoingMessage{}
	err := db.C("messages").Find(bson.M{"botid": botID, "inlinemsgid": inlineMsgID}).One(&msg)
	if err != nil {
		return nil, err
	}
	msg.Message.om = &msg
	return &msg.Message, nil
}

func findLastOutgoingMessageInChat(db *mgo.Database, botID int64, chatID int64) (*Message, error) {

	fmt.Printf("findLastOutgoingMessageInChat: botID %d, chatID %d\n", botID, chatID)
	msg := OutgoingMessage{}
	err := db.C("messages").Find(bson.M{"chatid": chatID, "botid": botID, "fromid": botID}).Sort("-msgid").One(&msg)
	if err != nil {
		return nil, err
	}
	msg.Message.om = &msg
	return &msg.Message, nil
}

// SetChat sets the target chat to send the message
func (m *OutgoingMessage) SetChat(id int64) *OutgoingMessage {
	m.ChatID = id
	return m
}

// SetBackupChat set backup chat id that will be used in case message failed to sent to private chat (f.e. bot stopped or not initialized)
func (m *OutgoingMessage) SetBackupChat(id int64) *OutgoingMessage {
	m.BackupChatID = id
	return m
}

// SetDocument adds the file located at localPath with name fileName to the message
func (m *OutgoingMessage) SetDocument(localPath string, fileName string) *OutgoingMessage {
	m.FilePath = localPath
	m.FileName = fileName
	m.FileType = "document"
	return m
}

// SetImage adds the image file located at localPath with name fileName to the message
func (m *OutgoingMessage) SetImage(localPath string, fileName string) *OutgoingMessage {
	m.FilePath = localPath
	m.FileName = fileName
	m.FileType = "image"
	return m
}

// SetKeyboard sets the keyboard markup and Selective bool. If Selective is true keyboard will sent only for target users that you must @mention people in text or specify with SetReplyToMsgID
func (m *OutgoingMessage) SetKeyboard(k KeyboardMarkup, selective bool) *OutgoingMessage {
	m.Keyboard = true
	m.KeyboardMarkup = k.Keyboard()
	m.Selective = selective
	//todo: here is workaround for QT version. Keyboard with selective is not working
	return m
}

// SetInlineKeyboard sets the inline keyboard markup
func (m *OutgoingMessage) SetInlineKeyboard(k InlineKeyboardMarkup) *OutgoingMessage {
	m.InlineKeyboardMarkup = k.Keyboard()
	return m
}

// SetSelective sets the Selective mode for the keyboard. If Selective is true keyboard make sure to @mention people in text or specify message to reply with SetReplyToMsgID
func (m *OutgoingMessage) SetSelective(b bool) *OutgoingMessage {
	m.Selective = b
	return m
}

// SetSilent turns off notifications on iOS and make it silent on Android
func (m *OutgoingMessage) SetSilent(b bool) *OutgoingMessage {
	m.Silent = b
	return m
}

// SetOneTimeKeyboard sets the Onetime mode for keyboard. Keyboard will be hided after 1st use
func (m *OutgoingMessage) SetOneTimeKeyboard(b bool) *OutgoingMessage {
	m.OneTimeKeyboard = b
	return m
}

// SetResizeKeyboard sets the ResizeKeyboard to collapse keyboard wrapper to match the actual underneath keyboard
func (m *OutgoingMessage) SetResizeKeyboard(b bool) *OutgoingMessage {
	m.ResizeKeyboard = b
	return m
}

// SetCallbackAction sets the callback func that will be called when user press inline button with Data field
// !!! Please note that you must omit first arg *integram.Context, because it will be automatically prepended as message reply received and will contain actual context
func (m *IncomingMessage) SetCallbackAction(handlerFunc interface{}, args ...interface{}) *IncomingMessage {
	m.Message.SetCallbackAction(handlerFunc, args...)
	//TODO: save reply action

	return m
}

// SetCallbackAction sets the callback func that will be called when user press inline button with Data field
func (m *OutgoingMessage) SetCallbackAction(handlerFunc interface{}, args ...interface{}) *OutgoingMessage {
	m.Message.SetCallbackAction(handlerFunc, args...)
	return m
}

// SetReplyAction sets the reply func that will be called when user reply the message
// !!! Please note that you must omit first arg *integram.Context, because it will be automatically prepended as message reply received and will contain actual context
func (m *IncomingMessage) SetReplyAction(handlerFunc interface{}, args ...interface{}) *IncomingMessage {
	m.Message.SetReplyAction(handlerFunc, args...)
	//TODO: save reply action

	return m
}

// SetReplyAction sets the reply func that will be called when user reply the message
// !!! Please note that you must omit first arg *integram.Context, because it will be automatically prepended as message reply received and will contain actual context
func (m *OutgoingMessage) SetReplyAction(handlerFunc interface{}, args ...interface{}) *OutgoingMessage {
	m.Message.SetReplyAction(handlerFunc, args...)
	return m
}

// SetCallbackAction sets the reply func that will be called when user reply the message
// !!! Please note that you must omit first arg *integram.Context, because it will be automatically prepended as message reply received and will contain actual context
func (m *Message) SetCallbackAction(handlerFunc interface{}, args ...interface{}) *Message {
	funcName := runtime.FuncForPC(reflect.ValueOf(handlerFunc).Pointer()).Name()

	if _, ok := actionFuncs[funcName]; !ok {
		log.Panic(errors.New("Action for '" + funcName + "' not registred in service's configuration!"))
		return m
	}

	err := verifyTypeMatching(handlerFunc, args...)

	if err != nil {
		log.WithError(err).Error("Can't verify onCallback args for " + funcName + ". Be sure to omit first arg of type '*integram.Context'")
		return m
	}

	bytes, err := encode(args)

	if err != nil {
		log.WithError(err).Error("Can't encode onCallback args")
		return m
	}

	m.OnCallbackData = bytes
	m.OnCallbackAction = funcName

	return m
}

// SetReplyAction sets the reply func that will be called when user reply the message
// !!! Please note that you must omit first arg *integram.Context, because it will be automatically prepended as message reply received and will contain actual context
func (m *Message) SetReplyAction(handlerFunc interface{}, args ...interface{}) *Message {
	funcName := runtime.FuncForPC(reflect.ValueOf(handlerFunc).Pointer()).Name()

	if _, ok := actionFuncs[funcName]; !ok {
		log.Panic(errors.New("Action for '" + funcName + "' not registred in service's configuration!"))
		return m
	}

	err := verifyTypeMatching(handlerFunc, args...)

	if err != nil {
		log.WithError(err).Error("Can't verify onReply args for " + funcName + ". Be sure to omit first arg of type '*integram.Context'")
		return m
	}

	bytes, err := encode(args)

	if err != nil {
		log.WithError(err).Error("Can't encode onReply args")
		return m
	}

	m.OnReplyData = bytes
	m.OnReplyAction = funcName

	return m
}

// HideKeyboard will hide existing keyboard in the chat where message will be sent
func (m *OutgoingMessage) HideKeyboard() *OutgoingMessage {
	m.KeyboardHide = true
	return m
}

// EnableForceReply will automatically set the reply to this message and focus on the input field
func (m *OutgoingMessage) EnableForceReply() *OutgoingMessage {
	m.ForceReply = true
	return m
}

type messageSender interface {
	Send(m *OutgoingMessage) error
}

type scheduleMessageSender struct{}

var activeMessageSender = scheduleMessageSender{}

func (t *scheduleMessageSender) Send(m *OutgoingMessage) error {
	if m.processed {
		return nil
	}
	if m.AntiFlood {
		db := mongoSession.Clone().DB(mongo.Database)
		defer db.Session.Close()
		msg, _ := findLastOutgoingMessageInChat(db, m.BotID, m.ChatID)
		if msg != nil && msg.Text == m.Text && time.Now().Sub(msg.Date).Seconds() < antiFloodTimeout {
			log.Errorf("flood. mins %v", time.Now().Sub(msg.Date).Minutes())
			return nil
		}
	}
	if m.Selective && m.ChatID > 0 {
		m.Selective = false
	}
	m.ID = bson.NewObjectId()

	if m.Selective && len(m.findUsernames()) == 0 && m.ReplyToMsgID == 0 {
		err := errors.New("Inconsistence. Selective is true but there are no @mention or ReplyToMsgID specified")
		log.WithField("message", m).Error(err)
		return err
	}

	if m.ParseMode == "HTML" {
		text := ""
		var err error
		if m.FilePath == "" {
			text, err = sanitize.HTMLAllowing(m.Text, []string{"a", "b", "strong", "i", "em", "a", "code", "pre"}, []string{"href"})
		} else {
			// formatiing is not supported for file captions
			text = sanitize.HTML(m.Text)
		}

		if err == nil && text != "" {
			m.Text = text
		}
	} else {
		text := sanitize.HTML(m.Text)
		if text != "" {
			m.Text = text
		}
	}

	_, err := sendMessageJob.Schedule(0, time.Now(), &m)
	if err != nil {
		log.WithField("message", m).WithError(err).Error("Can't schedule sendMessageJob")
	} else {
		m.processed = true
	}
	return err
}

// Send put the message to the jobs queue
func (m *OutgoingMessage) Send() error {
	return activeMessageSender.Send(m)
}

// SendAndGetID put the message to the jobs queue and return the message's BSON Object ID
func (m *OutgoingMessage) SendAndGetID() (bson.ObjectId, error) {
	err := activeMessageSender.Send(m)
	return m.ID, err
}

// AddEventID attach one or more event ID. You can use eventid to edit the message in case of additional webhook received or to ignore in case of duplicate
func (m *OutgoingMessage) AddEventID(id ...string) *OutgoingMessage {
	m.EventID = append(m.EventID, id...)
	return m
}

// EnableAntiFlood will check if the message wasn't already sent within last antiFloodTimeout seconds
func (m *OutgoingMessage) EnableAntiFlood() *OutgoingMessage {
	m.AntiFlood = true

	return m
}

// SetTextFmt is a shorthand for SetText(fmt.Sprintf("%s %s %s", a, b, c))
func (m *OutgoingMessage) SetTextFmt(text string, a ...interface{}) *OutgoingMessage {
	m.Text = fmt.Sprintf(text, a...)
	return m
}

// SetText set the text of message to sent
// In case of documents and photo messages this text will be used in the caption
func (m *OutgoingMessage) SetText(text string) *OutgoingMessage {
	m.Text = text
	return m
}

// DisableWebPreview indicates TG clients to not trying to resolve the URL's in the message
func (m *OutgoingMessage) DisableWebPreview() *OutgoingMessage {
	m.WebPreview = false
	return m
}

// EnableMarkdown sets parseMode to Markdown
func (m *OutgoingMessage) EnableMarkdown() *OutgoingMessage {
	m.ParseMode = "Markdown"
	return m
}

// EnableHTML sets parseMode to HTML
func (m *OutgoingMessage) EnableHTML() *OutgoingMessage {
	m.ParseMode = "HTML"
	return m
}

// SetParseMode sets parseMode: 'HTML' and 'markdown' supporting for now
func (m *OutgoingMessage) SetParseMode(s string) *OutgoingMessage {
	m.ParseMode = s
	return m
}

// SetReplyToMsgID sets parseMode: 'HTML' and 'markdown' supporting for now
func (m *OutgoingMessage) SetReplyToMsgID(id int) *OutgoingMessage {
	m.ReplyToMsgID = id
	return m
}

// UpdateEventsID sets the event id and update it in DB
func (m *Message) UpdateEventsID(db *mgo.Database, eventID ...string) error {
	m.EventID = append(m.EventID, eventID...)
	return db.C("messages").Update(bson.M{"chatid": m.ChatID, "botid": m.BotID, "msgid": m.MsgID}, bson.M{"$addToSet": bson.M{"eventid": bson.M{"$each": eventID}}})
}

// Update will update existing message in DB
func (m *Message) Update(db *mgo.Database) error {

	if m.ID.Valid() {
		return db.C("messages").UpdateId(m.ID, bson.M{"$set": m})
	}
	return errors.New("Can't update message: ID is not set")
}

func initBots() error {
	var err error

	if err != nil {
		return err
	}
	gob.Register(&OutgoingMessage{})

	poolSize := 10 // Maximum simultaneously message sending
	if p, err := strconv.Atoi(os.Getenv("INTEGRAM_TG_POOL")); err != nil && p > 0 {
		poolSize = p
	}

	pool, err := jobs.NewPool(&jobs.PoolConfig{
		Key:        "_telegram",
		NumWorkers: poolSize,
		BatchSize:  10,
	})

	if err != nil {
		return err
	}

	pool.SetMiddleware(beforeJob)
	pool.SetAfterFunc(afterJob)

	err = pool.Start()
	if err != nil {
		return err
	}
	log.Infof("Job pool %v[%d] is ready", "_telegram", poolSize)

	// 23 retries mean maximum of 8 hours deferment (fibonacci sequence)
	sendMessageJob, err = jobs.RegisterTypeWithPoolKey("sendMessage", "_telegram", 23, sendMessage)

	if err != nil {
		log.WithError(err).Panic("RegisterTypeWithPoolKey sendMessage failed")
	}
	for _, service := range services {

		bot := service.Bot()
		if bot == nil {
			continue
		}
		if !service.UseWebhookInsteadOfLongPolling {
			bot.listen()
		} else {
			_, err := bot.API.SetWebhook(tg.WebhookConfig{URL: bot.webhookURL()})
			if err != nil {
				log.WithError(err).WithField("botID", bot.ID).Error("Error on initial SetWebhook")
			}
		}
		log.Infof("@%v added for %v", bot.Username, service.Name)
	}

	return nil
}

var sendMessageJob *jobs.Type

func (m *Message) findUsernames() []string {
	r, _ := regexp.Compile("@([a-zA-Z0-9_]{5,})") // according to TG docs minimum username length is 5
	usernames := r.FindAllString(m.Text, -1)

	for index, username := range usernames {
		usernames[index] = username[1:]
	}
	return usernames

}

func detectTargetUsersID(db *mgo.Database, m *Message) []int64 {
	if m.ChatID > 0 {
		return []int64{m.ChatID}
	}

	var usersID []int64

	// 1) If message is reply to message - add original message's sender
	if m.ReplyToMsgID > 0 {
		msg, err := findMessage(db, m.ChatID, m.BotID, m.ReplyToMsgID)
		if err == nil && msg.FromID > 0 {
			usersID = append(usersID, msg.FromID)
		}
	}

	// 2) Trying to find mentions in the message's text
	usernames := m.findUsernames()

	var users []struct {
		ID int64 `bson:"_id"`
	}
	db.C("users").Find(bson.M{"username": bson.M{"$in": usernames}}).Select(bson.M{"_id": 1}).All(&users)

	for _, user := range users {
		if len(usersID) == 0 || usersID[0] != user.ID {
			usersID = append(usersID, user.ID)
		}
	}
	return usersID
}

func botByID(ID int64) *Bot {
	if bot, exists := botPerID[ID]; exists {
		return bot
	}

	return nil
}

func sendMessage(m *OutgoingMessage) error {
	log.Debugf("Trying to send message: %+v\n", m)
	msg := tg.MessageConfig{Text: m.Text, BaseChat: tg.BaseChat{ChatID: m.ChatID}}

	if m.ChatID == 0 {
		return errors.New("ChatID empty")
	}
	var err error
	var tgMsg tg.Message
	if m.FilePath != "" {
		if m.FileType == "image" {
			msg := tg.NewPhotoUpload(m.ChatID, m.FilePath)
			msg.FileName = m.FileName
			msg.Caption = m.Text
			if m.ReplyToMsgID != 0 {
				msg.BaseChat.ReplyToMessageID = m.ReplyToMsgID
			}
			tgMsg, err = botByID(m.BotID).API.Send(msg)

		} else {
			msg := tg.NewDocumentUpload(m.ChatID, m.FilePath)
			msg.FileName = m.FileName
			msg.Caption = m.Text
			if m.ReplyToMsgID != 0 {
				msg.BaseChat.ReplyToMessageID = m.ReplyToMsgID
			}
			tgMsg, err = botByID(m.BotID).API.Send(msg)

		}

	} else {

		if m.KeyboardHide {
			msg.ReplyMarkup = tg.ReplyKeyboardHide{HideKeyboard: true, Selective: m.Selective}
		}

		if m.ForceReply {
			msg.ReplyMarkup = tg.ForceReply{ForceReply: true, Selective: m.Selective}
		}
		// Keyboard will overridde HideKeyboard
		if m.KeyboardMarkup != nil && len(m.KeyboardMarkup) > 0 {
			msg.ReplyMarkup = tg.ReplyKeyboardMarkup{Keyboard: m.KeyboardMarkup.tg(), OneTimeKeyboard: m.OneTimeKeyboard, Selective: m.Selective, ResizeKeyboard: m.ResizeKeyboard}
		}

		if len(m.InlineKeyboardMarkup.Buttons) > 0 {
			msg.ReplyMarkup = tg.InlineKeyboardMarkup{InlineKeyboard: m.InlineKeyboardMarkup.tg()}
		}

		msg.DisableWebPagePreview = !m.WebPreview

		msg.DisableNotification = m.Silent

		if m.ReplyToMsgID != 0 {
			msg.BaseChat.ReplyToMessageID = m.ReplyToMsgID
		}

		if m.ParseMode != "" {
			msg.ParseMode = m.ParseMode
		}
		tgMsg, err = botByID(m.BotID).API.Send(msg)
	}

	if err == nil {

		db := mongoSession.Clone().DB(mongo.Database)
		defer db.Session.Close()

		log.Debugf("Successfully sent, id=%v\n", tgMsg.MessageID)
		m.MsgID = tgMsg.MessageID
		m.Date = time.Now()

		err = db.C("messages").Insert(&m)
		if err != nil {
			log.WithError(err).Error("Error outgoing inserting message in db")
		}

		err = saveKeyboard(m, db)
		if err != nil {
			log.WithError(err).Error("Error processing keyboard")
		}
		return nil
	}

	if tgErr, ok := err.(tg.Error); ok {
		//  Todo: Bad workaround to catch network errors
		if tgErr.Code == 0 {
			log.WithError(err).Warn("Network error while sending a message")
			// pass through the error so the job will be rescheduled
			return err
		} else if tgErr.Code == 500 {
			log.WithError(err).Warn("TG dc is down while sending a message")
			// pass through the error so the job will be rescheduled
			return err
		} else if tgErr.IsMessageNotFound() {

			log.WithError(err).WithFields(log.Fields{"msgid": m.ReplyToMsgID, "chat": m.ChatID, "bot": m.BotID}).Warn("TG message we are replying on is no longer exists")
			// looks like the message we replying on is no longer exists...
			m.ReplyToMsgID = 0
			_, err := sendMessageJob.Schedule(0, time.Now(), &m)
			if err != nil {
				log.WithField("message", m).WithError(err).Error("Can't reschedule sendMessageJob")
			}
			return nil
		} else if chatID := tgErr.ChatMigratedToChatID(); chatID != 0 {
			// looks like the the chat we trying to send the message is migrated to supergroup
			log.Warnf("sendMessage error: Migrated to %v", chatID)

			db := mongoSession.Clone().DB(mongo.Database)
			defer db.Session.Close()
			migrateToSuperGroup(db, m.ChatID, chatID)

			// todo: in rare case this can produce duplicate messages for incoming webhooks
			m.ChatID = chatID
			//_, err := sendMessageJob.Schedule(0, time.Now(), &m)
			if err != nil {
				log.WithField("message", m).WithError(err).Error("Can't reschedule sendMessageJob")
			}

			return nil
		} else if tgErr.BotStoppedForUser() {
			// Todo: Problems can appear when we rely on this user message (e.g. not webhook msg)
			log.WithField("chat", m.ChatID).WithField("bot", m.BotID).Warn("sendMessage error: Bot stopped by user")
			if m.BackupChatID != 0 {
				if m.BackupChatID != m.ChatID {
					// if this fall from private messages - add the mention and selective to grace notifications and protect the keyboard
					if m.ChatID > 0 && m.BackupChatID < 0 {
						db := mongoSession.Clone().DB(mongo.Database)
						defer db.Session.Close()
						username := findUsernameByID(db, m.ChatID)
						if username != "" {
							m.Text = "@" + username + " " + m.Text
							m.Selective = true
						}
					}
					m.ChatID = m.BackupChatID
					_, err := sendMessageJob.Schedule(0, time.Now(), &m)
					return err
				}

				return errors.New("BackupChatID failed")

			}
			return nil
		} else if tgErr.ChatNotFound() {
			// usually this means that user not initialized the private chat with the bot
			log.WithField("chat", m.ChatID).WithField("bot", m.BotID).Warn("sendMessage error: Chat not found")
			if m.BackupChatID != 0 {
				if m.BackupChatID != m.ChatID {
					// if this fall from private messages - add the mention and selective to grace notifications and protect the keyboard
					if m.ChatID > 0 && m.BackupChatID < 0 {
						db := mongoSession.Clone().DB(mongo.Database)
						defer db.Session.Close()
						username := findUsernameByID(db, m.ChatID)
						if username != "" {
							m.Text = "@" + username + " " + m.Text
							m.Selective = true
						}
					}
					m.ChatID = m.BackupChatID
					_, err := sendMessageJob.Schedule(0, time.Now(), &m)
					return err
				}

				return errors.New("BackupChatID failed")
			}
			return nil
		} else if tgErr.BotKicked() {

			db := mongoSession.Clone().DB(mongo.Database)
			defer db.Session.Close()
			bot := botByID(m.BotID)

			if len(bot.services) == 1 {
				removeHooksForChat(db, bot.services[0].Name, m.ChatID)
			}

			log.WithField("chat", m.ChatID).WithField("bot", m.BotID).Warn("sendMessage error: Bot kicked")

			return nil
		} else if tgErr.ChatDiactivated() {
			db := mongoSession.Clone().DB(mongo.Database)
			defer db.Session.Close()
			bot := botByID(m.BotID)

			if len(bot.services) == 1 {
				removeHooksForChat(db, bot.services[0].Name, m.ChatID)
			}

			db.C("chats").UpdateId(m.ChatID, bson.M{"$set": bson.M{"deactivated": true}})
			log.WithField("chat", m.ChatID).WithField("bot", m.BotID).Warn("sendMessage error: Chat deactivated")
			return nil
		} else if tgErr.TooManyRequests() {
			log.WithField("chat", m.ChatID).WithField("bot", m.BotID).Warn("sendMessage error: TooManyRequests")

			_, err := sendMessageJob.Schedule(0, time.Now().Add(time.Duration(10+rand.Intn(30))*time.Second), &m)
			return err
		}

		log.WithError(err).WithField("message", m).Error("TG error while sending a message")
		return nil
	}
	log.WithError(err).WithField("message", m).Error("Error while sending a message")
	return err
}
