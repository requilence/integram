package integram

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"runtime"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/requilence/integram/url"
	"golang.org/x/oauth2"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

var (
	mongoSession *mgo.Session  // Session stores mongo session
	mongo        *mgo.DialInfo // MongoDB Connection info
)

func ObjectIdHex(s string) bson.ObjectId {
	return bson.ObjectIdHex(s)
}

func ensureIndexes() {
	db := mongoSession.DB(mongo.Database)
	db.C("messages").DropIndex("chatid", "botid", "msgid")

	db.C("messages").EnsureIndex(mgo.Index{Key: []string{"botid", "eventid"}})
	db.C("messages").EnsureIndex(mgo.Index{Key: []string{"chatid", "botid", "msgid", "inlinemsgid"}, Unique: true})
	db.C("messages").EnsureIndex(mgo.Index{Key: []string{"chatid", "botid", "fromid"}})
	db.C("messages").EnsureIndex(mgo.Index{Key: []string{"chatid", "botid", "eventid"}}) //todo: test eventID uniqueness

	db.C("previews").EnsureIndex(mgo.Index{Key: []string{"title", "headline", "text", "url", "imageURL"}})

	db.C("chats").EnsureIndex(mgo.Index{Key: []string{"hooks.token"}, Unique: true, Sparse: true})
	db.C("chats").EnsureIndex(mgo.Index{Key: []string{"_id", "membersids"}, Unique: true})

	db.C("users").EnsureIndex(mgo.Index{Key: []string{"hooks.token"}, Unique: true, Sparse: true})
	db.C("users").EnsureIndex(mgo.Index{Key: []string{"protected"}})
	db.C("users").EnsureIndex(mgo.Index{Key: []string{"username"}}) // should be unique but what if users swap usernames... hm
	db.C("users").EnsureIndex(mgo.Index{Key: []string{"keyboardperchat.chatid", "_id"}, Unique: true, Sparse: true})

	db.C("users_cache").EnsureIndex(mgo.Index{Key: []string{"expiresat"}, ExpireAfter: time.Second})
	db.C("users_cache").EnsureIndex(mgo.Index{Key: []string{"key", "userid", "service"}, Unique: true})

	db.C("services_cache").EnsureIndex(mgo.Index{Key: []string{"expiresat"}, ExpireAfter: time.Second})
	db.C("services_cache").EnsureIndex(mgo.Index{Key: []string{"key", "service"}, Unique: true})

	db.C("chats_cache").EnsureIndex(mgo.Index{Key: []string{"expiresat"}, ExpireAfter: time.Second})
	db.C("chats_cache").EnsureIndex(mgo.Index{Key: []string{"key", "chatid", "service"}, Unique: true})

}

func dbConnect() {
	uri := os.Getenv("INTEGRAM_MONGO_URL")

	if uri == "" {
		uri = "mongodb://localhost:27017/integram"
	}

	var err error
	mongo, err = mgo.ParseURL(uri)
	if err != nil {
		log.WithError(err).WithField("url", uri).Panic("Can't parse MongoDB URL")
		panic(err.Error())
	}
	mongoSession, err = mgo.Dial(uri)
	if err != nil {
		log.WithError(err).WithField("url", uri).Panic("Can't connect to MongoDB")
		panic(err.Error())
	}
	mongoSession.SetSafe(&mgo.Safe{})
	log.Infof("MongoDB connected: %s", uri)

	ensureIndexes()
}

func bindInterfaceToInterface(in interface{}, out interface{}, path ...string) error {
	// TODO: need to workaround marshal-unmarshal trick
	var m bson.M
	var err error
	var ok bool
	var inner interface{}
	if reflect.TypeOf(out).Kind() != reflect.Ptr {
		err := errors.New("bindInterfaceToInterface: out interface must be a pointer")
		panic(err)
	}
	if reflect.TypeOf(in).Kind() == reflect.Ptr {
		inner = reflect.ValueOf(in).Elem().Interface()
	} else {
		inner = in
	}

	for _, pathel := range path {
		m, ok = inner.(bson.M)
		if !ok {
			return fmt.Errorf("Can't assert bson.M on %v in %v", pathel, path)
		}
		inner, ok = m[pathel]
		if !ok {
			return fmt.Errorf("Can't get nested level %v in %v", pathel, path)
		}
	}
	innerType := reflect.TypeOf(inner).Kind()
	if innerType == reflect.Slice || innerType == reflect.Array || innerType == reflect.Map || innerType == reflect.Interface {
		var j []byte
		j, err = bson.Marshal(inner)
		if err != nil {
			log.Error(err)
			return err
		}
		err = bson.Unmarshal(j, out)
		if err != nil {
			return err
		}

	} else {
		reflect.ValueOf(out).Elem().Set(reflect.ValueOf(inner))
	}

	if err != nil {
		log.WithField("path", path).WithError(err).Error("can't get nested struct. decode error")
		return err
	}
	return nil
}

func findUsernameByID(db *mgo.Database, id int64) string {
	d := struct{ Username string }{}
	db.C("chats").FindId(id).Select(bson.M{"username": 1}).One(&d)
	return d.Username
}
func (c *Context) findChat(query bson.M) (chatData, error) {
	chat := chatData{}
	serviceID := c.getServiceID()

	err := c.db.C("chats").Find(query).Select(bson.M{"firstname": 1, "lastname": 1, "username": 1, "title": 1, "settings." + serviceID: 1, "keyboardperbot": 1, "tz": 1, "hooks": 1}).One(&chat)
	if err != nil {
		//c.Log().WithError(err).WithField("query", query).Error("Can't find chat")
		return chat, err
	}
	chat.ctx = c

	return chat, nil
}

func (c *Context) findUser(query bson.M) (userData, error) {
	user := userData{}
	serviceID := c.getServiceID()
	var err error
	if serviceID != "" {
		err = c.db.C("users").Find(query).Select(bson.M{"firstname": 1, "lastname": 1, "username": 1, "settings." + serviceID: 1, "protected." + serviceID: 1, "keyboardperchat": bson.M{"$elemMatch": bson.M{"chatid": c.Chat.ID}}, "tz": 1, "hooks": 1}).One(&user) // TODO: IS it ok to lean on c.Chat.ID here?
	} else {
		err = c.db.C("users").Find(query).Select(bson.M{"firstname": 1, "lastname": 1, "username": 1, "settings": 1, "protected": 1, "keyboardperchat": bson.M{"$elemMatch": bson.M{"chatid": c.Chat.ID}}, "tz": 1, "hooks": 1}).One(&user) // TODO: IS it ok to lean on c.Chat.ID here?
	}
	user.ctx = c

	if err != nil {
		return user, err
	}

	return user, nil
}

func (c *Context) findUsers(query bson.M) ([]userData, error) {
	users := []userData{}
	serviceID := c.getServiceID()
	var err error
	if serviceID != "" {
		err = c.db.C("users").Find(query).Select(bson.M{"firstname": 1, "lastname": 1, "username": 1, "settings." + serviceID: 1, "protected." + serviceID: 1, "keyboardperchat": bson.M{"$elemMatch": bson.M{"chatid": c.Chat.ID}}, "tz": 1, "hooks": 1}).All(&users) // TODO: IS it ok to lean on c.Chat.ID here?
	} else {
		err = c.db.C("users").Find(query).Select(bson.M{"firstname": 1, "lastname": 1, "username": 1, "settings": 1, "protected": 1, "keyboardperchat": bson.M{"$elemMatch": bson.M{"chatid": c.Chat.ID}}, "tz": 1, "hooks": 1}).All(&users) // TODO: IS it ok to lean on c.Chat.ID here?
	}

	if err != nil {
		return users, err
	}


	for i, _:=range users{
		users[i].ctx = c
	}


	return users, nil
}

func (c *Context) updateCacheVal(cacheType string, key string, update interface{}, res interface{}) (exists bool) {

	KeyType := reflect.TypeOf("1")
	ElemType := reflect.ValueOf(res).Elem().Type()
	serviceID := c.getServiceID()

	mi := reflect.MakeMap(reflect.MapOf(KeyType, ElemType)).Interface()
	var err error
	//var info *mgo.ChangeInfo
	if cacheType == "user" {
		_, err = c.db.C("users_cache").Find(bson.M{"userid": c.User.ID, "service": serviceID, "key": strings.ToLower(key)}).Select(bson.M{"_id": 0, "val": 1}).Limit(1).Apply(mgo.Change{Update: update, ReturnNew: true}, mi)
	} else if cacheType == "chat" {
		_, err = c.db.C("chats_cache").Find(bson.M{"chatid": c.Chat.ID, "service": serviceID, "key": strings.ToLower(key)}).Select(bson.M{"_id": 0, "val": 1}).Limit(1).Apply(mgo.Change{Update: update, ReturnNew: true}, mi)
	} else if cacheType == "service" {
		_, err = c.db.C("services_cache").Find(bson.M{"service": serviceID, "key": strings.ToLower(key)}).Select(bson.M{"_id": 0, "val": 1}).Limit(1).Apply(mgo.Change{Update: update, ReturnNew: true}, mi)
	} else {
		panic("updateCacheVal, type " + cacheType + " not exists")
	}

	if err != nil {
		log.WithField("service", serviceID).WithField("key", key).WithField("user", c.User.ID).WithField("chat", c.Chat.ID).Debugf(cacheType+" cache updating error: %v", err)
		return false
	}

	if mi == nil {
		return false
	}

	// Wow. Such reflection. Much deep.
	val := reflect.ValueOf(reflect.ValueOf(mi).MapIndex(reflect.ValueOf("val")).Interface())

	if val.IsValid() {
		resVal := reflect.ValueOf(res)
		if resVal.Kind() != reflect.Ptr {
			log.Panic("You need to pass pointer to result interface, not an interface")
			return false
		}

		if !resVal.Elem().IsValid() || !resVal.Elem().CanSet() {
			log.WithField("key", key).Error(cacheType + " cache, can't set to res interface")
			return false
		}
		resVal.Elem().Set(val)
		return true
	}

	return false
}

func (c *Context) getCacheVal(cacheType string, key string, res interface{}) (exists bool) {

	KeyType := reflect.TypeOf("1")
	ElemType := reflect.ValueOf(res).Elem().Type()
	serviceID := c.getServiceID()
	if serviceID == "" {
		c.Log().Errorf("getCacheVal type %s, service not set", cacheType)
		return false
	}

	mi := reflect.MakeMap(reflect.MapOf(KeyType, ElemType)).Interface()
	var err error
	if cacheType == "user" {
		err = c.db.C("users_cache").Find(bson.M{"userid": c.User.ID, "service": serviceID, "key": strings.ToLower(key)}).Select(bson.M{"_id": 0, "val": 1}).One(mi)
	} else if cacheType == "chat" {
		err = c.db.C("chats_cache").Find(bson.M{"chatid": c.Chat.ID, "service": serviceID, "key": strings.ToLower(key)}).Select(bson.M{"_id": 0, "val": 1}).One(mi)
	} else if cacheType == "service" {
		err = c.db.C("services_cache").Find(bson.M{"service": serviceID, "key": strings.ToLower(key)}).Select(bson.M{"_id": 0, "val": 1}).One(mi)
	} else {
		c.Log().Panic("getCacheVal, type " + cacheType + " not exists")
		return false
	}

	if err != nil {
		return false
	}

	if mi == nil {
		return false
	}

	if !reflect.ValueOf(mi).MapIndex(reflect.ValueOf("val")).IsValid(){
		return false
	}
	// Wow. Such reflection. Much deep.
	val := reflect.ValueOf(reflect.ValueOf(mi).MapIndex(reflect.ValueOf("val")).Interface())

	if val.IsValid() {
		resVal := reflect.ValueOf(res)
		if resVal.Kind() != reflect.Ptr {
			log.Panic("You need to pass pointer to result interface, not an interface")
			return false
		}

		if !resVal.Elem().IsValid() || !resVal.Elem().CanSet() {
			log.WithField("key", key).Error(cacheType + " cache, can't set to res interface")
			return false
		}
		resVal.Elem().Set(val)
		return true
	}

	return false
}

// Cache returns if User's cache for specific key exists and try to bind it to res
func (user *User) Cache(key string, res interface{}) (exists bool) {
	return user.ctx.getCacheVal("user", key, res)
}

// Cache returns if Chat's cache for specific key exists and try to bind it to res
func (chat *Chat) Cache(key string, res interface{}) (exists bool) {
	return chat.ctx.getCacheVal("chat", key, res)
}

// ServiceCache returns if Services's cache for specific key exists and try to bind it to res
func (c *Context) ServiceCache(key string, res interface{}) (exists bool) {
	return c.getCacheVal("service", key, res)
}

// IsPrivateStarted indicates if user started the private dialog with a bot (e.g. pressed the start button)
func (user *User) IsPrivateStarted() bool {
	p, _ := user.protectedSettings()
	if p == nil {
		return false
	}
	return p.PrivateStarted
}

// SetCache set the User's cache with specific key and TTL
func (user *User) SetCache(key string, val interface{}, ttl time.Duration) error {
	expiresAt := time.Now().Add(ttl)

	serviceID := user.ctx.getServiceID()
	key = strings.ToLower(key)

	if val == nil {
		err := user.ctx.db.C("users_cache").Remove(bson.M{"userid": user.ID, "service": serviceID, "key": key})
		return err
	}
	_, err := user.ctx.db.C("users_cache").Upsert(bson.M{"userid": user.ID, "service": serviceID, "key": key}, bson.M{"$set": bson.M{"val": val, "expiresat": expiresAt}})
	if err != nil {
		// workaround for WiredTiger bug: https://jira.mongodb.org/browse/SERVER-14322
		if mgo.IsDup(err) {
			return user.ctx.db.C("users_cache").Update(bson.M{"userid": user.ID, "service": serviceID, "key": key}, bson.M{"$set": bson.M{"val": val, "expiresat": expiresAt}})
		}
		log.WithError(err).WithField("key", key).Error("Can't set user cache value")
	}
	return err
}

// UpdateCache updates the per User cache using MongoDB Update query
func (user *User) UpdateCache(key string, update interface{}, res interface{}) error {

	exists := user.ctx.updateCacheVal("user", key, update, res)

	if !exists {
		log.WithField("key", key).Error("Can't update user cache value")
	}
	return nil
}

// SetCache set the Chats's cache with specific key and TTL
func (chat *Chat) SetCache(key string, val interface{}, ttl time.Duration) error {
	expiresAt := time.Now().Add(ttl)
	serviceID := chat.ctx.getServiceID()
	key = strings.ToLower(key)

	if val == nil {
		err := chat.ctx.db.C("chats_cache").Remove(bson.M{"chatid": chat.ID, "service": serviceID, "key": key})
		return err
	}
	_, err := chat.ctx.db.C("chats_cache").Upsert(bson.M{"chatid": chat.ID, "service": serviceID, "key": key}, bson.M{"$set": bson.M{"val": val, "expiresat": expiresAt}})
	if err != nil {
		// workaround for WiredTiger bug: https://jira.mongodb.org/browse/SERVER-14322
		if mgo.IsDup(err) {
			return chat.ctx.db.C("chats_cache").Update(bson.M{"chatid": chat.ID, "service": serviceID, "key": key}, bson.M{"$set": bson.M{"val": val, "expiresat": expiresAt}})
		}
		log.WithError(err).WithField("key", key).Error("Can't set user cache value")
	}
	return err
}

// UpdateCache updates the per Chat cache using MongoDB Update query (see trello service as example)
func (chat *Chat) UpdateCache(key string, update interface{}, res interface{}) error {

	exists := chat.ctx.updateCacheVal("chat", key, update, res)

	if !exists {
		log.WithField("key", key).Error("Can't update chat cache value")
	}
	return nil
}

// SetServiceCache set the Services's cache with specific key and TTL
func (c *Context) SetServiceCache(key string, val interface{}, ttl time.Duration) error {
	expiresAt := time.Now().Add(ttl)
	serviceID := c.getServiceID()
	key = strings.ToLower(key)

	if val == nil {
		err := c.db.C("services_cache").Remove(bson.M{"service": serviceID, "key": key})
		return err
	}

	_, err := c.db.C("services_cache").Upsert(bson.M{"service": serviceID, "key": key}, bson.M{"$set": bson.M{"val": val, "expiresat": expiresAt}})
	if err != nil {
		// workaround for WiredTiger bug: https://jira.mongodb.org/browse/SERVER-14322
		if mgo.IsDup(err) {
			return c.db.C("services_cache").Update(bson.M{"service": serviceID, "key": key}, bson.M{"$set": bson.M{"val": val, "expiresat": expiresAt}})
		}
		log.WithError(err).WithField("key", key).Error("Can't set sevices cache value")
	}
	return err
}

// UpdateServiceCache updates the Services's cache using MongoDB Update query (see trello service as example)
func (c *Context) UpdateServiceCache(key string, update interface{}, res interface{}) error {

	exists := c.updateCacheVal("service", key, update, res)

	if !exists {
		log.WithField("key", key).Error("Can't update sevices cache value")
	}
	return nil
}

func (user *User) updateData() error {
	_, err := user.ctx.db.C("users").UpsertId(user.ID,  bson.M{ "$set": user, "$setOnInsert":bson.M{"createdat":time.Now()}})
	user.data.User = *user

	return err
}

func (chat *Chat) updateData() error {
	_, err := chat.ctx.db.C("chats").UpsertId(chat.ID, bson.M{"$set": chat, "$setOnInsert":bson.M{"createdat":time.Now()}})
	chat.data.Chat = *chat
	return err
}

func (chat *Chat) getData() (*chatData, error) {

	if chat.ID == 0 {
		return nil, errors.New("chat is empty")
	}

	if chat.data != nil {
		fmt.Println("chat.data already loaded")
		return chat.data, nil
	}
	cdata, _ := chat.ctx.findChat(bson.M{"_id": chat.ID})
	chat.data = &cdata

	var err error
	if cdata.Type == "" {
		err = chat.updateData()
	}

	return chat.data, err

}

func (user *User) getData() (*userData, error) {

	if user.ID == 0 {
		return nil, errors.New("user is empty")
	}
	if user.data != nil {
		return user.data, nil
	}
	if user.ctx == nil {
		panic("nil user context")
	}

	udata, err := user.ctx.findUser(bson.M{"_id": user.ID})

	user.data = &udata
	user.Tz = user.data.Tz

	if user.data.FirstName == "" {
		err = user.updateData()
	}

	return user.data, err

}
func (c *Context) getServiceID() string {
	s := c.Service()

	if s == nil {
		// todo: is error handling is need here
		return ""
	}

	if c.ServiceBaseURL.Host == "" {
		return c.ServiceName
	}

	if s.DefaultBaseURL.Host == c.ServiceBaseURL.Host {
		return c.ServiceName
	}

	return s.Name + "_" + escapeDot(c.ServiceBaseURL.Host)

}

func (user *User) protectedSettings() (*userProtected, error) {

	data, err := user.getData()

	if err != nil {
		return nil, err
	}
	//	fmt.Printf("user.getData: %+v\n%v", data, err)

	serviceID := user.ctx.getServiceID()

	if data.Protected == nil {
		data.Protected = make(map[string]*userProtected)
	} else if protected, ok := data.Protected[serviceID]; ok {
		return protected, nil
	}

	data.Protected[serviceID] = &userProtected{}

	// Not a error – just empty settings
	return data.Protected[serviceID], err
}

// Settings bind User's settings for service to the interface
func (user *User) Settings(out interface{}) error {
	data, err := user.getData()

	if err != nil {
		return err
	}
	serviceID := user.ctx.getServiceID()

	if _, ok := data.Settings[serviceID]; ok {
		// TODO: workaround that creepy bindInterfaceToInterface
		err = bindInterfaceToInterface(data.Settings[serviceID], out)
		return err
	}

	// Not a error – just empty settings
	return nil
}

// Settings bind Chat's settings for service to the interface
func (chat *Chat) Settings(out interface{}) error {

	data, err := chat.getData()

	if err != nil {
		return err
	}
	serviceID := chat.ctx.getServiceID()

	if _, ok := data.Settings[serviceID]; ok {
		// TODO: workaround that creepy bindInterfaceToInterface
		err = bindInterfaceToInterface(data.Settings[serviceID], out)
		return err
	}

	// Not a error – just empty settings
	return nil
}

// Setting returns Chat's setting for service with specific key. NOTE! Only builtin types are supported (f.e. structs will become map)
func (chat *Chat) Setting(key string) (result interface{}, exists bool) {
	var settings map[string]interface{}

	err := chat.Settings(&settings)
	if err != nil {
		log.WithError(err).Error("Can't get UserSettings")
		return nil, false
	}

	if _, ok := settings[key]; ok {
		return settings[key], true
	}
	return nil, false
}

// Setting returns Chat's setting for service with specific key
func (user *User) Setting(key string) (result interface{}, exists bool) {
	var settings map[string]interface{}

	err := user.Settings(&settings)
	if err != nil {
		log.WithError(err).Error("Can't get ChatSettings")
		return nil, false
	}

	if _, ok := settings[key]; ok {
		return settings[key], true
	}
	return nil, false
}

// SaveSettings save Chat's setting for service
func (chat *Chat) SaveSettings(allSettings interface{}) error {

	serviceID := chat.ctx.getServiceID()

	_, err := chat.ctx.db.C("chats").UpsertId(chat.ID, bson.M{"$set": bson.M{"settings." + serviceID: allSettings}, "$setOnInsert":bson.M{"createdat":time.Now()}})

	if chat.data == nil {
		chat.data = &chatData{}
	}

	if chat.data.Settings == nil {
		chat.data.Settings = make(map[string]interface{})
	}

	chat.data.Settings[serviceID] = allSettings

	return err
}

// SaveSettings save User's setting for service
func (user *User) SaveSettings(allSettings interface{}) error {

	serviceID := user.ctx.getServiceID()

	_, err := user.ctx.db.C("users").UpsertId(user.ID, bson.M{"$set": bson.M{"settings." + serviceID: allSettings}, "$setOnInsert":bson.M{"createdat":time.Now()}})

	if user.data == nil {
		user.data = &userData{}
	}
	if user.data.Settings == nil {
		user.data.Settings = make(map[string]interface{})
	}
	user.data.Settings[serviceID] = allSettings

	return err
}

func (user *User) addHook(hook serviceHook) error {
	_, err := user.ctx.db.C("users").UpsertId(user.ID, bson.M{"$push": bson.M{"hooks": hook}})
	user.data.Hooks = append(user.data.Hooks, hook)

	return err
}

func (chat *Chat) addHook(hook serviceHook) error {
	_, err := chat.ctx.db.C("chats").UpsertId(chat.ID, bson.M{"$push": bson.M{"hooks": hook}})
	chat.data.Hooks = append(chat.data.Hooks, hook)

	return err
}

// ServiceHookToken returns User's hook token to use in webhook handling
func (user *User) ServiceHookToken() string {
	data, _ := user.getData()
	//TODO: test backward compatibility cases
	for _, hook := range data.Hooks {
		for _, service := range hook.Services {
			if service == user.ctx.ServiceName {
				return hook.Token
			}
		}
	}
	token := "u" + rndStr.Get(10)
	user.addHook(serviceHook{
		Token:    token,
		Services: []string{user.ctx.ServiceName},
	})
	return token
}

// ServiceHookToken returns Chats's hook token to use in webhook handling
func (chat *Chat) ServiceHookToken() string {
	data, _ := chat.getData()
	//TODO: test backward compatibility cases
	fmt.Printf("chatData: %+v\n", data.Hooks)
	for _, hook := range data.Hooks {
		for _, service := range hook.Services {
			if service == chat.ctx.ServiceName {
				return hook.Token
			}
		}
	}
	token := "c" + rndStr.Get(10)
	chat.addHook(serviceHook{
		Token:    token,
		Services: []string{chat.ctx.ServiceName},
	})
	return token
}

// ServiceHookURL returns User's webhook URL for service to use in webhook handling
// Used in case when incoming webhooks despatching on the user behalf to chats
func (user *User) ServiceHookURL() string {
	return BaseURL + "/" + user.ServiceHookToken()
}

// ServiceHookURL returns Chats's webhook URL for service to use in webhook handling
// Used in case when user need to put webhook URL to receive notifications to chat
func (chat *Chat) ServiceHookURL() string {
	return BaseURL + "/" + chat.ServiceHookToken()
}

// AddChatToHook adds the target chat to user's existing hook
func (user *User) AddChatToHook(chatID int64) error {
	data, _ := user.getData()
	token := user.ServiceHookToken()

	for i, hook := range data.Hooks {
		if hook.Token == token {
			for _, service := range hook.Services {
				if service == user.ctx.ServiceName {
					for _, existingChatID := range hook.Chats {
						if existingChatID == chatID {
							return nil
						}
					}
					data.Hooks[i].Chats = append(data.Hooks[i].Chats, chatID)
					err := user.ctx.db.C("users").Update(bson.M{"_id": user.ID, "hooks.services": service}, bson.M{"$addToSet": bson.M{"hooks.$.chats": chatID}})

					return err
				}
			}
		}
	}
	err := errors.New("Can't add chat to serviceHook. Can't find a hook.")
	user.ctx.Log().Error(err)
	return err
}

func (user *User) saveProtectedSettings() error {

	if user.ID == 0 {
		return errors.New("saveProtectedSettings: user is empty")

	}

	if user.data.Protected == nil {
		return errors.New("userData.protected is nil. I won't save it")
	}

	serviceID := user.ctx.getServiceID()
	info, err := user.ctx.db.C("users").UpsertId(user.ID, bson.M{"$set": bson.M{"protected." + serviceID: user.data.Protected[serviceID]}, "$setOnInsert":bson.M{"createdat":time.Now()}})

	fmt.Printf("saveProtectedSettings %v, %+v, %+v\n", err, info, user.data.Protected[serviceID])

	return err
}

func (user *User) saveProtectedSetting(key string, value interface{}) error {

	if user.ID == 0 {
		return errors.New("saveProtectedSetting: user is empty")

	}

	if user.data == nil {
		user.getData()
	}

	if user.data.Protected == nil {
		user.data.Protected = make(map[string]*userProtected)
	}
	serviceID := user.ctx.getServiceID()

	v := reflect.ValueOf(user.data.Protected[serviceID]).Elem().FieldByName(key)
	if v.IsValid() {
		s := reflect.ValueOf(value)
		if s.Type() != v.Type() {
			return errors.New("protected setting with key " + key + " has wrong Type")
		}
		if v.CanSet() {
			v.Set(s)
		}
	} else {
		return errors.New("protected setting with key " + key + " not exists")
	}

	info, err := user.ctx.db.C("users").UpsertId(user.ID, bson.M{"$set": bson.M{"protected." + serviceID + "." + strings.ToLower(key): value}})

	fmt.Printf("saveProtectedSetting %v=%v err %v info %+v\n", key, value, err, info)

	return err
}

// SaveSetting sets Chat's setting for service with specific key
func (chat *Chat) SaveSetting(key string, value interface{}) error {
	serviceID := chat.ctx.getServiceID()

	_, err := chat.ctx.db.C("chats").UpsertId(chat.ID, bson.M{"$set": bson.M{"settings." + serviceID + "." + strings.ToLower(key): value}})
	return err
}

// SaveSetting sets User's setting for service with specific key
func (user *User) SaveSetting(key string, value interface{}) error {

	if user.ID == 0 {
		return errors.New("SaveSetting: user is empty")
	}

	serviceID := user.ctx.getServiceID()

	_, err := user.ctx.db.C("users").UpsertId(user.ID, bson.M{"$set": bson.M{"settings." + serviceID + "." + strings.ToLower(key): value}})
	return err
}

// AuthTempToken returns Auth token used in OAuth process to associate TG user with OAuth creditianals
func (user *User) AuthTempToken() string {

	host := user.ctx.ServiceBaseURL.Host
	if host == "" {
		host = user.ctx.Service().DefaultBaseURL.Host
	}

	serviceBaseURL := user.ctx.ServiceBaseURL.String()
	if serviceBaseURL == "" {
		serviceBaseURL = user.ctx.Service().DefaultBaseURL.String()
	}

	ps, _ := user.protectedSettings()
	if ps.AuthTempToken != "" {
		fmt.Println("found AuthTempToken:" + ps.AuthTempToken)

		oAuthIDCacheFound := oAuthIDCacheVal{}
		if exists := user.Cache("auth_"+ps.AuthTempToken, &oAuthIDCacheFound); exists {

			u, _ := url.Parse(oAuthIDCacheFound.BaseURL)

			if u != nil && u.Host == host {
				return ps.AuthTempToken
			}
		}
	}

	rnd := strings.ToLower(rndStr.Get(16))
	user.SetCache("auth_"+rnd, oAuthIDCacheVal{BaseURL: serviceBaseURL}, time.Hour*24)

	err := user.saveProtectedSetting("AuthTempToken", rnd)

	if err != nil {
		user.ctx.Log().WithError(err).Error("Error saving AuthTempToken")
	}
	return rnd
}

// OauthRedirectURL used in OAuth process as returning URL
func (user *User) OauthRedirectURL() string {
	providerID := user.ctx.OAuthProvider().internalID()
	return BaseURL + "/auth/" + providerID
}

// OauthInitURL used in OAuth process as returning URL
func (user *User) OauthInitURL() string {
	authTempToken := user.AuthTempToken()
	s := user.ctx.Service()
	if authTempToken == "" {
		user.ctx.Log().Error("authTempToken is empty")
		return ""
	}
	if s.DefaultOAuth2 != nil {
		provider := user.ctx.OAuthProvider()

		return provider.OAuth2Client(user.ctx).AuthCodeURL(authTempToken, oauth2.AccessTypeOffline)
	}
	if s.DefaultOAuth1 != nil {
		return BaseURL + "/oauth1/" + authTempToken

	}
	return ""
}

func escapeDot(s string) string {
	return strings.Replace(s, ".", "_", -1)
}

// OAuthHTTPClient returns HTTP client with Bearer authorization headers
func (user *User) OAuthHTTPClient() *http.Client {

	ps, _ := user.protectedSettings()

	if ps == nil {
		return nil
	}

	if ps.OAuthToken == "" {
		return nil
	}

	if user.ctx.Service().DefaultOAuth2 != nil {
		ts := user.ctx.OAuthProvider().OAuth2Client(user.ctx).TokenSource(oauth2.NoContext, &oauth2.Token{AccessToken: ps.OAuthToken, RefreshToken: ps.OAuthRefreshToken, Expiry: *ps.OAuthExpireDate, TokenType: "Bearer"})
		if ps.OAuthExpireDate != nil && ps.OAuthExpireDate.Before(time.Now().Add(time.Second*5)) {
			token, err := ts.Token()
			if err != nil || token == nil {
				if strings.Contains(err.Error(), "revoked") {
					ps.OAuthToken = ""
					ps.OAuthExpireDate = nil
					user.saveProtectedSettings()
				}
				user.ctx.Log().WithError(err).Error("OAuth token refresh failed")
				return nil
			}
			ps.OAuthToken = token.AccessToken
			if token.RefreshToken != "" {
				ps.OAuthRefreshToken = token.RefreshToken
			}
			ps.OAuthExpireDate = &token.Expiry
			user.saveProtectedSettings()
		}
		return oauth2.NewClient(oauth2.NoContext, ts)
	} else if user.ctx.Service().DefaultOAuth1 != nil {
		//todo make a correct httpclient
		return http.DefaultClient
	}
	return nil
}

// OAuthValid checks if OAuthToken for service is set
func (user *User) OAuthValid() bool {
	ps, _ := user.protectedSettings()

	if ps == nil {
		return false
	}

	if ps.OAuthToken == "" {
		return false
	}
	return true
}

// OAuthToken returns OAuthToken for service
func (user *User) OAuthToken() string {
	// todo: oauthtoken per host?
	/*
		host := user.ctx.ServiceBaseURL.Host

		if host == "" {
			host = user.ctx.Service().DefaultBaseURL.Host
		}
	*/
	ps, _ := user.protectedSettings()

	if ps != nil {
		if ps.OAuthExpireDate != nil && ps.OAuthExpireDate.Before(time.Now().Add(time.Second*5)) {
			token, err := user.ctx.OAuthProvider().OAuth2Client(user.ctx).TokenSource(oauth2.NoContext, &oauth2.Token{AccessToken: ps.OAuthToken, Expiry: *ps.OAuthExpireDate, RefreshToken: ps.OAuthRefreshToken}).Token()
			if err != nil {
				user.ctx.Log().WithError(err).Error("OAuth token refresh failed")
				return ""
			}
			ps.OAuthToken = token.AccessToken
			if token.RefreshToken != "" {
				ps.OAuthRefreshToken = token.RefreshToken
			}
			ps.OAuthExpireDate = &token.Expiry
			user.saveProtectedSettings()

		}
		return ps.OAuthToken
	}

	return ""
}

// ResetOAuthToken reset OAuthToken for service
func (user *User) ResetOAuthToken() error {
	err := user.saveProtectedSetting("OAuthToken", "")
	if err != nil {
		user.ctx.Log().WithError(err).Error("ResetOAuthToken error")
	}
	return err
}

// SetAfterAuthAction sets the handlerFunc and it's args that will be triggered on success user Auth.
// F.e. you can use it to resume action interrupted because user didn't authed
// !!! Please note that you must ommit first arg *integram.Context, because it will be automatically prepended on auth success and will contains actual action context
func (user *User) SetAfterAuthAction(handlerFunc interface{}, args ...interface{}) error {
	err := verifyTypeMatching(handlerFunc, args...)
	if err != nil {
		log.WithError(err).Error("Can't verify SetUserAfterAuthHandler args")
		return err
	}

	bytes, err := encode(args)

	if err != nil {
		log.WithError(err).Error("Can't encode SetUserAfterAuthHandler args")
		return err
	}
	ps, _ := user.protectedSettings()

	ps.AfterAuthData = bytes
	ps.AfterAuthHandler = runtime.FuncForPC(reflect.ValueOf(handlerFunc).Pointer()).Name()

	user.saveProtectedSettings()

	return nil
}

func findOauthProviderByID(db *mgo.Database, id string) (*OAuthProvider, error) {
	oap := OAuthProvider{}

	if s, _ := serviceByName(id); s != nil {
		return s.DefaultOAuthProvider(), nil
	}

	err := db.C("oauth_providers").FindId(id).One(&oap)
	if err != nil {
		return nil, err
	}

	return &oap, nil
}

func findOauthProviderByHost(db *mgo.Database, host string) (*OAuthProvider, error) {
	oap := OAuthProvider{}
	err := db.C("oauth_providers").Find(bson.M{"baseurl.host": strings.ToLower(host)}).One(&oap)
	if err != nil {
		return nil, err
	}

	return &oap, nil
}

// WebPreview generate fake webpreview and store it in DB. Telegram will resolve it as we need
func (c *Context) WebPreview(title string, headline string, text string, serviceURL string, imageURL string) (WebPreviewURL string) {
	token := rndStr.Get(10)
	if title == "" {
		title = c.Service().NameToPrint
		c.Log().WithField("token", token).Warn("webPreview: title is empty")
	}

	if headline == "" {
		c.Log().WithField("token", token).Warn("webPreview: headline is empty")
		headline = "-"

	}
	wp := webPreview{
		title,
		headline,
		text,
		serviceURL,
		imageURL,
		token,
		0,
		time.Now(),
	}

	var wpExists webPreview
	c.db.C("previews").Find(bson.M{"title": title, "headline": headline, "text": text, "url": serviceURL, "imageurl": imageURL}).One(&wpExists)

	if wpExists.Token != "" {
		wp = wpExists
	} else {
		err := c.db.C("previews").Insert(wp)

		if err != nil {
			// Wow! So jackpot! Much collision
			wp.Token = rndStr.Get(10)
			err = c.db.C("previews").Insert(wp)
			c.Log().WithError(err).Error("Can't add webpreview")

		}
	}

	return BaseURL + "/a/" + wp.Token

}
