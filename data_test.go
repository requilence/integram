package integram

import (
	"net/http"
	"reflect"
	"testing"
	"time"

	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/requilence/url"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	tg "github.com/requilence/telegram-bot-api"
	"strings"
	"os"
	"strconv"
)

func clearData() {
	db.C("users").RemoveId(9999999999)
	db.C("users").RemoveId(9999999998)
	db.C("users").RemoveId(9999999997)
	db.C("users").RemoveId(9999999996)
	db.C("users").RemoveId(9999999995)
	db.C("users_cache").RemoveAll(bson.M{"_id": 9999999999})
	db.C("chats_cache").RemoveAll(bson.M{"_id": -9999999999})
	db.C("chats").RemoveId(9999999999)
	db.C("chats").RemoveId(-9999999999)
}
func TestUser_Cache(t *testing.T) {

	type s struct {
		A int
		B bool
		C string
	}

	c := &Context{db: db, Chat: Chat{ID: -9999999999}, User: User{ID: 9999999999}, ServiceName: "servicewithoauth1"}
	c.User.ctx = c

	c.User.SetCache("UserKey01", s{1, true, "str"}, time.Minute)
	c.User.SetCache("userKey02", 123, time.Minute)

	sOut := s{}
	var sOut2 int
	int123 := 123
	type fields struct {
		ID        int64
		FirstName string
		LastName  string
		UserName  string
		Tz        string
		ctx       *Context
		data      *userData
	}
	type args struct {
		key string
		res interface{}
	}
	tests := []struct {
		name       string
		fields     fields
		args       args
		wantExists bool
		wantRes    interface{}
	}{
		{"non exists - struct", fields{ID: 9999999999, ctx: c}, args{"UserKey123", &sOut}, false, &s{}},
		{"exists - struct", fields{ID: 9999999999, ctx: c}, args{"UserKey01", &sOut}, true, &s{1, true, "str"}},
		{"exists - int", fields{ID: 9999999999, ctx: c}, args{"userkey02", &sOut2}, true, &int123},
	}
	for _, tt := range tests {
		user := &User{
			ID:        tt.fields.ID,
			FirstName: tt.fields.FirstName,
			LastName:  tt.fields.LastName,
			UserName:  tt.fields.UserName,
			Tz:        tt.fields.Tz,
			ctx:       tt.fields.ctx,
			data:      tt.fields.data,
		}
		if gotExists := user.Cache(tt.args.key, tt.args.res); gotExists != tt.wantExists {
			t.Errorf("%q. User.Cache() = %v, want %v", tt.name, gotExists, tt.wantExists)
		}

		if !reflect.DeepEqual(tt.args.res, tt.wantRes) {
			t.Errorf("%q. User.Cache() = %v, want %v", tt.name, tt.args.res, tt.wantRes)
		}
	}
}

func TestChat_Cache(t *testing.T) {
	type s struct {
		A int
		B bool
		C string
	}

	c := &Context{db: db, Chat: Chat{ID: -9999999999}, User: User{ID: 9999999999}, ServiceName: "servicewithoauth1"}
	c.User.ctx = c
	c.Chat.ctx = c

	c.Chat.SetCache("ChatKey01", s{1, true, "str"}, time.Minute)
	c.Chat.SetCache("ChatKey02", 123, time.Minute)

	sOut := s{}
	var sOut2 int
	int123 := 123

	type fields struct {
		ID        int64
		Type      string
		FirstName string
		LastName  string
		UserName  string
		Title     string
		Tz        string
		ctx       *Context
		data      *chatData
	}
	type args struct {
		key string
		res interface{}
	}
	tests := []struct {
		name       string
		fields     fields
		args       args
		wantExists bool
		wantRes    interface{}
	}{
		{"non exists - struct", fields{ID: -9999999999, ctx: c}, args{"ChatKey111", &sOut}, false, &s{}},
		{"exists - struct", fields{ID: -9999999999, ctx: c}, args{"ChatKey01", &sOut}, true, &s{1, true, "str"}},
		{"exists - int", fields{ID: -9999999999, ctx: c}, args{"chatkey02", &sOut2}, true, &int123},
	}
	for _, tt := range tests {
		chat := &Chat{
			ID:        tt.fields.ID,
			Type:      tt.fields.Type,
			FirstName: tt.fields.FirstName,
			LastName:  tt.fields.LastName,
			UserName:  tt.fields.UserName,
			Title:     tt.fields.Title,
			Tz:        tt.fields.Tz,
			ctx:       tt.fields.ctx,
			data:      tt.fields.data,
		}
		if gotExists := chat.Cache(tt.args.key, tt.args.res); gotExists != tt.wantExists {
			t.Errorf("%q. Chat.Cache() = %v, want %v", tt.name, gotExists, tt.wantExists)
		}

		if !reflect.DeepEqual(tt.args.res, tt.wantRes) {
			t.Errorf("%q. Chat.Cache() = %v, want %v", tt.name, tt.args.res, tt.wantRes)
		}
	}
}

func TestContext_ServiceCache(t *testing.T) {
	type s struct {
		A int
		B bool
		C string
	}

	c := &Context{db: db, Chat: Chat{ID: -9999999999}, User: User{ID: 9999999999}, ServiceName: "servicewithoauth2"}
	c.User.ctx = c

	c.SetServiceCache("ServiceKey01", s{1, true, "str"}, time.Minute)
	c.SetServiceCache("ServiceKey02", 123, time.Minute)

	sOut := s{}
	var sOut2 int
	int123 := 123

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
		key string
		res interface{}
	}
	tests := []struct {
		name       string
		fields     fields
		args       args
		wantExists bool
		wantRes    interface{}
	}{
		{"non exists - struct", fields{db: db, ServiceName: "servicewithoauth2"}, args{"ServiceKey123", &sOut}, false, &s{}},
		{"exists - struct", fields{db: db, ServiceName: "servicewithoauth2"}, args{"ServiceKey01", &sOut}, true, &s{1, true, "str"}},
		{"exists - int", fields{db: db, ServiceName: "servicewithoauth2"}, args{"servicekey02", &sOut2}, true, &int123},
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
		if gotExists := c.ServiceCache(tt.args.key, tt.args.res); gotExists != tt.wantExists {
			t.Errorf("%q. Context.ServiceCache() = %v, want %v", tt.name, gotExists, tt.wantExists)
		}

		if !reflect.DeepEqual(tt.args.res, tt.wantRes) {
			t.Errorf("%q. Context.ServiceCache() = %v, want %v", tt.name, tt.args.res, tt.wantRes)
		}
	}
}

func TestUser_IsPrivateStarted(t *testing.T) {
	bt := strings.Split(os.Getenv("INTEGRAM_TEST_BOT_TOKEN"), ":")
	botID, _ := strconv.ParseInt(bt[0], 10, 64)

	msg := Message{ChatID: 9999999999, FromID: 9999999999, MsgID: 1, BotID: botID}
	db.C("messages").Insert(&msg)

	c := &Context{db: db, Chat: Chat{ID: -9999999999}, User: User{ID: 9999999999}, ServiceName: "servicewithbottoken"}
	c.User.ctx = c

	c2 := &Context{db: db, Chat: Chat{ID: -9999999998}, User: User{ID: 9999999998}, ServiceName: "servicewithbottoken"}
	c2.User.ctx = c2

	type fields struct {
		ID        int64
		FirstName string
		LastName  string
		UserName  string
		Tz        string
		ctx       *Context
		data      *userData
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		{"test1", fields{ID: 9999999999, ctx: c}, true},
		{"test2", fields{ID: 9999999998, ctx: c2}, false},
	}
	for _, tt := range tests {
		user := &User{
			ID:        tt.fields.ID,
			FirstName: tt.fields.FirstName,
			LastName:  tt.fields.LastName,
			UserName:  tt.fields.UserName,
			Tz:        tt.fields.Tz,
			ctx:       tt.fields.ctx,
			data:      tt.fields.data,
		}
		if got := user.IsPrivateStarted(); got != tt.want {
			t.Errorf("%q. User.IsPrivateStarted() = %v, want %v", tt.name, got, tt.want)
		}
	}
	db.C("users").RemoveId(9999999999)
	db.C("messages").RemoveId(msg.ID)

}

func TestUser_SetCache(t *testing.T) {
	type s struct {
		A int
		B bool
		C string
	}

	c := &Context{db: db, Chat: Chat{ID: -9999999999}, User: User{ID: 9999999999}, ServiceName: "servicewithbottoken"}
	c.User.ctx = c

	sOut := s{}

	type fields struct {
		ID        int64
		FirstName string
		LastName  string
		UserName  string
		Tz        string
		ctx       *Context
		data      *userData
	}
	type args struct {
		key string
		val interface{}
		ttl time.Duration
	}
	tests := []struct {
		name       string
		fields     fields
		args       args
		wantErr    bool
		wantExists bool
	}{
		{"set cache val", fields{ID: 9999999999, ctx: c}, args{"UserKey1", s{123, true, "abc"}, time.Minute}, false, true},
		{"remove cache val", fields{ID: 9999999999, ctx: c}, args{"UserKey1", nil, time.Minute}, false, false},
	}
	for _, tt := range tests {
		user := &User{
			ID:        tt.fields.ID,
			FirstName: tt.fields.FirstName,
			LastName:  tt.fields.LastName,
			UserName:  tt.fields.UserName,
			Tz:        tt.fields.Tz,
			ctx:       tt.fields.ctx,
			data:      tt.fields.data,
		}
		if err := user.SetCache(tt.args.key, tt.args.val, tt.args.ttl); (err != nil) != tt.wantErr {
			t.Errorf("%q. User.SetCache() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}

		exists := user.Cache(tt.args.key, &sOut)
		if exists != tt.wantExists {
			t.Errorf("%q. User.SetCache() exists = %v, wantExists %v", tt.name, exists, tt.wantExists)

		}
		if tt.wantExists && !reflect.DeepEqual(tt.args.val, sOut) {
			t.Errorf("%q. User.SetCache() = %v, want %v", tt.name, sOut, tt.args.val)
		}
	}
}

func TestChat_SetCache(t *testing.T) {
	type s struct {
		A int
		B bool
		C string
	}

	c := &Context{db: db, Chat: Chat{ID: -9999999999}, User: User{ID: 9999999999}, ServiceName: "servicewithbottoken"}
	c.Chat.ctx = c

	sOut := s{}

	type fields struct {
		ID        int64
		Type      string
		FirstName string
		LastName  string
		UserName  string
		Title     string
		Tz        string
		ctx       *Context
		data      *chatData
	}
	type args struct {
		key string
		val interface{}
		ttl time.Duration
	}
	tests := []struct {
		name       string
		fields     fields
		args       args
		wantErr    bool
		wantExists bool
	}{
		{"set cache val", fields{ID: -9999999999, ctx: c}, args{"ChatKey1", s{123, true, "abc"}, time.Minute}, false, true},
		{"remove cache val", fields{ID: -9999999999, ctx: c}, args{"ChatKey1", nil, time.Minute}, false, false},
	}
	for _, tt := range tests {
		chat := &Chat{
			ID:        tt.fields.ID,
			Type:      tt.fields.Type,
			FirstName: tt.fields.FirstName,
			LastName:  tt.fields.LastName,
			UserName:  tt.fields.UserName,
			Title:     tt.fields.Title,
			Tz:        tt.fields.Tz,
			ctx:       tt.fields.ctx,
			data:      tt.fields.data,
		}
		if err := chat.SetCache(tt.args.key, tt.args.val, tt.args.ttl); (err != nil) != tt.wantErr {
			t.Errorf("%q. Chat.SetCache() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}

		exists := chat.Cache(tt.args.key, &sOut)
		if exists != tt.wantExists {
			t.Errorf("%q. User.SetCache() exists = %v, wantExists %v", tt.name, exists, tt.wantExists)

		}
		if tt.wantExists && !reflect.DeepEqual(tt.args.val, sOut) {
			t.Errorf("%q. User.SetCache() = %v, want %v", tt.name, sOut, tt.args.val)
		}
	}
}

func TestContext_SetServiceCache(t *testing.T) {
	type s struct {
		A int
		B bool
		C string
	}

	sOut := s{}

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
		key string
		val interface{}
		ttl time.Duration
	}
	tests := []struct {
		name       string
		fields     fields
		args       args
		wantErr    bool
		wantExists bool
	}{
		{"set cache val", fields{db: db, Chat: Chat{ID: -9999999999}, User: User{ID: 9999999999}, ServiceName: "servicewithbottoken"}, args{"ServiceKey1", s{123, true, "abc"}, time.Minute}, false, true},
		{"remove cache val", fields{db: db, Chat: Chat{ID: -9999999999}, User: User{ID: 9999999999}, ServiceName: "servicewithbottoken"}, args{"ServiceKey1", nil, time.Minute}, false, false},
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
		if err := c.SetServiceCache(tt.args.key, tt.args.val, tt.args.ttl); (err != nil) != tt.wantErr {
			t.Errorf("%q. Context.SetServiceCache() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}

		exists := c.ServiceCache(tt.args.key, &sOut)
		if exists != tt.wantExists {
			t.Errorf("%q. Context.ServiceCache() exists = %v, wantExists %v", tt.name, exists, tt.wantExists)

		}
		if tt.wantExists && !reflect.DeepEqual(tt.args.val, sOut) {
			t.Errorf("%q. Context.ServiceCache() = %v, want %v", tt.name, sOut, tt.args.val)
		}
	}
}

func TestContext_UpdateServiceCache(t *testing.T) {
	type s struct {
		A int
		B bool
		C string
	}

	c := &Context{db: db, Chat: Chat{ID: -9999999999}, User: User{ID: 9999999999}, ServiceName: "servicewithoauth2"}
	c.User.ctx = c
	sOut := s{1, true, "str"}
	sOut2 := s{}

	c.SetServiceCache("ServiceKey01", sOut, time.Minute)
	c.SetServiceCache("ServiceKey02", 123, time.Minute)

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
		key    string
		update interface{}
		res    interface{}
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
		wantRes interface{}
	}{
		{"update cache val", fields{db: db, Chat: Chat{ID: -9999999999}, User: User{ID: 9999999999}, ServiceName: "servicewithoauth2"}, args{"ServiceKey01", bson.M{"$set": bson.M{"val.a": 2, "val.b": false, "val.c": "str2"}}, &sOut}, false, &s{A: 2, B: false, C: "str2"}},
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
		if err := c.UpdateServiceCache(tt.args.key, tt.args.update, tt.args.res); (err != nil) != tt.wantErr {
			t.Errorf("%q. Context.UpdateServiceCache() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}

		if !tt.wantErr && !reflect.DeepEqual(tt.wantRes, tt.args.res) {
			t.Errorf("%q. Context.UpdateServiceCache() = %v, want %v", tt.name, tt.args.res, tt.wantRes)
		}

		// double check from DB
		c.ServiceCache(tt.args.key, &sOut2)

		if !tt.wantErr && !reflect.DeepEqual(tt.wantRes, &sOut2) {
			t.Errorf("%q. Context.UpdateServiceCache() = %v, want %v", tt.name, &sOut2, tt.wantRes)
		}
	}
}

func TestUser_Settings(t *testing.T) {
	type s1 struct {
		A struct {
			A int
			B bool
		}
		B int
		C string
	}

	c1 := &Context{db: db, Chat: Chat{ID: -9999999999}, User: User{ID: 9999999999}, ServiceName: "servicewithoauth2"}
	c1.User.ctx = c1
	c1Out := s1{}
	c1.User.SaveSettings(s1{struct {
		A int
		B bool
	}{555, true}, 555555, "str"})

	type s2 struct {
		A float64
		B string
		C int
	}
	c2 := &Context{db: db, Chat: Chat{ID: -9999999999}, User: User{ID: 9999999999}, ServiceName: "servicewithoauth1"}
	c2.User.ctx = c2
	c2Out := s2{}
	c2.User.SaveSettings(s2{77777.9, "str2", 999999})

	type fields struct {
		ID        int64
		FirstName string
		LastName  string
		UserName  string
		Tz        string
		ctx       *Context
		data      *userData
	}
	type args struct {
		out interface{}
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
		wantRes interface{}
	}{
		{"test1", fields{ID: 9999999999, ctx: c2}, args{&c2Out}, false, &s2{77777.9, "str2", 999999}},
		{"test2", fields{ID: 9999999999, ctx: c2}, args{&c1Out}, false, &s1{}},
		{"test3", fields{ID: 9999999999, ctx: c1}, args{&c1Out}, false, &s1{struct {
			A int
			B bool
		}{555, true}, 555555, "str"}},
	}
	for _, tt := range tests {
		user := &User{
			ID:        tt.fields.ID,
			FirstName: tt.fields.FirstName,
			LastName:  tt.fields.LastName,
			UserName:  tt.fields.UserName,
			Tz:        tt.fields.Tz,
			ctx:       tt.fields.ctx,
			data:      tt.fields.data,
		}
		if err := user.Settings(tt.args.out); (err != nil) != tt.wantErr {
			t.Errorf("%q. User.Settings() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
		if !tt.wantErr && !reflect.DeepEqual(tt.wantRes, tt.args.out) {
			t.Errorf("%q. User.Settings() = %v, want %v", tt.name, tt.args.out, tt.wantRes)
		}
	}
	db.C("users").RemoveId(9999999999)
}

func TestChat_Settings(t *testing.T) {
	type s1 struct {
		A struct {
			A int
			B bool
		}
		B int
		C string
	}

	c1 := &Context{db: db, Chat: Chat{ID: -9999999999}, User: User{ID: 9999999999}, ServiceName: "servicewithbottoken"}
	c1.Chat.ctx = c1
	c1Out := s1{}
	c1.Chat.SaveSettings(s1{struct {
		A int
		B bool
	}{555, true}, 555555, "str"})

	type s2 struct {
		A float64
		B string
		C int
	}
	c2 := &Context{db: db, Chat: Chat{ID: -9999999999}, User: User{ID: 9999999999}, ServiceName: "servicewithactions"}
	c2.Chat.ctx = c2
	c2Out := s2{}
	c2.Chat.SaveSettings(s2{77777.9, "str2", 999999})

	type fields struct {
		ID        int64
		Type      string
		FirstName string
		LastName  string
		UserName  string
		Title     string
		Tz        string
		ctx       *Context
		data      *chatData
	}
	type args struct {
		out interface{}
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
		wantRes interface{}
	}{
		{"test1", fields{ID: -9999999999, ctx: c2}, args{&c2Out}, false, &s2{77777.9, "str2", 999999}},
		{"test2", fields{ID: -9999999999, ctx: c2}, args{&c1Out}, false, &s1{}},
		{"test3", fields{ID: -9999999999, ctx: c1}, args{&c1Out}, false, &s1{struct {
			A int
			B bool
		}{555, true}, 555555, "str"}}}
	for _, tt := range tests {
		chat := &Chat{
			ID:        tt.fields.ID,
			Type:      tt.fields.Type,
			FirstName: tt.fields.FirstName,
			LastName:  tt.fields.LastName,
			UserName:  tt.fields.UserName,
			Title:     tt.fields.Title,
			Tz:        tt.fields.Tz,
			ctx:       tt.fields.ctx,
			data:      tt.fields.data,
		}
		if err := chat.Settings(tt.args.out); (err != nil) != tt.wantErr {
			t.Errorf("%q. Chat.Settings() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}

		if !tt.wantErr && !reflect.DeepEqual(tt.wantRes, tt.args.out) {
			t.Errorf("%q. Chat.Settings() = %v, want %v", tt.name, tt.args.out, tt.wantRes)
		}
	}
	db.C("chats").RemoveId(-9999999999)
}

func TestChat_Setting(t *testing.T) {
	type s1 struct {
		A struct {
			A int
			B bool
		}
		B int
		C string
	}

	c1 := &Context{db: db, Chat: Chat{ID: -9999999999}, User: User{ID: 9999999999}, ServiceName: "servicewithbottoken"}
	c1.Chat.ctx = c1
	c1.Chat.SaveSettings(s1{struct {
		A int
		B bool
	}{555, true}, 555555, "str"})

	type s2 struct {
		A float64
		B string
		C int
	}
	c2 := &Context{db: db, Chat: Chat{ID: -9999999999}, User: User{ID: 9999999999}, ServiceName: "servicewithactions"}
	c2.Chat.ctx = c2
	c2.Chat.SaveSettings(s2{77777.9, "str2", 999999})

	type fields struct {
		ID        int64
		Type      string
		FirstName string
		LastName  string
		UserName  string
		Title     string
		Tz        string
		ctx       *Context
		data      *chatData
	}
	type args struct {
		key string
	}
	tests := []struct {
		name       string
		fields     fields
		args       args
		wantResult interface{}
		wantExists bool
	}{
		{"test1", fields{ID: -9999999999, ctx: c2}, args{"a"}, 77777.9, true},
		{"test2", fields{ID: -9999999999, ctx: c2}, args{"d"}, 0, false},
		{"test3", fields{ID: -9999999999, ctx: c1}, args{"a"}, map[string]interface{}{"a": 555, "b": true}, true},
	}
	for _, tt := range tests {
		chat := &Chat{
			ID:        tt.fields.ID,
			Type:      tt.fields.Type,
			FirstName: tt.fields.FirstName,
			LastName:  tt.fields.LastName,
			UserName:  tt.fields.UserName,
			Title:     tt.fields.Title,
			Tz:        tt.fields.Tz,
			ctx:       tt.fields.ctx,
			data:      tt.fields.data,
		}
		gotResult, gotExists := chat.Setting(tt.args.key)
		if gotExists != tt.wantExists {
			t.Errorf("%q. Chat.Setting() gotExists = %v, want %v", tt.name, gotExists, tt.wantExists)
		}

		if tt.wantExists && !reflect.DeepEqual(gotResult, tt.wantResult) {
			t.Errorf("%q. Chat.Setting() gotResult = %v, want %v", tt.name, gotResult, tt.wantResult)
		}

	}

	db.C("chats").RemoveId(-9999999999)
}

func TestUser_Setting(t *testing.T) {
	type s1 struct {
		A struct {
			A int
			B bool
		}
		B int
		C string
	}

	c1 := &Context{db: db, Chat: Chat{ID: -9999999999}, User: User{ID: 9999999999}, ServiceName: "servicewithbottoken"}
	c1.User.ctx = c1
	c1.User.SaveSettings(s1{struct {
		A int
		B bool
	}{555, true}, 555555, "str"})

	type s2 struct {
		A float64
		B string
		C int
	}
	c2 := &Context{db: db, Chat: Chat{ID: -9999999999}, User: User{ID: 9999999999}, ServiceName: "servicewithactions"}
	c2.User.ctx = c2
	c2.User.SaveSettings(s2{77777.9, "str2", 999999})

	type fields struct {
		ID        int64
		FirstName string
		LastName  string
		UserName  string
		Tz        string
		ctx       *Context
		data      *userData
	}
	type args struct {
		key string
	}
	tests := []struct {
		name       string
		fields     fields
		args       args
		wantResult interface{}
		wantExists bool
	}{
		{"test1", fields{ID: 9999999999, ctx: c2}, args{"a"}, 77777.9, true},
		{"test2", fields{ID: 9999999999, ctx: c2}, args{"d"}, 0, false},
		{"test3", fields{ID: 9999999999, ctx: c1}, args{"a"}, map[string]interface{}{"a": 555, "b": true}, true},
	}
	for _, tt := range tests {
		user := &User{
			ID:        tt.fields.ID,
			FirstName: tt.fields.FirstName,
			LastName:  tt.fields.LastName,
			UserName:  tt.fields.UserName,
			Tz:        tt.fields.Tz,
			ctx:       tt.fields.ctx,
			data:      tt.fields.data,
		}
		gotResult, gotExists := user.Setting(tt.args.key)
		if gotExists != tt.wantExists {
			t.Errorf("%q. User.Setting() gotExists = %v, want %v", tt.name, gotExists, tt.wantExists)
		}
		if tt.wantExists && !reflect.DeepEqual(gotResult, tt.wantResult) {
			t.Errorf("%q. User.Setting() gotResult = %v, want %v", tt.name, gotResult, tt.wantResult)
		}

	}

	db.C("users").RemoveId(9999999999)
}

func TestChat_SaveSettings(t *testing.T) {
	type s1 struct {
		A struct {
			A int
			B bool
		}
		B int
		C string
	}

	c1 := &Context{db: db, Chat: Chat{ID: -9999999999}, User: User{ID: 9999999999}, ServiceName: "servicewithbottoken"}
	c1.Chat.ctx = c1
	s1Out := s1{}

	type s2 struct {
		A float64
		B string
		C int
	}
	c2 := &Context{db: db, Chat: Chat{ID: -9999999999}, User: User{ID: 9999999999}, ServiceName: "servicewithactions"}
	c2.Chat.ctx = c2
	s2Out := s2{}

	type fields struct {
		ID        int64
		Type      string
		FirstName string
		LastName  string
		UserName  string
		Title     string
		Tz        string
		ctx       *Context
		data      *chatData
	}
	type args struct {
		allSettings interface{}
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		out     interface{}
		wantErr bool
	}{
		{"test1", fields{ID: -9999999999, ctx: c1}, args{&s1{struct {
			A int
			B bool
		}{555, true}, 555555, "str"}}, &s1Out, false},
		{"test2", fields{ID: -9999999999, ctx: c2}, args{&s2{77777.9, "str2", 999999}}, &s2Out, false},
		{"test3", fields{ID: -9999999999, ctx: c1}, args{&s1{struct {
			A int
			B bool
		}{222, false}, 55555522, "str23"}}, &s1Out, false},
		{"test4", fields{ID: -9999999999, ctx: c2}, args{&s2{88888.9, "str3", 3999999}}, &s2Out, false},
	}
	for _, tt := range tests {
		chat := &Chat{
			ID:        tt.fields.ID,
			Type:      tt.fields.Type,
			FirstName: tt.fields.FirstName,
			LastName:  tt.fields.LastName,
			UserName:  tt.fields.UserName,
			Title:     tt.fields.Title,
			Tz:        tt.fields.Tz,
			ctx:       tt.fields.ctx,
			data:      tt.fields.data,
		}
		if err := chat.SaveSettings(tt.args.allSettings); (err != nil) != tt.wantErr {
			t.Errorf("%q. Chat.SaveSettings() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}

		chat.Settings(tt.out)

		if !reflect.DeepEqual(tt.out, tt.args.allSettings) {
			t.Errorf("%q. Chat.SaveSettings() gotResult = %v, want %v", tt.name, tt.out, tt.args.allSettings)
		}
	}
	db.C("chats").RemoveId(-9999999999)

}

func TestUser_SaveSettings(t *testing.T) {
	type s1 struct {
		A struct {
			A int
			B bool
		}
		B int
		C string
	}

	c1 := &Context{db: db, Chat: Chat{ID: -9999999999}, User: User{ID: 9999999999}, ServiceName: "servicewithbottoken"}
	c1.Chat.ctx = c1
	c1.User.ctx = c1
	s1Out := s1{}

	type s2 struct {
		A float64
		B string
		C int
	}
	c2 := &Context{db: db, Chat: Chat{ID: -9999999999}, User: User{ID: 9999999999}, ServiceName: "servicewithactions"}
	c2.Chat.ctx = c2
	c2.User.ctx = c2
	s2Out := s2{}

	type fields struct {
		ID        int64
		FirstName string
		LastName  string
		UserName  string
		Tz        string
		ctx       *Context
		data      *userData
	}
	type args struct {
		allSettings interface{}
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		out     interface{}
		wantErr bool
	}{
		{"test1", fields{ID: 9999999999, ctx: c1}, args{&s1{struct {
			A int
			B bool
		}{555, true}, 555555, "str"}}, &s1Out, false},
		{"test2", fields{ID: 9999999999, ctx: c2}, args{&s2{77777.9, "str2", 999999}}, &s2Out, false},
		{"test3", fields{ID: 9999999999, ctx: c1}, args{&s1{struct {
			A int
			B bool
		}{222, false}, 55555522, "str23"}}, &s1Out, false},
		{"test4", fields{ID: 9999999999, ctx: c2}, args{&s2{88888.9, "str3", 3999999}}, &s2Out, false},
	}
	for _, tt := range tests {
		user := &User{
			ID:        tt.fields.ID,
			FirstName: tt.fields.FirstName,
			LastName:  tt.fields.LastName,
			UserName:  tt.fields.UserName,
			Tz:        tt.fields.Tz,
			ctx:       tt.fields.ctx,
			data:      tt.fields.data,
		}
		if err := user.SaveSettings(tt.args.allSettings); (err != nil) != tt.wantErr {
			t.Errorf("%q. User.SaveSettings() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}

		user.Settings(tt.out)

		if !reflect.DeepEqual(tt.out, tt.args.allSettings) {
			t.Errorf("%q. User.SaveSettings() gotResult = %v, want %v", tt.name, tt.out, tt.args.allSettings)
		}
	}

	db.C("users").RemoveId(9999999999)

}

func TestUser_ServiceHookToken(t *testing.T) {
	type fields struct {
		ID        int64
		FirstName string
		LastName  string
		UserName  string
		Tz        string
		ctx       *Context
		data      *userData
	}
	tests := []struct {
		name       string
		fields     fields
		wantPrefix string
	}{
		{"hook exists", fields{ID: 9999999999, ctx: &Context{db: db, ServiceName: "servicewithoauth1"}, data: &userData{Hooks: []serviceHook{{Token: "hereisthetoken", Services: []string{"servicewithoauth1"}}}}}, "hereisthetoken"},
		{"hook not exists", fields{ID: 9999999999, ctx: &Context{db: db, ServiceName: "servicewithoauth1"}, data: &userData{Hooks: []serviceHook{{Token: "hereisthetoken", Services: []string{"servicewithoauth2"}}}}}, "u"},
	}
	for _, tt := range tests {
		user := &User{
			ID:        tt.fields.ID,
			FirstName: tt.fields.FirstName,
			LastName:  tt.fields.LastName,
			UserName:  tt.fields.UserName,
			Tz:        tt.fields.Tz,
			ctx:       tt.fields.ctx,
			data:      tt.fields.data,
		}
		if got := user.ServiceHookToken(); !strings.HasPrefix(got, tt.wantPrefix) {
			t.Errorf("%q. User.ServiceHookToken() = %v, want prefix %v", tt.name, got, tt.wantPrefix)
		}
	}
}

func TestChat_ServiceHookToken(t *testing.T) {
	type fields struct {
		ID        int64
		Type      string
		FirstName string
		LastName  string
		UserName  string
		Title     string
		Tz        string
		ctx       *Context
		data      *chatData
	}
	tests := []struct {
		name       string
		fields     fields
		wantPrefix string
	}{
		{"hook exists", fields{ID: -9999999999, ctx: &Context{db: db, ServiceName: "servicewithoauth1"}, data: &chatData{Hooks: []serviceHook{{Token: "hereisthetoken", Services: []string{"servicewithoauth1"}}}}}, "hereisthetoken"},
		{"hook not exists", fields{ID: -9999999999, ctx: &Context{db: db, ServiceName: "servicewithoauth1"}, data: &chatData{Hooks: []serviceHook{{Token: "hereisthetoken", Services: []string{"servicewithoauth2"}}}}}, "c"},
	}
	for _, tt := range tests {
		chat := &Chat{
			ID:        tt.fields.ID,
			Type:      tt.fields.Type,
			FirstName: tt.fields.FirstName,
			LastName:  tt.fields.LastName,
			UserName:  tt.fields.UserName,
			Title:     tt.fields.Title,
			Tz:        tt.fields.Tz,
			ctx:       tt.fields.ctx,
			data:      tt.fields.data,
		}
		if got := chat.ServiceHookToken(); !strings.HasPrefix(got, tt.wantPrefix) {
			t.Errorf("%q. Chat.ServiceHookToken() = %v, want prefix %v", tt.name, got, tt.wantPrefix)
		}
	}
}

func TestUser_ServiceHookURL(t *testing.T) {
	type fields struct {
		ID        int64
		FirstName string
		LastName  string
		UserName  string
		Tz        string
		ctx       *Context
		data      *userData
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{"hook exists", fields{ID: 9999999999, ctx: &Context{db: db, ServiceName: "servicewithoauth1"}, data: &userData{Hooks: []serviceHook{{Token: "hereisthetoken", Services: []string{"servicewithoauth1"}}}}}, "https://integram.org/servicewithoauth1/hereisthetoken"},
	}
	for _, tt := range tests {
		user := &User{
			ID:        tt.fields.ID,
			FirstName: tt.fields.FirstName,
			LastName:  tt.fields.LastName,
			UserName:  tt.fields.UserName,
			Tz:        tt.fields.Tz,
			ctx:       tt.fields.ctx,
			data:      tt.fields.data,
		}
		if got := user.ServiceHookURL(); got != tt.want {
			t.Errorf("%q. User.ServiceHookURL() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestChat_ServiceHookURL(t *testing.T) {
	type fields struct {
		ID        int64
		Type      string
		FirstName string
		LastName  string
		UserName  string
		Title     string
		Tz        string
		ctx       *Context
		data      *chatData
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{"hook exists", fields{ID: -9999999999, ctx: &Context{db: db, ServiceName: "servicewithoauth1"}, data: &chatData{Hooks: []serviceHook{{Token: "hereisthetoken", Services: []string{"servicewithoauth1"}}}}}, "https://integram.org/servicewithoauth1/hereisthetoken"},
	}
	for _, tt := range tests {
		chat := &Chat{
			ID:        tt.fields.ID,
			Type:      tt.fields.Type,
			FirstName: tt.fields.FirstName,
			LastName:  tt.fields.LastName,
			UserName:  tt.fields.UserName,
			Title:     tt.fields.Title,
			Tz:        tt.fields.Tz,
			ctx:       tt.fields.ctx,
			data:      tt.fields.data,
		}
		if got := chat.ServiceHookURL(); got != tt.want {
			t.Errorf("%q. Chat.ServiceHookURL() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestUser_AddChatToHook(t *testing.T) {
	type fields struct {
		ID        int64
		FirstName string
		LastName  string
		UserName  string
		Tz        string
		ctx       *Context
		data      *userData
	}
	type args struct {
		chatID int64
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{"init hook and add the chat", fields{ID: 9999999999, ctx: &Context{db: db, ServiceName: "servicewithoauth1"}}, args{-9999999999}, false},
		{"add another chat", fields{ID: 9999999999, ctx: &Context{db: db, ServiceName: "servicewithoauth1"}}, args{-9999999998}, false},
	}
	for _, tt := range tests {
		user := &User{
			ID:        tt.fields.ID,
			FirstName: tt.fields.FirstName,
			LastName:  tt.fields.LastName,
			UserName:  tt.fields.UserName,
			Tz:        tt.fields.Tz,
			ctx:       tt.fields.ctx,
			data:      tt.fields.data,
		}

		if err := user.AddChatToHook(tt.args.chatID); (err != nil) != tt.wantErr {
			t.Errorf("%q. User.AddChatToHook() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
		u := User{ID: tt.fields.ID, ctx: &Context{db: db}}
		ud, err := u.getData()
		if err != nil {
			t.Errorf("%q. User.AddChatToHook() error = %v", tt.name, err)
		}
		found := false
		for _, hook := range ud.Hooks {
			if len(hook.Services) == 1 && hook.Services[0] == "servicewithoauth1" {
				for _, chat := range hook.Chats {
					if chat == tt.args.chatID {
						found = true
					}
				}
			}
		}
		if !found {
			t.Errorf("%q. User.AddChatToHook() chat not found in the hooks = %+v", tt.name, ud)
		}
	}
	db.C("users").RemoveId(9999999999)
}

func TestChat_SaveSetting(t *testing.T) {
	type s1 struct {
		A struct {
			A int
			B bool
		}
		B int
		C string
	}

	c1 := &Context{db: db, Chat: Chat{ID: -9999999999}, User: User{ID: 9999999999}, ServiceName: "servicewithbottoken"}
	c1.Chat.ctx = c1
	c1.Chat.SaveSettings(s1{struct {
		A int
		B bool
	}{555, true}, 555555, "str"})

	s1Out := s1{}
	s1Out2 := s1{}

	type fields struct {
		ID        int64
		Type      string
		FirstName string
		LastName  string
		UserName  string
		Title     string
		Tz        string
		ctx       *Context
		data      *chatData
	}
	type args struct {
		key   string
		value interface{}
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
		out     interface{}
		wantRes interface{}
	}{
		{"change existing settings", fields{ID: -9999999999, ctx: &Context{db: db, ServiceName: "servicewithbottoken"}}, args{"a", struct {
			A int
			B bool
		}{123, false}}, false, &s1Out, &s1{struct {
			A int
			B bool
		}{123, false}, 555555, "str"}},
		{"update the empty settings", fields{ID: -9999999999, ctx: &Context{db: db, ServiceName: "servicewithoauth1"}}, args{"b", 7777}, false, &s1Out2, &s1{B: 7777}},
	}
	for _, tt := range tests {
		chat := &Chat{
			ID:        tt.fields.ID,
			Type:      tt.fields.Type,
			FirstName: tt.fields.FirstName,
			LastName:  tt.fields.LastName,
			UserName:  tt.fields.UserName,
			Title:     tt.fields.Title,
			Tz:        tt.fields.Tz,
			ctx:       tt.fields.ctx,
			data:      tt.fields.data,
		}
		if err := chat.SaveSetting(tt.args.key, tt.args.value); (err != nil) != tt.wantErr {
			t.Errorf("%q. Chat.SaveSetting() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
		chat.Settings(tt.out)

		if !reflect.DeepEqual(tt.out, tt.wantRes) {
			t.Errorf("%q. Chat.SaveSetting() gotResult = %v, want %v", tt.name, tt.out, tt.wantRes)
		}

	}
	db.C("chats").RemoveId(-9999999999)

}

func TestUser_SaveSetting(t *testing.T) {
	type s1 struct {
		A struct {
			A int
			B bool
		}
		B int
		C string
	}

	c1 := &Context{db: db, Chat: Chat{ID: -9999999999}, User: User{ID: 9999999999}, ServiceName: "servicewithbottoken"}
	c1.User.ctx = c1
	c1.User.SaveSettings(s1{struct {
		A int
		B bool
	}{555, true}, 555555, "str"})

	s1Out := s1{}
	s1Out2 := s1{}

	type fields struct {
		ID        int64
		FirstName string
		LastName  string
		UserName  string
		Tz        string
		ctx       *Context
		data      *userData
	}
	type args struct {
		key   string
		value interface{}
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
		out     interface{}
		wantRes interface{}
	}{
		{"change existing settings", fields{ID: 9999999999, ctx: &Context{db: db, ServiceName: "servicewithbottoken"}}, args{"a", struct {
			A int
			B bool
		}{123, false}}, false, &s1Out, &s1{struct {
			A int
			B bool
		}{123, false}, 555555, "str"}},
		{"update the empty settings", fields{ID: 9999999999, ctx: &Context{db: db, ServiceName: "servicewithoauth1"}}, args{"b", 7777}, false, &s1Out2, &s1{B: 7777}},
	}
	for _, tt := range tests {
		user := &User{
			ID:        tt.fields.ID,
			FirstName: tt.fields.FirstName,
			LastName:  tt.fields.LastName,
			UserName:  tt.fields.UserName,
			Tz:        tt.fields.Tz,
			ctx:       tt.fields.ctx,
			data:      tt.fields.data,
		}
		if err := user.SaveSetting(tt.args.key, tt.args.value); (err != nil) != tt.wantErr {
			t.Errorf("%q. User.SaveSetting() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
	}
	db.C("users").RemoveId(9999999999)

}

type fakeStrGenerator struct {
	Value string
}

func (f fakeStrGenerator) Get(n int) string {
	fmt.Println("!!!:" + f.Value)
	return f.Value
}
func TestUser_AuthTempToken(t *testing.T) {
	fstr := &fakeStrGenerator{Value: "fake"}

	rndStr = strGenerator(fstr)

	type fields struct {
		ID        int64
		FirstName string
		LastName  string
		UserName  string
		Tz        string
		ctx       *Context
		data      *userData
	}
	tests := []struct {
		name             string
		fields           fields
		cacheKeyToSet    string
		cacheValToSet    interface{}
		cacheKeyToCheck  string
		cacheMustExists  bool
		cacheValToCheck  interface{}
		fakeStrGenerator string
		want             string
	}{
		{"AuthTempToken already set in protected settings, cache found, host is the same", fields{ID: 9999999999, ctx: &Context{db: db, ServiceBaseURL: *URLMustParse("https://sub.example.com"), ServiceName: "servicewithoauth2"}, data: &userData{Protected: map[string]*userProtected{"servicewithoauth2": {AuthTempToken: "testauthtemptoken"}}}}, "auth_testauthtemptoken", oAuthIDCacheVal{BaseURL: "https://sub.example.com"}, "auth_testauthtemptoken", true, oAuthIDCacheVal{BaseURL: "https://sub.example.com"}, "fake", "testauthtemptoken"},
		{"AuthTempToken already set in protected settings, cache found, but the host is different", fields{ID: 9999999999, ctx: &Context{ServiceBaseURL: *URLMustParse("https://sub.otherexample.com"), db: db, ServiceName: "servicewithoauth2"}, data: &userData{Protected: map[string]*userProtected{"servicewithoauth2": {AuthTempToken: "testauthtemptoken"}}}}, "", nil, "auth_fake2", true, oAuthIDCacheVal{BaseURL: "https://sub.otherexample.com"}, "fake2", "fake2"},
		{"AuthTempToken already set in protected settings, cache not found", fields{ID: 9999999999, ctx: &Context{db: db, ServiceName: "servicewithoauth2"}, data: &userData{Protected: map[string]*userProtected{"servicewithoauth2": {AuthTempToken: "testauthtemptoken"}}}}, "", nil, "auth_testauthtemptoken", true, oAuthIDCacheVal{BaseURL: "https://sub.example.com"}, "fake3", "testauthtemptoken"},
		{"AuthTempToken not set in protected settings", fields{ID: 9999999999, ctx: &Context{db: db, ServiceName: "servicewithoauth2"}}, "", nil, "auth_fake4", true, oAuthIDCacheVal{BaseURL: "https://sub.example.com"}, "fake4", "fake4"},
		{"AuthTempToken not set in protected, host is not default", fields{ID: 9999999999, ctx: &Context{ServiceBaseURL: *URLMustParse("http://otherdomain2.com"), db: db, ServiceName: "servicewithoauth2"}}, "", nil, "auth_fake5", true, oAuthIDCacheVal{BaseURL: "http://otherdomain2.com"}, "fake5", "fake5"},
	}
	for _, tt := range tests {
		tt.fields.ctx.User = User{
			ID:        tt.fields.ID,
			FirstName: tt.fields.FirstName,
			LastName:  tt.fields.LastName,
			UserName:  tt.fields.UserName,
			Tz:        tt.fields.Tz,
			ctx:       tt.fields.ctx,
			data:      tt.fields.data,
		}

		user := &tt.fields.ctx.User

		fstr.Value = tt.fakeStrGenerator

		if tt.cacheKeyToSet != "" {
			err := user.SetCache(tt.cacheKeyToSet, tt.cacheValToSet, time.Second*5)
			if err != nil {
				t.Errorf("%q. User.AuthTempToken() setcache error: %v", tt.name, err)
			}
		}

		if got := user.AuthTempToken(); got != tt.want {
			t.Errorf("%q. User.AuthTempToken() = %v, want %v", tt.name, got, tt.want)
		}

		if tt.cacheValToCheck != "" {
			cv := oAuthIDCacheVal{}
			exists := user.Cache(tt.cacheKeyToCheck, &cv)
			if tt.cacheMustExists != exists {
				t.Errorf("%q. User.AuthTempToken() cache key %v check: exists %v, want %v", tt.name, tt.cacheKeyToCheck, exists, tt.cacheMustExists)
			}

			if tt.cacheMustExists && !reflect.DeepEqual(cv, tt.cacheValToCheck) {
				t.Errorf("%q. User.AuthTempToken() cache val check: %v, want %v", tt.name, cv, tt.cacheValToCheck)
			}
		}

		db.C("users").RemoveId(tt.fields.ID)
	}
}

func TestUser_OauthRedirectURL(t *testing.T) {
	type fields struct {
		ID        int64
		FirstName string
		LastName  string
		UserName  string
		Tz        string
		ctx       *Context
		data      *userData
	}
	tests := []struct {
		name                  string
		fields                fields
		oauthProviderToInsert bson.M
		want                  string
	}{
		{"service with default host", fields{ID: 9999999999, ctx: &Context{db: db, ServiceName: "servicewithoauth2"}}, nil, "https://integram.org/auth/servicewithoauth2"},
		{"service with custom host", fields{ID: 9999999999, ctx: &Context{db: db, ServiceName: "servicewithoauth1", ServiceBaseURL: *URLMustParse("http://sub.someother.com")}}, bson.M{"_id": "5tOPeQ", "service": "servicewithoauth1", "baseurl": bson.M{"scheme": "https", "host": "sub.someother.com", "path": ""}}, "https://integram.org/auth/servicewithoauth1/5tOPeQ"},
	}
	for _, tt := range tests {
		user := &User{
			ID:        tt.fields.ID,
			FirstName: tt.fields.FirstName,
			LastName:  tt.fields.LastName,
			UserName:  tt.fields.UserName,
			Tz:        tt.fields.Tz,
			ctx:       tt.fields.ctx,
			data:      tt.fields.data,
		}
		if tt.oauthProviderToInsert != nil {
			db.C("oauth_providers").Insert(tt.oauthProviderToInsert)
		}
		if got := user.OauthRedirectURL(); got != tt.want {
			t.Errorf("%q. User.OauthRedirectURL() = %v, want %v", tt.name, got, tt.want)
		}
		db.C("users").RemoveId(tt.fields.ID)
		db.C("users_cache").Remove(bson.M{"userid": tt.fields.ID})
	}
}

func TestUser_OauthInitURL(t *testing.T) {
	fstr := &fakeStrGenerator{Value: "fake1"}

	rndStr = strGenerator(fstr)

	type fields struct {
		ID        int64
		FirstName string
		LastName  string
		UserName  string
		Tz        string
		ctx       *Context
		data      *userData
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{"oauth2", fields{ID: 9999999999, ctx: &Context{db: db, ServiceName: "servicewithoauth2"}}, "https://sub.example.com/oauth/authorize?access_type=offline&client_id=ID&response_type=code&state=fake1"},
		{"oauth1", fields{ID: 9999999999, ctx: &Context{db: db, ServiceName: "servicewithoauth1"}}, "https://integram.org/oauth1/servicewithoauth1/fake1"},
	}
	for _, tt := range tests {
		user := &User{
			ID:        tt.fields.ID,
			FirstName: tt.fields.FirstName,
			LastName:  tt.fields.LastName,
			UserName:  tt.fields.UserName,
			Tz:        tt.fields.Tz,
			ctx:       tt.fields.ctx,
			data:      tt.fields.data,
		}
		if got := user.OauthInitURL(); got != tt.want {
			t.Errorf("%q. User.OauthInitURL() = %v, want %v", tt.name, got, tt.want)

		}
		db.C("users").RemoveId(tt.fields.ID)
		db.C("users_cache").RemoveAll(bson.M{"userid": tt.fields.ID})

	}
}

func TestUser_OAuthHTTPClient(t *testing.T) {
	type fields struct {
		ID        int64
		FirstName string
		LastName  string
		UserName  string
		Tz        string
		ctx       *Context
		data      *userData
	}
	tests := []struct {
		name   string
		fields fields
		want   *http.Client
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		user := &User{
			ID:        tt.fields.ID,
			FirstName: tt.fields.FirstName,
			LastName:  tt.fields.LastName,
			UserName:  tt.fields.UserName,
			Tz:        tt.fields.Tz,
			ctx:       tt.fields.ctx,
			data:      tt.fields.data,
		}
		if got := user.OAuthHTTPClient(); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. User.OAuthHTTPClient() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestUser_OAuthValid(t *testing.T) {
	type fields struct {
		ID        int64
		FirstName string
		LastName  string
		UserName  string
		Tz        string
		ctx       *Context
		data      *userData
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		{"ok", fields{ID: 9999999999, ctx: &Context{db: db, ServiceName: "servicewithoauth2"}, data: &userData{Protected: map[string]*userProtected{"servicewithoauth2": {OAuthToken: "goodone"}}}}, true},
		{"empty token", fields{ID: 9999999999, ctx: &Context{db: db, ServiceName: "servicewithoauth2"}, data: &userData{Protected: map[string]*userProtected{"servicewithoauth2": {OAuthToken: ""}}}}, false},
		{"protected settings weren't inited", fields{ID: 9999999999, ctx: &Context{db: db, ServiceName: "servicewithoauth2"}}, false},
	}
	for _, tt := range tests {
		user := &User{
			ID:        tt.fields.ID,
			FirstName: tt.fields.FirstName,
			LastName:  tt.fields.LastName,
			UserName:  tt.fields.UserName,
			Tz:        tt.fields.Tz,
			ctx:       tt.fields.ctx,
			data:      tt.fields.data,
		}
		if got := user.OAuthValid(); got != tt.want {
			t.Errorf("%q. User.OAuthValid() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestUser_OAuthToken(t *testing.T) {
	// todo: test the token refresh case
	type fields struct {
		ID        int64
		FirstName string
		LastName  string
		UserName  string
		Tz        string
		ctx       *Context
		data      *userData
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{"ok", fields{ID: 9999999999, ctx: &Context{db: db, ServiceName: "servicewithoauth2"}, data: &userData{Protected: map[string]*userProtected{"servicewithoauth2": {OAuthToken: "goodone"}}}}, "goodone"},
		{"empty token", fields{ID: 9999999999, ctx: &Context{db: db, ServiceName: "servicewithoauth2"}, data: &userData{Protected: map[string]*userProtected{"servicewithoauth2": {OAuthToken: ""}}}}, ""},
		{"protected settings weren't inited", fields{ID: 9999999999, ctx: &Context{db: db, ServiceName: "servicewithoauth2"}}, ""},
	}
	for _, tt := range tests {
		user := &User{
			ID:        tt.fields.ID,
			FirstName: tt.fields.FirstName,
			LastName:  tt.fields.LastName,
			UserName:  tt.fields.UserName,
			Tz:        tt.fields.Tz,
			ctx:       tt.fields.ctx,
			data:      tt.fields.data,
		}
		if got := user.OAuthToken(); got != tt.want {
			t.Errorf("%q. User.OAuthToken() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestUser_ResetOAuthToken(t *testing.T) {
	type fields struct {
		ID        int64
		FirstName string
		LastName  string
		UserName  string
		Tz        string
		ctx       *Context
		data      *userData
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{"test1", fields{ID: 9999999999, ctx: &Context{db: db, ServiceName: "servicewithoauth2"}}, false},
	}
	for _, tt := range tests {
		user := &User{
			ID:        tt.fields.ID,
			FirstName: tt.fields.FirstName,
			LastName:  tt.fields.LastName,
			UserName:  tt.fields.UserName,
			Tz:        tt.fields.Tz,
			ctx:       tt.fields.ctx,
			data:      tt.fields.data,
		}
		ps, _ := user.protectedSettings()

		ps.OAuthToken = "aaa"
		user.saveProtectedSettings()
		user.data = nil
		if user.OAuthToken() == "" {
			t.Errorf("%q. before User.ResetOAuthToken() token is empty", tt.name)
		}

		if err := user.ResetOAuthToken(); (err != nil) != tt.wantErr {
			t.Errorf("%q. User.ResetOAuthToken() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
		if user.OAuthToken() != "" {
			t.Errorf("%q. User.ResetOAuthToken() token = %v, want empty", tt.name, user.OAuthToken())
		}

		user.data = nil

		// repeat from DB
		if user.OAuthToken() != "" {
			t.Errorf("%q. User.ResetOAuthToken() from db: token = %v, want empty", tt.name, user.OAuthToken())
		}

	}
}

func TestUser_SetAfterAuthAction(t *testing.T) {
	type fields struct {
		ID        int64
		FirstName string
		LastName  string
		UserName  string
		Tz        string
		ctx       *Context
		data      *userData
	}
	type args struct {
		handlerFunc interface{}
		args        []interface{}
	}
	tests := []struct {
		name         string
		fields       fields
		args         args
		wantUserData *userData
		wantErr      bool
	}{
		{"set from empty", fields{ID: 9999999999, ctx: &Context{db: db, ServiceName: "servicewithoauth2"}}, args{dumbFuncWithContextAndParam, []interface{}{true}}, &userData{Protected: map[string]*userProtected{"servicewithoauth2": {AfterAuthHandler: "github.com/requilence/integram.dumbFuncWithContextAndParam", AfterAuthData: []byte{12, 255, 129, 2, 1, 2, 255, 130, 0, 1, 16, 0, 0, 13, 255, 130, 0, 1, 4, 98, 111, 111, 108, 2, 2, 0, 1}}}}, false},
		{"override exists value", fields{ID: 9999999999, ctx: &Context{db: db, ServiceName: "servicewithoauth2"}}, args{dumbFuncWithContextAndParams, []interface{}{1, 2}}, &userData{Protected: map[string]*userProtected{"servicewithoauth2": {AfterAuthHandler: "github.com/requilence/integram.dumbFuncWithContextAndParams", AfterAuthData: []byte{12, 255, 129, 2, 1, 2, 255, 130, 0, 1, 16, 0, 0, 20, 255, 130, 0, 2, 3, 105, 110, 116, 4, 2, 0, 2, 3, 105, 110, 116, 4, 2, 0, 4}}}}, false},
	}
	for _, tt := range tests {
		user := &User{
			ID:        tt.fields.ID,
			FirstName: tt.fields.FirstName,
			LastName:  tt.fields.LastName,
			UserName:  tt.fields.UserName,
			Tz:        tt.fields.Tz,
			ctx:       tt.fields.ctx,
			data:      tt.fields.data,
		}
		err := user.SetAfterAuthAction(tt.args.handlerFunc, tt.args.args...)

		if (err != nil) != tt.wantErr {
			t.Errorf("%q. User.SetAfterAuthAction() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
		if !reflect.DeepEqual(user.data.Protected, tt.wantUserData.Protected) {

			t.Errorf("%q. User.SetAfterAuthAction() = %v, want %v", tt.name, user.data.Protected, tt.wantUserData.Protected)
		}

		user.data = nil
		user.getData()

		if !reflect.DeepEqual(user.data.Protected, tt.wantUserData.Protected) {
			t.Errorf("%q. User.SetAfterAuthAction() from DB = %v, want %v", tt.name, user.data.Protected, tt.wantUserData.Protected)
		}
	}

}

func TestContext_WebPreview(t *testing.T) {
	fstr := &fakeStrGenerator{Value: "fakewp"}
	rndStr = strGenerator(fstr)

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
		title      string
		headline   string
		text       string
		serviceURL string
		imageURL   string
	}
	tests := []struct {
		name              string
		fields            fields
		args              args
		fakeStrGenerator  string
		wantWebPreviewURL string
	}{
		{"new wp", fields{ServiceName: "servicewithbottoken", db: db}, args{"testTitle", "testHeadline", "testText", "https://telegram.org", "https://telegram.org/img/t_logo.png"}, "fakewptoken1", "https://integram.org/a/fakewptoken1"},
		{"same wp", fields{ServiceName: "servicewithbottoken", db: db}, args{"testTitle", "testHeadline", "testText", "https://telegram.org", "https://telegram.org/img/t_logo.png"}, "fakewptoken2", "https://integram.org/a/fakewptoken1"},
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
		fstr.Value = tt.fakeStrGenerator
		if gotWebPreviewURL := c.WebPreview(tt.args.title, tt.args.headline, tt.args.text, tt.args.serviceURL, tt.args.imageURL); gotWebPreviewURL != tt.wantWebPreviewURL {
			t.Errorf("%q. Context.WebPreview() = %v, want %v", tt.name, gotWebPreviewURL, tt.wantWebPreviewURL)
		}
	}
	db.C("previews").RemoveAll(bson.M{"headline": "testHeadline"})
}
