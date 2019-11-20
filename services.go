package integram

import (
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/mrjones/oauth"
	"github.com/requilence/jobs"
	"github.com/requilence/url"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"

	"gopkg.in/mgo.v2"
)

const standAloneServicesFileName = "standAloneServices.json"
// Map of Services configs per name. See Register func
var serviceMapMutex = sync.RWMutex{}
var services = make(map[string]*Service)

// Mapping job.Type by job alias names specified in service's config
type jobTypePerJobName map[string]*jobs.Type

var jobsPerService = make(map[string]jobTypePerJobName)

// Map of replyHandlers names to funcs. Use service's config to specify it
var actionFuncs = make(map[string]interface{})

// Channel that use to recover tgUpadates reader after panic inside it
var tgUpdatesRevoltChan = make(chan *Bot)

type Module struct {
	Jobs    []Job
	Actions []interface{}
}

// Service configuration
type Service struct {
	Name        string // Service lowercase name
	NameToPrint string // Service print name
	ImageURL    string // Service thumb image to use in WebPreview if there is no image specified in message. Useful for non-interactive integrations that uses main Telegram's bot.

	DefaultBaseURL url.URL        // Cloud(not self-hosted) URL
	DefaultOAuth1  *DefaultOAuth1 // Cloud(not self-hosted) app data
	DefaultOAuth2  *DefaultOAuth2 // Cloud(not self-hosted) app data
	OAuthRequired  bool           // Is OAuth required in order to receive webhook updates

	JobsPool int // Worker pool to be created for service. Default to 1 worker. Workers will be inited only if jobs types are available

	JobOldPrefix 	string
	Jobs []Job // Job types that can be scheduled

	Modules []Module // you can inject modules and use it across different services

	// Functions that can be triggered after message reply, inline button press or Auth success f.e. API query to comment the card on replying.
	// Please note that first argument must be an *integram.Context. Because all actions is triggered in some context.
	// F.e. when using action with onReply triggered with context of replied message (user, chat, bot).
	Actions []interface{}

	// Handler to produce the user/chat search query based on the http request. Set queryChat to true to perform chat search
	TokenHandler func(ctx *Context, request *WebhookContext) (queryChat bool, bsonQuery map[string]interface{}, err error)

	// Handler to receive webhooks from outside
	WebhookHandler func(ctx *Context, request *WebhookContext) error

	// Handler to receive already prepared data. Useful for manual interval grabbing jobs
	EventHandler func(ctx *Context, data interface{}) error

	// Worker wil be run in goroutine after service and framework started. In case of error or crash it will be restarted
	Worker func(ctx *Context) error

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

	// Can be used to automatically clean up old messages metadata from database
	RemoveMessagesOlderThan *time.Duration

	machineURL string // in case of multi-instance mode URL is used to talk with the service

	rootPackagePath string
}

const (
	// JobRetryLinear specify jobs retry politic as retry after fail
	JobRetryLinear = iota
	// JobRetryFibonacci specify jobs retry politic as delay after fail using fibonacci sequence
	JobRetryFibonacci
)

// Job 's handler that may be used when scheduling
type Job struct {
	HandlerFunc interface{} // Must be a func.
	Retries     uint        // Number of retries before fail
	RetryType   int         // JobRetryLinear or JobRetryFibonacci
}

// DefaultOAuth1 is the default OAuth1 config for the service
type DefaultOAuth1 struct {
	Key                              string
	Secret                           string
	RequestTokenURL                  string
	AuthorizeTokenURL                string
	AccessTokenURL                   string
	AdditionalAuthorizationURLParams map[string]string
	HTTPMethod                       string
	AccessTokenReceiver              func(serviceContext *Context, r *http.Request, requestToken *oauth.RequestToken) (token string, err error)
}

// DefaultOAuth2 is the default OAuth2 config for the service
type DefaultOAuth2 struct {
	oauth2.Config
	AccessTokenReceiver func(serviceContext *Context, r *http.Request) (token string, expiresAt *time.Time, refreshToken string, err error)

	// duration to cache temp token to associate with user
	// default(when zero) will be set to 30 days
	AuthTempTokenCacheTime time.Duration
}

func servicesHealthChecker() {

	defer func() {
		if r := recover(); r != nil {
			log.Errorf("HealthChecker panic recovered %v", r)
			servicesHealthChecker()
		}
	}()
	s := mongoSession.Clone()
	defer s.Close()

	for true {

		err := healthCheck(s.DB(mongo.Database))

		if err != nil {
			log.Errorf("HealthChecker main, error: %s", err.Error())
		}

		if Config.IsMainInstance() {

			for serviceName, _ := range services {

				resp, err := http.Get(fmt.Sprintf("%s/%s/healthcheck", Config.BaseURL, serviceName))

				if err != nil {
					log.Errorf("HealthChecker %s, network error: %s", serviceName, err.Error())
					continue
				}

				// always close the response-body, even if content is not required
				defer resp.Body.Close()

				if resp.StatusCode != 200 {
					b, err := ioutil.ReadAll(resp.Body)

					if err != nil {
						log.Errorf("HealthChecker %s, status %d, read error: %s", serviceName, resp.StatusCode, err.Error())
					}

					log.Errorf("HealthChecker %s, status %d, error: %s", serviceName, resp.StatusCode, string(b))
				}

			}
		}

		time.Sleep(time.Second * time.Duration(Config.HealthcheckIntervalInSecond))
	}
}

func healthCheck(db *mgo.Database) error {

	err := db.Session.Ping()
	if err != nil {
		return fmt.Errorf("DB fault: %s", err)

	}

	_, err = jobs.FindById("fake")
	if err != nil {
		if _, isThisNotFoundError := err.(jobs.ErrorJobNotFound); !isThisNotFoundError {
			return fmt.Errorf("Redis fault: %s", err)
		}
	}

	if int(time.Now().Sub(startedAt).Seconds()) < Config.HealthcheckIntervalInSecond {
		return fmt.Errorf("Instance was restarted")
	}

	return nil
}

func init() {

	jobs.Config.Db.Address = Config.RedisURL
	if Config.IsMainInstance() {
		err := loadStandAloneServicesFromFile()
		if err != nil {
			log.WithError(err).Error("loadStandAloneServicesFromFile error")
		}
		go servicesHealthChecker()

	} else {
		go func() {
			var b *Bot
			for {
				b = <-tgUpdatesRevoltChan
				log.Debugf("tgUpdatesRevoltChan received, bot %+v\n", b)

				b.listen()
			}
		}()
	}
}

func afterJob(job *jobs.Job) {
	// remove successed tasks from Redis
	err := job.Error()

	if err == nil {
		log.WithFields(log.Fields{"jobID": job.Id(), "jobType": job.TypeName(), "poolId": job.PoolId()}).WithError(err).Infof("Job succeed after %.2f sec", job.Duration().Seconds())
		job.Destroy()
	} else if err != nil && job.Retries() == 0 {
		log.WithFields(log.Fields{"jobID": job.Id(), "jobType": job.TypeName(), "poolId": job.PoolId()}).WithError(err).Errorf("Job failed after %.2f sec", job.Duration().Seconds())
		job.Destroy()
	} else if err != nil && job.Retries() > 0 {
		log.WithFields(log.Fields{"jobID": job.Id(), "jobType": job.TypeName(), "poolId": job.PoolId()}).WithError(err).Errorf("Job failed after %.2f sec, %d retries left", job.Duration().Seconds(), job.Retries())
		job.Destroy()
	}
}

func beforeJob(ch chan bool, job *jobs.Job, args *[]reflect.Value) {
	s := mongoSession.Clone()

	for i := 0; i < len(*args); i++ {

		if (*args)[i].Kind() == reflect.Ptr && (*args)[i].Type().String() == "*integram.Context" {
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

// Servicer is interface to match service's config from which the service itself can be produced
type Servicer interface {
	Service() *Service
}

func ensureStandAloneService(serviceName string, machineURL string, botToken string) error {

	log.Infof("Service '%s' discovered: %s", serviceName, machineURL)
	s, _ := serviceByName(serviceName)

	serviceMapMutex.Lock()
	defer serviceMapMutex.Unlock()

	if s == nil {
		s = &Service{Name: serviceName}
		services[serviceName] = s
	}

	s.machineURL = machineURL
	reverseProxiesMapMutex.Lock()
	_, ok := reverseProxiesMap[serviceName]
	if ok {
		delete(reverseProxiesMap, serviceName)
	}
	reverseProxiesMapMutex.Unlock()

	err := s.registerBot(botToken)

	if err != nil {
		log.WithError(err).Error("Service Ensure: registerBot error")
	}

	err = saveStandAloneServicesToFile()

	//if err != nil{
	//	log.WithError(err).Error("Service Ensure: saveServicesBotsTokensToCache error")
	//}
	return err
}

func loadStandAloneServicesFromFile() error {
	b, err := ioutil.ReadFile(Config.ConfigDir + string(os.PathSeparator) + standAloneServicesFileName)

	if err != nil {
		return err
	}
	var m map[string]externalService

	err = json.Unmarshal(b, &m)

	if err != nil {
		return err
	}

	for serviceName, es := range m {
		s, _ := serviceByName(serviceName)
		if s == nil {
			s = &Service{Name: serviceName, machineURL: es.URL}
			serviceMapMutex.Lock()
			services[serviceName] = s
			serviceMapMutex.Unlock()
		}

		err := s.registerBot(es.BotToken)

		if err != nil {
			log.WithError(err).Error("loadStandAloneServicesFromFile: registerBot error")
		}
	}
	return nil

}

type externalService struct {
	BotToken string
	URL      string
}

func saveStandAloneServicesToFile() error {
	m := map[string]externalService{}
	for _, s := range services {
		m[s.Name] = externalService{BotToken: s.Bot().tgToken(), URL: s.machineURL}
	}

	jsonData, err := json.Marshal(m)

	if err != nil {
		return err
	}

	return ioutil.WriteFile(Config.ConfigDir + string(os.PathSeparator) + standAloneServicesFileName, jsonData, 0655)
}

func (s *Service) getShortFuncPath(actionFunc interface{}) string {
	fullPath := runtime.FuncForPC(reflect.ValueOf(actionFunc).Pointer()).Name()
	if fullPath == "" {
		panic("getShortFuncPath")
	}
	return s.trimFuncPath(fullPath)
}

func (s *Service) trimFuncPath(fullPath string) string{
	// Trim funcPath for a specific service name and determined service's rootPackagePath
	// trello, github.com/requilence/integram/services/trello, github.com/requilence/integram/services/trello.cardReplied -> trello.cardReplied
	// trello, github.com/requilence/integram/services/Trello, github.com/requilence/integram/services/Trello.cardReplied -> trello.cardReplied
	// trello, github.com/requilence/trelloRepo, _/var/integram/trello.cardReplied -> trello.cardReplied
	// trello, github.com/requilence/trelloRepo, _/var/integram/another.cardReplied -> trello.cardReplied
	// trello, github.com/requilence/integram/services/trello, github.com/requilence/integram/services/trello/another.action -> trello/another.action
	// trello, github.com/requilence/integram/services/trello, _/var/integram/trello.cardReplied -> trello.cardReplied
	// trello, trello.cardReplied, github.com/requilence/integram/services/trello.cardReplied -> trello.cardReplied
	if s.rootPackagePath != "" && strings.HasPrefix(fullPath, s.rootPackagePath) {
		internalFuncPath := strings.TrimPrefix(fullPath, s.rootPackagePath)
		return s.Name + internalFuncPath
	} else if strings.HasPrefix(fullPath, s.Name+".") {
		return fullPath
	}
	funcPos := strings.LastIndex(fullPath, s.Name+".")
	if funcPos > -1 {
		return fullPath[funcPos:]
	}

	funcPos = strings.LastIndex(fullPath, ".")
	if funcPos > -1 {
		return s.Name + fullPath[funcPos:]
	}

	return fullPath
}

// Register the service's config and corresponding botToken
func Register(servicer Servicer, botToken string) {
	//jobs.Config.Db.Address="192.168.1.101:6379"
	db := mongoSession.Clone().DB(mongo.Database)
	service := servicer.Service()
	err := migrations(db, service.Name)
	if err != nil {
		log.Fatalf("failed to apply migrations: %s", err.Error())
	}

	if service.DefaultOAuth1 != nil {
		if service.DefaultOAuth1.AccessTokenReceiver == nil {
			err := errors.New("OAuth1 need an AccessTokenReceiver func to be specified\n")
			panic(err.Error())
		}
		service.DefaultBaseURL = *URLMustParse(service.DefaultOAuth1.AccessTokenURL)

		//mongoSession.DB(mongo.Database).C("users").EnsureIndex(mgo.Index{Key: []string{"settings." + service.Name + ".oauth_redirect_token"}})
	} else if service.DefaultOAuth2 != nil {
		service.DefaultBaseURL = *URLMustParse(service.DefaultOAuth2.Endpoint.AuthURL)
	}
	service.DefaultBaseURL.Path = ""
	service.DefaultBaseURL.RawPath = ""
	service.DefaultBaseURL.RawQuery = ""

	services[service.Name] = service

	if len(service.Jobs) > 0 || service.OAuthSuccessful != nil {
		if service.JobsPool == 0 {
			service.JobsPool = 1
		}
		pool, err := jobs.NewPool(&jobs.PoolConfig{
			Key:        "_" + service.Name,
			NumWorkers: service.JobsPool,
			BatchSize:  Config.TGPoolBatchSize,
		})
		if err != nil {
			log.Panicf("Can't create jobs pool: %v\n", err)
		} else {
			pool.SetMiddleware(beforeJob)
			pool.SetAfterFunc(afterJob)
		}

		//log.Infof("%s: workers pool [%d] is ready", service.Name, service.JobsPool)

		jobsPerService[service.Name] = make(map[string]*jobs.Type)

		if service.OAuthSuccessful != nil {
			service.Jobs = append(service.Jobs, Job{
				service.OAuthSuccessful, 10, JobRetryFibonacci,
			})
		}

		if len(service.Modules) > 0 {
			for _, module := range service.Modules {
				service.Actions = append(service.Actions, module.Actions...)
				service.Jobs = append(service.Jobs, module.Jobs...)
			}
		}

		for _, job := range service.Jobs {
			handlerType := reflect.TypeOf(job.HandlerFunc)
			m := make([]interface{}, handlerType.NumIn())

			for i := 0; i < handlerType.NumIn(); i++ {
				argType := handlerType.In(i)
				if argType.Kind() == reflect.Ptr {
					//argType = argType.Elem()
				}

				if argType.Kind() == reflect.Interface {
					gob.Register(reflect.Zero(argType))
				} else {
					gob.Register(reflect.Zero(argType).Interface())
				}
				m[i] = reflect.Zero(argType)
			}
			gob.Register(m)

			jobName := service.getShortFuncPath(job.HandlerFunc)

			jobType, err := jobs.RegisterTypeWithPoolKey(jobName, "_"+service.Name, job.Retries, job.HandlerFunc)
			if err != nil {
				fmt.Errorf("RegisterTypeWithPoolKey '%s', for %s: %s", jobName, service.Name, err.Error() )
			} else {
				jobsPerService[service.Name][jobName] = jobType
			}

		}

		rootPackagePath := reflect.TypeOf(servicer).PkgPath()
		service.rootPackagePath = rootPackagePath

		log.Debugf("RootPackagePath of %s is %s", service.Name, rootPackagePath)

		go func(pool *jobs.Pool, service *Service) {
			time.Sleep(time.Second * 5)

			err = pool.Start()

			if err != nil {
				log.Panicf("Can't start jobs pool: %v\n", err)
			}
			log.Infof("%s service: workers pool [%d] started", service.Name, service.JobsPool)

		}(pool, service)

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

				gob.Register(reflect.Zero(argType).Interface())
			}
			gob.Register(m)
			actionFuncs[service.getShortFuncPath(actionFunc)] = actionFunc
		}
	}
	if botToken == "" {
		return
	}

	err = service.registerBot(botToken)
	if err != nil {
		log.WithError(err).WithField("token", botToken).Panic("Can't register the bot")
	}
	go ServiceWorkerAutorespawnGoroutine(service)

	if service.Worker != nil {
		go ServiceWorkerAutorespawnGoroutine(service)
	}

	// todo: here is possible bug if service just want to use inline keyboard callbacks via setCallbackAction
	if service.TGNewMessageHandler == nil && service.TGInlineQueryHandler == nil {
		return
	}

}

func ServiceWorkerAutorespawnGoroutine(s *Service) {

	c := s.EmptyContext()
	defer func() {
		if r := recover(); r != nil {
			stack := stack(3)
			log.Errorf("Panic recovery at ServiceWorkerAutorespawnGoroutine -> %s\n%s\n", r, stack)
		}
		go ServiceWorkerAutorespawnGoroutine(s) // restart
	}()

	err := s.Worker(c)
	if err != nil {
		s.Log().WithError(err).Error("Worker return error")
	}
}

// Bot returns corresponding bot for the service
func (s *Service) Bot() *Bot {
	if bot, exists := botPerService[s.Name]; exists {
		return bot
	}
	log.WithField("service", s.Name).Error("Can't get bot for service")
	return nil
}

// DefaultOAuthProvider returns default(means cloud-based) OAuth client
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

// DoJob queues the job to run. The job must be registred in Service's config (Jobs field). Arguments must be identically types with hudlerFunc's input args
func (s *Service) DoJob(handlerFunc interface{}, data ...interface{}) (*jobs.Job, error) {
	return s.SheduleJob(handlerFunc, 0, time.Now(), data...)
}

// SheduleJob schedules the job for specific time with specific priority. The job must be registred in Service's config (Jobs field). Arguments must be identically types with hudlerFunc's input args
func (s *Service) SheduleJob(handlerFunc interface{}, priority int, time time.Time, data ...interface{}) (*jobs.Job, error) {
	if jobsPerName, ok := jobsPerService[s.Name]; ok {
		if jobType, ok := jobsPerName[s.getShortFuncPath(handlerFunc)]; ok {
			return jobType.Schedule(priority, time, data...)
		}
		panic("SheduleJob: Job type not found")
	}
	panic("SheduleJob: Service pool not found")
}

// EmptyContext returns context on behalf of service without user/chat relation
func (s *Service) EmptyContext() *Context {
	db := mongoSession.Clone().DB(mongo.Database)

	ctx := &Context{db: db, ServiceName: s.Name}
	return ctx
}

func serviceByName(name string) (*Service, error) {
	serviceMapMutex.RLock()
	defer serviceMapMutex.RUnlock()
	if val, ok := services[name]; ok {
		return val, nil
	}

	return nil, fmt.Errorf("Can't find service with name %s", name)
}

// Log returns logrus instance with related context info attached
func (s *Service) Log() *log.Entry {
	return log.WithField("service", s.Name)
}
