package trello

import (
	"encoding/json"
	"net/url"
)

const memberurl = "members"

// Trello Member.
type Member struct {
	Id              string
	Username        string
	FullName        string
	Url             string
	Bio             string
	IdOrganizations []string
	IdBoards        []string
	c               *Client `json:"-"`
}

// Member retrieves a trello member's (user) info
func (c *Client) Member(username string) (*Member, error) {
	extra := url.Values{"fields": {"username,fullName,url,bio,idBoards,idOrganizations"}}
	b, err := c.Request("GET", memberurl+"/"+username, nil, extra)
	if err != nil {
		return nil, err
	}

	m := Member{
		c: c,
	}

	err = json.Unmarshal(b, &m)
	if err != nil {
		return nil, err
	}

	return &m, nil
}
