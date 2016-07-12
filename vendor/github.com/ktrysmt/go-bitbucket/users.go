package bitbucket

type UsersService struct {
	c *Client
}

func (u *UsersService) Get(t string) (user *Actor, err error) {

	url := u.c.BaseURL + "/users/" + t + "/"
	err = u.c.execute("GET", url, "", user)
	return
}

func (c *Client) Get(t string) (user *Actor, err error) {

	url := c.BaseURL + "/users/" + t + "/"
	err = c.execute("GET", url, "", user)
	return
}

func (u *UsersService) Followers(t string) (users []*Actor, err error) {

	url := u.c.BaseURL + "/users/" + t + "/followers"
	err = u.c.execute("GET", url, "", users)
	return
}

func (u *UsersService) Following(t string) (users []*Actor, err error) {

	url := u.c.BaseURL + "/users/" + t + "/following"
	err = u.c.execute("GET", url, "", users)
	return
}
func (u *UsersService) Repositories(t string) (repos []*Repository, err error) {

	url := u.c.BaseURL + "/users/" + t + "/repositories"
	err = u.c.execute("GET", url, "", repos)
	return
}
