package bitbucket

import (
	"errors"
	"fmt"
	"strings"
	"time"

	api "github.com/ktrysmt/go-bitbucket"
	"github.com/requilence/integram"
	"golang.org/x/oauth2"
)

var m = integram.HTMLRichText{}

// Config contains OAuth data only
type Config struct {
	integram.OAuthProvider
}

// Service returns integram.Service from config
func (c Config) Service() *integram.Service {
	return &integram.Service{
		Name:                "bitbucket",
		NameToPrint:         "Bitbucket",
		TGNewMessageHandler: update,
		WebhookHandler:      webhookHandler,
		JobsPool:            1,
		Jobs:                []integram.Job{},

		Actions: []interface{}{
			issueInlineButtonPressed,
			prInlineButtonPressed,
		},
		DefaultOAuth2: &integram.DefaultOAuth2{
			Config: oauth2.Config{
				ClientID:     c.ID,
				ClientSecret: c.Secret,
				Endpoint: oauth2.Endpoint{
					AuthURL:  "https://bitbucket.org/site/oauth2/authorize",
					TokenURL: "https://bitbucket.org/site/oauth2/access_token",
				},
			},
		},
		OAuthSuccessful: oAuthSuccessful,
	}

}

func oAuthSuccessful(c *integram.Context) error {
	return c.NewMessage().SetText("Great! Now you can reply issues, commits, merge requests and snippets").Send()
}

func client(c *integram.Context) *api.Client {
	return api.NewWithHTTPClient(c.User.OAuthHTTPClient())
}

/*func me(c *integram.Context) (*gitlab.User, error) {
	api:=Api(c).Repositories.Commits.GetCommit()
	user := &gitlab.User{}

	c.User.Cache("me", user)
	if user.ID > 0 {
		return user, nil
	}

	user, _, err := Api(c).Users.CurrentUser()

	if err != nil {
		return nil, err
	}

	c.User.SetCache("me", user, time.Hour*24*30)

	return user, nil
}

func cacheNickMap(c *integram.Context) error {
	me, err := me(c)
	if err != nil {
		return err
	}
	c.SetServiceCache("nick_map_"+me.Username, c.User.UserName, time.Hour*24*365)
	err = c.SetServiceCache("nick_map_"+me.Email, c.User.UserName, time.Hour*24*365)
	return err
}

*/
func hostedAppSecretEntered(c *integram.Context, baseURL string, appID string) error {
	c.SetServiceBaseURL(baseURL)

	appSecret := strings.TrimSpace(c.Message.Text)
	if len(appSecret) != 64 {
		c.NewMessage().SetText("Looks like this *Application Secret* is incorrect. Must be a 64 HEX symbols. Please try again").EnableHTML().DisableWebPreview().SetReplyAction(hostedAppSecretEntered, baseURL).Send()
		return errors.New("Application Secret '" + appSecret + "' is incorrect")
	}
	conf := integram.OAuthProvider{BaseURL: c.ServiceBaseURL, ID: appID, Secret: appSecret}
	token, err := conf.OAuth2Client(c).Exchange(oauth2.NoContext, "-")

	if strings.Contains(err.Error(), `"error":"invalid_grant"`) {
		// means the app is exists
		c.SaveOAuthProvider(c.ServiceBaseURL, appID, appSecret)
		_, err := mustBeAuthed(c)

		return err
	}
	c.NewMessage().SetText("Application ID or Secret is incorrect. Please try again. Enter *Application Id*").
		EnableHTML().
		SetReplyAction(hostedAppIDEntered, baseURL).Send()

	fmt.Printf("Exchange: token: %+v, err:%v\n", token, err)

	return nil

}

func hostedAppIDEntered(c *integram.Context, baseURL string) error {
	c.SetServiceBaseURL(baseURL)

	appID := strings.TrimSpace(c.Message.Text)
	if len(appID) != 64 {
		c.NewMessage().SetText("Looks like this *Application Id* is incorrect. Must be a 64 HEX symbols. Please try again").
			EnableHTML().
			SetReplyAction(hostedAppIDEntered, baseURL).Send()
		return errors.New("Application Id '" + appID + "' is incorrect")
	}
	return c.NewMessage().SetText("Great! Now write me the *Secret* for this application").
		EnableHTML().
		SetReplyAction(hostedAppSecretEntered, baseURL, appID).Send()
}

func splitRepo(fullRepoName string) (owner string, repo string) {
	f := strings.Split(fullRepoName, "/")
	if len(f) == 2 {
		owner, repo = f[0], f[1]
	}
	return
}

func me(rest *api.Client, c *integram.Context) (actor *api.Actor, err error) {

	actor = &api.Actor{}
	if c.User.Cache("me", actor); actor.UUID != "" {
		return
	}

	actor, err = rest.User.Profile()
	if actor != nil {
		err = c.User.SetCache("me", actor, time.Hour*24*7)
	}
	return
}

func storeIssue(c *integram.Context, issue *api.Issue) error {
	return c.SetServiceCache(issueUniqueID(issue.Repository.FullName, issue.ID), &issue, time.Hour*24*365)
}

func prInlineButtonPressed(c *integram.Context, fullRepoName string, prID int) error {
	if ok, err := mustBeAuthed(c); !ok {
		return err
	}
	pr := api.PullRequest{}

	c.ServiceCache(issueUniqueID(fullRepoName, prID), &pr)

	switch c.Callback.Data {
	case "back":
		return c.EditPressedInlineKeyboard(prInlineKeyboard(&pr))
	case "assign":
		//rest.Repositories.ListPublic()
	}
	return nil

}

func issueInlineButtonPressed(c *integram.Context, fullRepoName string, issueID int) error {

	if ok, err := mustBeAuthed(c); !ok {
		return err
	}

	issue := api.Issue{}

	c.ServiceCache(issueUniqueID(fullRepoName, issueID), &issue)
	rest := client(c)

	fmt.Printf("issueID: %d\n", issueID)
	owner, repo := splitRepo(fullRepoName)

	if c.Callback.Message.InlineKeyboardMarkup.State == "status" {
		state := c.Callback.Data
		if state != "back" {
			err := rest.Repositories.Issues.SetState(&api.IssuesOptions{Owner: owner, Repo_slug: repo, Issue_id: issueID, State: state})
			if err != nil {
				return err
			}
			issue.State = state
			storeIssue(c, &issue)

			c.Callback.Data = "back"
		}
	}
	switch c.Callback.Data {
	case "back":
		return c.EditPressedInlineKeyboard(issueInlineKeyboard(&issue))
	/*case "assign":
	case "status":
		buttons := integram.InlineButtons{}
		for b, _ := range issueStates {
			if !strings.EqualFold(b, issue.State) {
				buttons.Append(b, strings.ToUpper(b[0:1])+b[1:])
			}
		}
		buttons.Append("back", "← Back")

		return c.EditPressedInlineKeyboard(buttons.Markup(1, "status"))
	*/
	case "vote":
		me, err := me(rest, c)

		if err != nil {
			return err
		}

		if integram.SliceContainsString(issue.VotedActorsUUID, me.UUID) {
			err = rest.Repositories.Issues.Unvote(&api.IssuesOptions{Owner: owner, Repo_slug: repo, Issue_id: issueID})
			if err != nil {
				return err
			}
			b := issue.VotedActorsUUID[:0]
			for _, x := range issue.VotedActorsUUID {
				if x != me.UUID {
					b = append(b, x)
				}
			}
			issue.VotedActorsUUID = b
			issue.Votes--

		} else {
			err = rest.Repositories.Issues.Vote(&api.IssuesOptions{Owner: owner, Repo_slug: repo, Issue_id: issueID})
			if err != nil {
				return err
			}
			issue.Votes++
			issue.VotedActorsUUID = append(issue.VotedActorsUUID, me.UUID)
		}
		storeIssue(c, &issue)

		kb := issueInlineKeyboard(&issue)
		err = c.EditPressedInlineKeyboard(kb)

		if err != nil {
			return err
		}
	}

	return nil

}

func mustBeAuthed(c *integram.Context) (bool, error) {

	provider := c.OAuthProvider()

	if !provider.IsSetup() {
		return false, c.NewMessage().SetText(fmt.Sprintf("To be able to use interactive replies in Telegram, first you need to add oauth application on your hosted Bitbucket instance (admin priveleges required): %s\nAdd application with any name(f.e. Telegram) and specify this *Redirect URI*: \n%s\n\nAfter you press *Submit* you will receive app info. First, send me the *Application Id*", c.ServiceBaseURL.String()+"/account/", provider.RedirectURL())).
			SetChat(c.User.ID).
			SetBackupChat(c.Chat.ID).
			EnableHTML().
			EnableForceReply().
			DisableWebPreview().
			SetReplyAction(hostedAppIDEntered, c.ServiceBaseURL.String()).Send()

	}
	if !c.User.OAuthValid() {
		if c.Callback != nil {
			c.AnswerCallbackQuery("You need to authorize me\nUse the \"Tap me to auth\" button", true)
		}
		if !c.User.IsPrivateStarted() && c.Callback != nil {
			kb := c.Callback.Message.InlineKeyboardMarkup
			kb.AddPMSwitchButton(c, "👉  Tap me to auth", "auth")
			c.EditPressedInlineKeyboard(kb)
		} else {
			return false, c.NewMessage().SetTextFmt("You need to authorize me to use interactive replies: %s", c.User.OauthInitURL()).
				DisableWebPreview().
				SetChat(c.User.ID).SetBackupChat(c.Chat.ID).Send()
		}
	}

	return true, nil

}

func mention(c *integram.Context, member *api.Actor) string {
	userName := ""
	c.ServiceCache("nick_map_"+member.UUID, &userName)
	if userName == "" {
		return m.Bold(member.DisplayName)
	}
	return "@" + userName
}

func update(c *integram.Context) error {

	command, param := c.Message.GetCommand()

	if c.Message.IsEventBotAddedToGroup() {
		command = "start"
	}

	if param == "silent" {
		command = ""
	}

	switch command {

	case "start":
		return c.NewMessage().EnableAntiFlood().EnableHTML().
			SetTextFmt("Hi here! To setup notifications for %s for your BitBucket repo, open Settings -> Webhooks and add this URL:\n%s\n%s", m.Bold("this chat"), m.Fixed(c.Chat.ServiceHookURL()), m.Bold("⚠️ Don't forget to add all triggers inside the \"Choose from a full list of triggers\" radio button!")).Send()

	case "cancel", "clean", "reset":
		return c.NewMessage().SetText("Clean").HideKeyboard().Send()

	}
	return nil
}
