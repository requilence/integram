package bitbucket

import (
	"encoding/json"
	"fmt"
	//	"github.com/k0kubun/pp"
	//	"os"

	"io/ioutil"
	"net/http"
	"strings"
)

type Client struct {
	Auth         *auth
	BaseURL      string
	Users        *UsersService
	User         *UserService
	Teams        *TeamsService
	Repositories *RepositoriesService
}

type auth struct {
	app_id, secret string
	user, password string
	client         *http.Client
}

func NewOAuth(i, s string) *Client {
	a := &auth{app_id: i, secret: s}
	return injectClient(a)
}

func NewWithHTTPClient(client *http.Client) *Client {
	a := &auth{client: client}
	return injectClient(a)
}

func NewBasicAuth(u, p string) *Client {
	a := &auth{user: u, password: p}
	return injectClient(a)
}

func injectClient(a *auth) *Client {
	c := &Client{Auth: a, BaseURL: API_BASE_URL}
	c.Repositories = &RepositoriesService{
		c:            c,
		PullRequests: &PullRequestsService{c: c},
		Repository:   &RepositoryService{c: c},
		Commits:      &CommitsService{c: c},
		Issues:       &IssuesService{c: c},
		Diff:         &DiffService{c: c},
		Webhooks:     &WebhooksService{c: c},
	}
	c.Users = &UsersService{c: c}
	c.User = &UserService{c: c}
	c.Teams = &TeamsService{c: c}
	return c
}

func (c *Client) execute(method, url, text string, res interface{}) error {

	body := strings.NewReader(text)
	req, err := http.NewRequest(method, url, body)
	if text != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	if err != nil {
		return err
	}

	if c.Auth.user != "" && c.Auth.password != "" {
		req.SetBasicAuth(c.Auth.user, c.Auth.password)
	}

	var client *http.Client
	if c.Auth.client != nil {
		client = c.Auth.client
	} else {
		client = http.DefaultClient
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if res != nil {
		//var result interface{}
		err = json.Unmarshal(buf, &res)
	}
	return err
}

func (c *Client) requestUrl(template string, args ...interface{}) string {

	if len(args) == 1 && args[0] == "" {
		return c.BaseURL + template
	} else {
		return c.BaseURL + fmt.Sprintf(template, args...)
	}
}
