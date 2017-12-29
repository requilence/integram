package bitbucket

import (
	"encoding/json"
	"time"
)

type IssuesService struct {
	c *Client
}

/* Tsss.. Looks like this API is unpublic */

func (cm *IssuesService) GetIssues(cmo *IssuesOptions) error {
	url := cm.c.requestUrl("/repositories/%s/%s/issues", cmo.Owner, cmo.Repo_slug)
	return cm.c.execute("GET", url, "", nil)
}

func (cm *IssuesService) GetIssue(cmo *IssuesOptions) error {
	url := cm.c.requestUrl("/repositories/%s/%s/issues/%d", cmo.Owner, cmo.Repo_slug, cmo.Issue_id)
	return cm.c.execute("GET", url, "", nil)
}

func (cm *IssuesService) GetIssueComments(cmo *IssuesOptions) error {
	url := cm.c.requestUrl("/repositories/%s/%s/issues/%d/comments", cmo.Owner, cmo.Repo_slug, cmo.Issue_id)
	return cm.c.execute("DELETE", url, "", nil)
}

func (cm *IssuesService) GetIssueComment(cmo *IssuesOptions) error {
	url := cm.c.requestUrl("/repositories/%s/%s/issues/%d/comments/%s", cmo.Owner, cmo.Repo_slug, cmo.Issue_id, cmo.Comment_id)
	return cm.c.execute("GET", url, "", nil)
}

func (cm *IssuesService) Vote(cmo *IssuesOptions) error {
	url := cm.c.requestUrl("/repositories/%s/%s/issues/%d/vote", cmo.Owner, cmo.Repo_slug, cmo.Issue_id)
	return cm.c.execute("PUT", url, "", nil)
}

func (cm *IssuesService) SetState(cmo *IssuesOptions) error {
	u := cm.c.requestUrl("/repositories/%s/%s/issues/%d", cmo.Owner, cmo.Repo_slug, cmo.Issue_id)
	body := map[string]string{"state": cmo.State}
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}

	return cm.c.execute("PUT", u, string(data), nil)
}

func (cm *IssuesService) Unvote(cmo *IssuesOptions) error {
	url := cm.c.requestUrl("/repositories/%s/%s/issues/%d/vote", cmo.Owner, cmo.Repo_slug, cmo.Issue_id)
	return cm.c.execute("DELETE", url, "", nil)
}

// Issue https://confluence.atlassian.com/bitbucket/event-payloads-740262817.html#EventPayloads-entity_issue
type Issue struct {
	ID        int    `json:"id"`
	Component string `json:"component"`
	Title     string `json:"title"`
	Content   struct {
		Raw    string `json:"raw"`
		HTML   string `json:"html"`
		Markup string `json:"markup"`
	} `json:"content"`
	Priority  string `json:"priority"`
	State     string `json:"state"`
	Type      string `json:"type"`
	Milestone struct {
		Name string `json:"name"`
	} `json:"milestone"`
	Version struct {
		Name string `json:"name"`
	} `json:"version"`
	Reporter        *Actor
	Votes           int
	Assignee        *Actor
	Repository      *Repository
	VotedActorsUUID []string
	CreatedOn       *time.Time `json:"created_on"`
	UpdatedOn       *time.Time `json:"updated_on"`
	Links           Links      `json:"links"`
}
