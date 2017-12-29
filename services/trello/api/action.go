package api

import (
	"time"
)

type Action struct {
	Data struct {
		Text string
	}
	Date            *time.Time
	Id              string
	IdMemberCreator string
	MemberCreator   *Member
	Type            string
	c               *Client `json:"-"`
}
