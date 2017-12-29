package bitbucket

import (
	"encoding/json"
	"github.com/k0kubun/pp"
	"os"
)

type RepositoryService struct {
	c *Client
}

func (r *RepositoryService) Create(ro *RepositoryOptions) interface{} {
	data := r.buildRepositoryBody(ro)
	url := r.c.requestUrl("/repositories/%s/%s", ro.Owner, ro.Repo_slug)
	return r.c.execute("POST", url, data, nil)
}

func (r *RepositoryService) Get(ro *RepositoryOptions) interface{} {
	url := r.c.requestUrl("/repositories/%s/%s", ro.Owner, ro.Repo_slug)
	return r.c.execute("GET", url, "", nil)
}

func (r *RepositoryService) Delete(ro *RepositoryOptions) interface{} {
	url := r.c.requestUrl("/repositories/%s/%s", ro.Owner, ro.Repo_slug)
	return r.c.execute("DELETE", url, "", nil)
}

func (r *RepositoryService) ListWatchers(ro *RepositoryOptions) interface{} {
	url := r.c.requestUrl("/repositories/%s/%s/watchers", ro.Owner, ro.Repo_slug)
	return r.c.execute("GET", url, "", nil)
}

func (r *RepositoryService) ListForks(ro *RepositoryOptions) interface{} {
	url := r.c.requestUrl("/repositories/%s/%s/forks", ro.Owner, ro.Repo_slug)
	return r.c.execute("GET", url, "", nil)
}

func (r *RepositoryService) buildRepositoryBody(ro *RepositoryOptions) string {

	body := map[string]interface{}{}

	if ro.Scm != "" {
		body["scm"] = ro.Scm
	}
	//if ro.Scm != "" {
	//		body["name"] = ro.Name
	//}
	if ro.Is_private != "" {
		body["is_private"] = ro.Is_private
	}
	if ro.Description != "" {
		body["description"] = ro.Description
	}
	if ro.Fork_policy != "" {
		body["fork_policy"] = ro.Fork_policy
	}
	if ro.Language != "" {
		body["language"] = ro.Language
	}
	if ro.Has_issues != "" {
		body["has_issues"] = ro.Has_issues
	}
	if ro.Has_wiki != "" {
		body["has_wiki"] = ro.Has_wiki
	}

	data, err := json.Marshal(body)
	if err != nil {
		pp.Println(err)
		os.Exit(9)
	}

	return string(data)
}

// Repository is a common struct used in several types
type Repository struct {
	Scm      string `json:"scm"`
	FullName string `json:"full_name"`
	Type     string `json:"type"`
	Website  string `json:"website"`
	Owner    struct {
		Username    string `json:"username"`
		Type        string `json:"type"`
		UUID        string `json:"uuid"`
		Links       Links  `json:"links"`
		DisplayName string `json:"display_name"`
	} `json:"owner"`
	UUID      string `json:"uuid"`
	Links     Links  `json:"links"`
	Name      string `json:"name"`
	IsPrivate bool   `json:"is_private"`
}
