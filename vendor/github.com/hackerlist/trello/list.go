package trello

import (
	"encoding/json"
)

var listurl = "lists"

type List struct {
	Closed  bool
	Id      string
	IdBoard string
	Name    string
	Pos     float64
	c       *Client `json:"-"`
}

type Label struct {
	Id      string
	Color   string
	IdBoard string
	Name    string
	Uses    int
}

func (c *Client) List(id string) (*List, error) {
	b, err := c.Request("GET", listurl+"/"+id, nil, nil)
	if err != nil {
		return nil, err
	}

	l := List{
		c: c,
	}

	err = json.Unmarshal(b, &l)
	if err != nil {
		return nil, err
	}

	return &l, nil
}

func (l *List) Cards() ([]*Card, error) {
	js, err := l.c.Request("GET", listurl+"/"+l.Id+"/cards", nil, nil)
	if err != nil {
		return nil, err
	}

	var cards []*Card

	err = json.Unmarshal(js, &cards)

	if err != nil {
		return nil, err
	}

	for _, c := range cards {
		c.c = l.c
	}

	return cards, nil
}
