package bitbucket

import "time"

const (
	API_BASE_URL = "https://bitbucket.org/api/2.0"
)

type RepositoriesOptions struct {
	Owner string `json:"owner"`
	Team  string `json:"team"`
	Role  string `json:"role"` // role=[owner|admin|contributor|member]
}

type RepositoryOptions struct {
	Owner     string `json:"owner"`
	Repo_slug string `json:"repo_slug"`
	Scm       string `json:"scm"`
	//	Name        string `json:"name"`
	Is_private  string `json:"is_private"`
	Description string `json:"description"`
	Fork_policy string `json:"fork_policy"`
	Language    string `json:"language"`
	Has_issues  string `json:"has_issues"`
	Has_wiki    string `json:"has_wiki"`
}

type PullRequestsOptions struct {
	Id                  int      `json:"id"`
	Comment_id          string   `json:"comment_id"`
	Owner               string   `json:"owner"`
	Repo_slug           string   `json:"repo_slug"`
	Title               string   `json:"title"`
	Description         string   `json:"description"`
	Close_source_branch bool     `json:"close_source_branch"`
	Source_branch       string   `json:"source_branch"`
	Source_repository   string   `json:"source_repository"`
	Destination_branch  string   `json:"destination_branch"`
	Destination_commit  string   `json:"destination_repository"`
	Message             string   `json:"message"`
	Reviewers           []string `json:"reviewers"`
}

type CommitsOptions struct {
	Owner       string `json:"owner"`
	Repo_slug   string `json:"repo_slug"`
	Revision    string `json:"revision"`
	Branchortag string `json:"branchortag"`
	Include     string `json:"include"`
	Exclude     string `json:"exclude"`
	Comment_id  int    `json:"comment_id"`
}

type IssuesOptions struct {
	Owner      string `json:"owner"`
	Repo_slug  string `json:"repo_slug"`
	Issue_id   int    `json:"issue_id"`
	Comment_id int    `json:"issue_id"`
	State      string `json:"state"`
}

type BranchRestrictionsOptions struct {
	Owner     string            `json:"owner"`
	Repo_slug string            `json:"repo_slug"`
	Id        string            `json:"id"`
	Groups    map[string]string `json:"groups"`
	Pattern   string            `json:"pattern"`
	Users     []string          `json:"users"`
	Kind      string            `json:"kind"`
	Full_slug string            `json:"full_slug"`
	Name      string            `json:"name"`
}

type DiffOptions struct {
	Owner     string `json:"owner"`
	Repo_slug string `json:"repo_slug"`
	Spec      string `json:"spec"`
}

type WebhooksOptions struct {
	Owner       string   `json:"owner"`
	Repo_slug   string   `json:"repo_slug"`
	Uuid        string   `json:"uuid"`
	Description string   `json:"description"`
	Url         string   `json:"url"`
	Active      bool     `json:"active"`
	Events      []string `json:"events"` // EX) {'repo:push','issue:created',..} REF) https://goo.gl/VTj93b
}

type Comment struct {
	ID     int `json:"id"`
	Parent struct {
		ID int `json:"id"`
	} `json:"parent"`
	Content struct {
		Raw    string `json:"raw"`
		HTML   string `json:"html"`
		Markup string `json:"markup"`
	} `json:"content"`
	Inline struct {
		Path string      `json:"path"`
		From interface{} `json:"from"`
		To   int         `json:"to"`
	} `json:"inline"`
	CreatedOn *time.Time `json:"created_on"`
	UpdatedOn *time.Time `json:"updated_on"`
	Links     Links      `json:"links"`
}
