package integram

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	tg "gopkg.in/telegram-bot-api.v3"
	gocontext "context"
)

var updateMapMutex = &sync.RWMutex{}
var updateMutexPerBotPerChat = make(map[string]*sync.Mutex)

var inlineQueriesPerBotPerChatMutex = &sync.Mutex{}
var inlineQueriesPerBotPerChat = make(map[string]gocontext.CancelFunc)

type msgInfo struct {
	TS       time.Time
	ID       int
	InlineID string
	BotID    int64
	ChatID   int64
}

var lastMsgIDByUser = make(map[int64]msgInfo)
var lastMsgIDByUserMutex = sync.Mutex{}

func updateRoutine(b *Bot, u *tg.Update) {

	if !Debug {
		defer func() {
			if r := recover(); r != nil {
				stack := stack(3)
				log.Errorf("Panic recovery at updateRoutine -> %s\n%s\n", r, stack)

			}
		}()
	}
	updateReceivedAt := time.Now()

	var chatID int64
	if u.Message != nil {
		chatID = u.Message.Chat.ID
	} else if u.CallbackQuery != nil {
		if u.CallbackQuery.Message != nil {
			chatID = u.CallbackQuery.Message.Chat.ID
		} else {
			chatID = u.CallbackQuery.From.ID
		}
	} else if u.EditedMessage != nil {
		chatID = u.EditedMessage.Chat.ID
	} else if u.ChosenInlineResult != nil {
		chatID = u.ChosenInlineResult.From.ID
	} else if u.InlineQuery != nil {
		chatID = u.InlineQuery.From.ID
	}
	mutexID := fmt.Sprintf("%d_%d", b.ID, chatID)

	if u.InlineQuery == nil {
		updateMapMutex.Lock()
		m, exists := updateMutexPerBotPerChat[mutexID]
		updateMapMutex.Unlock()
		if exists {
			m.Lock()
		} else {
			m = &sync.Mutex{}
			m.Lock()

			updateMapMutex.Lock()
			updateMutexPerBotPerChat[mutexID] = m
			updateMapMutex.Unlock()
		}
		defer m.Unlock()
	}

	db := mongoSession.Clone().DB(mongo.Database)

	defer db.Session.Close()

	service, context := tgUpdateHandler(u, b, db)

	if service == nil || context == nil {
		return
	}

	if context.Message != nil && !context.MessageEdited {

		if service.TGNewMessageHandler == nil {
			context.Log().Warn("Received Message but TGNewMessageHandler not set for service")
			return
		}

		err := service.TGNewMessageHandler(context)

		if err != nil {
			context.Log().WithError(err).Error("BotUpdateHandler error")
		}

		// Save incoming message after processing to be sure to save onReply actions
		err = context.Message.Message.saveToDB(db)

		if err != nil {
			log.WithError(err).Error("can't add incoming message to db")
		}

	} else if context.InlineQuery != nil {
		if service.TGInlineQueryHandler == nil {
			context.Log().Warn("Received InlineQuery but TGInlineQueryHandler not set for service")
			return
		}

		queryHandlerStarted := time.Now()
		inlineQueriesPerBotPerChatMutex.Lock()
		if cancelFunc, exists :=  inlineQueriesPerBotPerChat[mutexID]; exists{
			context.Log().Debug("Another inline request in process. Cancel the previous CancelContext")
			cancelFunc()
			delete(inlineQueriesPerBotPerChat, mutexID)
		}

		context.CancelContext, inlineQueriesPerBotPerChat[mutexID] = gocontext.WithCancel(gocontext.Background())

		inlineQueriesPerBotPerChatMutex.Unlock()

		err := service.TGInlineQueryHandler(context)
		inlineQueriesPerBotPerChatMutex.Lock()
		if _, exists :=  inlineQueriesPerBotPerChat[mutexID]; exists{
			delete(inlineQueriesPerBotPerChat, mutexID)
		}
		inlineQueriesPerBotPerChatMutex.Unlock()

		if err != nil {
			context.Log().WithError(err).WithField("secSpent", time.Now().Sub(queryHandlerStarted).Seconds()).WithField("secSpentSinceUpdate", time.Now().Sub(updateReceivedAt).Seconds()).Error("BotUpdateHandler InlineQuery error")
		} else {
			if context.inlineQueryAnsweredAt == nil {
				context.Log().WithError(err).Error("BotUpdateHandler InlineQuery not answered")
			} else {
				secsSpent := context.inlineQueryAnsweredAt.Sub(updateReceivedAt).Seconds()
				if secsSpent > 10 {
					context.Log().WithError(err).Errorf("BotUpdateHandler InlineQuery 10 sec exceeded: %.1f sec spent after update, %.1f sec after the handle", secsSpent, time.Now().Sub(queryHandlerStarted).Seconds())
				}
			}
		}

		return
	} else if context.ChosenInlineResult != nil {

		if service.TGChosenInlineResultHandler == nil {
			context.Log().Warn("Received ChosenInlineResult but TGChosenInlineResultHandler not set for service")
			return
		}

		err := service.TGChosenInlineResultHandler(context)

		if err != nil {
			context.Log().WithError(err).Error("BotUpdateHandler error")
		}
		return
	}

	// If this message is keyboard answer - remove that keyboard from user/chat
	if context.Message.ReplyToMessage != nil && context.Message.ReplyToMessage.om != nil && context.Message.ReplyToMessage.om.OneTimeKeyboard {
		key, _ := context.KeyboardAnswer()

		if key != "" {
			var err error
			// if received reply is result of button pressed
			// TODO: fix for non-selective one_time_keyboard in group chat
			_, err = db.C("users").UpdateAll(bson.M{}, bson.M{"$pull": bson.M{"keyboardperchat": bson.M{"chatid": context.Chat.ID, "msgid": context.Message.ReplyToMessage.ID}}})
			if err != nil {
				log.WithError(err).Debugf("can't remove onetime keyboard from user")
			}

			if context.Chat.IsGroup() {
				err = db.C("chats").Update(bson.M{"_id": context.Chat.ID, "keyboard.msgid": context.Message.ReplyToMessage.ID}, bson.M{"$unset": bson.M{"keyboard": true}})
			} else {
				_, err = db.C("chats").UpdateAll(bson.M{"_id": context.Chat.ID}, bson.M{"$pull": bson.M{"keyboardperbot": bson.M{"botid": context.Message.BotID, "msgid": context.Message.ReplyToMessage.ID}}})
			}
			if err != nil {
				log.WithError(err).Debugf("can't remove onetime keyboard from chat")
			}
		}

	}

}

func (bot *Bot) listen() {
	api := bot.API
	if bot.updatesChan == nil {
		log.Debug("Open UpdatesChan for bot " + bot.Username)
		var err error
		bot.updatesChan, err = api.GetUpdatesChan(tg.UpdateConfig{Timeout: randomInRange(10, 20), Limit: 100})
		if err != nil {
			log.WithField("bot", bot.ID).WithError(err).Panic("Can't get updatesChan")
		}
	}
	go func(c <-chan tg.Update, b *Bot) {
		var context Context
		log.Info("Start listening for updates bot " + bot.Username)

		defer func() {
			fmt.Println("Stop listening for updates on bot " + b.Username)

			if r := recover(); r != nil {
				stack := stack(3)
				context.Log().Errorf("Panic recovery at updatesChan -> %s\n%s\n", r, stack)
				tgUpdatesRevoltChan <- b
			}
		}()

		for {
			u := <-c
			go updateRoutine(b, &u)

		}
	}(bot.updatesChan, bot)
}

func tgUserPointer(u *tg.User) *User {
	if u == nil {
		return nil
	}
	return &User{ID: u.ID, FirstName: u.FirstName, LastName: u.LastName, UserName: u.UserName}
}

func tgChatPointer(c *tg.Chat) *Chat {
	if c == nil {
		return nil
	}
	return &Chat{ID: c.ID, FirstName: c.FirstName, LastName: c.LastName, UserName: c.UserName, Title: c.Title, Type: c.Type}
}

func tgUser(u *tg.User) User {
	if u == nil {
		return User{}
	}
	return User{ID: u.ID, FirstName: u.FirstName, LastName: u.LastName, UserName: u.UserName}
}

func tgChat(chat *tg.Chat) Chat {
	if chat == nil {
		return Chat{}
	}
	return Chat{ID: chat.ID, Title: chat.Title, Type: chat.Type, FirstName: chat.FirstName, LastName: chat.LastName, UserName: chat.UserName}
}

func incomingMessageFromTGMessage(m *tg.Message) IncomingMessage {
	im := IncomingMessage{}

	// Base message struct
	im.MsgID = m.MessageID
	if m.From != nil {
		im.FromID = m.From.ID
	}
	im.ChatID = m.Chat.ID
	im.Date = time.Unix(int64(m.Date), 0)
	im.Text = m.Text

	if m.From != nil {
		im.From = tgUser(m.From)
	}
	im.Chat = Chat{ID: m.Chat.ID, Type: m.Chat.Type, FirstName: m.Chat.FirstName, LastName: m.Chat.LastName, UserName: m.Chat.UserName, Title: m.Chat.Title}

	im.ForwardFrom = tgUserPointer(m.ForwardFrom)
	im.ForwardFromChat = tgChatPointer(m.ForwardFromChat)
	im.ForwardDate = time.Unix(int64(m.ForwardDate), 0)

	if m.ReplyToMessage != nil {
		rm := m.ReplyToMessage
		im.ReplyToMessage = &Message{MsgID: rm.MessageID, Date: time.Unix(int64(rm.Date), 0), Text: rm.Text}
		if rm.Chat != nil {
			im.ReplyToMessage.ChatID = rm.Chat.ID
		}

		if rm.From != nil {
			im.ReplyToMessage.FromID = rm.From.ID
		}
	}

	im.Caption = m.Caption

	im.NewChatMember = tgUserPointer(m.NewChatMember)
	im.LeftChatMember = tgUserPointer(m.LeftChatMember)

	im.Audio = m.Audio
	im.Document = m.Document
	im.Photo = m.Photo
	im.Sticker = m.Sticker
	im.Video = m.Video
	im.Voice = m.Voice
	im.Contact = m.Contact
	im.Location = m.Location
	im.NewChatTitle = m.NewChatTitle
	im.NewChatPhoto = m.NewChatPhoto
	im.DeleteChatPhoto = m.DeleteChatPhoto
	im.GroupChatCreated = m.GroupChatCreated
	return im

}
func tgCallbackHandler(u *tg.Update, b *Bot, db *mgo.Database) (*Service, *Context) {
	var rm *Message
	var err error
	if u.CallbackQuery.Message != nil {
		rm, err = findMessage(db, u.CallbackQuery.Message.Chat.ID, b.ID, u.CallbackQuery.Message.MessageID)
		if err != nil {
			log.WithError(err).WithField("bot_id", b.ID).WithField("msg_id", u.CallbackQuery.Message.MessageID).Error("tgCallbackHandler can't find source message")
		}
	} else {
		rm, err = findInlineMessage(db, b.ID, u.CallbackQuery.InlineMessageID)
		if err != nil {
			log.WithError(err).WithField("bot_id", b.ID).WithField("msg_id", u.CallbackQuery.InlineMessageID).Error("tgCallbackHandler can't find source message")
		}
	}

	if rm != nil {
		service, err := detectServiceByBot(b.ID)

		if err != nil {
			log.WithError(err).WithField("bot", b.ID).Error("Can't detect service")
		}
		cbData := u.CallbackQuery.Data
		cbState := 0
		if []byte(cbData)[0] == inlineButtonStateKeyword {
			log.Debug("INLINE_BUTTON_STATE_KEYWORD found")
			cbState, err = strconv.Atoi(cbData[1:2])
			cbData = cbData[2:]
			if err != nil {
				log.WithError(err).Errorf("INLINE_BUTTON_STATE_KEYWORD found but next symbol is %s", cbData[1:2])
			}
		}
		ctx := &Context{
			db:          db,
			ServiceName: service.Name,
			User:        tgUser(u.CallbackQuery.From),
			Callback:    &callback{ID: u.CallbackQuery.ID, Data: cbData, Message: rm.om, State: cbState}}
		var chat Chat
		if u.CallbackQuery.InlineMessageID != "" && rm.ChatID != 0 {
			chatData, err := ctx.FindChat(bson.M{"_id": rm.ChatID})
			if err != nil {
				ctx.Log().WithError(err).WithField("chatID", rm.ChatID).Error("find chat for inline msg' callback")
				chat = Chat{ID: rm.ChatID}
			} else {
				chat = chatData.Chat
			}

		} else if u.CallbackQuery.Message != nil {
			chat = tgChat(u.CallbackQuery.Message.Chat)
		} else {
			chat = tgChat(&tg.Chat{ID: u.CallbackQuery.From.ID, LastName: u.CallbackQuery.From.LastName, UserName: u.CallbackQuery.From.UserName, Type: "private", FirstName: u.CallbackQuery.From.FirstName})
		}
		ctx.Chat = chat
		ctx.User.ctx = ctx
		ctx.Chat.ctx = ctx

		if rm.OnCallbackAction != "" {
			log.Debugf("CallbackAction found %s", rm.OnCallbackAction)
			// Instantiate a new variable to hold this argument
			if handler, ok := actionFuncs[rm.OnCallbackAction]; ok {
				handlerType := reflect.TypeOf(handler)
				log.Debugf("handler %v: %v %v\n", rm.OnCallbackAction, handlerType.String(), handlerType.Kind().String())
				handlerArgsInterfaces := make([]interface{}, handlerType.NumIn()-1)
				handlerArgs := make([]reflect.Value, handlerType.NumIn())

				for i := 1; i < handlerType.NumIn(); i++ {
					dataVal := reflect.New(handlerType.In(i))
					handlerArgsInterfaces[i-1] = dataVal.Interface()
				}
				if err := decode(rm.OnCallbackData, &handlerArgsInterfaces); err != nil {
					ctx.Log().WithField("handler", rm.OnCallbackAction).WithError(err).Error("Can't decode replyHandler's args")
				}
				handlerArgs[0] = reflect.ValueOf(ctx)
				for i := 0; i < len(handlerArgsInterfaces); i++ {
					handlerArgs[i+1] = reflect.ValueOf(handlerArgsInterfaces[i])
				}

				if len(handlerArgs) > 0 {
					handlerVal := reflect.ValueOf(handler)
					returnVals := handlerVal.Call(handlerArgs)

					if !returnVals[0].IsNil() {
						err := returnVals[0].Interface().(error)
						// NOTE: panics will be caught by the recover statement above
						ctx.Log().WithField("handler", rm.OnCallbackAction).WithError(err).Error("callbackAction failed")
						ctx.AnswerCallbackQuery("Oops! Please try again", false)
					} else {
						if ctx.Callback.AnsweredAt == nil {
							ctx.AnswerCallbackQuery("", false)
						}
					}
				}
			} else {
				ctx.Log().WithField("handler", rm.OnCallbackAction).Error("Reply handler not registered")
			}

		}
		return nil, ctx
	}

	return nil, nil

}
func tgInlineQueryHandler(u *tg.Update, b *Bot, db *mgo.Database) (*Service, *Context) {
	service, err := detectServiceByBot(b.ID)
	if err != nil {
		log.WithError(err).WithField("bot", b.ID).Error("Can't detect service")
	}
	user := tgUser(u.InlineQuery.From)
	ctx := &Context{ServiceName: service.Name, User: user, db: db, InlineQuery: u.InlineQuery}
	ctx.User.ctx = ctx

	return service, ctx
}
func tgChosenInlineResultHandler(u *tg.Update, b *Bot, db *mgo.Database) (*Service, *Context) {

	service, err := detectServiceByBot(b.ID)
	if err != nil {
		log.WithError(err).WithField("bot", b.ID).Error("Can't detect service")
	}
	user := tgUser(u.ChosenInlineResult.From)
	ctx := &Context{ServiceName: service.Name, User: user, db: db, ChosenInlineResult: &chosenInlineResult{ChosenInlineResult: *u.ChosenInlineResult}}
	ctx.User.ctx = ctx

	/*chatID:=0
	for _,hook:=range ctx.User.data.Hooks{
		if SliceContainsString(hook.Services,service.Name) {
			for _, chat := range hook.Chats {

			}
		}
	}*/
	log.Debug("InlineMessageID: ", u.ChosenInlineResult.InlineMessageID)
	msg := OutgoingMessage{
		ParseMode:  "HTML",
		WebPreview: true,
		Message: Message{
			ID:          bson.NewObjectId(),
			InlineMsgID: u.ChosenInlineResult.InlineMessageID,
			Text:        u.ChosenInlineResult.Query, // Todo: thats a lie. The actual message content is known while producing inline results
			FromID:      u.ChosenInlineResult.From.ID,
			BotID:       b.ID,
			Date:        time.Now(),
		}}

	// workaround to match between inline_msg_id and msg_id
	dupFound := false
	var l int64
	lastMsgIDByUserMutex.Lock()
	if lm, exists := lastMsgIDByUser[u.ChosenInlineResult.From.ID]; exists {
		if lm.BotID == b.ID && lm.ID != 0 {
			l = time.Now().Sub(lm.TS).Nanoseconds()
			if l < 1000000000 {
				log.Debugf("chosen: dup found (msg %v) for %v (user %v), after %d", lm.ID, u.ChosenInlineResult.InlineMessageID, u.ChosenInlineResult.From.ID, l)

				dupFound = true
				msg.MsgID = lm.ID
				msg.ChatID = lm.ChatID
			}
		}
	}
	if !dupFound {
		log.Debugf("chosen: dup not found for %v (user %v), after %d", u.ChosenInlineResult.InlineMessageID, u.ChosenInlineResult.From.ID, l)
		lastMsgIDByUser[u.ChosenInlineResult.From.ID] = msgInfo{InlineID: u.ChosenInlineResult.InlineMessageID, TS: time.Now(), BotID: b.ID}
	}
	lastMsgIDByUserMutex.Unlock()

	// we need to save this message!

	err = db.C("messages").Insert(&msg)

	if err != nil {
		ctx.Log().WithError(err).Error("tgChosenInlineResultHandler: msg insert")
	}
	ctx.ChosenInlineResult.Message = &msg
	ctx.Log().WithField("message", &msg).Debug("tgChosenInlineResultHandler")
	return service, ctx
}

func tgEditedMessageHandler(u *tg.Update, b *Bot, db *mgo.Database) (*Service, *Context) {
	im := incomingMessageFromTGMessage(u.EditedMessage)
	im.BotID = b.ID
	service, err := detectServiceByBot(b.ID)

	if err != nil {
		log.WithError(err).WithField("bot", b.ID).Error("Can't detect service")
	}
	ctx := &Context{ServiceName: service.Name, Chat: im.Chat, db: db}
	if im.From.ID != 0 {
		ctx.User = im.From
		ctx.User.ctx = ctx
	}
	ctx.Chat.ctx = ctx

	ctx.Message = &im
	ctx.MessageEdited = true
	rm, _ := findMessage(db, im.ChatID, b.ID, im.MsgID)
	if rm != nil {
		log.Debugf("Received edit for message %d", rm.MsgID)

		if rm.OnEditAction != "" {
			log.Debugf("onEditHandler found %s", rm.OnEditAction)
			// Instantiate a new variable to hold this argument
			if handler, ok := actionFuncs[rm.OnEditAction]; ok {
				handlerType := reflect.TypeOf(handler)
				log.Debugf("handler %v: %v %v\n", rm.OnEditAction, handlerType.String(), handlerType.Kind().String())
				handlerArgsInterfaces := make([]interface{}, handlerType.NumIn()-1)
				handlerArgs := make([]reflect.Value, handlerType.NumIn())

				for i := 1; i < handlerType.NumIn(); i++ {
					dataVal := reflect.New(handlerType.In(i))
					handlerArgsInterfaces[i-1] = dataVal.Interface()
				}
				if err := decode(rm.OnEditData, &handlerArgsInterfaces); err != nil {
					log.WithField("handler", rm.OnEditAction).WithError(err).Error("Can't decode editHandler's args")
				}
				handlerArgs[0] = reflect.ValueOf(ctx)
				for i := 0; i < len(handlerArgsInterfaces); i++ {
					handlerArgs[i+1] = reflect.ValueOf(handlerArgsInterfaces[i])
				}

				if len(handlerArgs) > 0 {
					handlerVal := reflect.ValueOf(handler)
					returnVals := handlerVal.Call(handlerArgs)

					if !returnVals[0].IsNil() {
						err := returnVals[0].Interface().(error)
						// NOTE: panics will be caught by the recover statement above
						log.WithField("handler", rm.OnEditAction).WithError(err).Error("editHandler failed")
					}
				}
			} else {
				log.WithField("handler", rm.OnEditAction).Error("Edit handler not registered")
			}

		}

	}

	return service, ctx
}

func tgIncomingMessageHandler(u *tg.Update, b *Bot, db *mgo.Database) (*Service, *Context) {
	im := incomingMessageFromTGMessage(u.Message)
	im.BotID = b.ID

	// workaround to match between inline_msg_id and msg_id
	dupFound := false
	var l int64
	if u.Message.From != nil {
		lastMsgIDByUserMutex.Lock()

		if lm, exists := lastMsgIDByUser[u.Message.From.ID]; exists {
			if lm.BotID == b.ID && lm.InlineID != "" {
				l = time.Now().Sub(lm.TS).Nanoseconds()

				if l < 1000000000 {
					dupFound = true
					log.Debugf("message: dup found (inlinemsgid %v) for %v (user %v), after %d", lm.InlineID, u.Message.MessageID, u.Message.From.ID, l)
					db.C("messages").Update(bson.M{"botid": b.ID, "inlinemsgid": lm.InlineID}, bson.M{"$set": bson.M{"chatid": im.ChatID, "msgid": im.MsgID}})
					lastMsgIDByUserMutex.Unlock()
					//log.Error(bson.M{"botid": b.ID, "inlinemsgid": lm.InlineID}, bson.M{"$set": bson.M{"chatid": im.ChatID, "msgid": im.MsgID}}, err)
					return nil, nil
				}
			}
		}
		if !dupFound {
			lastMsgIDByUser[u.Message.From.ID] = msgInfo{ID: u.Message.MessageID, TS: time.Now(), BotID: b.ID, ChatID: u.Message.Chat.ID}
		}
		lastMsgIDByUserMutex.Unlock()
	}
	service, err := detectServiceByBot(b.ID)
	//fmt.Printf("detectService: %+v\n", service)

	if err != nil {
		log.WithError(err).WithField("bot", b.ID).Error("Can't detect service")
	}
	ctx := &Context{ServiceName: service.Name, Chat: im.Chat, db: db}
	if im.From.ID != 0 {
		ctx.User = im.From
		ctx.User.ctx = ctx
	}
	ctx.Chat.ctx = ctx

	var rm *Message
	if im.ReplyToMessage != nil && im.ReplyToMessage.MsgID != 0 {
		rm, _ = findMessage(db, im.ReplyToMessage.ChatID, b.ID, im.ReplyToMessage.MsgID)
		im.ReplyToMessage = rm
		if rm != nil {
			im.ReplyToMessage.BotID = b.ID
			im.ReplyToMsgID = im.ReplyToMessage.MsgID
		}

	} else if im.Chat.IsPrivate() {
		// For private chat all messages is reply for the last message. We need to parse message for /commands first
		cmd, _ := im.GetCommand()
		if cmd == "" {
			// If there is active keyboard – received message is reply for the source message
			kb, _ := ctx.keyboard()
			if kb.MsgID > 0 {
				rm, err = findMessage(db, im.Chat.ID, b.ID, kb.MsgID)
				if rm == nil {
					ctx.Log().WithError(err).WithField("msgid", kb.MsgID).WithField("botid", b.ID).Error("Keyboard message source not found")
				}
			}

			if rm == nil {
				rm, err = findLastOutgoingMessageInChat(db, b.ID, im.ChatID)

				if err != nil && err.Error() != "not found" {
					ctx.Log().WithError(err).Error("Error on findLastOutgoingMessageInChat")
				} else if rm != nil {
					if rm.om.DisablePMReplyIfTheLast || rm.om.OnReplyAction == "" {
						rm = nil
					}
				}
			}

			// Leave ReplyToMessage empty to avoid unnecessary db request in case we don't need prev message.
			im.ReplyToMessage = rm
			if rm != nil {
				im.ReplyToMessage.BotID = b.ID
				im.ReplyToMsgID = im.ReplyToMessage.MsgID
			}
		}
	}

	ctx.Message = &im
	if rm != nil {
		log.Debugf("Received reply for message %d", rm.MsgID)
		// TODO: detect service by ReplyHandler
		if rm.OnReplyAction != "" {
			log.Debugf("ReplyHandler found %s", rm.OnReplyAction)
			// Instantiate a new variable to hold this argument
			if handler, ok := actionFuncs[rm.OnReplyAction]; ok {
				handlerType := reflect.TypeOf(handler)
				log.Debugf("handler %v: %v %v\n", rm.OnReplyAction, handlerType.String(), handlerType.Kind().String())
				handlerArgsInterfaces := make([]interface{}, handlerType.NumIn()-1)
				handlerArgs := make([]reflect.Value, handlerType.NumIn())

				for i := 1; i < handlerType.NumIn(); i++ {
					dataVal := reflect.New(handlerType.In(i))
					handlerArgsInterfaces[i-1] = dataVal.Interface()
				}
				if err := decode(rm.OnReplyData, &handlerArgsInterfaces); err != nil {
					log.WithField("handler", rm.OnReplyAction).WithError(err).Error("Can't decode replyHandler's args")
				}
				handlerArgs[0] = reflect.ValueOf(ctx)
				for i := 0; i < len(handlerArgsInterfaces); i++ {
					handlerArgs[i+1] = reflect.ValueOf(handlerArgsInterfaces[i])
				}

				if len(handlerArgs) > 0 {
					handlerVal := reflect.ValueOf(handler)
					returnVals := handlerVal.Call(handlerArgs)

					if !returnVals[0].IsNil() {
						err := returnVals[0].Interface().(error)
						// NOTE: panics will be caught by the recover statement above
						log.WithField("handler", rm.OnReplyAction).WithError(err).Error("replyHandler failed")
					}
				}
			} else {
				log.WithField("handler", rm.OnReplyAction).Error("Reply handler not registered")
			}

		}

	}

	// update PrivateStarted in case we received private message from user
	if ctx != nil && ctx.Message != nil && ctx.Message.ChatID == ctx.User.ID {
		protected, err := ctx.User.protectedSettings()
		if err != nil {
			log.WithError(err).Error("tgIncomingMessageHandler protectedSettings error")
		} else {
			if !protected.PrivateStarted {
				protected.PrivateStarted = true
				ctx.User.saveProtectedSettings()
			}
		}
	}

	return service, ctx

}

func removeHooksForChat(db *mgo.Database, serviceName string, chatID int64) {
	err := db.C("users").Update(bson.M{"hooks.services": []string{serviceName}, "hooks.chats": chatID}, bson.M{"$pull": bson.M{"hooks.$.chats": chatID}})
	if err != nil {
		log.WithError(err).Error("removeHooksForChat remove outdated chats hook from users")
	}

	err = db.C("chats").Update(bson.M{"_id": chatID, "hooks.services": []string{serviceName}}, bson.M{"$unset": bson.M{"hooks.$": 0}})
	if err != nil {
		log.WithError(err).Error("removeHooksForChat remove outdated hook from chats")
	}
}

func migrateToSuperGroup(db *mgo.Database, fromChatID int64, toChatID int64) {
	var chat chatData
	_, err := db.C("chats").FindId(fromChatID).Apply(mgo.Change{
		Update: bson.M{"$set": bson.M{"migratedtochatid": toChatID}, "$unset": bson.M{"hooks": "", "membersids": ""}},
	}, &chat)
	if err != nil {
		log.WithError(err).Error("migrateToSuperGroup remove")
	}

	if chat.ID != 0 {
		chat.ID = toChatID
		chat.Type = "supergroup"
		err := db.C("chats").Insert(chat)
		if err != nil {
			log.WithError(err).Error("migrateToSuperGroup insert")
		}
	}

	err = db.C("users").Update(bson.M{"hooks.chats": fromChatID}, bson.M{"$addToSet": bson.M{"hooks.$.chats": toChatID}})
	if err != nil {
		log.WithError(err).Error("migrateToSuperGroup add new hook chats")
	}
	err = db.C("users").Update(bson.M{"hooks.chats": toChatID}, bson.M{"$pull": bson.M{"hooks.$.chats": fromChatID}})
	if err != nil {
		log.WithError(err).Error("migrateToSuperGroup remove outdated hook chats")
	}
}
func tgUpdateHandler(u *tg.Update, b *Bot, db *mgo.Database) (*Service, *Context) {

	if u.Message != nil {
		if u.Message.LeftChatMember != nil {
			db.C("chats").UpdateId(u.Message.Chat.ID, bson.M{"$pull": bson.M{"membersids": u.Message.From.ID}})
			return nil, nil
		} else if u.Message.MigrateToChatID != 0 {
			log.Infof("Group %v migrated to supergroup %v", u.Message.Chat.ID, u.Message.MigrateToChatID)
			migrateToSuperGroup(db, u.Message.Chat.ID, u.Message.MigrateToChatID)
			return nil, nil
		} else {
			//todo: need optimization
			if u.Message.Chat.IsGroup() || u.Message.Chat.IsSuperGroup() {
				db.C("chats").UpdateId(u.Message.Chat.ID, bson.M{"$addToSet": bson.M{"membersids": u.Message.From.ID}})
			}
		}
		return tgIncomingMessageHandler(u, b, db)
	} else if u.CallbackQuery != nil {
		if u.CallbackQuery.Message != nil && (u.CallbackQuery.Message.Chat.IsGroup() || u.CallbackQuery.Message.Chat.IsSuperGroup()) {
			db.C("chats").UpdateId(u.CallbackQuery.Message.Chat.ID, bson.M{"$addToSet": bson.M{"membersids": u.CallbackQuery.From.ID}})
		}
		return tgCallbackHandler(u, b, db)
	} else if u.InlineQuery != nil {

		return tgInlineQueryHandler(u, b, db)
	} else if u.ChosenInlineResult != nil {

		return tgChosenInlineResultHandler(u, b, db)
	} else if u.EditedMessage != nil {

		return tgEditedMessageHandler(u, b, db)
	} else if u.ChannelPost != nil {
		u.Message = u.ChannelPost
		return tgIncomingMessageHandler(u, b, db)
	} else if u.EditedChannelPost != nil {
		u.EditedMessage = u.EditedChannelPost
		return tgEditedMessageHandler(u, b, db)
	}

	return nil, nil

}

/*func (im *IncomingMessage) ButtonAnswer(db *mgo.Database) (key string, text string) {
	if im.Message.ReplyToMsgID == 0 {
		return
	}

	om, err := im.ReplyToMessage.FindOutgoingMessage(db)
	if err != nil || om == nil {
		return
	}
	if val, ok := om.Buttons[im.Text]; ok {
		return val, im.Text
	}
	return
}*/

func (m *Message) saveToDB(db *mgo.Database) error {
	return db.C("messages").Insert(m)
}

// IsEventBotAddedToGroup returns true if user created a new group with bot as member or add the bot to existing group
func (m *IncomingMessage) IsEventBotAddedToGroup() bool {
	if (m.NewChatMember != nil && m.NewChatMember.ID == m.BotID) || m.GroupChatCreated || m.SuperGroupChatCreated {
		return true
	}
	return false
}

// GetCommand parses received message text for bot command. Returns the command and after command text if presented
func (m *IncomingMessage) GetCommand() (string, string) {
	text := m.Text

	if !strings.HasPrefix(text, "/") {
		return "", text
	}
	r, _ := regexp.Compile("^/([a-zA-Z0-9_]+)(?:@[a-zA-Z0-9_]+)?.?(.*)?$")
	match := r.FindStringSubmatch(text)
	if len(match) == 3 {
		return match[1], match[2]
	} else if len(match) == 2 {
		return match[1], ""
	}
	return "", ""

}

func detectServiceByBot(botID int64) (*Service, error) {
	serviceName := ""
	if botID > 0 {
		if bot := botByID(botID); bot != nil {
			if len(bot.services) == 1 {
				serviceName = bot.services[0].Name
			}
		}
	}
	if serviceName == "" {
		return &Service{}, fmt.Errorf("Can't determine active service for bot with ID=%d. No messages found.", botID)
	}
	if val, ok := services[serviceName]; ok {
		return val, nil
	}
	return &Service{}, errors.New("Unknown service: " + serviceName)

}
func (m *Message) detectService(db *mgo.Database) (*Service, error) {
	serviceName := ""

	if m.BotID > 0 {
		if bot := botByID(m.BotID); bot != nil {
			if len(bot.services) == 1 {
				serviceName = bot.services[0].Name
			}
		}
	}

	if serviceName == "" {
		return &Service{}, fmt.Errorf("Can't determine active service for bot with ID=%d. No messages found.", m.BotID)
	}
	if val, ok := services[serviceName]; ok {
		return val, nil
	}
	return &Service{}, errors.New("Unknown service: " + serviceName)
}
