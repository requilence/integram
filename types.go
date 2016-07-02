package integram

import (
	"time"

	"github.com/requilence/integram/url"

	log "github.com/Sirupsen/logrus"
	"github.com/mrjones/oauth"
	"gopkg.in/mgo.v2/bson"
)

type User struct {
	ID        int64 `bson:"_id"`
	FirstName string
	LastName  string `bson:",omitempty"`
	UserName  string `bson:",omitempty"`
	Tz        string

	ctx  *Context // provide pointer to Context for convenient nesting and DB quering
	data *userData
}

type Chat struct {
	ID        int64  `bson:"_id"`
	Type      string `bson:",omitempty"`
	FirstName string
	LastName  string `bson:",omitempty"`
	UserName  string `bson:",omitempty"`
	Title     string `bson:",omitempty"`
	Tz        string `bson:",omitempty"`

	ctx  *Context // provide pointer to Context for convenient nesting and DB quering
	data *chatData
}

type OAuthProvider struct {
	Service string  // Service name
	BaseURL url.URL // Scheme + Host. Default https://{service.DefaultHost}
	ID      string  // OAuth ID
	Secret  string  // OAuth Secret
}

// workaround to save urls as struct
func (o *OAuthProvider) toBson() bson.M {
	return bson.M{"baseurl": struct {
		Scheme string
		Host   string
		Path   string
	}{o.BaseURL.Scheme, o.BaseURL.Host, o.BaseURL.Path},
		"id":      o.ID,
		"secret":  o.Secret,
		"service": o.Service}

}

func (oap *OAuthProvider) internalID() string {
	if oap == nil {
		log.Errorf("OAuthProvider is empty")
	}
	s, _ := serviceByName(oap.Service)
	//spew.Dump(oap, s)

	if s != nil && oap.BaseURL.Host == s.DefaultBaseURL.Host {
		return s.Name
	}

	return checksumString(s.Name + oap.BaseURL.Host)
}

// returns impersonal Redirect URL, useful when setting up the OAuth Client
func (oap *OAuthProvider) RedirectURL() string {
	return BaseURL + "/auth/" + oap.internalID()

}

// True if OAuth provider(app,client) has ID and Secret. Should be always true for service's default provider
func (oap *OAuthProvider) IsSetup() bool {
	if oap == nil {
		return false
	}

	return oap.ID != "" && oap.Secret != ""
}

type oAuthIDCache struct {
	UserID  int
	Service string
	Val     oAuthIDCacheVal
}
type oAuthIDCacheVal struct {
	oauth.RequestToken `bson:",omitempty"`
	BaseURL            string
}

// Webhook token for service
type serviceHook struct {
	Token    string
	Services []string // For backward compatibility with universal hook
	Chats    []int64  `bson:",omitempty"` // Chats that will receive notifications on this hook
}

// Struct for user's data. Used to store in MongoDB
type userData struct {
	User            `bson:",inline"`
	KeyboardPerChat []chatKeyboard            // stored map for Telegram Bot's keyboard
	Protected       map[string]*userProtected // Protected settings used for some core functional
	Settings        map[string]interface{}
	Hooks           []serviceHook
}

// Core settings for Telegram User behavior per Service
type userProtected struct {
	PrivateStarted    bool   // if user previously send any private msg to bot
	OAuthToken        string // Oauth token. Used to perform API queries
	OAuthExpireDate   *time.Time
	OAuthRefreshToken string
	AuthTempToken     string // Temp token for redirect to time-limited Oauth URL to authorize the user (F.e. Trello)

	AfterAuthHandler string // Used to store function that will be called after succesfull auth. F.e. in case of interactive reply in chat for non-authed user
	AfterAuthData    []byte // Gob encoded arg's
}

// Struct for chat's data. Used to store in MongoDB
type chatData struct {
	Chat             `bson:",inline"`
	KeyboardPerBot   []chatKeyboard `bson:",omitempty"`
	Settings         map[string]interface{}
	Hooks            []serviceHook
	MembersCount     int
	MembersIDs       []int64
	Deactivated      bool  `bson:",omitempty"`
	MigratedToChatID int64 `bson:",omitempty"`
}

type chatKeyboard struct {
	MsgID    int               // ID of message sent with this keyboard
	ChatID   int64             `bson:",minsize"` // ID of chat where this keyboard shown
	BotID    int64             `bson:",minsize"` // ID of bot who sent this keyboard
	Date     time.Time         // Date when keyboard was sent
	Keyboard map[string]string // Keyboard's md5(text):key map
}

// Struct used to store WebPreview redirection trick in MongoDB
type webPreview struct {
	Title     string
	Headline  string
	Text      string
	Url       string
	ImageURL  string
	Token     string `bson:"_id"`
	Redirects int
	Created   time.Time
}

// @username if available. First + Last name otherwise
func (u *User) Mention() string {

	if u.UserName != "" {
		return "@" + u.UserName
	}

	name := u.FirstName
	if u.LastName != "" {
		name += " " + u.LastName
	}

	return name
}

// Method to implement String interface
func (u *User) String() string {
	if u.UserName != "" {
		return u.UserName
	}

	name := u.FirstName
	if u.LastName != "" {
		name += " " + u.LastName
	}

	return name
}

// Detected User timezone
func (u *User) TzLocation() *time.Location {
	return tzLocation(u.Tz)
}

func (c *Chat) IsGroup() bool {
	if c.ID < 0 {
		return true
	} else {
		return false
	}
}

func (c *Chat) IsPrivate() bool {
	if c.ID > 0 {
		return true
	} else {
		return false
	}
}
