package trello

import (
	"encoding/json"
	"net/url"
)

const checklisturl = "checklists"

type CheckItem struct {
	Id   string
	Name string
	//nameData
	Pos   float64
	State string
	c     *Client `json:"-"`
}

type Checklist struct {
	Id         string
	IdCard     string
	IdBoard    string
	Pos        float64
	Name       string
	CheckItems []*CheckItem
	c          *Client `json:"-"`
}

// Checklist retrieves a checklist by id
func (c *Client) Checklist(id string) (*Checklist, error) {
	b, err := c.Request("GET", checklisturl+"/"+id, nil, nil)

	if err != nil {
		return nil, err
	}

	checklist := Checklist{
		c: c,
	}

	err = json.Unmarshal(b, &checklist)
	if err != nil {
		return nil, err
	}

	for _, ci := range checklist.CheckItems {
		ci.c = c
	}

	return &checklist, nil
}

func (c *Checklist) AddItem(name string) (*CheckItem, error) {
	extra := url.Values{"name": {name}}

	b, err := c.c.Request("POST", checklisturl+"/"+c.Id+"/checkItems", nil, extra)
	if err != nil {
		return nil, err
	}

	var ci *CheckItem
	err = json.Unmarshal(b, &ci)

	return ci, err
}

// CheckItem changes whether a checklist item id is marked as complete or not.
func (c *Checklist) CheckItem(id string, checked bool) error {
	extra := url.Values{}
	if checked {
		extra.Add("value", "complete")
	} else {
		extra.Add("value", "incomplete")
	}

	_, err := c.c.Request("PUT", cardurl+"/"+c.IdCard+"/checklist/"+c.Id+"/checkItem/"+id+"/state", nil, extra)
	if err != nil {
		return err
	}

	return nil
}
