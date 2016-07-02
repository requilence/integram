package trello

import (
	"encoding/json"
)

var orgurl = "organizations"

type Organization struct {
	Desc string
	//	DescData
	DisplayName string
	Id          string
	//	LogoHash
	Name string
	//	PowerUps
	//	Products
	Url     string
	Website string
	c       *Client `json:"-"`
}

// Organization retrieves a trello organization
func (c *Client) Organization(name string) (*Organization, error) {
	b, err := c.Request("GET", orgurl+"/"+name, nil, nil)
	if err != nil {
		return nil, err
	}

	o := Organization{
		c: c,
	}
	err = json.Unmarshal(b, &o)
	if err != nil {
		return nil, err
	}

	return &o, nil
}

func (o *Organization) Members() ([]*Member, error) {
	b, err := o.c.Request("GET", orgurl+"/"+o.Name+"/members", nil, nil)
	if err != nil {
		return nil, err
	}
	var members []struct{ FullName, Id, Username string }

	err = json.Unmarshal(b, &members)

	if err != nil {
		return nil, err
	}

	var out []*Member
	for _, m := range members {
		mem, err := o.c.Member(m.Username)
		if err != nil {
			return nil, err
		}
		out = append(out, mem)
	}
	return out, nil
}

// Get a Organization's boards
func (o *Organization) Boards() ([]*Board, error) {
	b, err := o.c.Request("GET", orgurl+"/"+o.Name+"/boards", nil, nil)
	if err != nil {
		return nil, err
	}
	var boards []*Board

	err = json.Unmarshal(b, &boards)

	if err != nil {
		return nil, err
	}

	for _, b := range boards {
		b.c = o.c
	}

	return boards, nil
}
