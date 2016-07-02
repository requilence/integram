package integram

import (
	"encoding/gob"
	"errors"
	"net/http"
	"os"
	"reflect"
	"runtime"
	"time"

	"golang.org/x/oauth2"

	"github.com/requilence/integram/url"

	log "github.com/Sirupsen/logrus"
	"github.com/mrjones/oauth"
	"github.com/requilence/jobs"
)

var BaseURL = "https://integram.org"

// Map of Services configs per name. See Register func
var services = make(map[string]*Service)

// Mapping job.Type by job alias names specified in service's config
type jobTypePerJobName map[string]*jobs.Type

var jobsPerService = make(map[string]jobTypePerJobName)

// Map of replyHandlers names to funcs. Use service's config to specify it
var actionFuncs = make(map[string]interface{})

// Channel that use to recover tgUpadates reader after panic inside it
var tgUpdatesRevoltChan = make(chan *Bot)

type Service struct {
	Name        string // Service lowercase name
	NameToPrint string // Service print name
	ImageURL    string // Service thumb image to use in WebPreview if there is no image specified in message. Useful for non-interactive integrations that uses main Telegram's bot.

	DefaultBaseURL url.URL        // Cloud(not self-hosted) URL
	DefaultOAuth1  *DefaultOAuth1 // Cloud(not self-hosted) app data
	DefaultOAuth2  *DefaultOAuth2 // Cloud(not self-hosted) app data
	OAuthRequired  bool           // Is OAuth required in order to receive webhook updates

	JobsPool int // Worker pool to be created for service. Default to 1 worker. Workers will be inited only if jobs types are available

	Jobs []Job // Job types that can be scheduled

	// Functions that can be triggered after message reply, inline button press or Auth success f.e. API query to comment the card on replying.
	// Please note that first argument must be an *integram.Context. Because all actions is triggered in some context.
	// F.e. when using action with onReply triggered with context of replied message (user, chat, bot).
	Actions []interface{}

	// Handler to receive webhooks from outside
	WebhookHandler func(ctx *Context, request *WebhookContext) error

	// Handler to receive new messages from Telegram
	TGNewMessageHandler func(ctx *Context) error

	// Handler to receive new messages from Telegram
	TGEditMessageHandler func(ctx *Context) error

	// Handler to receive inline queries from Telegram
	TGInlineQueryHandler func(ctx *Context) error

	// Handler to receive chosen inline results from Telegram
	TGChosenInlineResultHandler func(ctx *Context) error

	OAuthSuccessful func(ctx *Context) error
	// Can be used for services with tiny load
	UseWebhookInsteadOfLongPolling bool
}

const (
	JobRetryLinear    = iota // 0
	JobRetryFibonacci        // 1
)

// Job's handlers that may be used when scheduling
type Job struct {
	HandlerFunc interface{} // Must be a func.
	Retries     uint        // Number of retries before fail
	RetryType   int         // JobRetryLinear or JobRetryFibonacci
}

type DefaultOAuth1 struct {
	Key                              string
	Secret                           string
	RequestTokenUrl                  string
	AuthorizeTokenUrl                string
	AccessTokenUrl                   string
	AdditionalAuthorizationUrlParams map[string]string
	HttpMethod                       string
	AccessTokenReceiver              func(serviceContext *Context, r *http.Request, requestToken *oauth.RequestToken) (token string, err error)
}

type DefaultOAuth2 struct {
	oauth2.Config
	AccessTokenReceiver func(serviceContext *Context, r *http.Request) (token string, expiresAt *time.Time, refreshToken string, err error)
}

// Redis job pool safe exit
func RunWorkers() {
	for k := range jobs.Pools {
		pool := jobs.Pools[k]
		defer func() {
			log.Infof("Closing job pool: %s\n", k)
			pool.Close()
			if err := pool.Wait(); err != nil {
				log.WithError(err).WithField("poolName", k).Error("Jobs pool stoped")
			}
		}()
	}
}

func init() {
	baseURL := os.Getenv("INTEGRAM_BASE_URL")

	if baseURL != "" {
		BaseURL = baseURL
	}
	log.Debugf("BaseURL: %s", baseURL)

	go func() {
		var b *Bot
		for {
			log.Debug("wait for tgUpdatesRevoltChan")
			b = <-tgUpdatesRevoltChan
			log.Debugf("tgUpdatesRevoltChan received, bot %+v\n", b)

			b.listen()
		}
	}()
}
func afterJob(job *jobs.Job) {
	log.Debugf("afterJob %v, poolID:%v, finished:%v\n", job.Id(), job.PoolId(), job.Finished().Unix())
	// remove successed tasks from Redis
	err := job.Error()
	if err != nil {
		log.WithFields(log.Fields{"jobID": job.Id(), "poolId": job.PoolId()}).WithError(err).Error("Job failed")
	}

	if err == nil || job.Retries() == 0 {
		log.Debugf("destroying the job %v finished(%v), status=%v, retriesLeft=%v, nextTime=%v", job.Id(), job.Finished(), job.Status(), job.Retries(), job.NextTime())
		job.Destroy()
	} else {
		log.Debugf("the job stays %v finished(%v), status=%v, retriesLeft=%v, nextTime=%v", job.Id(), job.Finished(), job.Status(), job.Retries(), job.NextTime())
	}
}

func beforeJob(ch chan bool, job *jobs.Job, args *[]reflect.Value) {
	log.Debugf("beforeJob %v, poolID:%v, finished:%v\n", job.Id(), job.PoolId(), job.Finished().Unix())
	s := mongoSession.Clone()

	for i := 0; i < len(*args); i++ {

		if (*args)[i].Kind() == reflect.Ptr && (*args)[i].Type().String() == "*integram.Context" {
			//log.Debugf("arg inside: " + (*args)[le].Kind().String() + " to " + (*args)[le].Type().String() + " addr " + (*args)[le].Addr().String() + "\n")
			ctx := (*args)[i].Interface().(*Context)

			ctx.db = s.DB(mongo.Database)
			ctx.User.ctx = ctx
			ctx.Chat.ctx = ctx

			break
		}
	}

	ch <- true
	<-ch
	s.Close()
}

type Servicer interface {
	Service() *Service
}

func Register(servicer Servicer, botToken string) {
	service := servicer.Service()
	if service.DefaultOAuth1 != nil {
		if service.DefaultOAuth1.AccessTokenReceiver == nil {
			err := errors.New("OAuth1 need an AccessTokenReceiver func to be specified\n")
			panic(err.Error())
		}
		service.DefaultBaseURL = *UrlMustParse(service.DefaultOAuth1.AccessTokenUrl)

		//mongoSession.DB(mongo.Database).C("users").EnsureIndex(mgo.Index{Key: []string{"settings." + service.Name + ".oauth_redirect_token"}})
	} else if service.DefaultOAuth2 != nil {
		service.DefaultBaseURL = *UrlMustParse(service.DefaultOAuth2.Endpoint.AuthURL)
	}
	service.DefaultBaseURL.Path = ""
	service.DefaultBaseURL.RawPath = ""
	service.DefaultBaseURL.RawQuery = ""

	services[service.Name] = service

	if len(service.Jobs) > 0 {
		if service.JobsPool == 0 {
			service.JobsPool = 1
		}
		pool, err := jobs.NewPool(&jobs.PoolConfig{
			Key:        "_" + service.Name,
			NumWorkers: service.JobsPool,
			BatchSize:  service.JobsPool,
		})
		if err != nil {
			log.Panicf("Can't create jobs pool: %v\n", err)
		} else {
			pool.SetMiddleware(beforeJob)
			pool.SetAfterFunc(afterJob)
		}
		err = pool.Start()
		if err != nil {
			log.Panicf("Can't start jobs pool: %v\n", err)
		}
		log.Infof("Jobs pool %v[%d] is ready", "_"+service.Name, service.JobsPool)

		jobsPerService[service.Name] = make(map[string]*jobs.Type)

		if service.OAuthSuccessful != nil {
			service.Jobs = append(service.Jobs, Job{
				service.OAuthSuccessful, 10, JobRetryFibonacci,
			})
		}
		for _, job := range service.Jobs {
			handlerType := reflect.TypeOf(job.HandlerFunc)
			m := make([]interface{}, handlerType.NumIn())

			for i := 0; i < handlerType.NumIn(); i++ {
				argType := handlerType.In(i)
				if argType.Kind() == reflect.Ptr {
					//argType = argType.Elem()
				}

				gob.Register(reflect.Zero(argType).Interface())
				m[i] = reflect.Zero(argType)
			}
			gob.Register(m)

			jobName := getFuncName(job.HandlerFunc)
			jobType, err := jobs.RegisterTypeWithPoolKey(jobName, "_"+service.Name, job.Retries, job.HandlerFunc)
			if err != nil {
				panic(err)
			} else {
				jobsPerService[service.Name][jobName] = jobType
			}
		}
	}
	if len(service.Actions) > 0 {
		for _, actionFunc := range service.Actions {
			actionFuncType := reflect.TypeOf(actionFunc)
			m := make([]interface{}, actionFuncType.NumIn())

			for i := 0; i < actionFuncType.NumIn(); i++ {
				argType := actionFuncType.In(i)
				if argType.Kind() == reflect.Ptr {
					//argType = argType.Elem()
				}
				//log.Debugf("ReplyHandlers Gob.Register %v\n", argType.String())

				gob.Register(reflect.Zero(argType).Interface())
			}
			gob.Register(m)
			actionFuncs[runtime.FuncForPC(reflect.ValueOf(actionFunc).Pointer()).Name()] = actionFunc
		}
	}
	if botToken == "" {
		return
	}
	err := service.registerBot(botToken)
	if err != nil {
		log.WithError(err).WithField("token", botToken).Panic("Can't register the bot")
	}

	// todo: here is possible bug if service just want to use inline keyboard callbacks via setCallbackAction
	if service.TGNewMessageHandler == nil && service.TGInlineQueryHandler == nil {
		return
	}

}

func (s *Service) Bot() *Bot {
	if bot, exists := botPerService[s.Name]; exists {
		return bot
	} else {
		log.WithField("service", s.Name).Error("Can't get bot for service")
		return nil
	}
}

func (s *Service) DefaultOAuthProvider() *OAuthProvider {
	oap := OAuthProvider{}
	oap.BaseURL = s.DefaultBaseURL
	oap.Service = s.Name
	if s.DefaultOAuth2 != nil {
		oap.ID = s.DefaultOAuth2.ClientID
		oap.Secret = s.DefaultOAuth2.ClientSecret
	} else if s.DefaultOAuth1 != nil {
		oap.ID = s.DefaultOAuth1.Key
		oap.Secret = s.DefaultOAuth1.Secret
	} else {
		s.Log().Error("Can't get OAuth client")
	}
	return &oap
}

// Run the queued job. The job must be registred in Service's config (Jobs field). Arguments must be identically types with hudlerFunc's input args
func (s *Service) DoJob(handlerFunc interface{}, data ...interface{}) (*jobs.Job, error) {
	return s.SheduleJob(handlerFunc, 0, time.Now(), data...)
}

// Shedule the queued job for specific time with specific priority. The job must be registred in Service's config (Jobs field). Arguments must be identically types with hudlerFunc's input args
func (s *Service) SheduleJob(handlerFunc interface{}, priority int, time time.Time, data ...interface{}) (*jobs.Job, error) {
	if jobsPerName, ok := jobsPerService[s.Name]; ok {
		if jobType, ok := jobsPerName[getFuncName(handlerFunc)]; ok {
			return jobType.Schedule(priority, time, data...)
		} else {
			panic("SheduleJob: Job type not found")
			//return nil, errors.New("SheduleJob: Job type not found")
		}
	} else {
		return nil, errors.New("SheduleJob: Service pool not found")
	}
}

func serviceByName(name string) (*Service, error) {
	if val, ok := services[name]; ok {
		return val, nil
	} else {
		return nil, errors.New("Can't find service with name ")
	}
}

func (s *Service) Log() *log.Entry {
	return log.WithField("service", s.Name)
}

/*func (o *OAuthProvider) RequestToken(db *mgo.Database, token string) *oauth.RequestToken {
	requestToken := oauth.RequestToken{}
	db.C("oauth1_request_tokens").Find(bson.M{"key": o.Key, "token": token}).Select(bson.M{"token": 1, "secret": 1}).One(&requestToken)
	return &requestToken
}

func (o *OAuthProvider) RemoveRequestToken(db *mgo.Database, token string) error {
	return db.C("oauth1_request_tokens").Remove(bson.M{"key": o.Key, "token": token})
}
*/
