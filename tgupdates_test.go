package integram

import (
	"testing"
	"time"

	tg "github.com/requilence/telegram-bot-api"
)

func TestIncomingMessage_IsEventBotAddedToGroup(t *testing.T) {
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
		NewChatMember         *User
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
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		{"NewChatMember with user equal to bot", fields{Message: Message{BotID: 12345}, NewChatMember: &User{ID: 12345}}, true},
		{"NewChatMember with user not equal to bot", fields{Message: Message{BotID: 123456}, NewChatMember: &User{ID: 12345}}, false},
		{"GroupChatCreated with user not equal to bot", fields{Message: Message{BotID: 123456}, GroupChatCreated: true}, true},
		{"NewChatMember with user not equal to bot", fields{Message: Message{BotID: 123456}, SuperGroupChatCreated: true}, true},
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
			NewChatMember:         tt.fields.NewChatMember,
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
		if got := m.IsEventBotAddedToGroup(); got != tt.want {
			t.Errorf("%q. IncomingMessage.IsEventBotAddedToGroup() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestIncomingMessage_GetCommand(t *testing.T) {
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
		NewChatMember         *User
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
	tests := []struct {
		name   string
		fields fields
		want   string
		want1  string
	}{
		{"just command", fields{Message: Message{Text: "/command123"}}, "command123", ""},
		{"command with param", fields{Message: Message{Text: "/command123 param"}}, "command123", "param"},
		{"command with bot name", fields{Message: Message{Text: "/command123@bot"}}, "command123", ""},
		{"command with bot name and param", fields{Message: Message{Text: "/command123@bot param"}}, "command123", "param"},
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
			NewChatMember:         tt.fields.NewChatMember,
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
		got, got1 := m.GetCommand()
		if got != tt.want {
			t.Errorf("%q. IncomingMessage.GetCommand() got = %v, want %v", tt.name, got, tt.want)
		}
		if got1 != tt.want1 {
			t.Errorf("%q. IncomingMessage.GetCommand() got1 = %v, want %v", tt.name, got1, tt.want1)
		}
	}
}
