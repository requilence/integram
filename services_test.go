package integram

import (
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/mrjones/oauth"
	"golang.org/x/oauth2"
	"gopkg.in/mgo.v2"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

type serviceTestConfig struct {
	service *Service
}

var totalServices int

// Service returns *integram.Service
func (sc serviceTestConfig) Service() *Service {
	return sc.service
}

func dumbFuncWithParam(a bool) error {
	if a {
		a = false
	}
	return nil
}

var dCounter = 0

func dumbFuncWithError(es string) error {
	dCounter++
	return errors.New(es)
}

type dumbStruct struct {
	A float64
	B struct {
		A string
		B int
	}
}

func dumbFuncWithParams(a, b int, d dumbStruct) error {
	if a != b {
		a = b
	}
	if d.B.A == "" {
		d.A = 0
	}
	dCounter++
	return nil
}

func dumbFuncWithContext(c *Context) error {
	dCounter++
	return nil
}

func dumbFuncWithContextAndParam(c *Context, a bool) error {
	if a {
		a = false
	}
	dCounter++
	return nil
}

func dumbFuncWithContextAndParams(c *Context, a, b int) error {
	if a != b {
		a = b
	}
	dCounter++
	return nil
}

var db *mgo.Database

func TestMain(t *testing.M) {
	db = mongoSession.Clone().DB(mongo.Database)
	defer db.Session.Close()

	registerServices()

	code := t.Run()
	clearData()
	os.Exit(code)
}

func TestRegister(t *testing.T) {
	if totalServices > len(services) {
		t.Errorf("Register() = len(services)==%d, want %d", len(services), totalServices)
	}
}

func registerServices() {

	if len(services) == 0 {
		log.SetLevel(log.DebugLevel)
		servicesToRegister := []struct {
			service  Servicer
			botToken string
		}{
			{serviceTestConfig{&Service{
				Name:        "servicewithoauth1",
				NameToPrint: "ServiceWithOAuth1",
				DefaultOAuth1: &DefaultOAuth1{
					Key:    "ID",
					Secret: "SECRET",

					RequestTokenURL:   "https://sub.example.com/1/OAuthGetRequestToken",
					AuthorizeTokenURL: "https://sub.example.com/1/OAuthAuthorizeToken",
					AccessTokenURL:    "https://sub.example.com/1/OAuthGetAccessToken",

					AdditionalAuthorizationURLParams: map[string]string{
						"name":       "Integram",
						"expiration": "never",
						"scope":      "read,write",
					},

					AccessTokenReceiver: func(serviceContext *Context, r *http.Request, requestToken *oauth.RequestToken) (token string, err error) {
						return "token", nil
					},
				},
			}}, ""},
			{serviceTestConfig{&Service{
				Name:        "servicewithoauth2",
				NameToPrint: "ServiceWithOAuth2",
				DefaultOAuth2: &DefaultOAuth2{
					Config: oauth2.Config{
						ClientID:     "ID",
						ClientSecret: "SECRET",
						Endpoint: oauth2.Endpoint{
							AuthURL:  "https://sub.example.com/oauth/authorize",
							TokenURL: "https://sub.example.com/oauth/token",
						},
					},
				},
			}}, ""},
			{serviceTestConfig{&Service{
				Name:        "servicewithjobs",
				NameToPrint: "ServiceWithJobs",
				Jobs: []Job{
					{dumbFuncWithParam, 10, JobRetryFibonacci},
					{dumbFuncWithParams, 10, JobRetryLinear},
					{dumbFuncWithError, 3, JobRetryFibonacci},
				},
			}}, ""},
			{serviceTestConfig{&Service{
				Name:        "servicewithactions",
				NameToPrint: "ServiceWithActions",
				Actions: []interface{}{
					dumbFuncWithContext,
					dumbFuncWithContextAndParam,
					dumbFuncWithContextAndParams,
				},
			}}, ""},
			{serviceTestConfig{&Service{
				Name:        "servicewithbottoken",
				NameToPrint: "ServiceWithBotToken",
			}}, os.Getenv("INTEGRAM_TEST_BOT_TOKEN")},
		}

		for _, s := range servicesToRegister {
			Register(s.service, s.botToken)
		}
		totalServices = len(servicesToRegister)
	}

	go func() { Run() }()
	time.Sleep(time.Second * 3)

}

func TestService_Bot(t *testing.T) {

	s, err := serviceByName("servicewithbottoken")
	if err != nil || s == nil {
		t.Errorf("TestService_Bot() 'servicewithbottoken' not found")
		return
	}

	bt := strings.Split(os.Getenv("INTEGRAM_TEST_BOT_TOKEN"), ":")
	bot := s.Bot()
	if bot == nil || fmt.Sprintf("%d", bot.ID) != bt[0] || len(bot.services) == 0 || bot.services[0].Name != "servicewithbottoken" || bot.token != bt[1] {
		t.Errorf("TestService_Bot() bad bot returned for services")
	}
}

func TestService_DefaultOAuthProvider(t *testing.T) {

	servicesWithOAP := []string{"servicewithoauth1", "servicewithoauth2"}
	for _, sn := range servicesWithOAP {
		s, err := serviceByName(sn)
		if err != nil || s == nil {
			t.Errorf("TestService_DefaultOAuthProvider() '%s' not found", sn)
			return
		}

		oap := s.DefaultOAuthProvider()
		if oap == nil || oap.ID == "" || oap.Service != sn || oap.BaseURL.String() != "https://sub.example.com" {
			t.Errorf("TestService_DefaultOAuthProvider() bad DefaultOAuthProvider returned for '%s'", sn)
		}
	}

}

func TestService_DoJob(t *testing.T) {
	s, err := serviceByName("servicewithjobs")
	if err != nil || s == nil {
		t.Error("TestService_DoJob 'servicewithjobs' not found")
		return
	}

	dCounter = 0
	job, err := s.DoJob(dumbFuncWithParams, 1, 2, dumbStruct{A: 1.0, B: struct {
		A string
		B int
	}{A: "_",
		B: 1.0}})
	if err != nil || job == nil || job.Id() == "" {
		t.Error("TestService_DoJob failed")
	}
	maxTimeToFinishJob := time.Second * 5

	for dCounter == 0 && maxTimeToFinishJob > 0 {
		time.Sleep(time.Millisecond * 50)
		maxTimeToFinishJob = maxTimeToFinishJob - time.Millisecond*50
	}

	if maxTimeToFinishJob <= 0 {
		t.Errorf("TestService_DoJob 'dumbFuncWithParams' not finished after maxTimeToFinishJob: instead got status: %s", job.Status())
	}

	// reset global retries counter
	dCounter = 0

	job, err = s.DoJob(dumbFuncWithError, "errText")
	if err != nil || job == nil || job.Id() == "" {
		t.Errorf("TestService_DoJob failed with err: %v", err)
	}
	err = job.Refresh()
	if err != nil {
		t.Error(err)
	}

	maxTimeToFinishRetries := time.Second * 6
	prevTime := job.Time()
	fmt.Println(prevTime)
	prevdCounter := 0
	for dCounter < 4 && maxTimeToFinishJob > 0 {
		maxTimeToFinishRetries = maxTimeToFinishRetries - time.Millisecond*100
		time.Sleep(time.Millisecond * 100)
		if dCounter > prevdCounter {
			fmt.Printf("dCounter: %d\n", dCounter)
			job.Refresh()
			fmt.Printf("time: %d\n", job.Time())
			jt := job.Time()
			if dCounter < 4 && jt <= prevTime {
				t.Error("TestService_DoJob next try job's1 time must be greater than on prev attempt")
			}
			prevTime = jt
			prevdCounter = dCounter
		}
	}

	if maxTimeToFinishJob <= 0 {
		t.Errorf("TestService_DoJob 'dumbFuncWithError' not enough retries after 6 secs. want 4, got %d instead", dCounter)
	}

}
