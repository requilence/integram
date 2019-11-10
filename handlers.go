package integram

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	stdlog "log"
	"net/http"
	"net/http/httputil"
	nativeurl "net/url"
	"os"
	"os/signal"
	"path"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/requilence/url"
	log "github.com/sirupsen/logrus"

	"github.com/weekface/mgorus"
	"golang.org/x/oauth2"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"

	"github.com/requilence/jobs"
)

var startedAt time.Time

func getCurrentDir() string {
	_, filename, _, _ := runtime.Caller(1)
	return path.Dir(filename)
}

func init() {

	if Config.Debug {
		mgo.SetDebug(true)
		gin.SetMode(gin.DebugMode)
		log.SetLevel(log.DebugLevel)
	} else {
		gin.SetMode(gin.ReleaseMode)
		log.SetLevel(log.InfoLevel)
	}
	if Config.InstanceMode != InstanceModeMultiProcessService && Config.InstanceMode != InstanceModeMultiProcessMain && Config.InstanceMode != InstanceModeSingleProcess {
		panic("WRONG InstanceMode " + Config.InstanceMode)
	}
	log.Infof("Integram mode: %s", Config.InstanceMode)

	if _, err := os.Stat(Config.ConfigDir); err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(Config.ConfigDir, os.ModePerm)
			if err != nil {
				log.WithError(err).Errorf("Failed to create the missing ConfigDir at '%s'", Config.ConfigDir)
			}
		}
	}

	startedAt = time.Now()

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

func ReverseProxy(target *url.URL) gin.HandlerFunc {
	proxy := httputil.NewSingleHostReverseProxy((*nativeurl.URL)(target))
	return func(c *gin.Context) {
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}

func gracefulShutdownJobPools() {
	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	exitCode := 0

	go func() {
		sig := <-sigs
		fmt.Printf("Got '%s' signal\n", sig.String())
		for name, pool := range jobs.Pools {
			fmt.Printf("Shutdown '%s' jobs pool...\n", name)
			pool.Close()
			err := pool.Wait()
			if err != nil {
				exitCode = 1
				fmt.Printf("Error while waiting for pool shutdown: %s\n", err.Error())
			}
		}
		fmt.Printf("All jobs pool finished\n")

		done <- true
	}()
	<-done
	syscall.Exit(exitCode)
}

// Run initiates Integram to listen webhooks, TG updates and start the workers pool
func Run() {
	if Config.Debug {
		gin.SetMode(gin.DebugMode)
		log.SetLevel(log.DebugLevel)
	} else {
		gin.SetMode(gin.ReleaseMode)
		log.SetLevel(log.InfoLevel)
	}

	if Config.MongoLogging {
		mongoURIParsed, _ := url.Parse(Config.MongoURL)

		hooker, err := mgorus.NewHooker(mongoURIParsed.Host, mongoURIParsed.Path[1:], "logs")

		if err == nil {
			log.AddHook(hooker)
		}
	}

	// This will test TG tokens and creates API
	time.Sleep(time.Second * 1)
	initBots()

	for _, s := range services {
		if Config.IsStandAloneServiceInstance() {
			// save the service info as a job to Redis. The MAIN instance will process it
			serviceURL := Config.StandAloneServiceURL
			if serviceURL == "" {
				serviceURL = fmt.Sprintf("http://%s:%s", s.Name, Config.Port)
			}
			_, err := ensureStandAloneServiceJob.Schedule(1, time.Now(), s.Name, serviceURL, s.Bot().tgToken())

			if err != nil {
				log.WithError(err).Panic("ensureStandAloneServiceJob error")
			}
		}
	}

	// Configure
	router := gin.New()

	// register some HTML templates
	templ := template.Must(template.New("webpreview").Parse(htmlTemplateWebpreview))
	template.Must(templ.New("determineTZ").Parse(htmlTemplateDetermineTZ))

	router.SetHTMLTemplate(templ)

	// Middlewares
	router.Use(cloneMiddleware)
	router.Use(ginRecovery)
	router.Use(ginLogger)

	if Config.Debug {
		router.Use(gin.Logger())
	}

	if Config.IsMainInstance() || Config.IsSingleProcessInstance() {
		router.StaticFile("/", "index.html")
	}

	router.NoRoute(func(c *gin.Context) {
		// todo: good 404
		if len(c.Request.RequestURI) > 10 && (c.Request.RequestURI[1:2] == "c" || c.Request.RequestURI[1:2] == "u" || c.Request.RequestURI[1:2] == "h") {
			c.String(404, "Hi here!! This link isn't working in a browser. Please follow the instructions in the chat")
		}
	})

	/*
		Possible URLs

		Service webhooks:

		token resolving handled by framework:
		/service_name/token
		/token - DEPRECATED, to be removed, auto-detect the service

		token resolving handled by service
		/service_name/service
		/service_name

		OAuth:
		/oauth1/service_name/auth_temp_id – OAuth1 initial redirect

		/auth/service_name - OAuth2 redirect URL
		/auth/service_name/provider_id - adds provider_id for the custom OAuth provider (e.g. self-hosted instance)


		WebPreview resolving:
		/a/token
	*/

	router.HEAD("/:param1/:param2/:param3", serviceHookHandler)
	router.GET("/:param1/:param2/:param3", serviceHookHandler)
	router.POST("/:param1/:param2/:param3", serviceHookHandler)

	router.HEAD("/:param1/:param2", serviceHookHandler)
	router.GET("/:param1/:param2", serviceHookHandler)
	router.POST("/:param1/:param2", serviceHookHandler)

	router.HEAD("/:param1", serviceHookHandler)
	router.GET("/:param1", serviceHookHandler)
	router.POST("/:param1", serviceHookHandler)

	// Start listening

	var err error

	go gracefulShutdownJobPools()

	if Config.Port == "443" || Config.Port == "1443" {
		if _, err := os.Stat(Config.ConfigDir + string(os.PathSeparator) + "ssl.crt"); !os.IsNotExist(err) {
			log.Infof("SSL: Using ssl.key/ssl.crt")
			err = router.RunTLS(":"+Config.Port, Config.ConfigDir+string(os.PathSeparator)+"ssl.crt", Config.ConfigDir+string(os.PathSeparator)+"ssl.key")
		} else {
			log.Fatalf("INTEGRAM_PORT set to 443, but ssl.crt and ssl.key files not found at '%s'", Config.ConfigDir)
		}

	} else {
		if Config.IsMainInstance() || Config.IsSingleProcessInstance() {
			log.Warnf("WARNING! It is recommended to use Integram with a SSL.\n"+
				"Set the INTEGRAM_PORT to 443 and put integram.crt & integram.key files at '%s'", Config.ConfigDir)
		}
		err = router.Run(":" + Config.Port)
	}

	if err != nil {
		log.WithError(err).Fatal("Can't start the router")
	}
}

func webPreviewHandler(c *gin.Context, token string) {
	db := c.MustGet("db").(*mgo.Database)
	wp := webPreview{}

	err := db.C("previews").Find(bson.M{"_id": token}).One(&wp)

	if err != nil {
		c.String(http.StatusNotFound, "Not found")
		return
	}

	if !strings.Contains(c.Request.UserAgent(), "TelegramBot") {
		db.C("previews").UpdateId(wp.Token, bson.M{"$inc": bson.M{"redirects": 1}})
		c.Redirect(http.StatusMovedPermanently, wp.URL)
		return
	}
	if wp.Text == "" && wp.ImageURL == "" {
		wp.ImageURL = "http://fakeurlaaaaaaa.com/fake/url"
	}

	p := gin.H{"title": wp.Title, "headline": wp.Headline, "text": wp.Text, "imageURL": wp.ImageURL}

	log.WithFields(log.Fields(p)).Debug("WP")

	c.HTML(http.StatusOK, "webpreview", p)

}

// TriggerEventHandler perform search query and trigger EventHandler in context of each chat/user
func (s *Service) TriggerEventHandler(queryChat bool, bsonQuery map[string]interface{}, data interface{}) error {

	if s.EventHandler == nil {
		return fmt.Errorf("EventHandler missed for %s service", s.Name)
	}

	if bsonQuery == nil {
		return nil
	}

	db := mongoSession.Clone().DB(mongo.Database)
	defer db.Session.Close()

	ctx := &Context{db: db, ServiceName: s.Name}
	atLeastOneWasHandled := false

	if queryChat {
		chats, err := ctx.FindChats(bsonQuery)

		if err != nil {
			s.Log().WithError(err).Error("FindChats error")
		}
		for _, chat := range chats {
			if chat.Deactivated || chat.BotWasKickedOrStopped() {
				continue
			}
			ctx.Chat = chat.Chat
			err := s.EventHandler(ctx, data)

			if err != nil {
				ctx.Log().WithError(err).Error("EventHandler returned error")
			} else {
				atLeastOneWasHandled = true
			}
		}
	} else {
		users, err := ctx.FindUsers(bsonQuery)

		if err != nil {
			s.Log().WithError(err).Error("findUsers error")
		}

		for _, user := range users {
			ctx.User = user.User
			ctx.User.ctx = ctx
			ctx.Chat = Chat{ID: user.ID, ctx: ctx}
			err := s.EventHandler(ctx, data)

			if err != nil {
				ctx.Log().WithError(err).Error("EventHandler returned error")
			} else {
				atLeastOneWasHandled = true
			}
		}
	}

	if !atLeastOneWasHandled {
		return errors.New("No single chat was handled")
	}
	return nil
}

var reverseProxiesMap = map[string]*httputil.ReverseProxy{}
var reverseProxiesMapMutex = sync.RWMutex{}

func reverseProxyForService(service string) *httputil.ReverseProxy {
	reverseProxiesMapMutex.RLock()

	if rp, exists := reverseProxiesMap[service]; exists {
		reverseProxiesMapMutex.RUnlock()
		return rp
	}

	reverseProxiesMapMutex.RUnlock()

	s, _ := serviceByName(service)

	if s == nil {
		return nil
	}

	u, _ := nativeurl.Parse(s.machineURL)
	reverseProxiesMapMutex.Lock()
	defer reverseProxiesMapMutex.Unlock()
	rp := httputil.NewSingleHostReverseProxy(u)

	buf := new(bytes.Buffer)
	rp.ErrorLog = stdlog.New(buf, "reverseProxy ", stdlog.LUTC)

	reverseProxiesMap[service] = rp

	return rp
}

func serviceHookHandler(c *gin.Context) {

	// temp ugly routing before deprecating hook URL without service name

	var service string
	var webhookToken string

	var s *Service
	p1 := c.Param("param1")
	p2 := c.Param("param2")
	p3 := c.Param("param3")

	switch p1 {
	// webpreview handler
	case "a":
		webPreviewHandler(c, p2)
		return

	// determine user's TZ and redirect (only withing baseURL)
	case "tz":
		c.HTML(http.StatusOK, "determineTZ", gin.H{"redirectURL": Config.BaseURL + c.Query("r")})
		return

	// /oauth1/service_name
	// /auth/service_name
	case "auth", "oauth1":
		service = p2

	default:

		if p2 != "" {
			// service known
			//
			// /service/token
			service = p1
			webhookToken = p2
		} else {
			// service unknown - to be determined
			//
			// /token
			webhookToken = p1
		}
	}

	if s, _ = serviceByName(service); service != "" && service != "healthcheck" && s == nil {
		c.String(404, "Service not found")
		return
	}

	// in case of multi-process mode redirect from the main process to the corresponding service
	if Config.IsMainInstance() && s != nil {
		proxy := reverseProxyForService(s.Name)
		proxy.ServeHTTP(c.Writer, c.Request)
		return
	}

	if p1 == "oauth1" {
		// /oauth1/service_name/auth_temp_id
		oAuthInitRedirect(c, p2, p3)

		return
	} else if p1 == "auth" {

		if p3 == "" {
			/*
				For the default(usually means non self-hosted) service's OAuth
				/auth/service_name == /auth/service_name/service_name
			*/
			p3 = p2
		}

		// /auth/service_name/provider_id
		oAuthCallback(c, p3)
		return
	}

	db := c.MustGet("db").(*mgo.Database)

	if p1 == "healthcheck" || p2 == "healthcheck" {
		err := healthCheck(db)
		if err != nil {
			c.String(500, err.Error())
			return
		}

		c.String(200, "OK")
		return
	}

	ctx := &Context{db: db, gin: c}

	if s != nil {
		ctx.ServiceName = s.Name
	}

	var hooks []serviceHook

	wctx := &WebhookContext{gin: c, requestID: rndStr.Get(10)}

	// if service has its own TokenHandler use it to resolve the URL query and get the user/chat db Query
	if s != nil && s.TokenHandler != nil {

		if c.Request.Method == "HEAD" {
			c.Status(http.StatusNoContent)
			return
		}

		queryChat, query, err := s.TokenHandler(ctx, wctx)

		if err != nil {
			log.WithFields(log.Fields{"token": webhookToken}).WithError(err).Error("TokenHandler error")
		}

		if query == nil {
			ctx.StatInc(StatWebhookProcessingError)
			c.Status(http.StatusNoContent)
			return
		}

		if queryChat {
			chats, err := ctx.FindChats(query)

			if err != nil {
				log.WithFields(log.Fields{"token": webhookToken}).WithError(err).Error("FindChats error")
			}

			if len(chats) == 0 {
				c.String(http.StatusAccepted, "Webhook accepted but no associated chats found")
				return
			}

			for _, chat := range chats {
				if chat.Deactivated || chat.BotWasKickedOrStopped() {
					continue
				}
				ctxCopy := *ctx
				ctxCopy.Chat = chat.Chat
				ctxCopy.Chat.ctx = &ctxCopy
				err := s.WebhookHandler(&ctxCopy, wctx)

				if err != nil {
					ctxCopy.StatIncChat(StatWebhookProcessingError)
					if err == ErrorFlood {
						c.String(http.StatusTooManyRequests, err.Error())
						return
					} else if strings.HasPrefix(err.Error(), ErrorBadRequstPrefix) {
						c.String(http.StatusBadRequest, err.Error())
						return
					} else {
						ctx.Log().WithFields(log.Fields{"token": webhookToken}).WithError(err).Error("WebhookHandler returned error")
					}
				} else {
					ctxCopy.StatIncChat(StatWebhookHandled)
				}
			}

		} else {
			users, err := ctx.FindUsers(query)

			if err != nil {
				log.WithFields(log.Fields{"token": webhookToken}).WithError(err).Error("findUsers error")
			}

			if len(users) == 0 {
				c.String(http.StatusAccepted, "Webhook accepted but no associated chats found")
				return
			}

			for _, user := range users {
				ctxCopy := *ctx
				ctxCopy.User = user.User
				ctxCopy.User.ctx = &ctxCopy
				ctxCopy.Chat = Chat{ID: user.ID, ctx: &ctxCopy}
				err := s.WebhookHandler(&ctxCopy, wctx)

				if err != nil {
					ctxCopy.StatIncUser(StatWebhookProcessingError)

					if err == ErrorFlood {
						c.String(http.StatusTooManyRequests, err.Error())
						return
					} else if strings.HasPrefix(err.Error(), ErrorBadRequstPrefix) {
						c.String(http.StatusBadRequest, err.Error())
						return
					} else {
						ctxCopy.Log().WithFields(log.Fields{"token": webhookToken}).WithError(err).Error("WebhookHandler returned error")
					}
				} else {
					ctxCopy.StatIncUser(StatWebhookHandled)
				}
			}

		}
		c.AbortWithStatus(http.StatusAccepted)
		return
	} else if webhookToken[0:1] == "u" {
		// Here is some trick
		// If token starts with u - this is notification with TG User behavior (id >0)
		// User can set which groups will receive notifications on this webhook
		// 1 notification can be mirrored to multiple chats

		// If token starts with c - this is notification with TG Chat behavior
		// So just one chat will receive this notification
		user, err := ctx.FindUser(bson.M{"hooks.token": webhookToken})
		// todo: improve this part

		if !(err == nil && user.ID != 0) {
			c.String(http.StatusNotFound, "Unknown user token")
			return
		} else {
			if c.Request.Method == "GET" {
				c.String(200, "Hi here! This link isn't working in a browser. Please follow the instructions in the chat")
				return
			}

			if c.Request.Method == "HEAD" {
				c.Status(200)
				return
			}

		}

		for i, hook := range user.Hooks {
			if hook.Token == webhookToken {
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

		hooks = user.Hooks
	} else if webhookToken[0:1] == "c" || webhookToken[0:1] == "h" {
		chat, err := ctx.FindChat(bson.M{"hooks.token": webhookToken})

		if !(err == nil && chat.ID != 0) {

			c.String(http.StatusNotFound, "Сhat not found")

			return
		} else if chat.Deactivated {
			c.String(http.StatusGone, "TG chat was deactivated")

			return
		} else if chat.BotWasKickedOrStopped() {
			c.String(http.StatusGone, "Bot was kicked or stopped in the TG chat")

			return
		} else {
			if c.Request.Method == "GET" {
				c.String(200, "Hi here! This link isn't working in a browser. Please follow the instructions in the chat")
				return
			}

			if c.Request.Method == "HEAD" {
				c.Status(200)
				return
			}
		}
		hooks = chat.Hooks
		ctx.Chat = chat.Chat
		ctx.Chat.ctx = ctx
	} else {
		c.String(http.StatusNotFound, "Unknown token format")
		return
	}
	atLeastOneChatProcessedWithoutErrors := false

	for _, hook := range hooks {
		if hook.Token != webhookToken {
			continue
		}

		if len(hook.Services) > 1 && Config.IsMainInstance() {
			if s == nil {
				sName := ""

				// temp hack for deprecated multi-service webhooks
				if c.Request.Header.Get("X-Event-Key") != "" {
					sName = "bitbucket"
				} else if c.Request.Header.Get("X-Gitlab-Event") != "" {
					sName = "gitlab"
				} else if c.Request.Header.Get("X-GitHub-Event") != "" {
					sName = "github"
				} else {
					sName = "webhook"
				}

				ctx.Log().Errorf("Deprecated multi-service hook: detected %s", sName)
				s, _ = serviceByName(sName)

			}
		}

		for _, serviceName := range hook.Services {
			// in case this requests contains serviceMame skip the others
			if s != nil && s.Name != serviceName {
				continue
			}

			s, _ = serviceByName(serviceName)

			if s == nil {
				continue
			}

			if Config.IsMainInstance() {
				proxy := reverseProxyForService(serviceName)
				proxy.ServeHTTP(c.Writer, c.Request)
				return
			}

			ctx.ServiceName = serviceName

			if len(hook.Chats) == 0 {
				if ctx.Chat.ID != 0 {
					hook.Chats = []int64{ctx.Chat.ID}
				} else {
					// Some services(f.e. Trello) removes webhook after received 410 HTTP Gone
					// In this case we can safely answer 410 code because we know that DB is up (token was found)
					c.String(http.StatusGone, "No TG chats associated with this webhook URL")
					return
				}
			}

			// todo: if bot kicked or stopped in all chats – need to remove the webhook?

			for _, chatID := range hook.Chats {
				ctxCopy := *ctx
				ctxCopy.Chat = Chat{ID: chatID, ctx: &ctxCopy}

				if ctx.Chat.ID == chatID {
					if ctx.Chat.BotWasKickedOrStopped() || ctx.Chat.data.Deactivated {
						continue
					}
				} else if d, _ := ctxCopy.Chat.getData(); d != nil && (d.BotWasKickedOrStopped() || d.Deactivated) {
					continue
				}
				err := s.WebhookHandler(&ctxCopy, wctx)

				if err != nil {
					if err == ErrorFlood {
						c.String(http.StatusTooManyRequests, err.Error())
						return
					} else if strings.HasPrefix(err.Error(), ErrorBadRequstPrefix) {
						c.String(http.StatusBadRequest, err.Error())
						return
					} else {
						ctxCopy.Log().WithFields(log.Fields{"token": webhookToken}).WithError(err).Error("WebhookHandler returned error")
					}
				} else {

					if ctxCopy.messageAnsweredAt != nil {
						ctxCopy.StatIncChat(StatWebhookProducedMessageToChat)
					}
					atLeastOneChatProcessedWithoutErrors = true
				}
			}
		}

		if atLeastOneChatProcessedWithoutErrors {
			ctx.StatIncUser(StatWebhookHandled)
			c.AbortWithStatus(200)
		} else {
			ctx.StatIncUser(StatWebhookProcessingError)
			log.WithField("token", webhookToken).Warn("Hook not handled")

			// need to answer 2xx otherwise we webhook will be retried and the error will reappear
			// todo: maybe throw 500 if error because of DB fault etc.
			c.AbortWithStatus(http.StatusAccepted)
		}
		return

	}
	c.AbortWithError(404, errors.New("No hooks"))
}

func oAuthInitRedirect(c *gin.Context, service string, authTempID string) {

	if Config.IsMainInstance() && service != "" {
		s, _ := serviceByName(service)

		if s != nil {
			proxy := reverseProxyForService(s.Name)
			proxy.ServeHTTP(c.Writer, c.Request)
			return
		} else {
			log.Errorf("oAuthInitRedirect reverse proxy failed. Service unknown: %s", service)
		}
	}

	db := c.MustGet("db").(*mgo.Database)

	val := oAuthIDCache{}

	err := db.C("users_cache").Find(bson.M{"key": "auth_" + authTempID}).One(&val)

	if !(err == nil && val.UserID > 0) {
		err := errors.New("Unknown auth token")

		log.WithFields(log.Fields{"token": authTempID}).Error(err)
		c.AbortWithError(http.StatusForbidden, errors.New("can't find user"))
		return
	}

	s, _ := serviceByName(val.Service)

	// user's TZ provided
	tzName := c.Request.URL.Query().Get("tz")

	if tzName != "" {
		l, err := time.LoadLocation(tzName)
		if err == nil && l != nil {
			db.C("users").Update(bson.M{"_id": val.UserID}, bson.M{"$set": bson.M{"tz": tzName}})
		} else {
			log.WithError(err).Errorf("oAuthInitRedirect: Bad TZ: %s", tzName)
		}
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
		requestToken, oauthURL, err := o.OAuth1Client(ctx).GetRequestTokenAndUrl(fmt.Sprintf("%s/auth/%s/%s/?state=%s", Config.BaseURL, s.Name, o.internalID(), authTempID))
		if err != nil {
			log.WithField("oauthID", authTempID).WithError(err).Error("Error getting OAuth request URL")
			c.String(http.StatusServiceUnavailable, "Error getting OAuth request URL")
			return
		}
		err = db.C("users_cache").Update(bson.M{"key": "auth_" + authTempID}, bson.M{"$set": bson.M{"val.requesttoken": requestToken}})

		if err != nil {
			ctx.Log().WithError(err).Error("oAuthInitRedirect error updating authTempID")
		}

		c.Redirect(303, oauthURL)
		fmt.Println("HTML")
	} else {
		c.String(http.StatusNotImplemented, "Redirect is for OAuth1 only")
		return
	}
}

func oAuthCallback(c *gin.Context, oauthProviderID string) {

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
		c.String(http.StatusForbidden, "This OAuth is not associated with any user")
		return
	}

	oap, err := findOauthProviderByID(db, oauthProviderID)

	if err != nil {
		log.WithError(err).WithField("OauthProviderID", oauthProviderID).Error("Can't get OauthProvider")
		c.String(http.StatusInternalServerError, "Error occured")
		return
	}

	ctx := &Context{ServiceBaseURL: oap.BaseURL, ServiceName: oap.Service, db: db, gin: c}

	userData, _ := ctx.FindUser(bson.M{"_id": val.UserID})
	s := ctx.Service()

	ctx.User = userData.User
	ctx.User.data = &userData
	ctx.User.ctx = ctx

	ctx.Chat = ctx.User.Chat()

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

	err = oauthTokenStore.SetOAuthAccessToken(&ctx.User, accessToken, expiresAt)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{"oauthID": oauthProviderID}).Error("Can't save OAuth token to store")

		c.String(http.StatusInternalServerError, "failed to save OAuth access token to token store")
		return
	}
	if refreshToken != "" {
		oauthTokenStore.SetOAuthRefreshToken(&ctx.User, refreshToken)
	}

	ctx.StatIncUser(StatOAuthSuccess)

	if s.OAuthSuccessful != nil {
		s.DoJob(s.OAuthSuccessful, ctx)
	}

	c.Redirect(302, "https://telegram.me/"+s.Bot().Username)
}
