package trello

import (
	"encoding/json"
	"net/url"
	"time"
)

const boardurl = "boards"

// Trello Board.
type Board struct {
	Closed           bool
	DateLastActivity *time.Time
	DateLastView     *time.Time
	Desc             string
	//	DescData string
	Id             string
	IdOrganization *string
	//	Invitations []string
	Invited bool
	//	LabelNames []LabelName
	//	Memberships []MembershiP
	Name string
	//	Pinned
	//	PowerUps
	Prefs struct {
		Background      string
		Voting          string
		BackgroundImage string
	}
	ShortLink string
	ShortUrl  string
	//	Starred
	//	Subcribed
	Url string
	c   *Client `json:"-"`
}

// Get a Member's boards
func (m *Member) Boards() ([]*Board, error) {
	b, err := m.c.Request("GET", memberurl+"/"+m.Username+"/boards", nil, nil)
	if err != nil {
		return nil, err
	}
	var boards []*Board

	err = json.Unmarshal(b, &boards)

	if err != nil {
		return nil, err
	}

	for _, b := range boards {
		b.c = m.c
	}

	return boards, nil
}

// CreateBoard creats a new board with the given name. Extra options can be passed
// through the extra parameter. For details on options, see
// https://trello.com/docs/api/board/index.html#post-1-boards
func (c *Client) CreateBoard(name string, extra url.Values) (*Board, error) {
	qp := url.Values{"name": {name}}
	for k, v := range extra {
		qp[k] = v
	}

	b, err := c.Request("POST", boardurl, nil, qp)
	if err != nil {
		return nil, err
	}

	board := Board{
		c: c,
	}

	err = json.Unmarshal(b, &board)
	if err != nil {
		return nil, err
	}

	return &board, nil
}

func (c *Client) Board(id string) (*Board, error) {
	b, err := c.Request("GET", boardurl+"/"+id, nil, nil)
	if err != nil {
		return nil, err
	}

	board := Board{
		c: c,
	}

	err = json.Unmarshal(b, &board)
	if err != nil {
		return nil, err
	}

	return &board, nil
}

func (b *Board) Cards() ([]*Card, error) {
	js, err := b.c.Request("GET", boardurl+"/"+b.Id+"/cards", nil, nil)
	if err != nil {
		return nil, err
	}

	var cards []*Card

	err = json.Unmarshal(js, &cards)

	if err != nil {
		return nil, err
	}

	for _, c := range cards {
		c.c = b.c
	}

	return cards, nil
}

// AddList creates a new list with the given name on a Board.
func (b *Board) AddList(name string) (*List, error) {
	qp := url.Values{"name": {name}}
	js, err := b.c.Request("POST", boardurl+"/"+b.Id+"/lists", nil, qp)
	if err != nil {
		return nil, err
	}

	list := List{
		c: b.c,
	}

	err = json.Unmarshal(js, &list)
	if err != nil {
		return nil, err
	}

	return &list, nil
}

func (b *Board) Lists() ([]*List, error) {
	js, err := b.c.Request("GET", boardurl+"/"+b.Id+"/lists", nil, nil)
	if err != nil {
		return nil, err
	}

	var lists []struct{ Id string }

	err = json.Unmarshal(js, &lists)

	if err != nil {
		return nil, err
	}

	var out []*List
	for _, ld := range lists {
		list, err := b.c.List(ld.Id)
		if err != nil {
			return nil, err
		}
		out = append(out, list)
	}
	return out, nil
}

// Members returns a list of the members of a board.
func (b *Board) Members() ([]*Member, error) {
	js, err := b.c.Request("GET", boardurl+"/"+b.Id+"/members", nil, nil)
	if err != nil {
		return nil, err
	}

	var memjs []struct{ Id string }

	err = json.Unmarshal(js, &memjs)

	if err != nil {
		return nil, err
	}

	var out []*Member
	for _, md := range memjs {
		member, err := b.c.Member(md.Id)
		if err != nil {
			return nil, err
		}
		out = append(out, member)
	}
	return out, nil
}

// Invite invites a member to a board by email.
// fullname cannot begin or end with a space and must be at least 4 characters long.
// typ may be one of normal, observer or admin.
func (b *Board) Invite(email, fullname, typ string) error {
	extra := url.Values{"email": {email}, "fullName": {fullname}, "type": {typ}}
	_, err := b.c.Request("PUT", boardurl+"/"+b.Id+"/members", nil, extra)
	if err != nil {
		return err
	}
	return nil
}

// AddMember adds an organization or member by id or name to a board.
// typ may be one of normal, observer or admin.
func (b *Board) AddMember(id, typ string) error {
	extra := url.Values{"type": {typ}}
	_, err := b.c.Request("PUT", boardurl+"/"+b.Id+"/members/"+id, nil, extra)
	if err != nil {
		return err
	}
	return nil
}
