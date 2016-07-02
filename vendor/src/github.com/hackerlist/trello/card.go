package trello

import (
	"encoding/json"
	"net/url"
	"strings"
	"time"
)

const cardurl = "cards"

type Card struct {
	//	Badges
	//	CheckItemStates
	Closed           bool
	DateLastActivity *time.Time
	Desc             string
	//	DescData
	Due *time.Time
	Id  string
	//	IdAttachmentCover
	Members        []*Member
	Labels         []*Label
	Checklists     []*Checklist
	MemberCreator  *Member
	IdMembersVoted []string
	IdMembers      []string
	IdShort        float64
	IdBoard        string
	IdList         string
	List           *List
	Board          *Board
	Actions        []*Action
	//	Labels                []string
	Name       string
	Pos        float64
	ShortLink  string
	ShortUrl   string
	Subscribed bool

	c *Client `json:"-"`
}

func (c *Card) URL() string {
	if c.ShortLink == "" && c.ShortUrl != "" {
		s := strings.Split(c.ShortUrl, "/")
		c.ShortLink = s[len(s)-1]
	}
	return "https://trello.com/c/" + c.ShortLink
}

// CreateCard create a card with given name on a given board. Extra options can
// be passed through the extra parameter. For details on options, see
// https://trello.com/docs/api/card/index.html#post-1-cards
func (c *Client) CreateCard(name string, idList string, extra url.Values) (*Card, error) {
	qp := url.Values{"name": {name}, "idList": {idList}}
	for k, v := range extra {
		qp[k] = v
	}
	//check required arguments 'urlSource'
	if _, found := qp["urlSource"]; !found {
		qp["urlSource"] = []string{"null"}
	}

	cardData, err := c.Request("POST", cardurl, nil, qp)
	if err != nil {
		return nil, err
	}

	card := Card{
		c: c,
	}

	err = json.Unmarshal(cardData, &card)
	if err != nil {
		return nil, err
	}

	//workaround to get card hash
	s := strings.Split(card.ShortUrl, "/")
	card.ShortLink = s[len(s)-1]

	return &card, nil
}

// AddCard add a card with given name to a card. Extra options can
// be passedthrough the extra parameter. For details on options, see
// https://trello.com/docs/api/card/index.html#post-1-cards
func (l *List) AddCard(name string, extra url.Values) (*Card, error) {
	return l.c.CreateCard(name, l.Id, extra)
}

// Card retrieves a trello card by ID
func (c *Client) Card(id string) (*Card, error) {
	b, err := c.Request("GET", cardurl+"/"+id, nil, url.Values{"actions": {"createCard"}, "action_fields": {"idMemberCreator"}, "members": {"true"}, "checkItemStates": {"true"}, "checklists": {"all"}, "board": {"true"}, "list": {"true"}, "membersVoted": {"true"}, "fields": {"badges,checkItemStates,closed,dateLastActivity,desc,due,idBoard,idChecklists,idLabels,idList,idMembers,idShort,labels,name,pos,shortUrl,idMembersVoted"}})

	if err != nil {
		return nil, err
	}

	card := Card{
		c: c,
	}

	err = json.Unmarshal(b, &card)
	if err != nil {
		return nil, err
	}

	// workaround to get memberCreator
	if len(card.Actions) > 0 && card.Actions[0].MemberCreator != nil {
		card.MemberCreator = card.Actions[0].MemberCreator
	}

	return &card, nil
}

func (c *Card) SetClient(cl *Client) {
	c.c = cl
}

func (c *Card) IsMemberVoted(memberID string) bool {
	for _, a := range c.IdMembersVoted {
		if a == memberID {
			return true
		}
	}
	return false
}

func (c *Card) IsMemberAssigned(memberID string) bool {
	for _, a := range c.Members {
		if a.Id == memberID {
			return true
		}
	}
	return false
}

func (c *Card) IsLabelAttached(id string) bool {
	for _, a := range c.Labels {
		if a.Id == id {
			return true
		}
	}
	return false
}

func (c *Card) AddComment(comment string) error {
	extra := url.Values{"text": {comment}}
	_, err := c.c.Request("POST", cardurl+"/"+c.Id+"/actions/comments", nil, extra)
	if err != nil {
		return err
	}
	return nil
}

func (c *Card) SetPosition(pos string) error {
	extra := url.Values{"value": {pos}}
	b, err := c.c.Request("PUT", cardurl+"/"+c.Id+"/pos", nil, extra)
	if err != nil {
		return err
	}
	var cu *Card
	err = json.Unmarshal(b, &cu)

	c.Pos = cu.Pos

	return nil
}
func (c *Card) SetDesc(desc string) error {
	extra := url.Values{"value": {desc}}
	b, err := c.c.Request("PUT", cardurl+"/"+c.Id+"/desc", nil, extra)
	if err != nil {
		return err
	}
	var cu *Card
	err = json.Unmarshal(b, &cu)

	c.Desc = cu.Desc

	return nil
}

func (c *Card) SetName(name string) error {
	extra := url.Values{"value": {name}}
	b, err := c.c.Request("PUT", cardurl+"/"+c.Id+"/name", nil, extra)
	if err != nil {
		return err
	}
	var cu *Card
	err = json.Unmarshal(b, &cu)

	c.Name = cu.Name

	return nil
}

// AddChecklist created a new checklist on the card.
func (c *Card) AddChecklist(name string) (*Checklist, error) {
	qp := url.Values{"name": {name}}
	b, err := c.c.Request("POST", cardurl+"/"+c.Id+"/checklists", nil, qp)
	if err != nil {
		return nil, err
	}

	var cl *Checklist
	err = json.Unmarshal(b, &cl)
	return cl, err
}

// Checklists retrieves all checklists from a trello card
func (c *Card) GetChecklists() ([]*Checklist, error) {
	b, err := c.c.Request("GET", cardurl+"/"+c.Id+"/checklists", nil, nil)
	if err != nil {
		return nil, err
	}

	var checklists []*Checklist

	err = json.Unmarshal(b, &checklists)
	if err != nil {
		return nil, err
	}

	for _, checklist := range checklists {
		checklist.c = c.c
		for _, ci := range checklist.CheckItems {
			ci.c = c.c
		}
	}

	return checklists, nil
}

// Actions retrieves a list of all actions (e.g. events, activity)
// performed on a card
func (c *Card) GetActions() ([]*Action, error) {
	b, err := c.c.Request("GET", cardurl+"/"+c.Id+"/actions", nil, nil)
	if err != nil {
		return nil, err
	}

	var act []*Action

	err = json.Unmarshal(b, &act)

	if err != nil {
		return nil, err
	}

	for _, a := range act {
		a.c = c.c
	}

	return act, nil
}
