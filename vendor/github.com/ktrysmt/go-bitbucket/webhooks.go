package bitbucket

import (
	"encoding/json"
	"github.com/k0kubun/pp"
	"os"
	"time"
)

type WebhooksService struct {
	c *Client
}

func (r *WebhooksService) buildWebhooksBody(ro *WebhooksOptions) string {

	body := map[string]interface{}{}

	if ro.Description != "" {
		body["description"] = ro.Description
	}
	if ro.Url != "" {
		body["url"] = ro.Url
	}
	if ro.Active == true || ro.Active == false {
		body["active"] = ro.Active
	}

	if n := len(ro.Events); n > 0 {
		for i, event := range ro.Events {
			body["events"].([]string)[i] = event
		}
	}

	data, err := json.Marshal(body)
	if err != nil {
		pp.Println(err)
		os.Exit(9)
	}

	return string(data)
}

func (r *WebhooksService) Gets(ro *WebhooksOptions) error {
	url := r.c.requestUrl("/repositories/%s/%s/hooks/", ro.Owner, ro.Repo_slug)
	return r.c.execute("GET", url, "", nil)
}

func (r *WebhooksService) Create(ro *WebhooksOptions) error {
	data := r.buildWebhooksBody(ro)
	url := r.c.requestUrl("/repositories/%s/%s/hooks", ro.Owner, ro.Repo_slug)
	return r.c.execute("POST", url, data, nil)
}

func (r *WebhooksService) Get(ro *WebhooksOptions) error {
	url := r.c.requestUrl("/repositories/%s/%s/hooks/%s", ro.Owner, ro.Repo_slug, ro.Uuid)
	return r.c.execute("GET", url, "", nil)
}

func (r *WebhooksService) Update(ro *WebhooksOptions) error {
	data := r.buildWebhooksBody(ro)
	url := r.c.requestUrl("/repositories/%s/%s/hooks/%s", ro.Owner, ro.Repo_slug, ro.Uuid)
	return r.c.execute("PUT", url, data, nil)
}

func (r *WebhooksService) Delete(ro *WebhooksOptions) error {
	url := r.c.requestUrl("/repositories/%s/%s/hooks/%s", ro.Owner, ro.Repo_slug, ro.Uuid)
	return r.c.execute("DELETE", url, "", nil)
}

// RepoPushEvent https://confluence.atlassian.com/bitbucket/event-payloads-740262817.html#EventPayloads-Push
type RepoPushEvent struct {
	Actor      Actor      `json:"actor"`
	Repository Repository `json:"repository"`
	Push       struct {
		Changes []struct {
			Forced    bool     `json:"forced"`
			Old       OldOrNew `json:"old"`
			New       OldOrNew `json:"new"`
			Closed    bool     `json:"closed"`
			Created   bool     `json:"created"`
			Truncated bool     `json:"truncated"`
			Links     `json:"links"`
			Commits   []Commit `json:"commits"`
		} `json:"changes"`
	} `json:"push"`
}

func (l LinkHref) String() string {
	return l.Href
}

// OldOrNew is used in the RepoPushEvent type
type OldOrNew struct {
	Repository struct {
		FullName string `json:"full_name"`
		UUID     string `json:"uuid"`
		Links    Links  `json:"links"`
		Name     string `json:"name"`
		Type     string `json:"type"`
	} `json:"repository"`
	Target struct {
		Date    *time.Time `json:"date"`
		Parents []struct {
			Hash  string `json:"hash"`
			Links Links  `json:"links"`
			Type  string `json:"type"`
		} `json:"parents"`
		Message string `json:"message"`
		Hash    string `json:"hash"`
		Author  Author `json:"author"`
		Links   Links  `json:"links"`
		Type    string `json:"type"`
	} `json:"target"`
	Links Links  `json:"links"`
	Name  string `json:"name"`
	Type  string `json:"type"`
}

// RepoForkEvent https://confluence.atlassian.com/bitbucket/event-payloads-740262817.html#EventPayloads-Fork
type RepoForkEvent struct {
	Actor      Actor      `json:"actor"`
	Repository Repository `json:"repository"`
	Fork       Repository `json:"fork"`
}

// RepoCommitCommentCreatedEvent https://confluence.atlassian.com/bitbucket/event-payloads-740262817.html#EventPayloads-CommitCommentCreated
type RepoCommitCommentCreatedEvent struct {
	Actor      Actor      `json:"actor"`
	Comment    Comment    `json:"comment"`
	Repository Repository `json:"repository"`
	Commit     struct {
		Hash string `json:"hash"`
	} `json:"commit"`
}

// A RepoCommitStatusEvent is not a BB event. This is the base for several CommitStatus* events.
type RepoCommitStatusEvent struct {
	Actor        Actor      `json:"actor"`
	Repository   Repository `json:"repository"`
	CommitStatus struct {
		Name        string     `json:"name"`
		Description string     `json:"description"`
		State       string     `json:"state"`
		Key         string     `json:"key"`
		URL         string     `json:"url"`
		Type        string     `json:"type"`
		CreatedOn   *time.Time `json:"created_on"`
		UpdatedOn   *time.Time `json:"updated_on"`
		Links       Links      `json:"links"`
	} `json:"commit_status"`
}

// RepoCommitStatusCreatedEvent https://confluence.atlassian.com/bitbucket/event-payloads-740262817.html#EventPayloads-CommitStatusCreated
type RepoCommitStatusCreatedEvent struct {
	RepoCommitStatusEvent
}

// RepoCommitStatusUpdatedEvent https://confluence.atlassian.com/bitbucket/event-payloads-740262817.html#EventPayloads-CommitStatusUpdated
type RepoCommitStatusUpdatedEvent struct {
	RepoCommitStatusEvent
}

// An IssueEvent is not a BB event. This is the base for several Issue* events.
type IssueEvent struct {
	Actor      Actor      `json:"actor"`
	Issue      Issue      `json:"issue"`
	Repository Repository `json:"repository"`
}

// IssueCreatedEvent https://confluence.atlassian.com/bitbucket/event-payloads-740262817.html#EventPayloads-Created
type IssueCreatedEvent struct {
	IssueEvent
}

// IssueUpdatedEvent https://confluence.atlassian.com/bitbucket/event-payloads-740262817.html#EventPayloads-Updated
type IssueUpdatedEvent struct {
	IssueEvent
	Comment Comment `json:"comment"`
	Changes struct {
		Status struct {
			Old string `json:"old"`
			New string `json:"new"`
		} `json:"status"`
		Content struct {
			Old string `json:"old"`
			New string `json:"new"`
		} `json:"status"`
		Title struct {
			Old string `json:"old"`
			New string `json:"new"`
		} `json:"status"`
	} `json:"changes"`
}

// IssueCommentCreatedEvent https://confluence.atlassian.com/bitbucket/event-payloads-740262817.html#EventPayloads-CommentCreated
type IssueCommentCreatedEvent struct {
	IssueEvent
	Comment Comment `json:"comment"`
}

// A PullRequestEvent is not a BB event. This is the base for several PullRequest* events.
type PullRequestEvent struct {
	Actor       Actor       `json:"actor"`
	PullRequest PullRequest `json:"pullrequest"`
	Repository  Repository  `json:"repository"`
}

// PullRequestCreatedEvent https://confluence.atlassian.com/bitbucket/event-payloads-740262817.html#EventPayloads-Created
type PullRequestCreatedEvent struct {
	PullRequestEvent
}

// PullRequestUpdatedEvent https://confluence.atlassian.com/bitbucket/event-payloads-740262817.html#EventPayloads-Updated.1
type PullRequestUpdatedEvent struct {
	PullRequestEvent
}

// PullRequestApprovedEvent https://confluence.atlassian.com/bitbucket/event-payloads-740262817.html#EventPayloads-Approved
type PullRequestApprovedEvent struct {
	PullRequestEvent
	Approval Approval `json:"approval"`
}

// PullRequestApprovalRemovedEvent https://confluence.atlassian.com/bitbucket/event-payloads-740262817.html#EventPayloads-ApprovalRemoved
type PullRequestApprovalRemovedEvent struct {
	PullRequestEvent
	Approval Approval `json:"approval"`
}

// PullRequestMergedEvent https://confluence.atlassian.com/bitbucket/event-payloads-740262817.html#EventPayloads-Merged
type PullRequestMergedEvent struct {
	PullRequestEvent
}

// PullRequestDeclinedEvent https://confluence.atlassian.com/bitbucket/event-payloads-740262817.html#EventPayloads-Declined
type PullRequestDeclinedEvent struct {
	PullRequestEvent
}

// A PullRequestCommentEvent doesn't exist. It is used as the base for several real events.
type PullRequestCommentEvent struct {
	PullRequestEvent
	Comment Comment `json:"comment"`
}

// PullRequestCommentCreatedEvent https://confluence.atlassian.com/bitbucket/event-payloads-740262817.html#EventPayloads-CommentCreated.1
type PullRequestCommentCreatedEvent struct {
	PullRequestCommentEvent
}

// PullRequestCommentUpdatedEvent https://confluence.atlassian.com/bitbucket/event-payloads-740262817.html#EventPayloads-CommentUpdated
type PullRequestCommentUpdatedEvent struct {
	PullRequestCommentEvent
}

// PullRequestCommentDeletedEvent https://confluence.atlassian.com/bitbucket/event-payloads-740262817.html#EventPayloads-CommentDeleted
type PullRequestCommentDeletedEvent struct {
	PullRequestCommentEvent
}
