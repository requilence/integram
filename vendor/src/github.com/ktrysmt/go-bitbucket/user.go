package bitbucket

import (
//"github.com/k0kubun/pp"
)

type UserService struct {
	c *Client
}

func (u *UserService) Profile() (a *Actor, err error) {
	url := u.c.BaseURL + "/user/"
	err = u.c.execute("GET", url, "", &a)
	return
}

func (u *UserService) Emails(res interface{}) error {

	url := u.c.BaseURL + "/user/emails"
	return u.c.execute("GET", url, "", res)
}

type Actor struct {
	Username    string `json:"username"`
	Type        string `json:"type"`
	UUID        string `json:"uuid"`
	Links       Links  `json:"links"`
	DisplayName string `json:"display_name"`
}

// Links is a common struct used in several types. Refer to the event documentation
// to find out which link types are populated in which events.
type Links struct {
	Avatar  LinkHref `json:"avatar"`
	HTML    LinkHref `json:"html"`
	Diff    LinkHref `json:"diff"`
	Self    LinkHref `json:"self"`
	Commits LinkHref `json:"commits"`
	Commit  LinkHref `json:"commit"`
}

type LinkHref struct {
	Href string `json:"href"`
}
