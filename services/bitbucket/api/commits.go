package bitbucket

import (
	"net/url"
	"time"
)

type CommitsService struct {
	c *Client
}
type Author struct {
	Raw  string `json:"raw"`
	User Actor  `json:"user"`
}

// Commit is a common struct used in several types
type Commit struct {
	Date    time.Time `json:"date"`
	Parents []struct {
		Hash  string `json:"hash"`
		Links Links  `json:"self"`
		Type  string `json:"type"`
	} `json:"parents"`
	Message            string `json:"message"`
	Hash               string `json:"hash"`
	Author             Author `json:"author"`
	Links              Links  `json:"links"`
	Type               string `json:"type"`
	ApprovedActorsUUID []string
}

func (cm *CommitsService) GetCommits(cmo *CommitsOptions) (commits []*Commit, err error) {
	url := cm.c.requestUrl("/repositories/%s/%s/commits/%s", cmo.Owner, cmo.Repo_slug, cmo.Branchortag)
	url += cm.buildCommitsQuery(cmo.Include, cmo.Exclude)
	err = cm.c.execute("GET", url, "", commits)
	return
}

func (cm *CommitsService) GetCommit(cmo *CommitsOptions) (commit *Commit, err error) {
	url := cm.c.requestUrl("/repositories/%s/%s/commit/%s", cmo.Owner, cmo.Repo_slug, cmo.Revision)
	err = cm.c.execute("GET", url, "", commit)
	return
}

func (cm *CommitsService) GetCommitComments(cmo *CommitsOptions) error {
	url := cm.c.requestUrl("/repositories/%s/%s/commit/%s/comments", cmo.Owner, cmo.Repo_slug, cmo.Revision)
	return cm.c.execute("DELETE", url, "", nil)
}

func (cm *CommitsService) GetCommitComment(cmo *CommitsOptions) error {
	url := cm.c.requestUrl("/repositories/%s/%s/commit/%s/comments/%s", cmo.Owner, cmo.Repo_slug, cmo.Revision, cmo.Comment_id)
	return cm.c.execute("GET", url, "", nil)
}

func (cm *CommitsService) GiveApprove(cmo *CommitsOptions) error {
	url := cm.c.requestUrl("/repositories/%s/%s/commit/%s/approve", cmo.Owner, cmo.Repo_slug, cmo.Revision)
	return cm.c.execute("POST", url, "", nil)
}

func (cm *CommitsService) RemoveApprove(cmo *CommitsOptions) error {
	url := cm.c.requestUrl("/repositories/%s/%s/commit/%s/approve", cmo.Owner, cmo.Repo_slug, cmo.Revision)
	return cm.c.execute("DELETE", url, "", nil)
}

func (cm *CommitsService) buildCommitsQuery(include, exclude string) string {

	p := url.Values{}

	if include != "" {
		p.Add("include", include)
	}
	if exclude != "" {
		p.Add("exclude", exclude)
	}

	return p.Encode()
}
