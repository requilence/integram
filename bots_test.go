package integram

import (
	"github.com/requilence/url"
	"reflect"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	tg "github.com/requilence/telegram-bot-api"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
)

func TestBot_PMURL(t *testing.T) {
	type fields struct {
		ID          int64
		Username    string
		token       string
		services    []*Service
		updatesChan <-chan tg.Update
		API         *tg.BotAPI
	}
	type args struct {
		param string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   string
	}{
		{"withparam", fields{Username: "Trello_bot"}, args{"pval"}, "https://telegram.me/Trello_bot?start=pval"},
		{"withoutparam", fields{Username: "Trello_bot"}, args{""}, "https://telegram.me/Trello_bot"},
	}
	for _, tt := range tests {
		c := &Bot{
			ID:          tt.fields.ID,
			Username:    tt.fields.Username,
			token:       tt.fields.token,
			services:    tt.fields.services,
			updatesChan: tt.fields.updatesChan,
			API:         tt.fields.API,
		}
		if got := c.PMURL(tt.args.param); got != tt.want {
			t.Errorf("%q. Bot.PMURL() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestInlineKeyboard_Find(t *testing.T) {
	type fields struct {
		Buttons    []InlineButtons
		FixedWidth bool
		State      string
		MaxRows    int
		RowOffset  int
	}
	type args struct {
		buttonData string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantI   int
		wantJ   int
		wantBut *InlineButton
	}{
		{"buttonFound", fields{Buttons: []InlineButtons{{{Data: "datafield", Text: "text"}}}}, args{"datafield"}, 0, 0, &InlineButton{Data: "datafield", Text: "text"}},
		{"buttonNotFound", fields{Buttons: []InlineButtons{{{Data: "datafield", Text: "text"}}}}, args{"otherdatafield"}, -1, -1, nil},
	}
	for _, tt := range tests {
		keyboard := &InlineKeyboard{
			Buttons:    tt.fields.Buttons,
			FixedWidth: tt.fields.FixedWidth,
			State:      tt.fields.State,
			MaxRows:    tt.fields.MaxRows,
			RowOffset:  tt.fields.RowOffset,
		}
		gotI, gotJ, gotBut := keyboard.Find(tt.args.buttonData)
		if gotI != tt.wantI {
			t.Errorf("%q. InlineKeyboard.Find() gotI = %v, want %v", tt.name, gotI, tt.wantI)
		}
		if gotJ != tt.wantJ {
			t.Errorf("%q. InlineKeyboard.Find() gotJ = %v, want %v", tt.name, gotJ, tt.wantJ)
		}
		if !reflect.DeepEqual(gotBut, tt.wantBut) {
			t.Errorf("%q. InlineKeyboard.Find() gotBut = %v, want %v", tt.name, gotBut, tt.wantBut)
		}
	}
}

func TestInlineKeyboard_EditText(t *testing.T) {
	type fields struct {
		Buttons    []InlineButtons
		FixedWidth bool
		State      string
		MaxRows    int
		RowOffset  int
	}
	type args struct {
		buttonData string
		newText    string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   []InlineButtons
	}{
		{"buttonFound", fields{Buttons: []InlineButtons{{{Data: "datafield", Text: "text"}}}}, args{"datafield", "newtext"}, []InlineButtons{{{Data: "datafield", Text: "newtext"}}}},
		{"buttonNotFound", fields{Buttons: []InlineButtons{{{Data: "datafield", Text: "text"}}}}, args{"wrongdatafield", "newtext"}, []InlineButtons{{{Data: "datafield", Text: "text"}}}},
	}
	for _, tt := range tests {
		keyboard := &InlineKeyboard{
			Buttons:    tt.fields.Buttons,
			FixedWidth: tt.fields.FixedWidth,
			State:      tt.fields.State,
			MaxRows:    tt.fields.MaxRows,
			RowOffset:  tt.fields.RowOffset,
		}
		keyboard.EditText(tt.args.buttonData, tt.args.newText)
		if !reflect.DeepEqual(keyboard.Buttons, tt.want) {
			t.Errorf("%q. InlineKeyboard.EditText() got = %v, want %v", tt.name, keyboard.Buttons, tt.want)
		}
	}
}

func TestInlineKeyboard_AddPMSwitchButton(t *testing.T) {
	type fields struct {
		Buttons    []InlineButtons
		FixedWidth bool
		State      string
		MaxRows    int
		RowOffset  int
	}
	type args struct {
		b     *Bot
		text  string
		param string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   []InlineButtons
	}{
		{"test1", fields{Buttons: []InlineButtons{{{Data: "datafield", Text: "text"}}}}, args{&Bot{Username: "Trello_bot"}, "Open PM", "pval"}, []InlineButtons{{{URL: "https://telegram.me/Trello_bot?start=pval", Text: "Open PM"}}, {{Data: "datafield", Text: "text"}}}},
	}

	for _, tt := range tests {
		keyboard := &InlineKeyboard{
			Buttons:    tt.fields.Buttons,
			FixedWidth: tt.fields.FixedWidth,
			State:      tt.fields.State,
			MaxRows:    tt.fields.MaxRows,
			RowOffset:  tt.fields.RowOffset,
		}
		keyboard.AddPMSwitchButton(tt.args.b, tt.args.text, tt.args.param)
		if !reflect.DeepEqual(keyboard.Buttons, tt.want) {
			t.Errorf("%q. InlineKeyboard.AddPMSwitchButton() got = %v, want %v", tt.name, keyboard.Buttons, tt.want)
		}
	}
}

func TestInlineKeyboard_AppendRows(t *testing.T) {
	type fields struct {
		Buttons    []InlineButtons
		FixedWidth bool
		State      string
		MaxRows    int
		RowOffset  int
	}
	type args struct {
		buttons []InlineButtons
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   []InlineButtons
	}{
		{"test1", fields{Buttons: []InlineButtons{{{Data: "datafield", Text: "text"}}}}, args{[]InlineButtons{{{Data: "datafield2", Text: "text2"}}}}, []InlineButtons{{{Data: "datafield", Text: "text"}}, {{Data: "datafield2", Text: "text2"}}}},
	}
	for _, tt := range tests {
		keyboard := &InlineKeyboard{
			Buttons:    tt.fields.Buttons,
			FixedWidth: tt.fields.FixedWidth,
			State:      tt.fields.State,
			MaxRows:    tt.fields.MaxRows,
			RowOffset:  tt.fields.RowOffset,
		}
		keyboard.AppendRows(tt.args.buttons...)
		if !reflect.DeepEqual(keyboard.Buttons, tt.want) {
			t.Errorf("%q. InlineKeyboard.AppendRows() got = %v, want %v", tt.name, keyboard.Buttons, tt.want)
		}
	}
}

func TestInlineKeyboard_PrependRows(t *testing.T) {
	type fields struct {
		Buttons    []InlineButtons
		FixedWidth bool
		State      string
		MaxRows    int
		RowOffset  int
	}
	type args struct {
		buttons []InlineButtons
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   []InlineButtons
	}{
		{"test1", fields{Buttons: []InlineButtons{{{Data: "datafield", Text: "text"}}}}, args{[]InlineButtons{{{Data: "datafield2", Text: "text2"}}}}, []InlineButtons{{{Data: "datafield2", Text: "text2"}}, {{Data: "datafield", Text: "text"}}}},
	}
	for _, tt := range tests {
		keyboard := &InlineKeyboard{
			Buttons:    tt.fields.Buttons,
			FixedWidth: tt.fields.FixedWidth,
			State:      tt.fields.State,
			MaxRows:    tt.fields.MaxRows,
			RowOffset:  tt.fields.RowOffset,
		}
		keyboard.PrependRows(tt.args.buttons...)
		if !reflect.DeepEqual(keyboard.Buttons, tt.want) {
			t.Errorf("%q. InlineKeyboard.PrependRows() got = %v, want %v", tt.name, keyboard.Buttons, tt.want)
		}
	}
}

func TestInlineButtons_Append(t *testing.T) {
	type args struct {
		data string
		text string
	}
	tests := []struct {
		name    string
		buttons InlineButtons
		args    args
		want    InlineButtons
	}{
		{"test1", InlineButtons{{Data: "datafield", Text: "text"}}, args{data: "datafield2", text: "text2"}, InlineButtons{{Data: "datafield", Text: "text"}, {Data: "datafield2", Text: "text2"}}},
	}
	for _, tt := range tests {
		tt.buttons.Append(tt.args.data, tt.args.text)
		if !reflect.DeepEqual(tt.buttons, tt.want) {
			t.Errorf("%q. InlineButtons.Append() got = %v, want %v", tt.name, tt.buttons, tt.want)
		}
	}
}

func TestInlineButtons_Prepend(t *testing.T) {
	type args struct {
		data string
		text string
	}
	tests := []struct {
		name    string
		buttons InlineButtons
		args    args
		want    InlineButtons
	}{
		{"test1", InlineButtons{{Data: "datafield", Text: "text"}}, args{data: "datafield2", text: "text2"}, InlineButtons{{Data: "datafield2", Text: "text2"}, {Data: "datafield", Text: "text"}}},
	}
	for _, tt := range tests {
		tt.buttons.Prepend(tt.args.data, tt.args.text)
		if !reflect.DeepEqual(tt.buttons, tt.want) {
			t.Errorf("%q. InlineButtons.Prepend() got = %v, want %v", tt.name, tt.buttons, tt.want)
		}
	}
}

func TestInlineButtons_AppendWithState(t *testing.T) {
	type args struct {
		state int
		data  string
		text  string
	}
	tests := []struct {
		name    string
		buttons InlineButtons
		args    args
		want    InlineButtons
	}{
		{"test1", InlineButtons{{Data: "datafield", Text: "text"}}, args{data: "datafield2", text: "text2", state: 2}, InlineButtons{{Data: "datafield", Text: "text"}, {Data: "datafield2", Text: "text2", State: 2}}},
	}
	for _, tt := range tests {
		tt.buttons.AppendWithState(tt.args.state, tt.args.data, tt.args.text)
		if !reflect.DeepEqual(tt.buttons, tt.want) {
			t.Errorf("%q. InlineButtons.AppendWithState() got = %v, want %v", tt.name, tt.buttons, tt.want)
		}
	}
}

func TestInlineButtons_PrependWithState(t *testing.T) {
	type args struct {
		state int
		data  string
		text  string
	}
	tests := []struct {
		name    string
		buttons InlineButtons
		args    args
		want    InlineButtons
	}{
		{"test1", InlineButtons{{Data: "datafield", Text: "text"}}, args{data: "datafield2", text: "text2", state: 2}, InlineButtons{{Data: "datafield2", Text: "text2", State: 2}, {Data: "datafield", Text: "text"}}},
	}
	for _, tt := range tests {
		tt.buttons.PrependWithState(tt.args.state, tt.args.data, tt.args.text)
		if !reflect.DeepEqual(tt.buttons, tt.want) {
			t.Errorf("%q. InlineButtons.PrependWithState() got = %v, want %v", tt.name, tt.buttons, tt.want)
		}
	}
}

func TestInlineButtons_AddURL(t *testing.T) {
	type args struct {
		url  string
		text string
	}
	tests := []struct {
		name    string
		buttons InlineButtons
		args    args
		want    InlineButtons
	}{
		{"test1", InlineButtons{{Data: "datafield", Text: "text"}}, args{url: "https://integram.org", text: "integram"}, InlineButtons{{Data: "datafield", Text: "text"}, {URL: "https://integram.org", Text: "integram"}}},
	}
	for _, tt := range tests {
		tt.buttons.AddURL(tt.args.url, tt.args.text)
		if !reflect.DeepEqual(tt.buttons, tt.want) {
			t.Errorf("%q. InlineButtons.AddURL() got = %v, want %v", tt.name, tt.buttons, tt.want)
		}
	}
}

func TestInlineButtons_Markup(t *testing.T) {
	type args struct {
		columns int
		state   string
	}
	tests := []struct {
		name    string
		buttons InlineButtons
		args    args
		want    InlineKeyboard
	}{
		{"3 buttons 1 columns", InlineButtons{{Data: "data1", Text: "text1"}, {Data: "data2", Text: "text2"}, {Data: "data3", Text: "text3"}}, args{columns: 1, state: "stateval"}, InlineKeyboard{State: "stateval", Buttons: []InlineButtons{{{Data: "data1", Text: "text1"}}, {{Data: "data2", Text: "text2"}}, {{Data: "data3", Text: "text3"}}}}},
		{"3 buttons 2 columns", InlineButtons{{Data: "data1", Text: "text1"}, {Data: "data2", Text: "text2"}, {Data: "data3", Text: "text3"}}, args{columns: 2, state: "stateval"}, InlineKeyboard{State: "stateval", Buttons: []InlineButtons{{{Data: "data1", Text: "text1"}, {Data: "data2", Text: "text2"}}, {{Data: "data3", Text: "text3"}}}}},
		{"3 buttons 3 columns", InlineButtons{{Data: "data1", Text: "text1"}, {Data: "data2", Text: "text2"}, {Data: "data3", Text: "text3"}}, args{columns: 3, state: "stateval"}, InlineKeyboard{State: "stateval", Buttons: []InlineButtons{{{Data: "data1", Text: "text1"}, {Data: "data2", Text: "text2"}, {Data: "data3", Text: "text3"}}}}},
		{"1 button 2 columns", InlineButtons{{Data: "data1", Text: "text1"}}, args{columns: 2, state: "stateval"}, InlineKeyboard{State: "stateval", Buttons: []InlineButtons{{{Data: "data1", Text: "text1"}}}}},
	}
	for _, tt := range tests {
		if got := tt.buttons.Markup(tt.args.columns, tt.args.state); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. InlineButtons.Markup() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestInlineKeyboard_Keyboard(t *testing.T) {
	type fields struct {
		Buttons    []InlineButtons
		FixedWidth bool
		State      string
		MaxRows    int
		RowOffset  int
	}
	tests := []struct {
		name   string
		fields fields
		want   InlineKeyboard
	}{
		{"test1", fields{State: "stateval", Buttons: []InlineButtons{{{Data: "data1", Text: "text1"}}, {{Data: "data2", Text: "text2"}}, {{Data: "data3", Text: "text3"}}}}, InlineKeyboard{State: "stateval", Buttons: []InlineButtons{{{Data: "data1", Text: "text1"}}, {{Data: "data2", Text: "text2"}}, {{Data: "data3", Text: "text3"}}}}},
	}
	for _, tt := range tests {
		keyboard := InlineKeyboard{
			Buttons:    tt.fields.Buttons,
			FixedWidth: tt.fields.FixedWidth,
			State:      tt.fields.State,
			MaxRows:    tt.fields.MaxRows,
			RowOffset:  tt.fields.RowOffset,
		}
		if got := keyboard.Keyboard(); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. InlineKeyboard.Keyboard() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestInlineButton_Keyboard(t *testing.T) {
	type fields struct {
		Text              string
		State             int
		URL               string
		Data              string
		SwitchInlineQuery string
		OutOfPagination   bool
	}
	tests := []struct {
		name   string
		fields fields
		want   InlineKeyboard
	}{
		{"test1", fields{Data: "data1", Text: "text1"}, InlineKeyboard{Buttons: []InlineButtons{{{Data: "data1", Text: "text1"}}}}},
	}
	for _, tt := range tests {
		button := InlineButton{
			Text:              tt.fields.Text,
			State:             tt.fields.State,
			URL:               tt.fields.URL,
			Data:              tt.fields.Data,
			SwitchInlineQuery: tt.fields.SwitchInlineQuery,
			OutOfPagination:   tt.fields.OutOfPagination,
		}
		if got := button.Keyboard(); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. InlineButton.Keyboard() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestInlineButtons_Keyboard(t *testing.T) {
	tests := []struct {
		name    string
		buttons InlineButtons
		want    InlineKeyboard
	}{
		{"test1", InlineButtons{{Data: "data1", Text: "text1"}, {Data: "data2", Text: "text2"}, {Data: "data3", Text: "text3"}}, InlineKeyboard{Buttons: []InlineButtons{{{Data: "data1", Text: "text1"}}, {{Data: "data2", Text: "text2"}}, {{Data: "data3", Text: "text3"}}}}},
	}
	for _, tt := range tests {
		if got := tt.buttons.Keyboard(); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. InlineButtons.Keyboard() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestKeyboard_AddRows(t *testing.T) {
	type args struct {
		buttons []Buttons
	}
	tests := []struct {
		name     string
		keyboard Keyboard
		args     args
		want     Keyboard
	}{
		{"test1", []Buttons{{{Text: "text"}}}, args{[]Buttons{{{Text: "text2"}}}}, []Buttons{{{Text: "text"}}, {{Text: "text2"}}}},
	}
	for _, tt := range tests {
		tt.keyboard.AddRows(tt.args.buttons...)
		if !reflect.DeepEqual(tt.keyboard, tt.want) {
			t.Errorf("%q. Keyboard.AddRows() got = %v, want %v", tt.name, tt.keyboard, tt.want)
		}
	}
}

func TestButtons_Prepend(t *testing.T) {
	type args struct {
		data string
		text string
	}
	tests := []struct {
		name    string
		buttons Buttons
		args    args
		want    Buttons
	}{
		{"test1", Buttons{{Text: "text"}}, args{text: "text2"}, Buttons{{Text: "text2"}, {Text: "text"}}},
	}
	for _, tt := range tests {
		tt.buttons.Prepend(tt.args.data, tt.args.text)
		if !reflect.DeepEqual(tt.buttons, tt.want) {
			t.Errorf("%q. Buttons.Prepend() got = %v, want %v", tt.name, tt.buttons, tt.want)
		}
	}
}

func TestButtons_Append(t *testing.T) {
	type args struct {
		data string
		text string
	}
	tests := []struct {
		name    string
		buttons Buttons
		args    args
		want    Buttons
	}{
		{"test1", Buttons{{Text: "text"}}, args{text: "text2"}, Buttons{{Text: "text"}, {Text: "text2"}}},
	}
	for _, tt := range tests {
		tt.buttons.Append(tt.args.data, tt.args.text)
		if !reflect.DeepEqual(tt.buttons, tt.want) {
			t.Errorf("%q. Buttons.Append() got = %v, want %v", tt.name, tt.buttons, tt.want)
		}
	}
}

func TestButtons_InlineButtons(t *testing.T) {
	tests := []struct {
		name    string
		buttons Buttons
		want    InlineButtons
	}{
		{"test1", Buttons{{Data: "data1", Text: "text1"}, {Data: "data2", Text: "text2"}}, InlineButtons{{Data: "data1", Text: "text1"}, {Data: "data2", Text: "text2"}}},
	}
	for _, tt := range tests {
		if got := tt.buttons.InlineButtons(); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. Buttons.InlineButtons() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestButtons_Markup(t *testing.T) {
	type args struct {
		columns int
	}
	tests := []struct {
		name    string
		buttons Buttons
		args    args
		want    Keyboard
	}{
		{"3 buttons 1 column", Buttons{{Data: "data1", Text: "text1"}, {Data: "data2", Text: "text2"}, {Data: "data3", Text: "text3"}}, args{1}, []Buttons{{{Data: "data1", Text: "text1"}}, {{Data: "data2", Text: "text2"}}, {{Data: "data3", Text: "text3"}}}},
		{"3 buttons 2 columns", Buttons{{Data: "data1", Text: "text1"}, {Data: "data2", Text: "text2"}, {Data: "data3", Text: "text3"}}, args{2}, []Buttons{{{Data: "data1", Text: "text1"}, {Data: "data2", Text: "text2"}}, {{Data: "data3", Text: "text3"}}}},
		{"3 buttons 3 columns", Buttons{{Data: "data1", Text: "text1"}, {Data: "data2", Text: "text2"}, {Data: "data3", Text: "text3"}}, args{3}, []Buttons{{{Data: "data1", Text: "text1"}, {Data: "data2", Text: "text2"}, {Data: "data3", Text: "text3"}}}},
	}
	for _, tt := range tests {
		if got := tt.buttons.Markup(tt.args.columns); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. Buttons.Markup() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestButtons_Keyboard(t *testing.T) {
	tests := []struct {
		name    string
		buttons Buttons
		want    Keyboard
	}{
		{"3 buttons", Buttons{{Data: "data1", Text: "text1"}, {Data: "data2", Text: "text2"}, {Data: "data3", Text: "text3"}}, []Buttons{{{Data: "data1", Text: "text1"}}, {{Data: "data2", Text: "text2"}}, {{Data: "data3", Text: "text3"}}}},
	}
	for _, tt := range tests {
		if got := tt.buttons.Keyboard(); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. Buttons.Keyboard() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestButton_Keyboard(t *testing.T) {
	type fields struct {
		Data string
		Text string
	}
	tests := []struct {
		name   string
		fields fields
		want   Keyboard
	}{
		{"test1", fields{Data: "data1", Text: "text1"}, []Buttons{{{Data: "data1", Text: "text1"}}}},
	}
	for _, tt := range tests {
		button := Button{
			Data: tt.fields.Data,
			Text: tt.fields.Text,
		}
		if got := button.Keyboard(); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. Button.Keyboard() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestKeyboard_Keyboard(t *testing.T) {
	tests := []struct {
		name     string
		keyboard Keyboard
		want     Keyboard
	}{
		{"3 buttons", []Buttons{{{Data: "data1", Text: "text1"}}, {{Data: "data2", Text: "text2"}}, {{Data: "data3", Text: "text3"}}}, []Buttons{{{Data: "data1", Text: "text1"}}, {{Data: "data2", Text: "text2"}}, {{Data: "data3", Text: "text3"}}}},
	}
	for _, tt := range tests {
		if got := tt.keyboard.Keyboard(); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. Keyboard.Keyboard() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestContext_FindMessageByEventID(t *testing.T) {
	bt := strings.Split(os.Getenv("INTEGRAM_TEST_BOT_TOKEN"), ":")
	botID, _ := strconv.ParseInt(bt[0], 10, 64)
	db.C("messages").Insert(Message{ID: bson.ObjectIdHex("55b63650ac126a250a5e3735"), EventID: []string{"eventval", "eventotherval"}, MsgID: 1234, ChatID: 123, BotID: botID})

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
		id string
	}
	msgWant := Message{ID: bson.ObjectIdHex("55b63650ac126a250a5e3735"), EventID: []string{"eventval", "eventotherval"}, MsgID: 1234, ChatID: 123, BotID: botID}
	msgWant.om = &OutgoingMessage{Message: msgWant}
	msgWant.om.Message.om = msgWant.om
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *Message
		wantErr bool
	}{
		{"msg exists - 1 val", fields{ServiceName: "servicewithbottoken", db: db, Chat: Chat{ID: 123}}, args{"eventval"}, &msgWant, false},
		{"msg exists - 2 val", fields{ServiceName: "servicewithbottoken", db: db, Chat: Chat{ID: 123}}, args{"eventotherval"}, &msgWant, false},
		{"bad eventid", fields{ServiceName: "servicewithbottoken", db: db, Chat: Chat{ID: 123}}, args{"eventbadval"}, nil, true},
		{"bad chatid ", fields{ServiceName: "servicewithbottoken", db: db, Chat: Chat{ID: 999}}, args{"eventval"}, nil, true},
		{"bad servicename(bot)", fields{ServiceName: "servicewithactions", db: db, Chat: Chat{ID: 123}}, args{"eventval"}, nil, true},
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
		got, err := c.FindMessageByEventID(tt.args.id)
		if (err != nil) != tt.wantErr {
			t.Errorf("%q. Context.FindMessageByEventID() error = %v, wantErr %v", tt.name, err, tt.wantErr)
			continue
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. Context.FindMessageByEventID() = %v, want %v", tt.name, got.om, tt.want.om)
		}
	}
	db.C("messages").RemoveId(bson.ObjectIdHex("55b63650ac126a250a5e3735"))
}

func TestOutgoingMessage_SetChat(t *testing.T) {
	type fields struct {
		Message              Message
		KeyboardHide         bool
		ResizeKeyboard       bool
		KeyboardMarkup       Keyboard
		InlineKeyboardMarkup InlineKeyboard
		Keyboard             bool
		ParseMode            string
		OneTimeKeyboard      bool
		Selective            bool
		ForceReply           bool
		WebPreview           bool
		Silent               bool
		FilePath             string
		FileName             string
		FileType             string
		processed            bool
	}
	type args struct {
		id int64
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *OutgoingMessage
	}{
		{"set from empty", fields{Message: Message{Text: "text", BotID: 1111}}, args{1234}, &OutgoingMessage{Message: Message{Text: "text", BotID: 1111, ChatID: 1234}}},
		{"override", fields{Message: Message{Text: "text", BotID: 1111, ChatID: 1000}}, args{1234}, &OutgoingMessage{Message: Message{Text: "text", BotID: 1111, ChatID: 1234}}},
	}
	for _, tt := range tests {
		m := &OutgoingMessage{
			Message:              tt.fields.Message,
			KeyboardHide:         tt.fields.KeyboardHide,
			ResizeKeyboard:       tt.fields.ResizeKeyboard,
			KeyboardMarkup:       tt.fields.KeyboardMarkup,
			InlineKeyboardMarkup: tt.fields.InlineKeyboardMarkup,
			Keyboard:             tt.fields.Keyboard,
			ParseMode:            tt.fields.ParseMode,
			OneTimeKeyboard:      tt.fields.OneTimeKeyboard,
			Selective:            tt.fields.Selective,
			ForceReply:           tt.fields.ForceReply,
			WebPreview:           tt.fields.WebPreview,
			Silent:               tt.fields.Silent,
			FilePath:             tt.fields.FilePath,
			FileName:             tt.fields.FileName,
			FileType:             tt.fields.FileType,
			processed:            tt.fields.processed,
		}
		if got := m.SetChat(tt.args.id); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. OutgoingMessage.SetChat() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestOutgoingMessage_SetBackupChat(t *testing.T) {
	type fields struct {
		Message              Message
		KeyboardHide         bool
		ResizeKeyboard       bool
		KeyboardMarkup       Keyboard
		InlineKeyboardMarkup InlineKeyboard
		Keyboard             bool
		ParseMode            string
		OneTimeKeyboard      bool
		Selective            bool
		ForceReply           bool
		WebPreview           bool
		Silent               bool
		FilePath             string
		FileName             string
		FileType             string
		processed            bool
	}
	type args struct {
		id int64
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *OutgoingMessage
	}{
		{"set from empty", fields{Message: Message{Text: "text", BotID: 1111, ChatID: 1000}}, args{-1234}, &OutgoingMessage{Message: Message{Text: "text", BotID: 1111, ChatID: 1000, BackupChatID: -1234}}},
		{"override", fields{Message: Message{Text: "text", BotID: 1111, ChatID: 1000, BackupChatID: -1000}}, args{-1234}, &OutgoingMessage{Message: Message{Text: "text", BotID: 1111, ChatID: 1000, BackupChatID: -1234}}},
	}
	for _, tt := range tests {
		m := &OutgoingMessage{
			Message:              tt.fields.Message,
			KeyboardHide:         tt.fields.KeyboardHide,
			ResizeKeyboard:       tt.fields.ResizeKeyboard,
			KeyboardMarkup:       tt.fields.KeyboardMarkup,
			InlineKeyboardMarkup: tt.fields.InlineKeyboardMarkup,
			Keyboard:             tt.fields.Keyboard,
			ParseMode:            tt.fields.ParseMode,
			OneTimeKeyboard:      tt.fields.OneTimeKeyboard,
			Selective:            tt.fields.Selective,
			ForceReply:           tt.fields.ForceReply,
			WebPreview:           tt.fields.WebPreview,
			Silent:               tt.fields.Silent,
			FilePath:             tt.fields.FilePath,
			FileName:             tt.fields.FileName,
			FileType:             tt.fields.FileType,
			processed:            tt.fields.processed,
		}
		if got := m.SetBackupChat(tt.args.id); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. OutgoingMessage.SetBackupChat() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestOutgoingMessage_SetDocument(t *testing.T) {
	type fields struct {
		Message              Message
		KeyboardHide         bool
		ResizeKeyboard       bool
		KeyboardMarkup       Keyboard
		InlineKeyboardMarkup InlineKeyboard
		Keyboard             bool
		ParseMode            string
		OneTimeKeyboard      bool
		Selective            bool
		ForceReply           bool
		WebPreview           bool
		Silent               bool
		FilePath             string
		FileName             string
		FileType             string
		processed            bool
	}
	type args struct {
		localPath string
		fileName  string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *OutgoingMessage
	}{
		{"set from empty", fields{Message: Message{BotID: 1111, ChatID: 1000}}, args{"/tmp/a", "pr.pdf"}, &OutgoingMessage{Message: Message{BotID: 1111, ChatID: 1000}, FileName: "pr.pdf", FilePath: "/tmp/a", FileType: "document"}},
		{"override", fields{Message: Message{BotID: 1111, ChatID: 1000}, FileName: "b.png", FilePath: "/tmp/b", FileType: "image"}, args{"/tmp/a", "pr.pdf"}, &OutgoingMessage{Message: Message{BotID: 1111, ChatID: 1000}, FileName: "pr.pdf", FilePath: "/tmp/a", FileType: "document"}},
	}
	for _, tt := range tests {
		m := &OutgoingMessage{
			Message:              tt.fields.Message,
			KeyboardHide:         tt.fields.KeyboardHide,
			ResizeKeyboard:       tt.fields.ResizeKeyboard,
			KeyboardMarkup:       tt.fields.KeyboardMarkup,
			InlineKeyboardMarkup: tt.fields.InlineKeyboardMarkup,
			Keyboard:             tt.fields.Keyboard,
			ParseMode:            tt.fields.ParseMode,
			OneTimeKeyboard:      tt.fields.OneTimeKeyboard,
			Selective:            tt.fields.Selective,
			ForceReply:           tt.fields.ForceReply,
			WebPreview:           tt.fields.WebPreview,
			Silent:               tt.fields.Silent,
			FilePath:             tt.fields.FilePath,
			FileName:             tt.fields.FileName,
			FileType:             tt.fields.FileType,
			processed:            tt.fields.processed,
		}
		if got := m.SetDocument(tt.args.localPath, tt.args.fileName); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. OutgoingMessage.SetDocument() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestOutgoingMessage_SetImage(t *testing.T) {
	type fields struct {
		Message              Message
		KeyboardHide         bool
		ResizeKeyboard       bool
		KeyboardMarkup       Keyboard
		InlineKeyboardMarkup InlineKeyboard
		Keyboard             bool
		ParseMode            string
		OneTimeKeyboard      bool
		Selective            bool
		ForceReply           bool
		WebPreview           bool
		Silent               bool
		FilePath             string
		FileName             string
		FileType             string
		processed            bool
	}
	type args struct {
		localPath string
		fileName  string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *OutgoingMessage
	}{
		{"set from empty", fields{Message: Message{BotID: 1111, ChatID: 1000}}, args{"/tmp/img", "img.png"}, &OutgoingMessage{Message: Message{BotID: 1111, ChatID: 1000}, FileName: "img.png", FilePath: "/tmp/img", FileType: "image"}},
		{"override", fields{Message: Message{BotID: 1111, ChatID: 1000}, FileName: "b.pdf", FilePath: "/tmp/b", FileType: "document"}, args{"/tmp/img", "img.png"}, &OutgoingMessage{Message: Message{BotID: 1111, ChatID: 1000}, FileName: "img.png", FilePath: "/tmp/img", FileType: "image"}},
	}
	for _, tt := range tests {
		m := &OutgoingMessage{
			Message:              tt.fields.Message,
			KeyboardHide:         tt.fields.KeyboardHide,
			ResizeKeyboard:       tt.fields.ResizeKeyboard,
			KeyboardMarkup:       tt.fields.KeyboardMarkup,
			InlineKeyboardMarkup: tt.fields.InlineKeyboardMarkup,
			Keyboard:             tt.fields.Keyboard,
			ParseMode:            tt.fields.ParseMode,
			OneTimeKeyboard:      tt.fields.OneTimeKeyboard,
			Selective:            tt.fields.Selective,
			ForceReply:           tt.fields.ForceReply,
			WebPreview:           tt.fields.WebPreview,
			Silent:               tt.fields.Silent,
			FilePath:             tt.fields.FilePath,
			FileName:             tt.fields.FileName,
			FileType:             tt.fields.FileType,
			processed:            tt.fields.processed,
		}
		if got := m.SetImage(tt.args.localPath, tt.args.fileName); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. OutgoingMessage.SetImage() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestOutgoingMessage_SetKeyboard(t *testing.T) {
	type fields struct {
		Message              Message
		KeyboardHide         bool
		ResizeKeyboard       bool
		KeyboardMarkup       Keyboard
		InlineKeyboardMarkup InlineKeyboard
		Keyboard             bool
		ParseMode            string
		OneTimeKeyboard      bool
		Selective            bool
		ForceReply           bool
		WebPreview           bool
		Silent               bool
		FilePath             string
		FileName             string
		FileType             string
		processed            bool
	}
	type args struct {
		k         KeyboardMarkup
		selective bool
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *OutgoingMessage
	}{
		{"set from empty", fields{Message: Message{Text: "text", BotID: 1111, ChatID: 1000}}, args{Buttons{{Text: "text1"}, {Text: "text2"}, {Text: "text3"}}, true}, &OutgoingMessage{Message: Message{Text: "text", BotID: 1111, ChatID: 1000}, Keyboard: true, Selective: true, KeyboardMarkup: []Buttons{{{Text: "text1"}}, {{Text: "text2"}}, {{Text: "text3"}}}}},
	}
	for _, tt := range tests {
		m := &OutgoingMessage{
			Message:              tt.fields.Message,
			KeyboardHide:         tt.fields.KeyboardHide,
			ResizeKeyboard:       tt.fields.ResizeKeyboard,
			KeyboardMarkup:       tt.fields.KeyboardMarkup,
			InlineKeyboardMarkup: tt.fields.InlineKeyboardMarkup,
			Keyboard:             tt.fields.Keyboard,
			ParseMode:            tt.fields.ParseMode,
			OneTimeKeyboard:      tt.fields.OneTimeKeyboard,
			Selective:            tt.fields.Selective,
			ForceReply:           tt.fields.ForceReply,
			WebPreview:           tt.fields.WebPreview,
			Silent:               tt.fields.Silent,
			FilePath:             tt.fields.FilePath,
			FileName:             tt.fields.FileName,
			FileType:             tt.fields.FileType,
			processed:            tt.fields.processed,
		}
		if got := m.SetKeyboard(tt.args.k, tt.args.selective); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. OutgoingMessage.SetKeyboard() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestOutgoingMessage_SetInlineKeyboard(t *testing.T) {
	type fields struct {
		Message              Message
		KeyboardHide         bool
		ResizeKeyboard       bool
		KeyboardMarkup       Keyboard
		InlineKeyboardMarkup InlineKeyboard
		Keyboard             bool
		ParseMode            string
		OneTimeKeyboard      bool
		Selective            bool
		ForceReply           bool
		WebPreview           bool
		Silent               bool
		FilePath             string
		FileName             string
		FileType             string
		processed            bool
	}
	type args struct {
		k InlineKeyboardMarkup
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *OutgoingMessage
	}{
		{"set from empty", fields{Message: Message{Text: "text", BotID: 1111, ChatID: 1000}}, args{InlineButtons{{Data: "data1", Text: "text1"}, {Data: "data2", Text: "text2"}, {Data: "data3", Text: "text3"}}}, &OutgoingMessage{Message: Message{Text: "text", BotID: 1111, ChatID: 1000}, InlineKeyboardMarkup: InlineKeyboard{Buttons: []InlineButtons{{{Data: "data1", Text: "text1"}}, {{Data: "data2", Text: "text2"}}, {{Data: "data3", Text: "text3"}}}}}},
	}
	for _, tt := range tests {
		m := &OutgoingMessage{
			Message:              tt.fields.Message,
			KeyboardHide:         tt.fields.KeyboardHide,
			ResizeKeyboard:       tt.fields.ResizeKeyboard,
			KeyboardMarkup:       tt.fields.KeyboardMarkup,
			InlineKeyboardMarkup: tt.fields.InlineKeyboardMarkup,
			Keyboard:             tt.fields.Keyboard,
			ParseMode:            tt.fields.ParseMode,
			OneTimeKeyboard:      tt.fields.OneTimeKeyboard,
			Selective:            tt.fields.Selective,
			ForceReply:           tt.fields.ForceReply,
			WebPreview:           tt.fields.WebPreview,
			Silent:               tt.fields.Silent,
			FilePath:             tt.fields.FilePath,
			FileName:             tt.fields.FileName,
			FileType:             tt.fields.FileType,
			processed:            tt.fields.processed,
		}
		if got := m.SetInlineKeyboard(tt.args.k); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. OutgoingMessage.SetInlineKeyboard() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestOutgoingMessage_SetSelective(t *testing.T) {
	type fields struct {
		Message              Message
		KeyboardHide         bool
		ResizeKeyboard       bool
		KeyboardMarkup       Keyboard
		InlineKeyboardMarkup InlineKeyboard
		Keyboard             bool
		ParseMode            string
		OneTimeKeyboard      bool
		Selective            bool
		ForceReply           bool
		WebPreview           bool
		Silent               bool
		FilePath             string
		FileName             string
		FileType             string
		processed            bool
	}
	type args struct {
		b bool
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *OutgoingMessage
	}{
		{"set from empty", fields{Message: Message{Text: "text", BotID: 1111, ChatID: 1000}}, args{true}, &OutgoingMessage{Message: Message{Text: "text", BotID: 1111, ChatID: 1000}, Selective: true}},
	}
	for _, tt := range tests {
		m := &OutgoingMessage{
			Message:              tt.fields.Message,
			KeyboardHide:         tt.fields.KeyboardHide,
			ResizeKeyboard:       tt.fields.ResizeKeyboard,
			KeyboardMarkup:       tt.fields.KeyboardMarkup,
			InlineKeyboardMarkup: tt.fields.InlineKeyboardMarkup,
			Keyboard:             tt.fields.Keyboard,
			ParseMode:            tt.fields.ParseMode,
			OneTimeKeyboard:      tt.fields.OneTimeKeyboard,
			Selective:            tt.fields.Selective,
			ForceReply:           tt.fields.ForceReply,
			WebPreview:           tt.fields.WebPreview,
			Silent:               tt.fields.Silent,
			FilePath:             tt.fields.FilePath,
			FileName:             tt.fields.FileName,
			FileType:             tt.fields.FileType,
			processed:            tt.fields.processed,
		}
		if got := m.SetSelective(tt.args.b); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. OutgoingMessage.SetSelective() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestOutgoingMessage_SetSilent(t *testing.T) {
	type fields struct {
		Message              Message
		KeyboardHide         bool
		ResizeKeyboard       bool
		KeyboardMarkup       Keyboard
		InlineKeyboardMarkup InlineKeyboard
		Keyboard             bool
		ParseMode            string
		OneTimeKeyboard      bool
		Selective            bool
		ForceReply           bool
		WebPreview           bool
		Silent               bool
		FilePath             string
		FileName             string
		FileType             string
		processed            bool
	}
	type args struct {
		b bool
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *OutgoingMessage
	}{
		{"set from empty", fields{Message: Message{Text: "text", BotID: 1111, ChatID: 1000}}, args{true}, &OutgoingMessage{Message: Message{Text: "text", BotID: 1111, ChatID: 1000}, Silent: true}},
	}
	for _, tt := range tests {
		m := &OutgoingMessage{
			Message:              tt.fields.Message,
			KeyboardHide:         tt.fields.KeyboardHide,
			ResizeKeyboard:       tt.fields.ResizeKeyboard,
			KeyboardMarkup:       tt.fields.KeyboardMarkup,
			InlineKeyboardMarkup: tt.fields.InlineKeyboardMarkup,
			Keyboard:             tt.fields.Keyboard,
			ParseMode:            tt.fields.ParseMode,
			OneTimeKeyboard:      tt.fields.OneTimeKeyboard,
			Selective:            tt.fields.Selective,
			ForceReply:           tt.fields.ForceReply,
			WebPreview:           tt.fields.WebPreview,
			Silent:               tt.fields.Silent,
			FilePath:             tt.fields.FilePath,
			FileName:             tt.fields.FileName,
			FileType:             tt.fields.FileType,
			processed:            tt.fields.processed,
		}
		if got := m.SetSilent(tt.args.b); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. OutgoingMessage.SetSilent() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestOutgoingMessage_SetOneTimeKeyboard(t *testing.T) {
	type fields struct {
		Message              Message
		KeyboardHide         bool
		ResizeKeyboard       bool
		KeyboardMarkup       Keyboard
		InlineKeyboardMarkup InlineKeyboard
		Keyboard             bool
		ParseMode            string
		OneTimeKeyboard      bool
		Selective            bool
		ForceReply           bool
		WebPreview           bool
		Silent               bool
		FilePath             string
		FileName             string
		FileType             string
		processed            bool
	}
	type args struct {
		b bool
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *OutgoingMessage
	}{
		{"set from empty", fields{Message: Message{Text: "text", BotID: 1111, ChatID: 1000}}, args{true}, &OutgoingMessage{Message: Message{Text: "text", BotID: 1111, ChatID: 1000}, OneTimeKeyboard: true}},
	}
	for _, tt := range tests {
		m := &OutgoingMessage{
			Message:              tt.fields.Message,
			KeyboardHide:         tt.fields.KeyboardHide,
			ResizeKeyboard:       tt.fields.ResizeKeyboard,
			KeyboardMarkup:       tt.fields.KeyboardMarkup,
			InlineKeyboardMarkup: tt.fields.InlineKeyboardMarkup,
			Keyboard:             tt.fields.Keyboard,
			ParseMode:            tt.fields.ParseMode,
			OneTimeKeyboard:      tt.fields.OneTimeKeyboard,
			Selective:            tt.fields.Selective,
			ForceReply:           tt.fields.ForceReply,
			WebPreview:           tt.fields.WebPreview,
			Silent:               tt.fields.Silent,
			FilePath:             tt.fields.FilePath,
			FileName:             tt.fields.FileName,
			FileType:             tt.fields.FileType,
			processed:            tt.fields.processed,
		}
		if got := m.SetOneTimeKeyboard(tt.args.b); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. OutgoingMessage.SetOneTimeKeyboard() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestOutgoingMessage_SetResizeKeyboard(t *testing.T) {
	type fields struct {
		Message              Message
		KeyboardHide         bool
		ResizeKeyboard       bool
		KeyboardMarkup       Keyboard
		InlineKeyboardMarkup InlineKeyboard
		Keyboard             bool
		ParseMode            string
		OneTimeKeyboard      bool
		Selective            bool
		ForceReply           bool
		WebPreview           bool
		Silent               bool
		FilePath             string
		FileName             string
		FileType             string
		processed            bool
	}
	type args struct {
		b bool
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *OutgoingMessage
	}{
		{"set from empty", fields{Message: Message{Text: "text", BotID: 1111, ChatID: 1000}}, args{true}, &OutgoingMessage{Message: Message{Text: "text", BotID: 1111, ChatID: 1000}, ResizeKeyboard: true}},
	}
	for _, tt := range tests {
		m := &OutgoingMessage{
			Message:              tt.fields.Message,
			KeyboardHide:         tt.fields.KeyboardHide,
			ResizeKeyboard:       tt.fields.ResizeKeyboard,
			KeyboardMarkup:       tt.fields.KeyboardMarkup,
			InlineKeyboardMarkup: tt.fields.InlineKeyboardMarkup,
			Keyboard:             tt.fields.Keyboard,
			ParseMode:            tt.fields.ParseMode,
			OneTimeKeyboard:      tt.fields.OneTimeKeyboard,
			Selective:            tt.fields.Selective,
			ForceReply:           tt.fields.ForceReply,
			WebPreview:           tt.fields.WebPreview,
			Silent:               tt.fields.Silent,
			FilePath:             tt.fields.FilePath,
			FileName:             tt.fields.FileName,
			FileType:             tt.fields.FileType,
			processed:            tt.fields.processed,
		}
		if got := m.SetResizeKeyboard(tt.args.b); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. OutgoingMessage.SetResizeKeyboard() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestIncomingMessage_SetCallbackAction(t *testing.T) {
	type fields struct {
		Message               Message
		From                  User
		Chat                  Chat
		ForwardFrom           *User
		ForwardDate           time.Time
		ReplyToMessage        *Message
		ForwardFromChat       *Chat
		EditDate              int
		Entities              *[]tg.MessageEntity
		Audio                 *tg.Audio
		Document              *tg.Document
		Photo                 *[]tg.PhotoSize
		Sticker               *tg.Sticker
		Video                 *tg.Video
		Voice                 *tg.Voice
		Caption               string
		Contact               *tg.Contact
		Location              *tg.Location
		Venue                 *tg.Venue
		NewChatMember         []*User
		LeftChatMember        *User
		NewChatTitle          string
		NewChatPhoto          *[]tg.PhotoSize
		DeleteChatPhoto       bool
		GroupChatCreated      bool
		SuperGroupChatCreated bool
		ChannelChatCreated    bool
		MigrateToChatID       int64
		MigrateFromChatID     int64
		PinnedMessage         *Message
		needToUpdateDB        bool
	}
	type args struct {
		handlerFunc interface{}
		args        []interface{}
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *IncomingMessage
	}{
		{"set from empty", fields{Message: Message{Text: "text", BotID: 1111, ChatID: 1000}}, args{dumbFuncWithContextAndParam, []interface{}{true}}, &IncomingMessage{Message: Message{Text: "text", BotID: 1111, ChatID: 1000, OnCallbackAction: "github.com/requilence/integram.dumbFuncWithContextAndParam", OnCallbackData: []byte{12, 255, 129, 2, 1, 2, 255, 130, 0, 1, 16, 0, 0, 13, 255, 130, 0, 1, 4, 98, 111, 111, 108, 2, 2, 0, 1}}}},
		{"override exists value", fields{Message: Message{Text: "text", BotID: 1111, ChatID: 1000}}, args{dumbFuncWithContextAndParams, []interface{}{1, 2}}, &IncomingMessage{Message: Message{Text: "text", BotID: 1111, ChatID: 1000, OnCallbackAction: "github.com/requilence/integram.dumbFuncWithContextAndParams", OnCallbackData: []byte{12, 255, 129, 2, 1, 2, 255, 130, 0, 1, 16, 0, 0, 20, 255, 130, 0, 2, 3, 105, 110, 116, 4, 2, 0, 2, 3, 105, 110, 116, 4, 2, 0, 4}}}},
	}
	for _, tt := range tests {
		m := &IncomingMessage{
			Message:               tt.fields.Message,
			From:                  tt.fields.From,
			Chat:                  tt.fields.Chat,
			ForwardFrom:           tt.fields.ForwardFrom,
			ForwardDate:           tt.fields.ForwardDate,
			ReplyToMessage:        tt.fields.ReplyToMessage,
			ForwardFromChat:       tt.fields.ForwardFromChat,
			EditDate:              tt.fields.EditDate,
			Entities:              tt.fields.Entities,
			Audio:                 tt.fields.Audio,
			Document:              tt.fields.Document,
			Photo:                 tt.fields.Photo,
			Sticker:               tt.fields.Sticker,
			Video:                 tt.fields.Video,
			Voice:                 tt.fields.Voice,
			Caption:               tt.fields.Caption,
			Contact:               tt.fields.Contact,
			Location:              tt.fields.Location,
			Venue:                 tt.fields.Venue,
			NewChatMembers:        tt.fields.NewChatMember,
			LeftChatMember:        tt.fields.LeftChatMember,
			NewChatTitle:          tt.fields.NewChatTitle,
			NewChatPhoto:          tt.fields.NewChatPhoto,
			DeleteChatPhoto:       tt.fields.DeleteChatPhoto,
			GroupChatCreated:      tt.fields.GroupChatCreated,
			SuperGroupChatCreated: tt.fields.SuperGroupChatCreated,
			ChannelChatCreated:    tt.fields.ChannelChatCreated,
			MigrateToChatID:       tt.fields.MigrateToChatID,
			MigrateFromChatID:     tt.fields.MigrateFromChatID,
			PinnedMessage:         tt.fields.PinnedMessage,
			needToUpdateDB:        tt.fields.needToUpdateDB,
		}
		if got := m.SetCallbackAction(tt.args.handlerFunc, tt.args.args...); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. IncomingMessage.SetCallbackAction() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestOutgoingMessage_SetCallbackAction(t *testing.T) {
	type fields struct {
		Message              Message
		KeyboardHide         bool
		ResizeKeyboard       bool
		KeyboardMarkup       Keyboard
		InlineKeyboardMarkup InlineKeyboard
		Keyboard             bool
		ParseMode            string
		OneTimeKeyboard      bool
		Selective            bool
		ForceReply           bool
		WebPreview           bool
		Silent               bool
		FilePath             string
		FileName             string
		FileType             string
		processed            bool
	}
	type args struct {
		handlerFunc interface{}
		args        []interface{}
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *OutgoingMessage
	}{
		{"set from empty", fields{Message: Message{Text: "text", BotID: 1111, ChatID: 1000}}, args{dumbFuncWithContextAndParam, []interface{}{true}}, &OutgoingMessage{Message: Message{Text: "text", BotID: 1111, ChatID: 1000, OnCallbackAction: "github.com/requilence/integram.dumbFuncWithContextAndParam", OnCallbackData: []byte{12, 255, 129, 2, 1, 2, 255, 130, 0, 1, 16, 0, 0, 13, 255, 130, 0, 1, 4, 98, 111, 111, 108, 2, 2, 0, 1}}}},
	}
	for _, tt := range tests {
		m := &OutgoingMessage{
			Message:              tt.fields.Message,
			KeyboardHide:         tt.fields.KeyboardHide,
			ResizeKeyboard:       tt.fields.ResizeKeyboard,
			KeyboardMarkup:       tt.fields.KeyboardMarkup,
			InlineKeyboardMarkup: tt.fields.InlineKeyboardMarkup,
			Keyboard:             tt.fields.Keyboard,
			ParseMode:            tt.fields.ParseMode,
			OneTimeKeyboard:      tt.fields.OneTimeKeyboard,
			Selective:            tt.fields.Selective,
			ForceReply:           tt.fields.ForceReply,
			WebPreview:           tt.fields.WebPreview,
			Silent:               tt.fields.Silent,
			FilePath:             tt.fields.FilePath,
			FileName:             tt.fields.FileName,
			FileType:             tt.fields.FileType,
			processed:            tt.fields.processed,
		}
		if got := m.SetCallbackAction(tt.args.handlerFunc, tt.args.args...); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. OutgoingMessage.SetCallbackAction() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestIncomingMessage_SetReplyAction(t *testing.T) {
	type fields struct {
		Message               Message
		From                  User
		Chat                  Chat
		ForwardFrom           *User
		ForwardDate           time.Time
		ReplyToMessage        *Message
		ForwardFromChat       *Chat
		EditDate              int
		Entities              *[]tg.MessageEntity
		Audio                 *tg.Audio
		Document              *tg.Document
		Photo                 *[]tg.PhotoSize
		Sticker               *tg.Sticker
		Video                 *tg.Video
		Voice                 *tg.Voice
		Caption               string
		Contact               *tg.Contact
		Location              *tg.Location
		Venue                 *tg.Venue
		NewChatMembers        []*User
		LeftChatMember        *User
		NewChatTitle          string
		NewChatPhoto          *[]tg.PhotoSize
		DeleteChatPhoto       bool
		GroupChatCreated      bool
		SuperGroupChatCreated bool
		ChannelChatCreated    bool
		MigrateToChatID       int64
		MigrateFromChatID     int64
		PinnedMessage         *Message
		needToUpdateDB        bool
	}
	type args struct {
		handlerFunc interface{}
		args        []interface{}
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *IncomingMessage
	}{
		{"set from empty", fields{Message: Message{Text: "text", BotID: 1111, ChatID: 1000}}, args{dumbFuncWithContextAndParam, []interface{}{true}}, &IncomingMessage{Message: Message{Text: "text", BotID: 1111, ChatID: 1000, OnReplyAction: "github.com/requilence/integram.dumbFuncWithContextAndParam", OnReplyData: []byte{12, 255, 129, 2, 1, 2, 255, 130, 0, 1, 16, 0, 0, 13, 255, 130, 0, 1, 4, 98, 111, 111, 108, 2, 2, 0, 1}}}},
	}
	for _, tt := range tests {
		m := &IncomingMessage{
			Message:               tt.fields.Message,
			From:                  tt.fields.From,
			Chat:                  tt.fields.Chat,
			ForwardFrom:           tt.fields.ForwardFrom,
			ForwardDate:           tt.fields.ForwardDate,
			ReplyToMessage:        tt.fields.ReplyToMessage,
			ForwardFromChat:       tt.fields.ForwardFromChat,
			EditDate:              tt.fields.EditDate,
			Entities:              tt.fields.Entities,
			Audio:                 tt.fields.Audio,
			Document:              tt.fields.Document,
			Photo:                 tt.fields.Photo,
			Sticker:               tt.fields.Sticker,
			Video:                 tt.fields.Video,
			Voice:                 tt.fields.Voice,
			Caption:               tt.fields.Caption,
			Contact:               tt.fields.Contact,
			Location:              tt.fields.Location,
			Venue:                 tt.fields.Venue,
			NewChatMembers:        tt.fields.NewChatMembers,
			LeftChatMember:        tt.fields.LeftChatMember,
			NewChatTitle:          tt.fields.NewChatTitle,
			NewChatPhoto:          tt.fields.NewChatPhoto,
			DeleteChatPhoto:       tt.fields.DeleteChatPhoto,
			GroupChatCreated:      tt.fields.GroupChatCreated,
			SuperGroupChatCreated: tt.fields.SuperGroupChatCreated,
			ChannelChatCreated:    tt.fields.ChannelChatCreated,
			MigrateToChatID:       tt.fields.MigrateToChatID,
			MigrateFromChatID:     tt.fields.MigrateFromChatID,
			PinnedMessage:         tt.fields.PinnedMessage,
			needToUpdateDB:        tt.fields.needToUpdateDB,
		}
		if got := m.SetReplyAction(tt.args.handlerFunc, tt.args.args...); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. IncomingMessage.SetReplyAction() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestOutgoingMessage_SetReplyAction(t *testing.T) {
	type fields struct {
		Message              Message
		KeyboardHide         bool
		ResizeKeyboard       bool
		KeyboardMarkup       Keyboard
		InlineKeyboardMarkup InlineKeyboard
		Keyboard             bool
		ParseMode            string
		OneTimeKeyboard      bool
		Selective            bool
		ForceReply           bool
		WebPreview           bool
		Silent               bool
		FilePath             string
		FileName             string
		FileType             string
		processed            bool
	}
	type args struct {
		handlerFunc interface{}
		args        []interface{}
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *OutgoingMessage
	}{
		{"set from empty", fields{Message: Message{Text: "text", BotID: 1111, ChatID: 1000}}, args{dumbFuncWithContextAndParam, []interface{}{true}}, &OutgoingMessage{Message: Message{Text: "text", BotID: 1111, ChatID: 1000, OnReplyAction: "github.com/requilence/integram.dumbFuncWithContextAndParam", OnReplyData: []byte{12, 255, 129, 2, 1, 2, 255, 130, 0, 1, 16, 0, 0, 13, 255, 130, 0, 1, 4, 98, 111, 111, 108, 2, 2, 0, 1}}}},
	}
	for _, tt := range tests {
		m := &OutgoingMessage{
			Message:              tt.fields.Message,
			KeyboardHide:         tt.fields.KeyboardHide,
			ResizeKeyboard:       tt.fields.ResizeKeyboard,
			KeyboardMarkup:       tt.fields.KeyboardMarkup,
			InlineKeyboardMarkup: tt.fields.InlineKeyboardMarkup,
			Keyboard:             tt.fields.Keyboard,
			ParseMode:            tt.fields.ParseMode,
			OneTimeKeyboard:      tt.fields.OneTimeKeyboard,
			Selective:            tt.fields.Selective,
			ForceReply:           tt.fields.ForceReply,
			WebPreview:           tt.fields.WebPreview,
			Silent:               tt.fields.Silent,
			FilePath:             tt.fields.FilePath,
			FileName:             tt.fields.FileName,
			FileType:             tt.fields.FileType,
			processed:            tt.fields.processed,
		}
		if got := m.SetReplyAction(tt.args.handlerFunc, tt.args.args...); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. OutgoingMessage.SetReplyAction() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestMessage_SetCallbackAction(t *testing.T) {
	type fields struct {
		ID               bson.ObjectId
		EventID          []string
		MsgID            int
		InlineMsgID      string
		BotID            int64
		FromID           int64
		ChatID           int64
		BackupChatID     int64
		ReplyToMsgID     int
		Date             time.Time
		Text             string
		AntiFlood        bool
		Deleted          bool
		OnCallbackAction string
		OnCallbackData   []byte
		OnReplyAction    string
		OnReplyData      []byte
		om               *OutgoingMessage
	}
	type args struct {
		handlerFunc interface{}
		args        []interface{}
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *Message
	}{
		{"set from empty", fields{Text: "text", BotID: 1111, ChatID: 1000}, args{dumbFuncWithContextAndParam, []interface{}{true}}, &Message{Text: "text", BotID: 1111, ChatID: 1000, OnCallbackAction: "github.com/requilence/integram.dumbFuncWithContextAndParam", OnCallbackData: []byte{12, 255, 129, 2, 1, 2, 255, 130, 0, 1, 16, 0, 0, 13, 255, 130, 0, 1, 4, 98, 111, 111, 108, 2, 2, 0, 1}}},
	}
	for _, tt := range tests {
		m := &Message{
			ID:               tt.fields.ID,
			EventID:          tt.fields.EventID,
			MsgID:            tt.fields.MsgID,
			InlineMsgID:      tt.fields.InlineMsgID,
			BotID:            tt.fields.BotID,
			FromID:           tt.fields.FromID,
			ChatID:           tt.fields.ChatID,
			BackupChatID:     tt.fields.BackupChatID,
			ReplyToMsgID:     tt.fields.ReplyToMsgID,
			Date:             tt.fields.Date,
			Text:             tt.fields.Text,
			AntiFlood:        tt.fields.AntiFlood,
			Deleted:          tt.fields.Deleted,
			OnCallbackAction: tt.fields.OnCallbackAction,
			OnCallbackData:   tt.fields.OnCallbackData,
			OnReplyAction:    tt.fields.OnReplyAction,
			OnReplyData:      tt.fields.OnReplyData,
			om:               tt.fields.om,
		}
		if got := m.SetCallbackAction(tt.args.handlerFunc, tt.args.args...); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. Message.SetCallbackAction() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestMessage_SetReplyAction(t *testing.T) {
	type fields struct {
		ID               bson.ObjectId
		EventID          []string
		MsgID            int
		InlineMsgID      string
		BotID            int64
		FromID           int64
		ChatID           int64
		BackupChatID     int64
		ReplyToMsgID     int
		Date             time.Time
		Text             string
		AntiFlood        bool
		Deleted          bool
		OnCallbackAction string
		OnCallbackData   []byte
		OnReplyAction    string
		OnReplyData      []byte
		om               *OutgoingMessage
	}
	type args struct {
		handlerFunc interface{}
		args        []interface{}
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *Message
	}{
		{"set from empty", fields{Text: "text", BotID: 1111, ChatID: 1000}, args{dumbFuncWithContextAndParam, []interface{}{true}}, &Message{Text: "text", BotID: 1111, ChatID: 1000, OnReplyAction: "github.com/requilence/integram.dumbFuncWithContextAndParam", OnReplyData: []byte{12, 255, 129, 2, 1, 2, 255, 130, 0, 1, 16, 0, 0, 13, 255, 130, 0, 1, 4, 98, 111, 111, 108, 2, 2, 0, 1}}},
	}
	for _, tt := range tests {
		m := &Message{
			ID:               tt.fields.ID,
			EventID:          tt.fields.EventID,
			MsgID:            tt.fields.MsgID,
			InlineMsgID:      tt.fields.InlineMsgID,
			BotID:            tt.fields.BotID,
			FromID:           tt.fields.FromID,
			ChatID:           tt.fields.ChatID,
			BackupChatID:     tt.fields.BackupChatID,
			ReplyToMsgID:     tt.fields.ReplyToMsgID,
			Date:             tt.fields.Date,
			Text:             tt.fields.Text,
			AntiFlood:        tt.fields.AntiFlood,
			Deleted:          tt.fields.Deleted,
			OnCallbackAction: tt.fields.OnCallbackAction,
			OnCallbackData:   tt.fields.OnCallbackData,
			OnReplyAction:    tt.fields.OnReplyAction,
			OnReplyData:      tt.fields.OnReplyData,
			om:               tt.fields.om,
		}
		if got := m.SetReplyAction(tt.args.handlerFunc, tt.args.args...); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. Message.SetReplyAction() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestOutgoingMessage_HideKeyboard(t *testing.T) {
	type fields struct {
		Message              Message
		KeyboardHide         bool
		ResizeKeyboard       bool
		KeyboardMarkup       Keyboard
		InlineKeyboardMarkup InlineKeyboard
		Keyboard             bool
		ParseMode            string
		OneTimeKeyboard      bool
		Selective            bool
		ForceReply           bool
		WebPreview           bool
		Silent               bool
		FilePath             string
		FileName             string
		FileType             string
		processed            bool
	}
	tests := []struct {
		name   string
		fields fields
		want   *OutgoingMessage
	}{
		{"set from empty", fields{Message: Message{Text: "text", BotID: 1111, ChatID: 1000}}, &OutgoingMessage{Message: Message{Text: "text", BotID: 1111, ChatID: 1000}, KeyboardHide: true}},
	}
	for _, tt := range tests {
		m := &OutgoingMessage{
			Message:              tt.fields.Message,
			KeyboardHide:         tt.fields.KeyboardHide,
			ResizeKeyboard:       tt.fields.ResizeKeyboard,
			KeyboardMarkup:       tt.fields.KeyboardMarkup,
			InlineKeyboardMarkup: tt.fields.InlineKeyboardMarkup,
			Keyboard:             tt.fields.Keyboard,
			ParseMode:            tt.fields.ParseMode,
			OneTimeKeyboard:      tt.fields.OneTimeKeyboard,
			Selective:            tt.fields.Selective,
			ForceReply:           tt.fields.ForceReply,
			WebPreview:           tt.fields.WebPreview,
			Silent:               tt.fields.Silent,
			FilePath:             tt.fields.FilePath,
			FileName:             tt.fields.FileName,
			FileType:             tt.fields.FileType,
			processed:            tt.fields.processed,
		}
		if got := m.HideKeyboard(); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. OutgoingMessage.HideKeyboard() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestOutgoingMessage_EnableForceReply(t *testing.T) {
	type fields struct {
		Message              Message
		KeyboardHide         bool
		ResizeKeyboard       bool
		KeyboardMarkup       Keyboard
		InlineKeyboardMarkup InlineKeyboard
		Keyboard             bool
		ParseMode            string
		OneTimeKeyboard      bool
		Selective            bool
		ForceReply           bool
		WebPreview           bool
		Silent               bool
		FilePath             string
		FileName             string
		FileType             string
		processed            bool
	}
	tests := []struct {
		name   string
		fields fields
		want   *OutgoingMessage
	}{
		{"set from empty", fields{Message: Message{Text: "text", BotID: 1111, ChatID: 1000}}, &OutgoingMessage{Message: Message{Text: "text", BotID: 1111, ChatID: 1000}, ForceReply: true}},
	}
	for _, tt := range tests {
		m := &OutgoingMessage{
			Message:              tt.fields.Message,
			KeyboardHide:         tt.fields.KeyboardHide,
			ResizeKeyboard:       tt.fields.ResizeKeyboard,
			KeyboardMarkup:       tt.fields.KeyboardMarkup,
			InlineKeyboardMarkup: tt.fields.InlineKeyboardMarkup,
			Keyboard:             tt.fields.Keyboard,
			ParseMode:            tt.fields.ParseMode,
			OneTimeKeyboard:      tt.fields.OneTimeKeyboard,
			Selective:            tt.fields.Selective,
			ForceReply:           tt.fields.ForceReply,
			WebPreview:           tt.fields.WebPreview,
			Silent:               tt.fields.Silent,
			FilePath:             tt.fields.FilePath,
			FileName:             tt.fields.FileName,
			FileType:             tt.fields.FileType,
			processed:            tt.fields.processed,
		}
		if got := m.EnableForceReply(); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. OutgoingMessage.EnableForceReply() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func Test_scheduleMessageSender_Send(t *testing.T) {
	chatID, _ := strconv.ParseInt(os.Getenv("INTEGRAM_TEST_USER"), 10, 64)

	bt := strings.Split(os.Getenv("INTEGRAM_TEST_BOT_TOKEN"), ":")
	botID, _ := strconv.ParseInt(bt[0], 10, 64)
	sms := scheduleMessageSender{}
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{0, 0, 255, 255}}, image.ZP, draw.Src)

	f, err := os.Create(os.TempDir() + "/draw.png")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	png.Encode(f, img)

	d1 := []byte("test\n")
	ioutil.WriteFile(os.TempDir()+"/dat1", d1, 0644)

	type args struct {
		m *OutgoingMessage
	}
	tests := []struct {
		name    string
		t       *scheduleMessageSender
		args    args
		wantErr bool
	}{
		{"with text", &sms, args{&OutgoingMessage{Message: Message{ChatID: chatID, BotID: botID, Text: "Test", FromID: botID}}}, false},
		{"with image", &sms, args{&OutgoingMessage{Message: Message{ChatID: chatID, BotID: botID, FromID: botID}, FilePath: os.TempDir() + "/draw.png", FileName: "draw.png", FileType: "image"}}, false},
		{"with doc", &sms, args{&OutgoingMessage{Message: Message{ChatID: chatID, BotID: botID, FromID: botID}, FilePath: os.TempDir() + "/dat1", FileName: "text.txt", FileType: "document"}}, false},
	}
	for _, tt := range tests {
		sms := &scheduleMessageSender{}
		if err := sms.Send(tt.args.m); (err != nil) != tt.wantErr {
			t.Errorf("%q. scheduleMessageSender.Send() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
		time.Sleep(time.Second * 3)
		msgFound := OutgoingMessage{}
		db.C("messages").FindId(tt.args.m.ID).One(&msgFound)
		if !tt.wantErr && msgFound.MsgID <= 0 {
			t.Errorf("%q. OutgoingMessage.Send() tg msg id must be > 0", tt.name)
		}
	}
}

type fakeMessageSender struct {
	sendFunc func(m *OutgoingMessage) error
}

func (fms fakeMessageSender) Send(m *OutgoingMessage) error {
	return fms.sendFunc(m)
}

func TestOutgoingMessage_Send(t *testing.T) {
	chatID, _ := strconv.ParseInt(os.Getenv("INTEGRAM_TEST_USER"), 10, 64)

	bt := strings.Split(os.Getenv("INTEGRAM_TEST_BOT_TOKEN"), ":")
	botID, _ := strconv.ParseInt(bt[0], 10, 64)
	goodMsgsProcessed := 0
	activeMessageSender = messageSender(fakeMessageSender{sendFunc: func(m *OutgoingMessage) error {
		goodMsgsProcessed++
		return nil
	}})
	type fields struct {
		Message              Message
		KeyboardHide         bool
		ResizeKeyboard       bool
		KeyboardMarkup       Keyboard
		InlineKeyboardMarkup InlineKeyboard
		Keyboard             bool
		ParseMode            string
		OneTimeKeyboard      bool
		Selective            bool
		ForceReply           bool
		WebPreview           bool
		Silent               bool
		FilePath             string
		FileName             string
		FileType             string
		processed            bool
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{"with text", fields{Message: Message{ChatID: chatID, BotID: botID, Text: "Test", FromID: botID}}, false},
		{"with image", fields{Message: Message{ChatID: chatID, BotID: botID, FromID: botID}, FilePath: os.TempDir() + "/draw.png", FileName: "draw.png", FileType: "image"}, false},
		{"with doc", fields{Message: Message{ChatID: chatID, BotID: botID, FromID: botID}, FilePath: os.TempDir() + "/dat1", FileName: "text.txt", FileType: "document"}, false},

		{"bad chatid", fields{Message: Message{BotID: botID, Text: "Test", FromID: botID}}, true},
		{"bad botid", fields{Message: Message{ChatID: chatID, Text: "Test", FromID: botID}}, true},
		{"bad text", fields{Message: Message{ChatID: chatID, BotID: botID, FromID: botID}}, true},
	}

	goodMsgs := 0
	for _, tt := range tests {
		m := &OutgoingMessage{
			Message:              tt.fields.Message,
			KeyboardHide:         tt.fields.KeyboardHide,
			ResizeKeyboard:       tt.fields.ResizeKeyboard,
			KeyboardMarkup:       tt.fields.KeyboardMarkup,
			InlineKeyboardMarkup: tt.fields.InlineKeyboardMarkup,
			Keyboard:             tt.fields.Keyboard,
			ParseMode:            tt.fields.ParseMode,
			OneTimeKeyboard:      tt.fields.OneTimeKeyboard,
			Selective:            tt.fields.Selective,
			ForceReply:           tt.fields.ForceReply,
			WebPreview:           tt.fields.WebPreview,
			Silent:               tt.fields.Silent,
			FilePath:             tt.fields.FilePath,
			FileName:             tt.fields.FileName,
			FileType:             tt.fields.FileType,
			processed:            tt.fields.processed,
		}
		if err := m.Send(); (err != nil) != tt.wantErr {
			t.Errorf("%q. OutgoingMessage.Send() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
		if !tt.wantErr {
			goodMsgs++
		}
	}
	if goodMsgs != goodMsgsProcessed {
		t.Errorf("OutgoingMessage.Send() got %d good msgs processed, want %d", goodMsgsProcessed, goodMsgs)
	}
}

func TestOutgoingMessage_AddEventID(t *testing.T) {
	type fields struct {
		Message              Message
		KeyboardHide         bool
		ResizeKeyboard       bool
		KeyboardMarkup       Keyboard
		InlineKeyboardMarkup InlineKeyboard
		Keyboard             bool
		ParseMode            string
		OneTimeKeyboard      bool
		Selective            bool
		ForceReply           bool
		WebPreview           bool
		Silent               bool
		FilePath             string
		FileName             string
		FileType             string
		processed            bool
	}
	type args struct {
		id []string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *OutgoingMessage
	}{
		{"set from empty", fields{Message: Message{Text: "text", BotID: 1111, ChatID: 1000}}, args{[]string{"event1", "event2"}}, &OutgoingMessage{Message: Message{Text: "text", BotID: 1111, ChatID: 1000, EventID: []string{"event1", "event2"}}}},
		{"add to existing", fields{Message: Message{Text: "text", BotID: 1111, ChatID: 1000, EventID: []string{"event1", "event2"}}}, args{[]string{"event3", "event4"}}, &OutgoingMessage{Message: Message{Text: "text", BotID: 1111, ChatID: 1000, EventID: []string{"event1", "event2", "event3", "event4"}}}},
	}
	for _, tt := range tests {
		m := &OutgoingMessage{
			Message:              tt.fields.Message,
			KeyboardHide:         tt.fields.KeyboardHide,
			ResizeKeyboard:       tt.fields.ResizeKeyboard,
			KeyboardMarkup:       tt.fields.KeyboardMarkup,
			InlineKeyboardMarkup: tt.fields.InlineKeyboardMarkup,
			Keyboard:             tt.fields.Keyboard,
			ParseMode:            tt.fields.ParseMode,
			OneTimeKeyboard:      tt.fields.OneTimeKeyboard,
			Selective:            tt.fields.Selective,
			ForceReply:           tt.fields.ForceReply,
			WebPreview:           tt.fields.WebPreview,
			Silent:               tt.fields.Silent,
			FilePath:             tt.fields.FilePath,
			FileName:             tt.fields.FileName,
			FileType:             tt.fields.FileType,
			processed:            tt.fields.processed,
		}
		if got := m.AddEventID(tt.args.id...); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. OutgoingMessage.AddEventID() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestOutgoingMessage_EnableAntiFlood(t *testing.T) {
	type fields struct {
		Message              Message
		KeyboardHide         bool
		ResizeKeyboard       bool
		KeyboardMarkup       Keyboard
		InlineKeyboardMarkup InlineKeyboard
		Keyboard             bool
		ParseMode            string
		OneTimeKeyboard      bool
		Selective            bool
		ForceReply           bool
		WebPreview           bool
		Silent               bool
		FilePath             string
		FileName             string
		FileType             string
		processed            bool
	}
	tests := []struct {
		name   string
		fields fields
		want   *OutgoingMessage
	}{
		{"set from empty", fields{Message: Message{Text: "text", BotID: 1111, ChatID: 1000}}, &OutgoingMessage{Message: Message{Text: "text", BotID: 1111, ChatID: 1000, AntiFlood: true}}},
	}
	for _, tt := range tests {
		m := &OutgoingMessage{
			Message:              tt.fields.Message,
			KeyboardHide:         tt.fields.KeyboardHide,
			ResizeKeyboard:       tt.fields.ResizeKeyboard,
			KeyboardMarkup:       tt.fields.KeyboardMarkup,
			InlineKeyboardMarkup: tt.fields.InlineKeyboardMarkup,
			Keyboard:             tt.fields.Keyboard,
			ParseMode:            tt.fields.ParseMode,
			OneTimeKeyboard:      tt.fields.OneTimeKeyboard,
			Selective:            tt.fields.Selective,
			ForceReply:           tt.fields.ForceReply,
			WebPreview:           tt.fields.WebPreview,
			Silent:               tt.fields.Silent,
			FilePath:             tt.fields.FilePath,
			FileName:             tt.fields.FileName,
			FileType:             tt.fields.FileType,
			processed:            tt.fields.processed,
		}
		if got := m.EnableAntiFlood(); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. OutgoingMessage.EnableAntiFlood() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestOutgoingMessage_SetTextFmt(t *testing.T) {
	type fields struct {
		Message              Message
		KeyboardHide         bool
		ResizeKeyboard       bool
		KeyboardMarkup       Keyboard
		InlineKeyboardMarkup InlineKeyboard
		Keyboard             bool
		ParseMode            string
		OneTimeKeyboard      bool
		Selective            bool
		ForceReply           bool
		WebPreview           bool
		Silent               bool
		FilePath             string
		FileName             string
		FileType             string
		processed            bool
	}
	type args struct {
		text string
		a    []interface{}
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *OutgoingMessage
	}{
		{"set from empty", fields{Message: Message{BotID: 1111, ChatID: 1000}}, args{"%d and %s", []interface{}{5, "text"}}, &OutgoingMessage{Message: Message{Text: "5 and text", BotID: 1111, ChatID: 1000}}},
	}
	for _, tt := range tests {
		m := &OutgoingMessage{
			Message:              tt.fields.Message,
			KeyboardHide:         tt.fields.KeyboardHide,
			ResizeKeyboard:       tt.fields.ResizeKeyboard,
			KeyboardMarkup:       tt.fields.KeyboardMarkup,
			InlineKeyboardMarkup: tt.fields.InlineKeyboardMarkup,
			Keyboard:             tt.fields.Keyboard,
			ParseMode:            tt.fields.ParseMode,
			OneTimeKeyboard:      tt.fields.OneTimeKeyboard,
			Selective:            tt.fields.Selective,
			ForceReply:           tt.fields.ForceReply,
			WebPreview:           tt.fields.WebPreview,
			Silent:               tt.fields.Silent,
			FilePath:             tt.fields.FilePath,
			FileName:             tt.fields.FileName,
			FileType:             tt.fields.FileType,
			processed:            tt.fields.processed,
		}
		if got := m.SetTextFmt(tt.args.text, tt.args.a...); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. OutgoingMessage.SetTextFmt() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestOutgoingMessage_SetText(t *testing.T) {
	type fields struct {
		Message              Message
		KeyboardHide         bool
		ResizeKeyboard       bool
		KeyboardMarkup       Keyboard
		InlineKeyboardMarkup InlineKeyboard
		Keyboard             bool
		ParseMode            string
		OneTimeKeyboard      bool
		Selective            bool
		ForceReply           bool
		WebPreview           bool
		Silent               bool
		FilePath             string
		FileName             string
		FileType             string
		processed            bool
	}
	type args struct {
		text string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *OutgoingMessage
	}{
		{"set from empty", fields{Message: Message{BotID: 1111, ChatID: 1000}}, args{"a and b"}, &OutgoingMessage{Message: Message{Text: "a and b", BotID: 1111, ChatID: 1000}}},
	}
	for _, tt := range tests {
		m := &OutgoingMessage{
			Message:              tt.fields.Message,
			KeyboardHide:         tt.fields.KeyboardHide,
			ResizeKeyboard:       tt.fields.ResizeKeyboard,
			KeyboardMarkup:       tt.fields.KeyboardMarkup,
			InlineKeyboardMarkup: tt.fields.InlineKeyboardMarkup,
			Keyboard:             tt.fields.Keyboard,
			ParseMode:            tt.fields.ParseMode,
			OneTimeKeyboard:      tt.fields.OneTimeKeyboard,
			Selective:            tt.fields.Selective,
			ForceReply:           tt.fields.ForceReply,
			WebPreview:           tt.fields.WebPreview,
			Silent:               tt.fields.Silent,
			FilePath:             tt.fields.FilePath,
			FileName:             tt.fields.FileName,
			FileType:             tt.fields.FileType,
			processed:            tt.fields.processed,
		}
		if got := m.SetText(tt.args.text); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. OutgoingMessage.SetText() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestOutgoingMessage_DisableWebPreview(t *testing.T) {
	type fields struct {
		Message              Message
		KeyboardHide         bool
		ResizeKeyboard       bool
		KeyboardMarkup       Keyboard
		InlineKeyboardMarkup InlineKeyboard
		Keyboard             bool
		ParseMode            string
		OneTimeKeyboard      bool
		Selective            bool
		ForceReply           bool
		WebPreview           bool
		Silent               bool
		FilePath             string
		FileName             string
		FileType             string
		processed            bool
	}
	tests := []struct {
		name   string
		fields fields
		want   *OutgoingMessage
	}{
		{"WebPreview false", fields{Message: Message{Text: "text", BotID: 1111, ChatID: 1000}}, &OutgoingMessage{Message: Message{Text: "text", BotID: 1111, ChatID: 1000}, WebPreview: false}},
	}
	for _, tt := range tests {
		m := &OutgoingMessage{
			Message:              tt.fields.Message,
			KeyboardHide:         tt.fields.KeyboardHide,
			ResizeKeyboard:       tt.fields.ResizeKeyboard,
			KeyboardMarkup:       tt.fields.KeyboardMarkup,
			InlineKeyboardMarkup: tt.fields.InlineKeyboardMarkup,
			Keyboard:             tt.fields.Keyboard,
			ParseMode:            tt.fields.ParseMode,
			OneTimeKeyboard:      tt.fields.OneTimeKeyboard,
			Selective:            tt.fields.Selective,
			ForceReply:           tt.fields.ForceReply,
			WebPreview:           tt.fields.WebPreview,
			Silent:               tt.fields.Silent,
			FilePath:             tt.fields.FilePath,
			FileName:             tt.fields.FileName,
			FileType:             tt.fields.FileType,
			processed:            tt.fields.processed,
		}
		if got := m.DisableWebPreview(); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. OutgoingMessage.DisableWebPreview() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestOutgoingMessage_EnableMarkdown(t *testing.T) {
	type fields struct {
		Message              Message
		KeyboardHide         bool
		ResizeKeyboard       bool
		KeyboardMarkup       Keyboard
		InlineKeyboardMarkup InlineKeyboard
		Keyboard             bool
		ParseMode            string
		OneTimeKeyboard      bool
		Selective            bool
		ForceReply           bool
		WebPreview           bool
		Silent               bool
		FilePath             string
		FileName             string
		FileType             string
		processed            bool
	}
	tests := []struct {
		name   string
		fields fields
		want   *OutgoingMessage
	}{
		{"set from empty", fields{Message: Message{Text: "text", BotID: 1111, ChatID: 1000}}, &OutgoingMessage{Message: Message{Text: "text", BotID: 1111, ChatID: 1000}, ParseMode: "Markdown"}},
	}
	for _, tt := range tests {
		m := &OutgoingMessage{
			Message:              tt.fields.Message,
			KeyboardHide:         tt.fields.KeyboardHide,
			ResizeKeyboard:       tt.fields.ResizeKeyboard,
			KeyboardMarkup:       tt.fields.KeyboardMarkup,
			InlineKeyboardMarkup: tt.fields.InlineKeyboardMarkup,
			Keyboard:             tt.fields.Keyboard,
			ParseMode:            tt.fields.ParseMode,
			OneTimeKeyboard:      tt.fields.OneTimeKeyboard,
			Selective:            tt.fields.Selective,
			ForceReply:           tt.fields.ForceReply,
			WebPreview:           tt.fields.WebPreview,
			Silent:               tt.fields.Silent,
			FilePath:             tt.fields.FilePath,
			FileName:             tt.fields.FileName,
			FileType:             tt.fields.FileType,
			processed:            tt.fields.processed,
		}
		if got := m.EnableMarkdown(); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. OutgoingMessage.EnableMarkdown() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestOutgoingMessage_EnableHTML(t *testing.T) {
	type fields struct {
		Message              Message
		KeyboardHide         bool
		ResizeKeyboard       bool
		KeyboardMarkup       Keyboard
		InlineKeyboardMarkup InlineKeyboard
		Keyboard             bool
		ParseMode            string
		OneTimeKeyboard      bool
		Selective            bool
		ForceReply           bool
		WebPreview           bool
		Silent               bool
		FilePath             string
		FileName             string
		FileType             string
		processed            bool
	}
	tests := []struct {
		name   string
		fields fields
		want   *OutgoingMessage
	}{
		{"set from empty", fields{Message: Message{Text: "text", BotID: 1111, ChatID: 1000}}, &OutgoingMessage{Message: Message{Text: "text", BotID: 1111, ChatID: 1000}, ParseMode: "HTML"}},
	}
	for _, tt := range tests {
		m := &OutgoingMessage{
			Message:              tt.fields.Message,
			KeyboardHide:         tt.fields.KeyboardHide,
			ResizeKeyboard:       tt.fields.ResizeKeyboard,
			KeyboardMarkup:       tt.fields.KeyboardMarkup,
			InlineKeyboardMarkup: tt.fields.InlineKeyboardMarkup,
			Keyboard:             tt.fields.Keyboard,
			ParseMode:            tt.fields.ParseMode,
			OneTimeKeyboard:      tt.fields.OneTimeKeyboard,
			Selective:            tt.fields.Selective,
			ForceReply:           tt.fields.ForceReply,
			WebPreview:           tt.fields.WebPreview,
			Silent:               tt.fields.Silent,
			FilePath:             tt.fields.FilePath,
			FileName:             tt.fields.FileName,
			FileType:             tt.fields.FileType,
			processed:            tt.fields.processed,
		}
		if got := m.EnableHTML(); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. OutgoingMessage.EnableHTML() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestOutgoingMessage_SetParseMode(t *testing.T) {
	type fields struct {
		Message              Message
		KeyboardHide         bool
		ResizeKeyboard       bool
		KeyboardMarkup       Keyboard
		InlineKeyboardMarkup InlineKeyboard
		Keyboard             bool
		ParseMode            string
		OneTimeKeyboard      bool
		Selective            bool
		ForceReply           bool
		WebPreview           bool
		Silent               bool
		FilePath             string
		FileName             string
		FileType             string
		processed            bool
	}
	type args struct {
		s string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *OutgoingMessage
	}{
		{"set html", fields{Message: Message{Text: "text", BotID: 1111, ChatID: 1000}}, args{"HTML"}, &OutgoingMessage{Message: Message{Text: "text", BotID: 1111, ChatID: 1000}, ParseMode: "HTML"}},
		{"set markdown", fields{Message: Message{Text: "text", BotID: 1111, ChatID: 1000}}, args{"markdown"}, &OutgoingMessage{Message: Message{Text: "text", BotID: 1111, ChatID: 1000}, ParseMode: "markdown"}},
	}
	for _, tt := range tests {
		m := &OutgoingMessage{
			Message:              tt.fields.Message,
			KeyboardHide:         tt.fields.KeyboardHide,
			ResizeKeyboard:       tt.fields.ResizeKeyboard,
			KeyboardMarkup:       tt.fields.KeyboardMarkup,
			InlineKeyboardMarkup: tt.fields.InlineKeyboardMarkup,
			Keyboard:             tt.fields.Keyboard,
			ParseMode:            tt.fields.ParseMode,
			OneTimeKeyboard:      tt.fields.OneTimeKeyboard,
			Selective:            tt.fields.Selective,
			ForceReply:           tt.fields.ForceReply,
			WebPreview:           tt.fields.WebPreview,
			Silent:               tt.fields.Silent,
			FilePath:             tt.fields.FilePath,
			FileName:             tt.fields.FileName,
			FileType:             tt.fields.FileType,
			processed:            tt.fields.processed,
		}
		if got := m.SetParseMode(tt.args.s); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. OutgoingMessage.SetParseMode() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestOutgoingMessage_SetReplyToMsgID(t *testing.T) {
	type fields struct {
		Message              Message
		KeyboardHide         bool
		ResizeKeyboard       bool
		KeyboardMarkup       Keyboard
		InlineKeyboardMarkup InlineKeyboard
		Keyboard             bool
		ParseMode            string
		OneTimeKeyboard      bool
		Selective            bool
		ForceReply           bool
		WebPreview           bool
		Silent               bool
		FilePath             string
		FileName             string
		FileType             string
		processed            bool
	}
	type args struct {
		id int
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *OutgoingMessage
	}{
		{"test1", fields{Message: Message{Text: "text", BotID: 1111, ChatID: 1000}}, args{123456}, &OutgoingMessage{Message: Message{Text: "text", BotID: 1111, ChatID: 1000, ReplyToMsgID: 123456}}},
	}
	for _, tt := range tests {
		m := &OutgoingMessage{
			Message:              tt.fields.Message,
			KeyboardHide:         tt.fields.KeyboardHide,
			ResizeKeyboard:       tt.fields.ResizeKeyboard,
			KeyboardMarkup:       tt.fields.KeyboardMarkup,
			InlineKeyboardMarkup: tt.fields.InlineKeyboardMarkup,
			Keyboard:             tt.fields.Keyboard,
			ParseMode:            tt.fields.ParseMode,
			OneTimeKeyboard:      tt.fields.OneTimeKeyboard,
			Selective:            tt.fields.Selective,
			ForceReply:           tt.fields.ForceReply,
			WebPreview:           tt.fields.WebPreview,
			Silent:               tt.fields.Silent,
			FilePath:             tt.fields.FilePath,
			FileName:             tt.fields.FileName,
			FileType:             tt.fields.FileType,
			processed:            tt.fields.processed,
		}
		if got := m.SetReplyToMsgID(tt.args.id); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. OutgoingMessage.SetReplyToMsgID() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestMessage_UpdateEventsID(t *testing.T) {
	bt := strings.Split(os.Getenv("INTEGRAM_TEST_BOT_TOKEN"), ":")
	botID, _ := strconv.ParseInt(bt[0], 10, 64)

	type args struct {
		db      *mgo.Database
		eventID []string
	}
	tests := []struct {
		name    string
		m       Message
		args    args
		wantErr bool
		want    Message
	}{
		{"add first event 1", Message{ID: bson.ObjectIdHex("55b63650ac136a250a5a2734"), MsgID: 1234, ChatID: 123, BotID: botID}, args{db, []string{"neweventval"}}, false, Message{ID: bson.ObjectIdHex("55b63650ac136a250a5a2734"), EventID: []string{"neweventval"}, MsgID: 1234, ChatID: 123, BotID: botID}},
		{"add more event", Message{ID: bson.ObjectIdHex("55b63650ac136a250a5a2735"), EventID: []string{"eventval"}, MsgID: 1234, ChatID: 123, BotID: botID}, args{db, []string{"neweventval"}}, false, Message{ID: bson.ObjectIdHex("55b63650ac136a250a5a2735"), EventID: []string{"eventval", "neweventval"}, MsgID: 1234, ChatID: 123, BotID: botID}},
		{"add more events", Message{ID: bson.ObjectIdHex("55b63650ac136a250a5a2736"), EventID: []string{"eventval", "eventotherval"}, MsgID: 1234, ChatID: 123, BotID: botID}, args{db, []string{"neweventval", "neweventval2"}}, false, Message{ID: bson.ObjectIdHex("55b63650ac136a250a5a2736"), EventID: []string{"eventval", "eventotherval", "neweventval", "neweventval2"}, MsgID: 1234, ChatID: 123, BotID: botID}},
	}
	for _, tt := range tests {
		db.C("messages").Insert(tt.m)

		if err := tt.m.UpdateEventsID(tt.args.db, tt.args.eventID...); (err != nil) != tt.wantErr {
			t.Errorf("%q. Message.UpdateEventsID() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}

		got := Message{}
		db.C("messages").FindId(tt.m.ID).One(&got)

		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. OutgoingMessage.UpdateEventsID() = %v, want %v", tt.name, got, tt.want)
		}
		db.C("messages").RemoveId(tt.m.ID)

	}
}

func TestMessage_Update(t *testing.T) {
	type fields struct {
		ID               bson.ObjectId
		EventID          []string
		MsgID            int
		InlineMsgID      string
		BotID            int64
		FromID           int64
		ChatID           int64
		BackupChatID     int64
		ReplyToMsgID     int
		Date             time.Time
		Text             string
		AntiFlood        bool
		Deleted          bool
		OnCallbackAction string
		OnCallbackData   []byte
		OnReplyAction    string
		OnReplyData      []byte
		om               *OutgoingMessage
	}
	type args struct {
		db *mgo.Database
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		m := &Message{
			ID:               tt.fields.ID,
			EventID:          tt.fields.EventID,
			MsgID:            tt.fields.MsgID,
			InlineMsgID:      tt.fields.InlineMsgID,
			BotID:            tt.fields.BotID,
			FromID:           tt.fields.FromID,
			ChatID:           tt.fields.ChatID,
			BackupChatID:     tt.fields.BackupChatID,
			ReplyToMsgID:     tt.fields.ReplyToMsgID,
			Date:             tt.fields.Date,
			Text:             tt.fields.Text,
			AntiFlood:        tt.fields.AntiFlood,
			Deleted:          tt.fields.Deleted,
			OnCallbackAction: tt.fields.OnCallbackAction,
			OnCallbackData:   tt.fields.OnCallbackData,
			OnReplyAction:    tt.fields.OnReplyAction,
			OnReplyData:      tt.fields.OnReplyData,
			om:               tt.fields.om,
		}
		if err := m.Update(tt.args.db); (err != nil) != tt.wantErr {
			t.Errorf("%q. Message.Update() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
	}
}
