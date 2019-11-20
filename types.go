package integram

import (
	"time"

	"crypto/md5"
	"encoding/base64"
	"github.com/mrjones/oauth"
	"github.com/requilence/url"
	log "github.com/sirupsen/logrus"
	"gopkg.in/mgo.v2/bson"
)

// User information initiated from TG
type User struct {
	ID        int64 `bson:"_id"`
	FirstName string
	LastName  string `bson:",omitempty"`
	UserName  string `bson:",omitempty"`
	Tz        string
	Lang	  string

	ctx  *Context // provide pointer to Context for convenient nesting and DB quering
	data *userData
}

// Chat information initiated from TG
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

// OAuthProvider contains OAuth application info
type OAuthProvider struct {
	Service string  // Service name
	BaseURL url.URL `envconfig:"OAUTH_BASEURL"`                // Scheme + Host. Default https://{service.DefaultHost}
	ID      string  `envconfig:"OAUTH_ID" required:"true"`     // OAuth ID
	Secret  string  `envconfig:"OAUTH_SECRET" required:"true"` // OAuth Secret
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

func (o *OAuthProvider) internalID() string {
	if o == nil {
		log.Errorf("OAuthProvider is empty")
	}
	s, _ := serviceByName(o.Service)

	if s != nil && o.BaseURL.Host == s.DefaultBaseURL.Host {
		return s.Name
	}

	return checksumString(s.Name + o.BaseURL.Host)
}

// RedirectURL returns impersonal Redirect URL, useful when setting up the OAuth Client
func (o *OAuthProvider) RedirectURL() string {
	return Config.BaseURL + "/auth/" + o.internalID()

}

// IsSetup returns true if OAuth provider(app,client) has ID and Secret. Should be always true for service's default provider
func (o *OAuthProvider) IsSetup() bool {
	if o == nil {
		return false
	}

	return o.ID != "" && o.Secret != ""
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
	OAuthToken        string // Oauth token. Used to perform API queries
	OAuthExpireDate   *time.Time
	OAuthRefreshToken string
	AuthTempToken     string // Temp token for redirect to time-limited Oauth URL to authorize the user (F.e. Trello)
	OAuthValid    	  bool // used for stat purposes
	OAuthStore    	  string // to detect whether non-standard store used

	AfterAuthHandler string // Used to store function that will be called after successful auth. F.e. in case of interactive reply in chat for non-authed user
	AfterAuthData    []byte // Gob encoded arg's
}

// Core settings for Telegram Chat behavior per Service
type chatProtected struct {
	BotStoppedOrKickedAt *time.Time `bson:",omitempty"`  // when we informed that bot was stopped by user
}

// Struct for chat's data. Used to store in MongoDB
type chatData struct {
	Chat               `bson:",inline"`
	KeyboardPerBot     []chatKeyboard `bson:",omitempty"`
	Settings           map[string]interface{}
	Protected          map[string]*chatProtected

	Hooks              []serviceHook
	MembersIDs         []int64
	Deactivated        bool  	  `bson:",omitempty"`
	MigratedToChatID   int64	  `bson:",omitempty"`
	MigratedFromChatID int64	  `bson:",omitempty"`
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
	URL       string
	ImageURL  string
	Token     string `bson:"_id"`
	Hash      string
	Redirects int
	Created   time.Time
}

func (wp *webPreview) CalculateHash() string {
	md5Hash := md5.Sum([]byte(wp.Title + wp.Headline + wp.Text + wp.URL + wp.ImageURL))

	return base64.URLEncoding.EncodeToString(md5Hash[:])
}

// Mention returns @username if available. First + Last name otherwise
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

// TzLocation retrieve User's timezone if stored in DB
func (u *User) TzLocation() *time.Location {
	return tzLocation(u.Tz)
}

// IsGroup returns true if chat is a group chat
func (c *Chat) IsGroup() bool {
	if c.ID < 0 {
		return true
	}
	return false

}

// IsPrivate returns true if chat is a private chat
func (c *Chat) IsPrivate() bool {
	if c.ID > 0 {
		return true
	}
	return false
}
