package bitbucket

import (
//"github.com/k0kubun/pp"
)

type RepositoriesService struct {
	c            *Client
	PullRequests *PullRequestsService
	Repository   *RepositoryService
	Commits      *CommitsService
	Issues       *IssuesService
	Diff         *DiffService
	Webhooks     *WebhooksService
}

func (r *RepositoriesService) ListForAccount(ro *RepositoriesOptions) error {
	url := r.c.requestUrl("/repositories/%s", ro.Owner)
	if ro.Role != "" {
		url += "?role=" + ro.Role
	}
	return r.c.execute("GET", url, "", nil)
}

func (r *RepositoriesService) ListForTeam(ro *RepositoriesOptions) error {
	url := r.c.requestUrl("/repositories/%s", ro.Owner)
	if ro.Role != "" {
		url += "?role=" + ro.Role
	}
	return r.c.execute("GET", url, "", nil)
}

func (r *RepositoriesService) ListPublic() error {
	url := r.c.requestUrl("/repositories/", "")
	return r.c.execute("GET", url, "", nil)
}
