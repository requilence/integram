package bitbucket

import (
	"encoding/json"
	"github.com/k0kubun/pp"
	"os"
	"strconv"
	"time"
)

type PullRequestsService struct {
	c *Client
}

// An Approval is used in pull requests
type Approval struct {
	Date *time.Time `json:"date"`
	User Actor      `json:"user"`
}

// Participant is the actual structure returned in PullRequest events
// Note: this doesn't match the docs!?
type Participant struct {
	Role     string `json:"role"`
	Type     string `json:"type"`
	Approved bool   `json:"approved"`
	User     Actor  `json:"user"`
}

// PullRequest https://confluence.atlassian.com/bitbucket/event-payloads-740262817.html#EventPayloads-entity_pullrequest
type PullRequest struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	State       string `json:"state"`
	Author      Actor  `json:"author"`
	Source      struct {
		Branch struct {
			Name string `json:"name"`
		} `json:"branch"`
		Commit struct {
			Hash string `json:"hash"`
		} `json:"commit"`
		Repository Repository `json:"repository"`
	} `json:"source"`
	Destination struct {
		Branch struct {
			Name string `json:"name"`
		} `json:"branch"`
		Commit struct {
			Hash string `json:"hash"`
		} `json:"commit"`
		Repository Repository `json:"repository"`
	} `json:"destination"`
	MergeCommit struct {
		Hash string `json:"hash"`
	} `json:"merge_commit"`
	Participants      []Participant `json:"participants"`
	Reviewers         []Actor       `json:"reviewers"`
	CloseSourceBranch bool          `json:"close_source_branch"`
	ClosedBy          Actor         `json:"closed_by"`
	Reason            string        `json:"reason"`
	CreatedOn         *time.Time    `json:"created_on"`
	UpdatedOn         *time.Time    `json:"updated_on"`
	Links             Links         `json:"links"`
}

func (p *PullRequestsService) Create(po *PullRequestsOptions) error {
	data := p.buildPullRequestBody(po)
	url := p.c.requestUrl("/repositories/%s/%s/pullrequests/", po.Owner, po.Repo_slug)
	return p.c.execute("POST", url, data, nil)
}

func (p *PullRequestsService) Update(po *PullRequestsOptions) error {
	data := p.buildPullRequestBody(po)
	url := p.c.BaseURL + "/repositories/" + po.Owner + "/" + po.Repo_slug + "/pullrequests/" + strconv.Itoa(po.Id)
	return p.c.execute("PUT", url, data, nil)
}

func (p *PullRequestsService) Gets(po *PullRequestsOptions) error {
	url := p.c.BaseURL + "/repositories/" + po.Owner + "/" + po.Repo_slug + "/pullrequests/"
	return p.c.execute("GET", url, "", nil)
}

func (p *PullRequestsService) Get(po *PullRequestsOptions) error {
	url := p.c.BaseURL + "/repositories/" + po.Owner + "/" + po.Repo_slug + "/pullrequests/" + strconv.Itoa(po.Id)
	return p.c.execute("GET", url, "", nil)
}

func (p *PullRequestsService) Activities(po *PullRequestsOptions) error {
	url := p.c.BaseURL + "/repositories/" + po.Owner + "/" + po.Repo_slug + "/pullrequests/activity"
	return p.c.execute("GET", url, "", nil)
}

func (p *PullRequestsService) Activity(po *PullRequestsOptions) error {
	url := p.c.BaseURL + "/repositories/" + po.Owner + "/" + po.Repo_slug + "/pullrequests/" + strconv.Itoa(po.Id) + "/activity"
	return p.c.execute("GET", url, "", nil)
}

func (p *PullRequestsService) Commits(po *PullRequestsOptions) error {
	url := p.c.BaseURL + "/repositories/" + po.Owner + "/" + po.Repo_slug + "/pullrequests/" + strconv.Itoa(po.Id) + "/commits"
	return p.c.execute("GET", url, "", nil)
}

func (p *PullRequestsService) Patch(po *PullRequestsOptions) error {
	url := p.c.BaseURL + "/repositories/" + po.Owner + "/" + po.Repo_slug + "/pullrequests/" + strconv.Itoa(po.Id) + "/patch"
	return p.c.execute("GET", url, "", nil)
}

func (p *PullRequestsService) Diff(po *PullRequestsOptions) error {
	url := p.c.BaseURL + "/repositories/" + po.Owner + "/" + po.Repo_slug + "/pullrequests/" + strconv.Itoa(po.Id) + "/diff"
	return p.c.execute("GET", url, "", nil)
}

func (p *PullRequestsService) Merge(po *PullRequestsOptions) error {
	data := p.buildPullRequestBody(po)
	url := p.c.BaseURL + "/repositories/" + po.Owner + "/" + po.Repo_slug + "/pullrequests/" + strconv.Itoa(po.Id) + "/merge"
	return p.c.execute("POST", url, data, nil)
}

func (p *PullRequestsService) Decline(po *PullRequestsOptions) error {
	data := p.buildPullRequestBody(po)
	url := p.c.BaseURL + "/repositories/" + po.Owner + "/" + po.Repo_slug + "/pullrequests/" + strconv.Itoa(po.Id) + "/decline"
	return p.c.execute("POST", url, data, nil)
}

func (p *PullRequestsService) GetComments(po *PullRequestsOptions) error {
	url := p.c.BaseURL + "/repositories/" + po.Owner + "/" + po.Repo_slug + "/pullrequests/" + strconv.Itoa(po.Id) + "/comments/"
	return p.c.execute("GET", url, "", nil)
}

func (p *PullRequestsService) GetComment(po *PullRequestsOptions) error {
	url := p.c.BaseURL + "/repositories/" + po.Owner + "/" + po.Repo_slug + "/pullrequests/" + strconv.Itoa(po.Id) + "/comments/" + po.Comment_id
	return p.c.execute("GET", url, "", nil)
}

func (p *PullRequestsService) buildPullRequestBody(po *PullRequestsOptions) string {

	body := map[string]interface{}{}
	body["source"] = map[string]interface{}{}
	body["destination"] = map[string]interface{}{}
	body["reviewers"] = []map[string]string{}
	body["title"] = ""
	body["description"] = ""
	body["message"] = ""
	body["close_source_branch"] = false

	if n := len(po.Reviewers); n > 0 {
		for i, user := range po.Reviewers {
			body["reviewers"].([]map[string]string)[i] = map[string]string{"username": user}
		}
	}

	if po.Source_branch != "" {
		body["source"].(map[string]interface{})["branch"] = map[string]string{"name": po.Source_branch}
	}

	if po.Source_repository != "" {
		body["source"].(map[string]interface{})["repository"] = map[string]interface{}{"full_name": po.Source_repository}
	}

	if po.Destination_branch != "" {
		body["destination"].(map[string]interface{})["branch"] = map[string]interface{}{"name": po.Destination_branch}
	}

	if po.Destination_commit != "" {
		body["destination"].(map[string]interface{})["commit"] = map[string]interface{}{"hash": po.Destination_commit}
	}

	if po.Title != "" {
		body["title"] = po.Title
	}

	if po.Description != "" {
		body["description"] = po.Description
	}

	if po.Message != "" {
		body["message"] = po.Message
	}

	if po.Close_source_branch == true || po.Close_source_branch == false {
		body["close_source_branch"] = po.Close_source_branch
	}

	data, err := json.Marshal(body)
	if err != nil {
		pp.Println(err)
		os.Exit(9)
	}

	return string(data)
}
