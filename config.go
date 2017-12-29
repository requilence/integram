package integram

import (
	"fmt"
	"github.com/kelseyhightower/envconfig"
	log "github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"syscall"
	"github.com/requilence/integram/url"
)

type Mode string

const (
	InstanceModeMain              Mode = "main"    // run only the main worker. It will process the outgoing messages queue and route the incoming webhooks to specific services
	InstanceModeStandAloneService Mode = "service" // run only one the registred services and their workers. Main instance must be running in order to the outgoing TG  messages could be sent
	InstanceModeSingleProcess     Mode = "single"  // run all in one process – main worker and all registred services
)

type BotConfig struct {
	Token string `envconfig:"BOT_TOKEN" required:"true"`
}

type config struct {
	BaseURL      string `envconfig:"INTEGRAM_BASE_URL" required:"true"`
	InstanceMode Mode   `envconfig:"INTEGRAM_INSTANCE_MODE" default:"single"` // please refer to the constants declaration

	TGPool       int    `envconfig:"INTEGRAM_TG_POOL" default:"10"` // Maximum simultaneously message sending
	MongoURL     string `envconfig:"INTEGRAM_MONGO_URL" default:"mongodb://localhost:27017/integram"`
	RedisURL     string `envconfig:"INTEGRAM_REDIS_URL" default:"127.0.0.1:6379"`
	Port         string `envconfig:"INTEGRAM_PORT" default:"7000"`
	Debug        bool   `envconfig:"INTEGRAM_DEBUG" default:"1"`
	MongoLogging bool   `envconfig:"INTEGRAM_MONGO_LOGGING" default:"0"`

	// -----
	// only make sense for InstanceModeStandAloneService
	HealthcheckIntervalInSecond int    `envconfig:"INTEGRAM_HEALTHCHECK_INTERVAL" default:"30"` // interval to ping each service instance by the main instance
	StandAloneServiceURL        string `envconfig:"INTEGRAM_STANDALONE_SERVICE_URL"`            // default will be depending on the each service's name, e.g. http://trello:7000

}

var Config config

func (c *config) IsMainInstance() bool {
	if c.InstanceMode == InstanceModeMain {
		return true
	}
	return false
}

func (c *config) ParseBaseURL() *url.URL {
	u, err := url.Parse(c.BaseURL)
	if err != nil {
		panic("PANIC: can't parse INTEGRAM_BASE_URL: '"+c.BaseURL+"'")
	}
	if u.Scheme == "" {
		u.Scheme = "https"
	}
	return u
}

func (c *config) IsStandAloneServiceInstance() bool {
	if c.InstanceMode == InstanceModeStandAloneService {
		return true
	}
	return false
}

func (c *config) IsSingleProcessInstance() bool {
	if c.InstanceMode == InstanceModeSingleProcess {
		return true
	}
	return false
}



func init() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP)

	go func() {
		for sig := range c {
			println(sig)
			fmt.Println("Got A HUP Signal: reloading config from ENV")
			err := envconfig.Process("", &Config)
			if err != nil {
				log.WithError(err).Error("HUP envconfig error")
			}
		}
	}()

	envconfig.MustProcess("", &Config)

	Config.ParseBaseURL()
}
