package integram

import (
	"errors"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"github.com/requilence/integram/url"
	"github.com/weekface/mgorus"
	"golang.org/x/oauth2"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	tg "gopkg.in/telegram-bot-api.v3"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var pwd string

// Debug used to control logging verbose mode
var Debug = false

func getCurrentDir() string {
	_, filename, _, _ := runtime.Caller(1)
	return path.Dir(filename)
}

func init() {

	if Debug {
		mgo.SetDebug(true)
		//	mgo.SetLogger(log.WithField("mongo", "mongo").Out)

		gin.SetMode(gin.DebugMode)
		log.SetLevel(log.DebugLevel)
	} else {
		gin.SetMode(gin.ReleaseMode)
		log.SetLevel(log.InfoLevel)
	}
	pwd = getCurrentDir()
	dbConnect()
}

func cloneMiddleware(c *gin.Context) {
	s := mongoSession.Clone()

	defer s.Close()

	c.Set("db", s.DB(mongo.Database))
	c.Next()
}

func ginLogger(c *gin.Context) {
	statusCode := c.Writer.Status()
	if statusCode < 200 || statusCode > 299 && statusCode != 404 {
		log.WithFields(log.Fields{
			"path":   c.Request.URL.Path,
			"ip":     c.ClientIP(),
			"method": c.Request.Method,
			"ua":     c.Request.UserAgent(),
			"code":   statusCode,
		}).Error(c.Errors.ByType(gin.ErrorTypePrivate).String())
	}
	c.Next()
}
func ginRecovery(c *gin.Context) {
	defer func() {
		if err := recover(); err != nil {
			stack := stack(3)
			log.WithFields(log.Fields{
				"path":   c.Request.URL.Path,
				"ip":     c.ClientIP(),
				"method": c.Request.Method,
				"ua":     c.Request.UserAgent(),
				"code":   500,
			}).Errorf("Panic recovery -> %s\n%s\n", err, stack)
			c.String(500, "Oops. Something not good.")
		}
	}()
	c.Next()
}

// Run initiates Integram to listen webhooks, TG updates and start the workers pool
func Run() {
	if Debug {
		gin.SetMode(gin.DebugMode)
		log.SetLevel(log.DebugLevel)
	} else {
		gin.SetMode(gin.ReleaseMode)
		log.SetLevel(log.InfoLevel)
	}

	hooker, err := mgorus.NewHooker("localhost:27017", "integram", "logs")
	// This will test TG tokens and creates API
	time.Sleep(time.Second * 1)
	initBots()

	if err == nil {
		log.AddHook(hooker)
	}

	// Configure
	router := gin.New()

	// Middlewares
	router.Use(cloneMiddleware)
	router.Use(ginRecovery)
	router.Use(ginLogger)

	if Debug {
		router.Use(gin.Logger())
	}

	router.LoadHTMLFiles(pwd+"/webpreview.tmpl", pwd+"/oauthredirect.tmpl")

	//router.POST("/:param/tg", tgwebhook)
	router.GET("/a/:param", webPreviewHandler)
	/*router.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "https://integram.org/"+c.Request.RequestURI)
	})*/
	router.StaticFile("/", "index.html")

	router.GET("/oauth1/:param", oAuthInitRedirect)
	router.GET("/auth/:param", oAuthCallback)
	router.GET("/auth", oAuthCallback)
	router.NoRoute(func(c *gin.Context) {
		// todo: good 404
		if len(c.Request.RequestURI) > 10 && (c.Request.RequestURI[1:2] == "c" || c.Request.RequestURI[1:2] == "u" || c.Request.RequestURI[1:2] == "h") {
			c.String(404, "Hi here! This link isn't working in a browser. Please follow the instructions in the chat")
		}
	})
	router.HEAD("/:param")
	router.POST("/:param", serviceHookHandler)

	router.POST("/:param/:service", serviceHookHandler)

	//}

	// Start listening
	port := os.Getenv("INTEGRAM_PORT")
	if port == "" {
		port = "7000"
	}
	if port == "443" || port == "1443" {
		router.RunTLS(":"+port, "integram.crt", "integram.key")
	} else {
		router.Run(":" + port)
	}
	//RunWorkers()

}

func webPreviewHandler(c *gin.Context) {
	db := c.MustGet("db").(*mgo.Database)
	wp := webPreview{}

	err := db.C("previews").Find(bson.M{"_id": c.Param("param")}).One(&wp)

	if err != nil {
		c.AbortWithError(http.StatusNotFound, errors.New("Not found"))
	}

	if !strings.Contains(c.Request.UserAgent(), "TelegramBot") {
		db.C("previews").UpdateId(wp.Token, bson.M{"$inc": bson.M{"redirects": 1}})
		c.Redirect(http.StatusMovedPermanently, wp.URL)
		return
	}
	if wp.Text == "" && wp.ImageURL == "" {
		wp.ImageURL = "http://fake"
	}

	p := gin.H{"title": wp.Title, "headline": wp.Headline, "text": wp.Text, "imageURL": wp.ImageURL}

	log.WithFields(log.Fields(p)).Debug("WP")
	c.HTML(http.StatusOK, "webpreview.tmpl", p)

}

func tgwebhook(c *gin.Context) {

	db := c.MustGet("db").(*mgo.Database)
	u := tg.Update{}
	c.Bind(&u)
	botID, _ := strconv.ParseInt(c.Param("param"), 10, 64)

	bot := botByID(botID)
	if compactHash(bot.token) != c.Query("secret") {
		err := errors.New("Wrong secret provided for TG webhook")
		log.WithField("botID", botID).Error(err)
		c.AbortWithError(http.StatusForbidden, err)
		return
	}

	service, context := tgUpdateHandler(&u, bot, db)
	if context.Message != nil {
		service.TGNewMessageHandler(context)
	}
}

func serviceHookHandler(c *gin.Context) {

	db := c.MustGet("db").(*mgo.Database)

	ctx := &Context{db: db, gin: c}

	token := c.Param("param")
	service := c.Param("service")

	if service != "" {
		ctx.ServiceName = service
	}

	var hooks []serviceHook

	// Here is some trick
	// If token starts with u - this is notification with TG User behavior (id >0)
	// User can set which groups will receive notifications on this webhook
	// 1 notification can be mirrored to multiple chats

	// If token starts with c - this is notification with TG Chat behavior
	// So just one chat will receive this notification

	// If token starts with h - this auto detection. Used for backward compability with previous Integram version

	if token[0:1] == "u" {
		user, err := ctx.findUser(bson.M{"hooks.token": token})
		// todo: improve this part

		for i, hook := range user.Hooks {
			if hook.Token == token {
				user.Hooks = user.Hooks[i : i+1]
				if len(hook.Services) == 1 {
					ctx.ServiceName = hook.Services[0]
				}
				for serviceName := range user.Protected {
					if !SliceContainsString(hook.Services, serviceName) {
						delete(user.Protected, serviceName)
					}
				}

				for serviceName := range user.Settings {
					if !SliceContainsString(hook.Services, serviceName) {
						delete(user.Settings, serviceName)
					}
				}

				break
			}
		}

		ctx.User = user.User
		ctx.User.ctx = ctx

		if !(err == nil && user.ID > 0) {
			err := errors.New("Unknown user token")
			x, _ := ioutil.ReadAll(c.Request.Body)

			ioutil.WriteFile(fmt.Sprintf("./raw/%v_%d.json", token, time.Now().Unix()), x, 0644)

			log.WithFields(log.Fields{"token": token}).Error(err)
			// Todo: Some services(f.e. Trello) removes webhook after received 410 HTTP Gone
			// But this is not safe in case of db unavailable
			//
			// c.AbortWithError(http.StatusGone, err)
			return
		}
		hooks = user.Hooks
	} else if token[0:1] == "c" || token[0:1] == "h" {
		chat, err := ctx.findChat(bson.M{"hooks.token": token})

		if !(err == nil && chat.ID != 0) {
			x, _ := ioutil.ReadAll(c.Request.Body)
			ioutil.WriteFile(fmt.Sprintf("./raw/%v_%d.json", token, time.Now().Unix()), x, 0644)

			err := errors.New("Unknown chat token")
			log.WithFields(log.Fields{"token": token}).Error(err)
			// Todo: Some services(f.e. Trello) removes webhook after received 410 HTTP Gone
			// But this is not safe in case of db unavailable
			//
			// c.AbortWithError(http.StatusGone, err)
			return
		}
		hooks = chat.Hooks
		ctx.Chat = chat.Chat
		ctx.Chat.ctx = ctx
	} else {
		c.AbortWithError(http.StatusNotFound, nil)
		return
	}

	//if ctx.Chat.ID > 0 {
	//	ctx.User = User{ID: ctx.User.ID, FirstName: ctx.User.FirstName, LastName: ctx.User.LastName, UserName: ctx.Chat.UserName}
	//	ctx.User.ctx = ctx
	//}

	//var msg *OutgoingMessage
	wctx := &WebhookContext{gin: c, requestID: randString(10)}

	for _, hook := range hooks {
		if hook.Token == token {
			isHandled := false
			for _, serviceName := range hook.Services {
				s, _ := serviceByName(serviceName)
				if s != nil {
					ctx.ServiceName = serviceName
					if len(hook.Chats) == 0 && ctx.Chat.ID != 0 {
						hook.Chats = []int64{ctx.Chat.ID}
					}

					if len(hook.Chats) > 0 {
						for _, chatID := range hook.Chats {
							ctx.Chat = Chat{ID: chatID, ctx: ctx}
							//spew.Dump("WebhookHandler", ctx)
							err := s.WebhookHandler(ctx, wctx)

							if err != nil {
								ctx.Log().WithFields(log.Fields{"token": token}).WithError(err).Error("WebhookHandler returned error")
							} else {
								isHandled = true
							}

						}
					} else {
						//todo: maybe inform user?
						ctx.Log().WithField("token", token).Warn("No target chats for token")
					}
				}
			}
			if isHandled {
				//c.AbortWithStatus(200)
			} else {
				log.WithField("token", token).Warn("Hook not handled")
			}
			c.AbortWithStatus(200)
			return
		}
	}

}

func oAuthInitRedirect(c *gin.Context) {
	db := c.MustGet("db").(*mgo.Database)

	val := oAuthIDCache{}
	authTempID := c.Param("param")

	err := db.C("users_cache").Find(bson.M{"key": "auth_" + authTempID}).One(&val)

	if !(err == nil && val.UserID > 0) {
		err := errors.New("Unknown auth token")

		log.WithFields(log.Fields{"token": authTempID}).Error(err)
		c.AbortWithError(http.StatusForbidden, errors.New("can't find user"))
		return
	}

	s, _ := serviceByName(val.Service)

	// Ajax request with time zone provided
	tz := c.Request.URL.Query().Get("tz")
	if tz != "" {
		db.C("users").Update(bson.M{"_id": val.UserID}, bson.M{"$set": bson.M{"tz": tz}})
		c.AbortWithStatus(200)
		return
	}

	if s.DefaultOAuth1 != nil {

		u, _ := url.Parse(val.Val.BaseURL)

		if u == nil {
			log.WithField("oauthID", authTempID).WithError(err).Error("BaseURL empty")
			c.String(http.StatusInternalServerError, "Error occurred")
			return
		}
		// Todo: Self-hosted services not implemented for OAuth1
		ctx := &Context{ServiceName: val.Service, ServiceBaseURL: *u, gin: c}
		o := ctx.OAuthProvider()
		requestToken, url, err := o.OAuth1Client(ctx).GetRequestTokenAndUrl(BaseURL + "/auth/" + o.internalID() + "/?state=" + authTempID)
		if err != nil {
			log.WithField("oauthID", authTempID).WithError(err).Error("Error getting OAuth request URL")
			c.String(http.StatusServiceUnavailable, "Error getting OAuth request URL")
			return
		}
		err = db.C("users_cache").Update(bson.M{"key": "auth_" + authTempID}, bson.M{"$set": bson.M{"val.requesttoken": requestToken}})

		// hijack JS redirect to determine user's timezone
		c.HTML(http.StatusOK, "oauthredirect.tmpl", gin.H{"url": url})
		fmt.Println("HTML")
	} else {
		c.String(http.StatusNotImplemented, "Redirect is for OAuth1 only")
		return
	}
}

func oAuthCallback(c *gin.Context) {
	db := c.MustGet("db").(*mgo.Database)

	authTempID := c.Query("u")

	if authTempID == "" {
		authTempID = c.Query("state")
	}

	val := oAuthIDCache{}
	err := db.C("users_cache").Find(bson.M{"key": "auth_" + authTempID}).One(&val)

	if !(err == nil && val.UserID > 0) {
		err := errors.New("Unknown auth token")

		log.WithFields(log.Fields{"token": authTempID}).Error(err)
		c.AbortWithError(http.StatusForbidden, errors.New("can't find user"))
		return
	}
	oauthProviderID := c.Param("param")

	oap, err := findOauthProviderByID(db, oauthProviderID)
	if err != nil {
		log.WithError(err).WithField("OauthProviderID", oauthProviderID).Error("Can't get OauthProvider")
		c.String(http.StatusInternalServerError, "Error occured")
		return
	}

	ctx := &Context{ServiceBaseURL: oap.BaseURL, ServiceName: oap.Service, db: db, gin: c}

	userData, _ := ctx.findUser(bson.M{"_id": val.UserID})
	s := ctx.Service()

	ctx.User = userData.User
	ctx.User.data = &userData

	ctx.User.ctx = ctx
	ctx.Chat.ctx = ctx

	accessToken := ""
	refreshToken := ""
	var expiresAt *time.Time

	if s.DefaultOAuth2 != nil {
		if s.DefaultOAuth2.AccessTokenReceiver != nil {
			accessToken, expiresAt, refreshToken, err = s.DefaultOAuth2.AccessTokenReceiver(ctx, c.Request)
		} else {
			code := c.Request.FormValue("code")

			if code == "" {
				ctx.Log().Error("OAuth2 code is empty")
				return
			}

			var otoken *oauth2.Token
			otoken, err = ctx.OAuthProvider().OAuth2Client(ctx).Exchange(oauth2.NoContext, code)
			if otoken != nil {
				//domain, _ := getDomainFromUrl(baseURL)
				accessToken = otoken.AccessToken
				refreshToken = otoken.RefreshToken
				expiresAt = &otoken.Expiry
			}
		}

	} else if s.DefaultOAuth1 != nil {
		accessToken, err = s.DefaultOAuth1.AccessTokenReceiver(ctx, c.Request, &val.Val.RequestToken)
	}

	if accessToken == "" {
		log.WithError(err).WithFields(log.Fields{"oauthID": oauthProviderID}).Error("Can't verify OAuth token")

		c.String(http.StatusForbidden, err.Error())
		return
	}

	ps, err := ctx.User.protectedSettings()

	ps.OAuthToken = accessToken
	ps.OAuthRefreshToken = refreshToken
	if expiresAt != nil {
		ps.OAuthExpireDate = expiresAt
	}
	ctx.User.saveProtectedSettings()

	if s.OAuthSuccessful != nil {
		s.DoJob(s.OAuthSuccessful, ctx)
	}

	c.Redirect(302, "telegram://"+s.Bot().Username)
}
