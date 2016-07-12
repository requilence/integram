package bitbucket

type DiffService struct {
	c *Client
}

func (d *DiffService) GetDiff(do *DiffOptions) error {
	url := d.c.requestUrl("/repositories/%s/%s/diff/%s", do.Owner, do.Repo_slug, do.Spec)
	return d.c.execute("GET", url, "", nil)
}

func (d *DiffService) GetPatch(do *DiffOptions) error {
	url := d.c.requestUrl("/repositories/%s/%s/patch/%s", do.Owner, do.Repo_slug, do.Spec)
	return d.c.execute("GET", url, "", nil)
}
