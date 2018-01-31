package integram

import (
	uurl "net/url"
	"reflect"
	"testing"
	"time"

	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"github.com/mrjones/oauth"
	"github.com/requilence/url"
	"golang.org/x/oauth2"
	"gopkg.in/mgo.v2"
	tg "github.com/requilence/telegram-bot-api"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"crypto/md5"
)

func TestContext_SetServiceBaseURL(t *testing.T) {
	type fields struct {
		ServiceName           string
		ServiceBaseURL        url.URL
		db                    *mgo.Database
		gin                   *gin.Context
		User                  User
		Chat                  Chat
		Message               *IncomingMessage
		InlineQuery           *tg.InlineQuery
		ChosenInlineResult    *chosenInlineResult
		Callback              *callback
		inlineQueryAnsweredAt *time.Time
	}
	type args struct {
		domainOrURL string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *Context
	}{
		{"set from URL", fields{}, args{"https://sub.domain.com/a"}, &Context{ServiceBaseURL: url.URL{Scheme: "https", Host: "sub.domain.com"}}},
		{"set from domain", fields{}, args{"sub.domain2.com"}, &Context{ServiceBaseURL: url.URL{Scheme: "https", Host: "sub.domain2.com"}}},
	}
	for _, tt := range tests {
		c := &Context{
			ServiceName:           tt.fields.ServiceName,
			ServiceBaseURL:        tt.fields.ServiceBaseURL,
			db:                    tt.fields.db,
			gin:                   tt.fields.gin,
			User:                  tt.fields.User,
			Chat:                  tt.fields.Chat,
			Message:               tt.fields.Message,
			InlineQuery:           tt.fields.InlineQuery,
			ChosenInlineResult:    tt.fields.ChosenInlineResult,
			Callback:              tt.fields.Callback,
			inlineQueryAnsweredAt: tt.fields.inlineQueryAnsweredAt,
		}

		if c.SetServiceBaseURL(tt.args.domainOrURL); !reflect.DeepEqual(c, tt.want) {
			t.Errorf("%q. Context.SetServiceBaseURL() = %v, want %v", tt.name, c, tt.want)
		}
	}

}

func TestContext_SaveOAuthProvider(t *testing.T) {
	type fields struct {
		ServiceName           string
		ServiceBaseURL        url.URL
		db                    *mgo.Database
		gin                   *gin.Context
		User                  User
		Chat                  Chat
		Message               *IncomingMessage
		InlineQuery           *tg.InlineQuery
		ChosenInlineResult    *chosenInlineResult
		Callback              *callback
		inlineQueryAnsweredAt *time.Time
	}
	type args struct {
		baseURL url.URL
		id      string
		secret  string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *OAuthProvider
		wantErr bool
	}{
		{"OAuth1", fields{ServiceName: "servicewithoauth1", db: db}, args{*URLMustParse("https://sub.another.com"), "Abcd", "Efgh"}, &OAuthProvider{"servicewithoauth1", url.URL{Scheme: "https", Host: "sub.another.com"}, "Abcd", "Efgh"}, false},
		{"OAuth2", fields{ServiceName: "servicewithoauth2", db: db}, args{*URLMustParse("https://sub.other.com"), "Abcd", "Efgh"}, &OAuthProvider{"servicewithoauth2", url.URL{Scheme: "https", Host: "sub.other.com"}, "Abcd", "Efgh"}, false},
	}
	for _, tt := range tests {
		c := &Context{
			ServiceName:           tt.fields.ServiceName,
			ServiceBaseURL:        tt.fields.ServiceBaseURL,
			db:                    tt.fields.db,
			gin:                   tt.fields.gin,
			User:                  tt.fields.User,
			Chat:                  tt.fields.Chat,
			Message:               tt.fields.Message,
			InlineQuery:           tt.fields.InlineQuery,
			ChosenInlineResult:    tt.fields.ChosenInlineResult,
			Callback:              tt.fields.Callback,
			inlineQueryAnsweredAt: tt.fields.inlineQueryAnsweredAt,
		}
		got, err := c.SaveOAuthProvider(tt.args.baseURL, tt.args.id, tt.args.secret)
		if (err != nil) != tt.wantErr {
			t.Errorf("%q. Context.SaveOAuthProvider() error = %v, wantErr %v", tt.name, err, tt.wantErr)
			continue
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. Context.SaveOAuthProvider() = %v, want %v", tt.name, got, tt.want)
		}
		oap := OAuthProvider{}
		c.db.C("oauth_providers").FindId(got.internalID()).One(&oap)

		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. Context.SaveOAuthProvider() db check: %v, want %v", tt.name, oap, tt.want)
		}
	}
}

func TestContext_OAuthProvider(t *testing.T) {
	type fields struct {
		ServiceName           string
		ServiceBaseURL        url.URL
		db                    *mgo.Database
		gin                   *gin.Context
		User                  User
		Chat                  Chat
		Message               *IncomingMessage
		InlineQuery           *tg.InlineQuery
		ChosenInlineResult    *chosenInlineResult
		Callback              *callback
		inlineQueryAnsweredAt *time.Time
	}
	tests := []struct {
		name   string
		fields fields
		want   *OAuthProvider
	}{
		{"OAuth1", fields{ServiceName: "servicewithoauth1", db: db}, &OAuthProvider{"servicewithoauth1", url.URL{Scheme: "https", Host: "sub.example.com"}, "ID", "SECRET"}},
		{"OAuth2", fields{ServiceName: "servicewithoauth2", db: db}, &OAuthProvider{"servicewithoauth2", url.URL{Scheme: "https", Host: "sub.example.com"}, "ID", "SECRET"}},
	}
	for _, tt := range tests {
		c := &Context{
			ServiceName:           tt.fields.ServiceName,
			ServiceBaseURL:        tt.fields.ServiceBaseURL,
			db:                    tt.fields.db,
			gin:                   tt.fields.gin,
			User:                  tt.fields.User,
			Chat:                  tt.fields.Chat,
			Message:               tt.fields.Message,
			InlineQuery:           tt.fields.InlineQuery,
			ChosenInlineResult:    tt.fields.ChosenInlineResult,
			Callback:              tt.fields.Callback,
			inlineQueryAnsweredAt: tt.fields.inlineQueryAnsweredAt,
		}
		if got := c.OAuthProvider(); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. Context.OAuthProvider() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestOAuthProvider_OAuth1Client(t *testing.T) {
	type fields struct {
		Service string
		BaseURL url.URL
		ID      string
		Secret  string
	}
	type args struct {
		c *Context
	}
	oAuth1Consumer := oauth.NewConsumer("ID", "SECRET", oauth.ServiceProvider{RequestTokenUrl: "https://sub.example.com/1/OAuthGetRequestToken",
		AuthorizeTokenUrl: "https://sub.example.com/1/OAuthAuthorizeToken",
		AccessTokenUrl:    "https://sub.example.com/1/OAuthGetAccessToken"})
	oAuth1Consumer.AdditionalAuthorizationUrlParams = map[string]string{
		"name":       "Integram",
		"expiration": "never",
		"scope":      "read,write",
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		want   *oauth.Consumer
	}{
		{"OAuth1", fields{"servicewithoauth1", *URLMustParse("https://sub.example.com"), "ID", "SECRET"}, args{&Context{ServiceName: "servicewithoauth1", db: db}}, oAuth1Consumer},
	}
	for _, tt := range tests {
		o := &OAuthProvider{
			Service: tt.fields.Service,
			BaseURL: tt.fields.BaseURL,
			ID:      tt.fields.ID,
			Secret:  tt.fields.Secret,
		}
		// DeepEqual of oauth.Consumer not working because of some static nonce generators created during oauth.NewConsumer()
		if got := o.OAuth1Client(tt.args.c); got == nil || !reflect.DeepEqual(got.AdditionalAuthorizationUrlParams, tt.want.AdditionalAuthorizationUrlParams) || !reflect.DeepEqual(got.AdditionalHeaders, tt.want.AdditionalHeaders) || !reflect.DeepEqual(got.AdditionalParams, tt.want.AdditionalParams) {
			t.Errorf("%q. Context.OAuth1Client() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestOAuthProvider_OAuth2Client(t *testing.T) {
	type fields struct {
		Service string
		BaseURL url.URL
		ID      string
		Secret  string
	}
	type args struct {
		c *Context
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *oauth2.Config
	}{
		{"OAuth2", fields{"servicewithoauth2", *URLMustParse("https://sub.example2.com"), "ID", "SECRET"}, args{&Context{ServiceName: "servicewithoauth2", db: db}}, &oauth2.Config{ClientID: "ID", ClientSecret: "SECRET", Endpoint: oauth2.Endpoint{
			AuthURL:  "https://sub.example2.com/oauth/authorize",
			TokenURL: "https://sub.example2.com/oauth/token",
		}}},
	}
	for _, tt := range tests {
		o := &OAuthProvider{
			Service: tt.fields.Service,
			BaseURL: tt.fields.BaseURL,
			ID:      tt.fields.ID,
			Secret:  tt.fields.Secret,
		}
		if got := o.OAuth2Client(tt.args.c); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. OAuthProvider.OAuth2Client() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestWebhookContext_FirstParse(t *testing.T) {
	type fields struct {
		gin        *gin.Context
		body       []byte
		firstParse bool
		requestID  string
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		{"true", fields{firstParse: true}, true},
		{"false", fields{firstParse: false}, false},
	}
	for _, tt := range tests {
		wc := &WebhookContext{
			gin:        tt.fields.gin,
			body:       tt.fields.body,
			firstParse: tt.fields.firstParse,
			requestID:  tt.fields.requestID,
		}
		if got := wc.FirstParse(); got != tt.want {
			t.Errorf("%q. WebhookContext.FirstParse() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestWebhookContext_Headers(t *testing.T) {
	type fields struct {
		gin        *gin.Context
		body       []byte
		firstParse bool
		requestID  string
	}
	tests := []struct {
		name   string
		fields fields
		want   map[string][]string
	}{
		{"some headers", fields{gin: &gin.Context{Request: &http.Request{Header: http.Header{"a1": []string{"v1"}, "a2": []string{"v2", "v3"}}}}}, map[string][]string{"a1": []string{"v1"}, "a2": []string{"v2", "v3"}}},
		{"no headers", fields{gin: &gin.Context{Request: &http.Request{}}}, nil},
	}
	for _, tt := range tests {
		wc := &WebhookContext{
			gin:        tt.fields.gin,
			body:       tt.fields.body,
			firstParse: tt.fields.firstParse,
			requestID:  tt.fields.requestID,
		}
		if got := wc.Headers(); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. WebhookContext.Headers() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestWebhookContext_Header(t *testing.T) {
	type fields struct {
		gin        *gin.Context
		body       []byte
		firstParse bool
		requestID  string
	}
	type args struct {
		key string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   string
	}{
		{"some headers - found", fields{gin: &gin.Context{Request: &http.Request{Header: http.Header{"Header1": {"v1"}, "Header2": {"v2", "v3"}}}}}, args{"header2"}, "v2"},
		{"some headers - not found", fields{gin: &gin.Context{Request: &http.Request{Header: http.Header{"Header1": {"v1"}, "Header2": {"v2", "v3"}}}}}, args{"header3"}, ""},
		{"no headers", fields{gin: &gin.Context{Request: &http.Request{}}}, args{"d"}, ""},
	}
	for _, tt := range tests {
		wc := &WebhookContext{
			gin:        tt.fields.gin,
			body:       tt.fields.body,
			firstParse: tt.fields.firstParse,
			requestID:  tt.fields.requestID,
		}
		if got := wc.Header(tt.args.key); got != tt.want {
			t.Errorf("%q. WebhookContext.Header() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestContext_KeyboardAnswer(t *testing.T) {
	bt := strings.Split(os.Getenv("INTEGRAM_TEST_BOT_TOKEN"), ":")
	botID, _ := strconv.ParseInt(bt[0], 10, 64)

	db.C("users").UpsertId(9999999999, userData{User: User{ID: 9999999999, FirstName: "Matthew", UserName: "matthew9999999999"}, KeyboardPerChat: []chatKeyboard{{MsgID: 99999999991, ChatID: 9999999999, BotID: botID, Keyboard: map[string]string{"CTPzBw": "val1", "rd6Lew": "val2"}}}})

	type fields struct {
		ServiceName           string
		ServiceBaseURL        url.URL
		db                    *mgo.Database
		gin                   *gin.Context
		User                  User
		Chat                  Chat
		Message               *IncomingMessage
		InlineQuery           *tg.InlineQuery
		ChosenInlineResult    *chosenInlineResult
		Callback              *callback
		inlineQueryAnsweredAt *time.Time
	}
	tests := []struct {
		name           string
		fields         fields
		wantData       string
		wantButtonText string
	}{
		{"button pressed", fields{db: db, ServiceName: "servicewithbottoken", Chat: Chat{ID: 9999999999}, User: User{ID: 9999999999}, Message: &IncomingMessage{Message: Message{Text: "key1"}}}, "val1", "key1"},
		{"other text (not match any button)", fields{db: db, ServiceName: "servicewithbottoken", Chat: Chat{ID: 9999999999}, User: User{ID: 9999999999}, Message: &IncomingMessage{Message: Message{Text: "other text"}}}, "", ""},
	}
	for _, tt := range tests {
		c := &Context{
			ServiceName:           tt.fields.ServiceName,
			ServiceBaseURL:        tt.fields.ServiceBaseURL,
			db:                    tt.fields.db,
			gin:                   tt.fields.gin,
			User:                  tt.fields.User,
			Chat:                  tt.fields.Chat,
			Message:               tt.fields.Message,
			InlineQuery:           tt.fields.InlineQuery,
			ChosenInlineResult:    tt.fields.ChosenInlineResult,
			Callback:              tt.fields.Callback,
			inlineQueryAnsweredAt: tt.fields.inlineQueryAnsweredAt,
		}
		c.User.ctx = c
		c.Chat.ctx = c

		gotData, gotButtonText := c.KeyboardAnswer()
		if gotData != tt.wantData {
			t.Errorf("%q. Context.KeyboardAnswer() gotData = %v, want %v", tt.name, gotData, tt.wantData)
		}
		if gotButtonText != tt.wantButtonText {
			t.Errorf("%q. Context.KeyboardAnswer() gotButtonText = %v, want %v", tt.name, gotButtonText, tt.wantButtonText)
		}
	}

	db.C("users").RemoveId(9999999999)
}

func TestContext_Log(t *testing.T) {

	type fields struct {
		ServiceName           string
		ServiceBaseURL        url.URL
		db                    *mgo.Database
		gin                   *gin.Context
		User                  User
		Chat                  Chat
		Message               *IncomingMessage
		InlineQuery           *tg.InlineQuery
		ChosenInlineResult    *chosenInlineResult
		Callback              *callback
		inlineQueryAnsweredAt *time.Time
	}
	tests := []struct {
		name   string
		fields fields
		want   *log.Entry
	}{
		{"test1", fields{ServiceName: "servicewithbottoken", ServiceBaseURL: *URLMustParse("https://sub.example.com"), User: User{ID: 123}, Chat: Chat{ID: -456}, Message: &IncomingMessage{Message: Message{Text: "text"}}}, &log.Entry{Logger: log.StandardLogger(), Data: log.Fields{"user": int64(123), "chat": int64(-456), "msg": "1cb251ec0d568de6a929b520c4aed8d1", "domain": "sub.example.com", "service": "servicewithbottoken"}}},
	}
	for _, tt := range tests {
		c := &Context{
			ServiceName:           tt.fields.ServiceName,
			ServiceBaseURL:        tt.fields.ServiceBaseURL,
			db:                    tt.fields.db,
			gin:                   tt.fields.gin,
			User:                  tt.fields.User,
			Chat:                  tt.fields.Chat,
			Message:               tt.fields.Message,
			InlineQuery:           tt.fields.InlineQuery,
			ChosenInlineResult:    tt.fields.ChosenInlineResult,
			Callback:              tt.fields.Callback,
			inlineQueryAnsweredAt: tt.fields.inlineQueryAnsweredAt,
		}
		if got := c.Log(); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. Context.Log() = %+v, want %+v", tt.name, got.Data, tt.want.Data)
		}
	}

}

func TestContext_Db(t *testing.T) {
	type fields struct {
		ServiceName           string
		ServiceBaseURL        url.URL
		db                    *mgo.Database
		gin                   *gin.Context
		User                  User
		Chat                  Chat
		Message               *IncomingMessage
		InlineQuery           *tg.InlineQuery
		ChosenInlineResult    *chosenInlineResult
		Callback              *callback
		inlineQueryAnsweredAt *time.Time
	}
	tests := []struct {
		name   string
		fields fields
		want   *mgo.Database
	}{
		{"test1", fields{db: db}, db},
	}
	for _, tt := range tests {
		c := &Context{
			ServiceName:           tt.fields.ServiceName,
			ServiceBaseURL:        tt.fields.ServiceBaseURL,
			db:                    tt.fields.db,
			gin:                   tt.fields.gin,
			User:                  tt.fields.User,
			Chat:                  tt.fields.Chat,
			Message:               tt.fields.Message,
			InlineQuery:           tt.fields.InlineQuery,
			ChosenInlineResult:    tt.fields.ChosenInlineResult,
			Callback:              tt.fields.Callback,
			inlineQueryAnsweredAt: tt.fields.inlineQueryAnsweredAt,
		}
		if got := c.Db(); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. Context.Db() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestContext_Service(t *testing.T) {
	type fields struct {
		ServiceName           string
		ServiceBaseURL        url.URL
		db                    *mgo.Database
		gin                   *gin.Context
		User                  User
		Chat                  Chat
		Message               *IncomingMessage
		InlineQuery           *tg.InlineQuery
		ChosenInlineResult    *chosenInlineResult
		Callback              *callback
		inlineQueryAnsweredAt *time.Time
	}
	tests := []struct {
		name   string
		fields fields
		want   *Service
	}{
		{"servicewithbottoken", fields{ServiceName: "servicewithbottoken"}, services["servicewithbottoken"]},
		{"servicewithoauth1", fields{ServiceName: "servicewithoauth1"}, services["servicewithoauth1"]},
	}
	for _, tt := range tests {
		c := &Context{
			ServiceName:           tt.fields.ServiceName,
			ServiceBaseURL:        tt.fields.ServiceBaseURL,
			db:                    tt.fields.db,
			gin:                   tt.fields.gin,
			User:                  tt.fields.User,
			Chat:                  tt.fields.Chat,
			Message:               tt.fields.Message,
			InlineQuery:           tt.fields.InlineQuery,
			ChosenInlineResult:    tt.fields.ChosenInlineResult,
			Callback:              tt.fields.Callback,
			inlineQueryAnsweredAt: tt.fields.inlineQueryAnsweredAt,
		}
		if got := c.Service(); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. Context.Service() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestContext_Bot(t *testing.T) {
	bt := strings.Split(os.Getenv("INTEGRAM_TEST_BOT_TOKEN"), ":")
	botID, _ := strconv.ParseInt(bt[0], 10, 64)
	type fields struct {
		ServiceName           string
		ServiceBaseURL        url.URL
		db                    *mgo.Database
		gin                   *gin.Context
		User                  User
		Chat                  Chat
		Message               *IncomingMessage
		InlineQuery           *tg.InlineQuery
		ChosenInlineResult    *chosenInlineResult
		Callback              *callback
		inlineQueryAnsweredAt *time.Time
	}
	tests := []struct {
		name   string
		fields fields
		want   *Bot
	}{
		{"servicewithbottoken", fields{ServiceName: "servicewithbottoken"}, botPerID[botID]},
	}
	for _, tt := range tests {
		c := &Context{
			ServiceName:           tt.fields.ServiceName,
			ServiceBaseURL:        tt.fields.ServiceBaseURL,
			db:                    tt.fields.db,
			gin:                   tt.fields.gin,
			User:                  tt.fields.User,
			Chat:                  tt.fields.Chat,
			Message:               tt.fields.Message,
			InlineQuery:           tt.fields.InlineQuery,
			ChosenInlineResult:    tt.fields.ChosenInlineResult,
			Callback:              tt.fields.Callback,
			inlineQueryAnsweredAt: tt.fields.inlineQueryAnsweredAt,
		}
		if got := c.Bot(); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. Context.Bot() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

var msgWithInlineKB *Message

var msgWithEventID *Message

func sendMessageWithInlineKB(t *testing.T) *Message {
	if msgWithInlineKB != nil {
		msgWithInlineKB, _ = findMessageByBsonID(db, msgWithInlineKB.ID)

		return msgWithInlineKB
	}

	chatID, _ := strconv.ParseInt(os.Getenv("INTEGRAM_TEST_USER"), 10, 64)
	activeMessageSender = scheduleMessageSender{}

	c := Context{ServiceName: "servicewithbottoken", Chat: Chat{ID: chatID}, db: db}
	m := c.NewMessage()
	m.AddEventID(fmt.Sprintf("%d", time.Now().Unix())).SetText("text").SetInlineKeyboard(InlineKeyboard{State: "kbstateval", Buttons: []InlineButtons{{{Text: "Key1", Data: "Data1"}}, {{Text: "Key2", Data: "Data2"}}}}).Send()

	time.Sleep(time.Second * 2)
	var err error

	msgWithInlineKB, err = findMessageByBsonID(db, m.ID)

	if err != nil {
		t.Errorf("sendMessageWithInlineKB error on msg sent = %v", err)
	}
	return msgWithInlineKB
}

func sendMessageWithEventID(t *testing.T, eventID string) *Message {
	if msgWithEventID != nil {
		msgWithEventID, _ = findMessageByBsonID(db, msgWithEventID.ID)

		return msgWithEventID
	}

	chatID, _ := strconv.ParseInt(os.Getenv("INTEGRAM_TEST_USER"), 10, 64)
	activeMessageSender = scheduleMessageSender{}

	c := Context{ServiceName: "servicewithbottoken", Chat: Chat{ID: chatID}, db: db}
	m := c.NewMessage()
	m.AddEventID(eventID).SetTextFmt("Msg with eventid <b>%s</b>", eventID).EnableHTML().Send()

	time.Sleep(time.Second * 2)
	var err error

	msgWithEventID, err = findMessageByBsonID(db, m.ID)

	if err != nil {
		t.Errorf("sendMessageWithInlineKB error on msg sent = %v", err)
	}
	return msgWithEventID
}

func TestContext_EditPressedMessageText(t *testing.T) {
	chatID, _ := strconv.ParseInt(os.Getenv("INTEGRAM_TEST_USER"), 10, 64)
	msg := sendMessageWithInlineKB(t)

	type fields struct {
		ServiceName           string
		ServiceBaseURL        url.URL
		db                    *mgo.Database
		gin                   *gin.Context
		User                  User
		Chat                  Chat
		Message               *IncomingMessage
		InlineQuery           *tg.InlineQuery
		ChosenInlineResult    *chosenInlineResult
		Callback              *callback
		inlineQueryAnsweredAt *time.Time
	}
	type args struct {
		text string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{"test1", fields{ServiceName: "servicewithbottoken", db: db, User: User{ID: chatID}, Chat: Chat{ID: chatID}, Callback: &callback{ID: "random", Message: msg.om, Data: "Data1"}}, args{"EditPressedMessageText"}, false},
	}
	for _, tt := range tests {
		c := &Context{
			ServiceName:           tt.fields.ServiceName,
			ServiceBaseURL:        tt.fields.ServiceBaseURL,
			db:                    tt.fields.db,
			gin:                   tt.fields.gin,
			User:                  tt.fields.User,
			Chat:                  tt.fields.Chat,
			Message:               tt.fields.Message,
			InlineQuery:           tt.fields.InlineQuery,
			ChosenInlineResult:    tt.fields.ChosenInlineResult,
			Callback:              tt.fields.Callback,
			inlineQueryAnsweredAt: tt.fields.inlineQueryAnsweredAt,
		}
		if err := c.EditPressedMessageText(tt.args.text); (err != nil) != tt.wantErr {
			t.Errorf("%q. Context.EditPressedMessageText() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
		time.Sleep(time.Millisecond * 1000)

		msg, _ := findMessageByBsonID(db, msg.ID)
		textHash:=fmt.Sprintf(fmt.Sprintf("%x", md5.Sum([]byte(tt.args.text))))
		if msg.om.TextHash != textHash {
			t.Errorf("%q. Context.EditPressedMessageText() db check got text hash = %s, want %s", tt.name, msg.om.TextHash, textHash)
		}
	}
}

func TestContext_EditPressedMessageTextAndInlineKeyboard(t *testing.T) {
	chatID, _ := strconv.ParseInt(os.Getenv("INTEGRAM_TEST_USER"), 10, 64)
	msg := sendMessageWithInlineKB(t)

	type fields struct {
		ServiceName           string
		ServiceBaseURL        url.URL
		db                    *mgo.Database
		gin                   *gin.Context
		User                  User
		Chat                  Chat
		Message               *IncomingMessage
		InlineQuery           *tg.InlineQuery
		ChosenInlineResult    *chosenInlineResult
		Callback              *callback
		inlineQueryAnsweredAt *time.Time
	}
	type args struct {
		text string
		kb   InlineKeyboard
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{"test1", fields{ServiceName: "servicewithbottoken", db: db, User: User{ID: chatID}, Chat: Chat{ID: chatID}, Callback: &callback{ID: "random", Message: msg.om, Data: "Data1"}}, args{"EditPressedMessageTextAndInlineKeyboard", InlineKeyboard{Buttons: []InlineButtons{{InlineButton{Text: "Key1", Data: "Data1"}}, {InlineButton{Text: "Key2", Data: "Data2"}}, {InlineButton{Text: "Key3", Data: "Data3"}}}}}, false},
	}
	for _, tt := range tests {
		c := &Context{
			ServiceName:           tt.fields.ServiceName,
			ServiceBaseURL:        tt.fields.ServiceBaseURL,
			db:                    tt.fields.db,
			gin:                   tt.fields.gin,
			User:                  tt.fields.User,
			Chat:                  tt.fields.Chat,
			Message:               tt.fields.Message,
			InlineQuery:           tt.fields.InlineQuery,
			ChosenInlineResult:    tt.fields.ChosenInlineResult,
			Callback:              tt.fields.Callback,
			inlineQueryAnsweredAt: tt.fields.inlineQueryAnsweredAt,
		}
		if err := c.EditPressedMessageTextAndInlineKeyboard(tt.args.text, tt.args.kb); (err != nil) != tt.wantErr {
			t.Errorf("%q. Context.EditPressedMessageTextAndInlineKeyboard() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}

		msg, _ := findMessageByBsonID(db, msg.ID)

		textHash := fmt.Sprintf("%x", md5.Sum([]byte(tt.args.text)))
		if msg.om.TextHash != textHash {
			t.Errorf("%q. Context.EditPressedMessageTextAndInlineKeyboard() db check got TextHash = %s, want %s", tt.name, msg.om.TextHash, textHash)
		}

		if !reflect.DeepEqual(msg.om.InlineKeyboardMarkup, tt.args.kb) {
			t.Errorf("%q. Context.EditPressedMessageTextAndInlineKeyboard() db check got kb = %v, want %v", tt.name, msg.om.InlineKeyboardMarkup, tt.args.kb)
		}
	}
}

func TestContext_EditPressedInlineKeyboard(t *testing.T) {
	chatID, _ := strconv.ParseInt(os.Getenv("INTEGRAM_TEST_USER"), 10, 64)
	msg := sendMessageWithInlineKB(t)

	type fields struct {
		ServiceName           string
		ServiceBaseURL        url.URL
		db                    *mgo.Database
		gin                   *gin.Context
		User                  User
		Chat                  Chat
		Message               *IncomingMessage
		InlineQuery           *tg.InlineQuery
		ChosenInlineResult    *chosenInlineResult
		Callback              *callback
		inlineQueryAnsweredAt *time.Time
	}
	type args struct {
		kb InlineKeyboard
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{"test1", fields{ServiceName: "servicewithbottoken", db: db, User: User{ID: chatID}, Chat: Chat{ID: chatID}, Callback: &callback{ID: "random", Message: msg.om, Data: "Data1"}}, args{InlineKeyboard{State: "kbstateval", Buttons: []InlineButtons{{InlineButton{Text: "Key1", Data: "Data1"}}, {InlineButton{Text: "Key2", Data: "Data2"}}, {InlineButton{Text: "Key3", Data: "Data3"}, InlineButton{Text: "Key4", Data: "Data4"}}}}}, false},
	}
	for _, tt := range tests {
		c := &Context{
			ServiceName:           tt.fields.ServiceName,
			ServiceBaseURL:        tt.fields.ServiceBaseURL,
			db:                    tt.fields.db,
			gin:                   tt.fields.gin,
			User:                  tt.fields.User,
			Chat:                  tt.fields.Chat,
			Message:               tt.fields.Message,
			InlineQuery:           tt.fields.InlineQuery,
			ChosenInlineResult:    tt.fields.ChosenInlineResult,
			Callback:              tt.fields.Callback,
			inlineQueryAnsweredAt: tt.fields.inlineQueryAnsweredAt,
		}
		if err := c.EditPressedInlineKeyboard(tt.args.kb); (err != nil) != tt.wantErr {
			t.Errorf("%q. Context.EditPressedInlineKeyboard() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}

		msg, _ := findMessageByBsonID(db, msg.ID)

		if !reflect.DeepEqual(msg.om.InlineKeyboardMarkup, tt.args.kb) {
			t.Errorf("%q. Context.EditPressedInlineKeyboard() db check got kb = %v, want %v", tt.name, msg.om.InlineKeyboardMarkup, tt.args.kb)
		}
	}
}

func TestContext_EditPressedInlineButton(t *testing.T) {
	chatID, _ := strconv.ParseInt(os.Getenv("INTEGRAM_TEST_USER"), 10, 64)
	msg := sendMessageWithInlineKB(t)

	type fields struct {
		ServiceName           string
		ServiceBaseURL        url.URL
		db                    *mgo.Database
		gin                   *gin.Context
		User                  User
		Chat                  Chat
		Message               *IncomingMessage
		InlineQuery           *tg.InlineQuery
		ChosenInlineResult    *chosenInlineResult
		Callback              *callback
		inlineQueryAnsweredAt *time.Time
	}
	type args struct {
		newState int
		newText  string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{"test1", fields{ServiceName: "servicewithbottoken", db: db, User: User{ID: chatID}, Chat: Chat{ID: chatID}, Callback: &callback{ID: "random", Message: msg.om, Data: "Data2"}}, args{1, "EditPressedInlineButton"}, false},
	}
	for _, tt := range tests {
		c := &Context{
			ServiceName:           tt.fields.ServiceName,
			ServiceBaseURL:        tt.fields.ServiceBaseURL,
			db:                    tt.fields.db,
			gin:                   tt.fields.gin,
			User:                  tt.fields.User,
			Chat:                  tt.fields.Chat,
			Message:               tt.fields.Message,
			InlineQuery:           tt.fields.InlineQuery,
			ChosenInlineResult:    tt.fields.ChosenInlineResult,
			Callback:              tt.fields.Callback,
			inlineQueryAnsweredAt: tt.fields.inlineQueryAnsweredAt,
		}
		if err := c.EditPressedInlineButton(tt.args.newState, tt.args.newText); (err != nil) != tt.wantErr {
			t.Errorf("%q. Context.EditPressedInlineButton() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}

		msg, _ := findMessageByBsonID(db, msg.ID)
		_, _, but := msg.om.InlineKeyboardMarkup.Find(tt.fields.Callback.Data)

		if but.Text != tt.args.newText {
			t.Errorf("%q. Context.EditPressedInlineKeyboard() db check got btn text = %s, want %s", tt.name, but.Text, tt.args.newText)
		}

		if but.State != tt.args.newState {
			t.Errorf("%q. Context.EditPressedInlineKeyboard() db check got btn state = %s, want %s", tt.name, but.State, tt.args.newState)
		}
	}
}

func TestContext_EditMessageText(t *testing.T) {
	chatID, _ := strconv.ParseInt(os.Getenv("INTEGRAM_TEST_USER"), 10, 64)
	msg := sendMessageWithInlineKB(t)

	type fields struct {
		ServiceName           string
		ServiceBaseURL        url.URL
		db                    *mgo.Database
		gin                   *gin.Context
		User                  User
		Chat                  Chat
		Message               *IncomingMessage
		InlineQuery           *tg.InlineQuery
		ChosenInlineResult    *chosenInlineResult
		Callback              *callback
		inlineQueryAnsweredAt *time.Time
	}
	type args struct {
		om   *OutgoingMessage
		text string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{"test1", fields{ServiceName: "servicewithbottoken", db: db, User: User{ID: chatID}, Chat: Chat{ID: chatID}, Callback: &callback{ID: "random", Message: msg.om, Data: "Data2"}}, args{msg.om, "EditMessageText"}, false},
	}
	for _, tt := range tests {
		c := &Context{
			ServiceName:           tt.fields.ServiceName,
			ServiceBaseURL:        tt.fields.ServiceBaseURL,
			db:                    tt.fields.db,
			gin:                   tt.fields.gin,
			User:                  tt.fields.User,
			Chat:                  tt.fields.Chat,
			Message:               tt.fields.Message,
			InlineQuery:           tt.fields.InlineQuery,
			ChosenInlineResult:    tt.fields.ChosenInlineResult,
			Callback:              tt.fields.Callback,
			inlineQueryAnsweredAt: tt.fields.inlineQueryAnsweredAt,
		}
		if err := c.EditMessageText(tt.args.om, tt.args.text); (err != nil) != tt.wantErr {
			t.Errorf("%q. Context.EditMessageText() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
		time.Sleep(time.Millisecond * 100)
		msg, _ := findMessageByBsonID(db, msg.ID)
		textHash:=fmt.Sprintf(fmt.Sprintf("%x", md5.Sum([]byte(tt.args.text))))

		if msg.om.TextHash !=  textHash {
			t.Errorf("%q. Context.EditMessageText() db check got text hash = %s, want %s", tt.name, msg.om.TextHash, textHash)
		}

	}
}

func TestContext_EditMessagesTextWithEventID(t *testing.T) {
	chatID, _ := strconv.ParseInt(os.Getenv("INTEGRAM_TEST_USER"), 10, 64)
	msg := sendMessageWithInlineKB(t)

	sendMessageWithEventID(t, msg.EventID[0])

	type fields struct {
		ServiceName           string
		ServiceBaseURL        url.URL
		db                    *mgo.Database
		gin                   *gin.Context
		User                  User
		Chat                  Chat
		Message               *IncomingMessage
		InlineQuery           *tg.InlineQuery
		ChosenInlineResult    *chosenInlineResult
		Callback              *callback
		inlineQueryAnsweredAt *time.Time
	}
	type args struct {
		eventID string
		text    string
	}
	tests := []struct {
		name       string
		fields     fields
		args       args
		wantEdited int
		wantErr    bool
	}{
		{"test1", fields{ServiceName: "servicewithbottoken", db: db, User: User{ID: chatID}, Chat: Chat{ID: chatID}}, args{msg.EventID[0], fmt.Sprintf("EditMessagesTextWithEventID: edited msg with event id <b>%s</b>", msg.EventID[0])}, 2, false},
	}
	for _, tt := range tests {
		c := &Context{
			ServiceName:           tt.fields.ServiceName,
			ServiceBaseURL:        tt.fields.ServiceBaseURL,
			db:                    tt.fields.db,
			gin:                   tt.fields.gin,
			User:                  tt.fields.User,
			Chat:                  tt.fields.Chat,
			Message:               tt.fields.Message,
			InlineQuery:           tt.fields.InlineQuery,
			ChosenInlineResult:    tt.fields.ChosenInlineResult,
			Callback:              tt.fields.Callback,
			inlineQueryAnsweredAt: tt.fields.inlineQueryAnsweredAt,
		}
		gotEdited, err := c.EditMessagesTextWithEventID(tt.args.eventID, tt.args.text)

		if (err != nil) != tt.wantErr {
			t.Errorf("%q. Context.EditMessagesTextWithEventID() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}

		if gotEdited != tt.wantEdited {
			t.Errorf("%q. Context.EditMessagesTextWithEventID() = %v, want %v", tt.name, gotEdited, tt.wantEdited)
		}
	}
}

func TestContext_EditMessagesWithEventID(t *testing.T) {
	chatID, _ := strconv.ParseInt(os.Getenv("INTEGRAM_TEST_USER"), 10, 64)
	msg := sendMessageWithInlineKB(t)

	sendMessageWithEventID(t, msg.EventID[0])

	type fields struct {
		ServiceName           string
		ServiceBaseURL        url.URL
		db                    *mgo.Database
		gin                   *gin.Context
		User                  User
		Chat                  Chat
		Message               *IncomingMessage
		InlineQuery           *tg.InlineQuery
		ChosenInlineResult    *chosenInlineResult
		Callback              *callback
		inlineQueryAnsweredAt *time.Time
	}
	type args struct {
		eventID   string
		fromState string
		text      string
		kb        InlineKeyboard
	}
	tests := []struct {
		name       string
		fields     fields
		args       args
		wantEdited int
		wantErr    bool
	}{
		{"test1", fields{ServiceName: "servicewithbottoken", db: db, User: User{ID: chatID}, Chat: Chat{ID: chatID}}, args{msg.EventID[0], "kbstateval", fmt.Sprintf("EditMessagesTextWithEventID: edited msg with event id <b>%s</b> 1", msg.EventID[0]), msg.om.InlineKeyboardMarkup}, 1, false},
		{"test2", fields{ServiceName: "servicewithbottoken", db: db, User: User{ID: chatID}, Chat: Chat{ID: chatID}}, args{msg.EventID[0], "",           fmt.Sprintf("EditMessagesTextWithEventID: edited msg with event id <b>%s</b> 2", msg.EventID[0]), msg.om.InlineKeyboardMarkup}, 2, false},
	}
	for _, tt := range tests {
		c := &Context{
			ServiceName:           tt.fields.ServiceName,
			ServiceBaseURL:        tt.fields.ServiceBaseURL,
			db:                    tt.fields.db,
			gin:                   tt.fields.gin,
			User:                  tt.fields.User,
			Chat:                  tt.fields.Chat,
			Message:               tt.fields.Message,
			InlineQuery:           tt.fields.InlineQuery,
			ChosenInlineResult:    tt.fields.ChosenInlineResult,
			Callback:              tt.fields.Callback,
			inlineQueryAnsweredAt: tt.fields.inlineQueryAnsweredAt,
		}
		gotEdited, err := c.EditMessagesWithEventID(tt.args.eventID, tt.args.fromState, tt.args.text, tt.args.kb)
		if (err != nil) != tt.wantErr {
			t.Errorf("%q. Context.EditMessagesWithEventID() error = %v, wantErr %v", tt.name, err, tt.wantErr)
			continue
		}
		if gotEdited != tt.wantEdited {
			t.Errorf("%q. Context.EditMessagesWithEventID() = %v, want %v", tt.name, gotEdited, tt.wantEdited)
		}
	}
}

func TestContext_EditMessageTextAndInlineKeyboard(t *testing.T) {
	chatID, _ := strconv.ParseInt(os.Getenv("INTEGRAM_TEST_USER"), 10, 64)
	msg := sendMessageWithInlineKB(t)

	type fields struct {
		ServiceName           string
		ServiceBaseURL        url.URL
		db                    *mgo.Database
		gin                   *gin.Context
		User                  User
		Chat                  Chat
		Message               *IncomingMessage
		InlineQuery           *tg.InlineQuery
		ChosenInlineResult    *chosenInlineResult
		Callback              *callback
		inlineQueryAnsweredAt *time.Time
	}
	type args struct {
		om        *OutgoingMessage
		fromState string
		text      string
		kb        InlineKeyboard
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{"test1", fields{ServiceName: "servicewithbottoken", db: db, User: User{ID: chatID}, Chat: Chat{ID: chatID}}, args{msg.om, "", "EditMessageTextAndInlineKeyboard", msg.om.InlineKeyboardMarkup}, false},
	}
	for _, tt := range tests {
		c := &Context{
			ServiceName:           tt.fields.ServiceName,
			ServiceBaseURL:        tt.fields.ServiceBaseURL,
			db:                    tt.fields.db,
			gin:                   tt.fields.gin,
			User:                  tt.fields.User,
			Chat:                  tt.fields.Chat,
			Message:               tt.fields.Message,
			InlineQuery:           tt.fields.InlineQuery,
			ChosenInlineResult:    tt.fields.ChosenInlineResult,
			Callback:              tt.fields.Callback,
			inlineQueryAnsweredAt: tt.fields.inlineQueryAnsweredAt,
		}
		if err := c.EditMessageTextAndInlineKeyboard(tt.args.om, tt.args.fromState, tt.args.text, tt.args.kb); (err != nil) != tt.wantErr {
			t.Errorf("%q. Context.EditMessageTextAndInlineKeyboard() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
	}
}

func TestContext_EditInlineKeyboard(t *testing.T) {
	chatID, _ := strconv.ParseInt(os.Getenv("INTEGRAM_TEST_USER"), 10, 64)
	msg := sendMessageWithInlineKB(t)

	type fields struct {
		ServiceName           string
		ServiceBaseURL        url.URL
		db                    *mgo.Database
		gin                   *gin.Context
		User                  User
		Chat                  Chat
		Message               *IncomingMessage
		InlineQuery           *tg.InlineQuery
		ChosenInlineResult    *chosenInlineResult
		Callback              *callback
		inlineQueryAnsweredAt *time.Time
	}
	type args struct {
		om        *OutgoingMessage
		fromState string
		kb        InlineKeyboard
	}
	msg.om.InlineKeyboardMarkup.EditText("Data1", "EditInlineKeyboard")

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{"test1", fields{ServiceName: "servicewithbottoken", db: db, User: User{ID: chatID}, Chat: Chat{ID: chatID}}, args{msg.om, "kbstateval", msg.om.InlineKeyboardMarkup}, false},
	}
	for _, tt := range tests {
		c := &Context{
			ServiceName:           tt.fields.ServiceName,
			ServiceBaseURL:        tt.fields.ServiceBaseURL,
			db:                    tt.fields.db,
			gin:                   tt.fields.gin,
			User:                  tt.fields.User,
			Chat:                  tt.fields.Chat,
			Message:               tt.fields.Message,
			InlineQuery:           tt.fields.InlineQuery,
			ChosenInlineResult:    tt.fields.ChosenInlineResult,
			Callback:              tt.fields.Callback,
			inlineQueryAnsweredAt: tt.fields.inlineQueryAnsweredAt,
		}
		if err := c.EditInlineKeyboard(tt.args.om, tt.args.fromState, tt.args.kb); (err != nil) != tt.wantErr {
			t.Errorf("%q. Context.EditInlineKeyboard() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
	}
}

func TestContext_EditInlineButton(t *testing.T) {
	chatID, _ := strconv.ParseInt(os.Getenv("INTEGRAM_TEST_USER"), 10, 64)
	msg := sendMessageWithInlineKB(t)

	type fields struct {
		ServiceName           string
		ServiceBaseURL        url.URL
		db                    *mgo.Database
		gin                   *gin.Context
		User                  User
		Chat                  Chat
		Message               *IncomingMessage
		InlineQuery           *tg.InlineQuery
		ChosenInlineResult    *chosenInlineResult
		Callback              *callback
		inlineQueryAnsweredAt *time.Time
	}
	type args struct {
		om            *OutgoingMessage
		kbState       string
		buttonData    string
		newButtonText string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{"test1", fields{ServiceName: "servicewithbottoken", db: db, User: User{ID: chatID}, Chat: Chat{ID: chatID}}, args{msg.om, "kbstateval", "Data4", "EditInlineButton"}, false},
	}
	for _, tt := range tests {
		c := &Context{
			ServiceName:           tt.fields.ServiceName,
			ServiceBaseURL:        tt.fields.ServiceBaseURL,
			db:                    tt.fields.db,
			gin:                   tt.fields.gin,
			User:                  tt.fields.User,
			Chat:                  tt.fields.Chat,
			Message:               tt.fields.Message,
			InlineQuery:           tt.fields.InlineQuery,
			ChosenInlineResult:    tt.fields.ChosenInlineResult,
			Callback:              tt.fields.Callback,
			inlineQueryAnsweredAt: tt.fields.inlineQueryAnsweredAt,
		}
		if err := c.EditInlineButton(tt.args.om, tt.args.kbState, tt.args.buttonData, tt.args.newButtonText); (err != nil) != tt.wantErr {
			t.Errorf("%q. Context.EditInlineButton() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
	}
}

func TestContext_EditInlineStateButton(t *testing.T) {
	chatID, _ := strconv.ParseInt(os.Getenv("INTEGRAM_TEST_USER"), 10, 64)
	msg := sendMessageWithInlineKB(t)

	type fields struct {
		ServiceName           string
		ServiceBaseURL        url.URL
		db                    *mgo.Database
		gin                   *gin.Context
		User                  User
		Chat                  Chat
		Message               *IncomingMessage
		InlineQuery           *tg.InlineQuery
		ChosenInlineResult    *chosenInlineResult
		Callback              *callback
		inlineQueryAnsweredAt *time.Time
	}
	type args struct {
		om             *OutgoingMessage
		kbState        string
		oldButtonState int
		buttonData     string
		newButtonState int
		newButtonText  string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{"test1", fields{ServiceName: "servicewithbottoken", db: db, User: User{ID: chatID}, Chat: Chat{ID: chatID}}, args{msg.om, "kbstateval", 0, "Data3", 1, "EditInlineStateButton"}, false},
	}
	for _, tt := range tests {
		c := &Context{
			ServiceName:           tt.fields.ServiceName,
			ServiceBaseURL:        tt.fields.ServiceBaseURL,
			db:                    tt.fields.db,
			gin:                   tt.fields.gin,
			User:                  tt.fields.User,
			Chat:                  tt.fields.Chat,
			Message:               tt.fields.Message,
			InlineQuery:           tt.fields.InlineQuery,
			ChosenInlineResult:    tt.fields.ChosenInlineResult,
			Callback:              tt.fields.Callback,
			inlineQueryAnsweredAt: tt.fields.inlineQueryAnsweredAt,
		}
		if err := c.EditInlineStateButton(tt.args.om, tt.args.kbState, tt.args.oldButtonState, tt.args.buttonData, tt.args.newButtonState, tt.args.newButtonText); (err != nil) != tt.wantErr {
			t.Errorf("%q. Context.EditInlineStateButton() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
	}
}

func TestContext_AnswerInlineQueryWithResults(t *testing.T) {
	type fields struct {
		ServiceName           string
		ServiceBaseURL        url.URL
		db                    *mgo.Database
		gin                   *gin.Context
		User                  User
		Chat                  Chat
		Message               *IncomingMessage
		InlineQuery           *tg.InlineQuery
		ChosenInlineResult    *chosenInlineResult
		Callback              *callback
		inlineQueryAnsweredAt *time.Time
	}
	type args struct {
		res        []interface{}
		cacheTime  int
		isPersonal bool
		nextOffset string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr string
	}{
		{"test1", fields{ServiceName: "servicewithbottoken", db: db, User: User{ID: 9999999999}, Chat: Chat{ID: 9999999999}, InlineQuery: &tg.InlineQuery{ID: "fakeid"}}, args{[]interface{}{tg.NewInlineQueryResultArticle("id", "title", "text")}, 0, true, ""}, "TG returned 400: Bad Request: QUERY_ID_INVALID"},
	}
	for _, tt := range tests {
		c := &Context{
			ServiceName:           tt.fields.ServiceName,
			ServiceBaseURL:        tt.fields.ServiceBaseURL,
			db:                    tt.fields.db,
			gin:                   tt.fields.gin,
			User:                  tt.fields.User,
			Chat:                  tt.fields.Chat,
			Message:               tt.fields.Message,
			InlineQuery:           tt.fields.InlineQuery,
			ChosenInlineResult:    tt.fields.ChosenInlineResult,
			Callback:              tt.fields.Callback,
			inlineQueryAnsweredAt: tt.fields.inlineQueryAnsweredAt,
		}
		if err := c.AnswerInlineQueryWithResults(tt.args.res, tt.args.cacheTime, tt.args.isPersonal, tt.args.nextOffset); err == nil || err.Error() != tt.wantErr {
			t.Errorf("%q. Context.AnswerInlineQueryWithResults() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
	}
}

func TestContext_AnswerInlineQueryWithPM(t *testing.T) {
	type fields struct {
		ServiceName           string
		ServiceBaseURL        url.URL
		db                    *mgo.Database
		gin                   *gin.Context
		User                  User
		Chat                  Chat
		Message               *IncomingMessage
		InlineQuery           *tg.InlineQuery
		ChosenInlineResult    *chosenInlineResult
		Callback              *callback
		inlineQueryAnsweredAt *time.Time
	}
	type args struct {
		text      string
		parameter string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr string
	}{
		{"test1", fields{ServiceName: "servicewithbottoken", db: db, User: User{ID: 9999999999}, Chat: Chat{ID: 9999999999}, InlineQuery: &tg.InlineQuery{ID: "fakeid"}}, args{"text", "param"}, "TG returned 400: Bad Request: QUERY_ID_INVALID"},
	}
	for _, tt := range tests {
		c := &Context{
			ServiceName:           tt.fields.ServiceName,
			ServiceBaseURL:        tt.fields.ServiceBaseURL,
			db:                    tt.fields.db,
			gin:                   tt.fields.gin,
			User:                  tt.fields.User,
			Chat:                  tt.fields.Chat,
			Message:               tt.fields.Message,
			InlineQuery:           tt.fields.InlineQuery,
			ChosenInlineResult:    tt.fields.ChosenInlineResult,
			Callback:              tt.fields.Callback,
			inlineQueryAnsweredAt: tt.fields.inlineQueryAnsweredAt,
		}
		if err := c.AnswerInlineQueryWithPM(tt.args.text, tt.args.parameter); err == nil || err.Error() != tt.wantErr {
			t.Errorf("%q. Context.AnswerInlineQueryWithPM() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
	}
}

func TestContext_AnswerCallbackQuery(t *testing.T) {
	chatID, _ := strconv.ParseInt(os.Getenv("INTEGRAM_TEST_USER"), 10, 64)
	msg := sendMessageWithInlineKB(t)

	type fields struct {
		ServiceName           string
		ServiceBaseURL        url.URL
		db                    *mgo.Database
		gin                   *gin.Context
		User                  User
		Chat                  Chat
		Message               *IncomingMessage
		InlineQuery           *tg.InlineQuery
		ChosenInlineResult    *chosenInlineResult
		Callback              *callback
		inlineQueryAnsweredAt *time.Time
	}
	type args struct {
		text      string
		showAlert bool
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr string
	}{
		{"test1", fields{ServiceName: "servicewithbottoken", db: db, User: User{ID: chatID}, Chat: Chat{ID: chatID}, Callback: &callback{ID: "fakeid", Message: msg.om, Data: "Data1"}}, args{"text", false}, "TG returned 400: Bad Request: QUERY_ID_INVALID"},
	}
	for _, tt := range tests {
		c := &Context{
			ServiceName:           tt.fields.ServiceName,
			ServiceBaseURL:        tt.fields.ServiceBaseURL,
			db:                    tt.fields.db,
			gin:                   tt.fields.gin,
			User:                  tt.fields.User,
			Chat:                  tt.fields.Chat,
			Message:               tt.fields.Message,
			InlineQuery:           tt.fields.InlineQuery,
			ChosenInlineResult:    tt.fields.ChosenInlineResult,
			Callback:              tt.fields.Callback,
			inlineQueryAnsweredAt: tt.fields.inlineQueryAnsweredAt,
		}
		if err := c.AnswerCallbackQuery(tt.args.text, tt.args.showAlert); (err == nil) || err.Error() != tt.wantErr {
			t.Errorf("%q. Context.AnswerCallbackQuery() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
	}
}

func TestContext_NewMessage(t *testing.T) {
	bt := strings.Split(os.Getenv("INTEGRAM_TEST_BOT_TOKEN"), ":")
	botID, _ := strconv.ParseInt(bt[0], 10, 64)

	type fields struct {
		ServiceName           string
		ServiceBaseURL        url.URL
		db                    *mgo.Database
		gin                   *gin.Context
		User                  User
		Chat                  Chat
		Message               *IncomingMessage
		InlineQuery           *tg.InlineQuery
		ChosenInlineResult    *chosenInlineResult
		Callback              *callback
		inlineQueryAnsweredAt *time.Time
	}
	tests := []struct {
		name   string
		fields fields
		want   *OutgoingMessage
	}{
		{"test1", fields{ServiceName: "servicewithbottoken", User: User{ID: 9999999999}, Chat: Chat{ID: -9999999999}}, &OutgoingMessage{Message: Message{BotID: botID, FromID: botID, ChatID: -9999999999}, WebPreview: true}},
		{"test2", fields{ServiceName: "servicewithbottoken", User: User{ID: 9999999999}}, &OutgoingMessage{Message: Message{BotID: botID, FromID: botID, ChatID: 9999999999}, WebPreview: true}},
	}
	for _, tt := range tests {
		c := &Context{
			ServiceName:           tt.fields.ServiceName,
			ServiceBaseURL:        tt.fields.ServiceBaseURL,
			db:                    tt.fields.db,
			gin:                   tt.fields.gin,
			User:                  tt.fields.User,
			Chat:                  tt.fields.Chat,
			Message:               tt.fields.Message,
			InlineQuery:           tt.fields.InlineQuery,
			ChosenInlineResult:    tt.fields.ChosenInlineResult,
			Callback:              tt.fields.Callback,
			inlineQueryAnsweredAt: tt.fields.inlineQueryAnsweredAt,
		}

		tt.want.ctx = c
		if got := c.NewMessage(); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. Context.NewMessage() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestContext_SendAction(t *testing.T) {
	chatID, _ := strconv.ParseInt(os.Getenv("INTEGRAM_TEST_USER"), 10, 64)

	type fields struct {
		ServiceName           string
		ServiceBaseURL        url.URL
		db                    *mgo.Database
		gin                   *gin.Context
		User                  User
		Chat                  Chat
		Message               *IncomingMessage
		InlineQuery           *tg.InlineQuery
		ChosenInlineResult    *chosenInlineResult
		Callback              *callback
		inlineQueryAnsweredAt *time.Time
	}
	type args struct {
		s string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{"test1", fields{ServiceName: "servicewithbottoken", User: User{ID: chatID}, Chat: Chat{ID: chatID}}, args{"typing"}, false},
	}
	for _, tt := range tests {
		c := &Context{
			ServiceName:           tt.fields.ServiceName,
			ServiceBaseURL:        tt.fields.ServiceBaseURL,
			db:                    tt.fields.db,
			gin:                   tt.fields.gin,
			User:                  tt.fields.User,
			Chat:                  tt.fields.Chat,
			Message:               tt.fields.Message,
			InlineQuery:           tt.fields.InlineQuery,
			ChosenInlineResult:    tt.fields.ChosenInlineResult,
			Callback:              tt.fields.Callback,
			inlineQueryAnsweredAt: tt.fields.inlineQueryAnsweredAt,
		}
		if err := c.SendAction(tt.args.s); (err != nil) != tt.wantErr {
			t.Errorf("%q. Context.SendAction() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
	}
}

func TestContext_DownloadURL(t *testing.T) {
	type fields struct {
		ServiceName           string
		ServiceBaseURL        url.URL
		db                    *mgo.Database
		gin                   *gin.Context
		User                  User
		Chat                  Chat
		Message               *IncomingMessage
		InlineQuery           *tg.InlineQuery
		ChosenInlineResult    *chosenInlineResult
		Callback              *callback
		inlineQueryAnsweredAt *time.Time
	}
	type args struct {
		url string
	}
	tests := []struct {
		name         string
		fields       fields
		args         args
		wantFileSha1 string
		wantErr      bool
	}{
		{"good URL", fields{ServiceName: "servicewithbottoken", User: User{ID: 9999999999}, Chat: Chat{ID: -9999999999}}, args{"https://raw.githubusercontent.com/Requilence/integram/master/LICENSE"}, "fe3eea6c599e23a00c08c5f5cb2320c30adc8f8687db5fcec9b79a662c53ff6b", false},
		{"bad URL", fields{ServiceName: "servicewithbottoken", User: User{ID: 9999999999}, Chat: Chat{ID: -9999999999}}, args{"http://integram.org/bad"}, "", true},
	}
	for _, tt := range tests {
		c := &Context{
			ServiceName:           tt.fields.ServiceName,
			ServiceBaseURL:        tt.fields.ServiceBaseURL,
			db:                    tt.fields.db,
			gin:                   tt.fields.gin,
			User:                  tt.fields.User,
			Chat:                  tt.fields.Chat,
			Message:               tt.fields.Message,
			InlineQuery:           tt.fields.InlineQuery,
			ChosenInlineResult:    tt.fields.ChosenInlineResult,
			Callback:              tt.fields.Callback,
			inlineQueryAnsweredAt: tt.fields.inlineQueryAnsweredAt,
		}
		gotFilePath, err := c.DownloadURL(tt.args.url)
		if (err != nil) != tt.wantErr {
			t.Errorf("%q. Context.DownloadURL() error = %v, wantErr %v", tt.name, err, tt.wantErr)
			continue
		}
		if tt.wantErr {
			continue
		}
		hasher := sha256.New()
		fmt.Println(gotFilePath)

		f, err := os.Open(gotFilePath)
		if err != nil {
			t.Errorf("%q. Context.DownloadURL() file open error = %v", tt.name, err)
			continue
		}

		defer f.Close()
		if _, err := io.Copy(hasher, f); err != nil {
			t.Errorf("%q. Context.DownloadURL() sha1 error = %v", tt.name, err)
		}

		got := hex.EncodeToString(hasher.Sum(nil))

		if got != tt.wantFileSha1 {
			t.Errorf("%q. Context.DownloadURL() sha1 got %v, want %v", tt.name, got, tt.wantFileSha1)
		}
	}
}

func TestWebhookContext_RAW(t *testing.T) {
	type fields struct {
		gin        *gin.Context
		body       []byte
		firstParse bool
		requestID  string
	}
	r, _ := http.NewRequest("GET", "https://integram.org/uGs32432novfdc", bytes.NewReader([]byte("test")))
	want := []byte("test")

	tests := []struct {
		name    string
		fields  fields
		want    *[]byte
		wantErr bool
	}{
		{"test1", fields{gin: &gin.Context{Request: r}}, &want, false},
	}
	for _, tt := range tests {
		wc := &WebhookContext{
			gin:        tt.fields.gin,
			body:       tt.fields.body,
			firstParse: tt.fields.firstParse,
			requestID:  tt.fields.requestID,
		}
		got, err := wc.RAW()
		if (err != nil) != tt.wantErr {
			t.Errorf("%q. WebhookContext.RAW() error = %v, wantErr %v", tt.name, err, tt.wantErr)
			continue
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. WebhookContext.RAW() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestWebhookContext_JSON(t *testing.T) {
	type fields struct {
		gin        *gin.Context
		body       []byte
		firstParse bool
		requestID  string
	}
	type args struct {
		out interface{}
	}

	r1, _ := http.NewRequest("POST", "https://integram.org/uGs32432novfdc", bytes.NewReader([]byte("{\"status\": \"ok\"}")))
	r2, _ := http.NewRequest("POST", "https://integram.org/uGs32432novfdc", bytes.NewReader([]byte("{\"status\": \"ok\"}")))
	r3, _ := http.NewRequest("POST", "https://integram.org/uGs32432novfdc", bytes.NewReader([]byte("status\": \"ok\"}")))

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{"good json", fields{gin: &gin.Context{Request: r1}}, args{&struct{ Status string }{}}, false},
		{"can't marshal", fields{gin: &gin.Context{Request: r2}}, args{&struct{ Status int }{}}, true},
		{"bad json", fields{gin: &gin.Context{Request: r3}}, args{&struct{ Status string }{}}, true},
	}
	for _, tt := range tests {
		wc := &WebhookContext{
			gin:        tt.fields.gin,
			body:       tt.fields.body,
			firstParse: tt.fields.firstParse,
			requestID:  tt.fields.requestID,
		}
		if err := wc.JSON(tt.args.out); (err != nil) != tt.wantErr {
			t.Errorf("%q. WebhookContext.JSON() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
	}
}

func TestWebhookContext_Form(t *testing.T) {
	type fields struct {
		gin        *gin.Context
		body       []byte
		firstParse bool
		requestID  string
	}
	r1, _ := http.NewRequest("POST", "https://integram.org/uGs32432novfdc", bytes.NewReader([]byte("key1=val1&key2=val2")))
	r1.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	gc := gin.Context{Request: r1}

	r2, _ := http.NewRequest("POST", "https://integram.org/uGs32432novfdc", bytes.NewReader([]byte("baddata")))

	tests := []struct {
		name   string
		fields fields
		want   uurl.Values
	}{
		{"good form", fields{gin: &gc}, uurl.Values{"key1": {"val1"}, "key2": {"val2"}}},
		{"good form - extract another time", fields{gin: &gc}, uurl.Values{"key1": {"val1"}, "key2": {"val2"}}},
		{"bad form", fields{gin: &gin.Context{Request: r2}}, uurl.Values{}},
	}
	for _, tt := range tests {
		wc := &WebhookContext{
			gin:        tt.fields.gin,
			body:       tt.fields.body,
			firstParse: tt.fields.firstParse,
			requestID:  tt.fields.requestID,
		}
		if got := wc.Form(); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. WebhookContext.Form() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestWebhookContext_FormValue(t *testing.T) {
	type fields struct {
		gin        *gin.Context
		body       []byte
		firstParse bool
		requestID  string
	}
	type args struct {
		key string
	}
	r1, _ := http.NewRequest("POST", "https://integram.org/uGs32432novfdc", bytes.NewReader([]byte("key1=val1&key2=val2")))
	bytes.NewReader([]byte("key1=val1&key2=val2"))
	r1.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	r2, _ := http.NewRequest("POST", "https://integram.org/uGs32432novfdc", bytes.NewReader([]byte("baddata")))

	tests := []struct {
		name   string
		fields fields
		args   args
		want   string
	}{
		{"good form - key not exists", fields{gin: &gin.Context{Request: r1}}, args{"key3"}, ""},
		{"good form - key not exists", fields{gin: &gin.Context{Request: r1}}, args{"key1"}, "val1"},
		{"bad form", fields{gin: &gin.Context{Request: r2}}, args{"key1"}, ""},
	}
	for _, tt := range tests {
		wc := &WebhookContext{
			gin:        tt.fields.gin,
			body:       tt.fields.body,
			firstParse: tt.fields.firstParse,
			requestID:  tt.fields.requestID,
		}
		if got := wc.FormValue(tt.args.key); got != tt.want {
			t.Errorf("%q. WebhookContext.FormValue() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestWebhookContext_HookID(t *testing.T) {
	type fields struct {
		gin        *gin.Context
		body       []byte
		firstParse bool
		requestID  string
	}
	r1, _ := http.NewRequest("POST", "https://integram.org/uGs32432novfdc", bytes.NewReader([]byte("{\"status\": \"ok\"}")))

	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{"good", fields{gin: &gin.Context{Request: r1, Params: gin.Params{{Key: "param", Value: "uGs32432novfdc"}}}}, "uGs32432novfdc"},
	}
	for _, tt := range tests {
		wc := &WebhookContext{
			gin:        tt.fields.gin,
			body:       tt.fields.body,
			firstParse: tt.fields.firstParse,
			requestID:  tt.fields.requestID,
		}
		if got := wc.HookID(); got != tt.want {
			t.Errorf("%q. WebhookContext.HookID() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestWebhookContext_RequestID(t *testing.T) {
	type fields struct {
		gin        *gin.Context
		body       []byte
		firstParse bool
		requestID  string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{"good", fields{requestID: "requestID"}, "requestID"},
	}
	for _, tt := range tests {
		wc := &WebhookContext{
			gin:        tt.fields.gin,
			body:       tt.fields.body,
			firstParse: tt.fields.firstParse,
			requestID:  tt.fields.requestID,
		}
		if got := wc.RequestID(); got != tt.want {
			t.Errorf("%q. WebhookContext.RequestID() = %v, want %v", tt.name, got, tt.want)
		}
	}
}
