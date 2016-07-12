package bitbucket

import ()

type TeamsService struct {
	c *Client
}

func (t *TeamsService) List(role string) error {
	url := t.c.requestUrl("/teams/?role=%s", role)
	return t.c.execute("GET", url, "", nil)
}

func (t *TeamsService) Profile(teamname string) error {
	url := t.c.requestUrl("/teams/%s/", teamname)
	return t.c.execute("GET", url, "", nil)
}

func (t *TeamsService) Members(teamname string) error {
	url := t.c.requestUrl("/teams/%s/members", teamname)
	return t.c.execute("GET", url, "", nil)
}

func (t *TeamsService) Followers(teamname string) error {
	url := t.c.requestUrl("/teams/%s/followers", teamname)
	return t.c.execute("GET", url, "", nil)
}

func (t *TeamsService) Following(teamname string) error {
	url := t.c.requestUrl("/teams/%s/following", teamname)
	return t.c.execute("GET", url, "", nil)
}

func (t *TeamsService) Repositories(teamname string) error {
	url := t.c.requestUrl("/teams/%s/repositories", teamname)
	return t.c.execute("GET", url, "", nil)
}
