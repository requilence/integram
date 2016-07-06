package gitlab

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"net/url"
	"errors"
	"net/http"
	"github.com/requilence/integram"
	m "github.com/requilence/integram/html"
	api "github.com/xanzy/go-gitlab"
	"golang.org/x/oauth2"
)

type Config struct {
	integram.OAuthProvider
}

const API_URI_SUFFIX = "/api/v3/"

//var service integram.Service

func (c Config) Service() *integram.Service {
	return &integram.Service{
		Name:                "gitlab",
		NameToPrint:         "GitLab",
		TGNewMessageHandler: Update,
		WebhookHandler:      WebhookHandler,
		JobsPool:            1,
		Jobs: []integram.Job{
			{sendIssueComment, 10, integram.JobRetryFibonacci},
			{sendSnippetComment, 10, integram.JobRetryFibonacci},
			{sendMRComment, 10, integram.JobRetryFibonacci},
			{sendCommitComment, 10, integram.JobRetryFibonacci},
			{cacheNickMap, 10, integram.JobRetryFibonacci},
		},

		Actions: []interface{}{
			hostedAppIdEntered,
			hostedAppSecretEntered,
			issueReplied,
			mrReplied,
			snippetReplied,
			commitReplied,
			commitToReplySelected,
			commitsReplied,
		},
		DefaultOAuth2: &integram.DefaultOAuth2{
			Config: oauth2.Config{
				ClientID:     c.ID,
				ClientSecret: c.Secret,
				Endpoint: oauth2.Endpoint{
					AuthURL:  "https://gitlab.com/oauth/authorize",
					TokenURL: "https://gitlab.com/oauth/token",
				},
			},
		},
		OAuthSuccessful: OAuthSuccessful,
	}

}
func OAuthSuccessful(c *integram.Context) error {
	c.Service().SheduleJob(cacheNickMap, 0, time.Now().Add(time.Second*5), c)
	return c.NewMessage().SetText("Great! Now you can reply issues, commits, merge requests and snippets").Send()
}

func me(c *integram.Context) (*api.User, error) {
	user := &api.User{}

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

type Repository struct {
	Name        string
	Url         string
	Description string
	Homepage    string
}
type Author struct {
	Name  string
	Email string
}

type User struct {
	Name       string
	Username   string
	Avatar_url string
}

type Commit struct {
	Id        string
	Message   string
	Timestamp time.Time
	Author    Author
	Url       string
	Added     []string
	Modified  []string
	Removed   []string
}
type Attributes struct {
	Id            int
	Title         string
	Note          string
	Noteable_type string
	Assignee_id   int
	Author_id     int
	Project_id    int
	Created_at    string
	Updated_at    string
	Commit_id     string
	Position      int
	Branch_name   string
	Description   string
	Milestone_id  int
	Noteable_id   int
	State         string
	Iid           int
	Url           string
	Action        string
}

type MergeRequest struct {
	Id            int
	Target_branch string
	Source_branch string
	Assignee_id   int
	Author_id     int
	State         string
	Title         string
	Merge_status  string
	Description   string
}

type Issue struct {
	Id    int
	Title string
	State string
	Iid   int
}
type Snippet struct {
	Id        int
	Title     string
	File_name string
}

type Webhook struct {
	Object_kind       string
	Ref               string
	Before            string
	User              User
	User_id           int
	User_name         string
	User_email        string
	User_avatar       string
	Object_attributes *Attributes
	//Project_id        int
	Repository    Repository
	Project_id    int
	Issue         *Issue
	Snippet       *Snippet
	After         string
	Commits       []Commit
	Commit        *Commit
	Merge_request *MergeRequest
}

func Mention(c *integram.Context, name string, email string) string {
	userName := ""
	c.ServiceCache("nick_map_"+name, &userName)
	if userName == "" {
		c.ServiceCache("nick_map_"+email, &userName)
	}
	if userName == "" {
		return m.Bold(name)
	}
	return "@" + userName
}

func compareURL(home string, before string, after string) string {
	return home + "/compare/" + before + "..." + after
}

func mrMessageID(c *integram.Context, mergeRequestID int) int {
	msg, err := c.FindMessageByEventID("mr_" + strconv.Itoa(mergeRequestID))

	if err == nil && msg != nil {
		return msg.MsgID
	}
	return 0
}

func commitMessageID(c *integram.Context, commitId string) int {
	msg, err := c.FindMessageByEventID("commit_" + commitId)

	if err == nil && msg != nil {
		return msg.MsgID
	}
	return 0
}

func issueMessageID(c *integram.Context, issueID int) int {
	msg, err := c.FindMessageByEventID("issue_" + strconv.Itoa(issueID))

	if err == nil && msg != nil {
		return msg.MsgID
	}
	return 0
}

func snippetMessageID(c *integram.Context, snippetID int) int {
	msg, err := c.FindMessageByEventID("snippet_" + strconv.Itoa(snippetID))

	if err == nil && msg != nil {
		return msg.MsgID
	}
	return 0
}

func hostedAppSecretEntered(c *integram.Context, baseURL string, appId string) error {
	c.SetServiceBaseURL(baseURL)

	appSecret := strings.TrimSpace(c.Message.Text)
	if len(appSecret) != 64 {
		c.NewMessage().SetText("Looks like this *Application Secret* is incorrect. Must be a 64 HEX symbols. Please try again").EnableHTML().DisableWebPreview().SetReplyAction(hostedAppSecretEntered, baseURL).Send()
		return errors.New("Application Secret '" + appSecret + "' is incorrect")
	}
	conf := integram.OAuthProvider{BaseURL: c.ServiceBaseURL, ID: appId, Secret: appSecret}
	_, err := conf.OAuth2Client(c).Exchange(oauth2.NoContext, "-")

	if strings.Contains(err.Error(), `"error":"invalid_grant"`) {
		// means the app is exists
		c.SaveOAuthProvider(c.ServiceBaseURL, appId, appSecret)
		_, err := mustBeAuthed(c)

		return err
	} else {
		c.NewMessage().SetText("Application ID or Secret is incorrect. Please try again. Enter *Application Id*").
			EnableHTML().
			SetReplyAction(hostedAppIdEntered, baseURL).Send()
	}

	return nil

}

func hostedAppIdEntered(c *integram.Context, baseURL string) error {
	c.SetServiceBaseURL(baseURL)

	appId := strings.TrimSpace(c.Message.Text)
	if len(appId) != 64 {
		c.NewMessage().SetText("Looks like this *Application Id* is incorrect. Must be a 64 HEX symbols. Please try again").
			EnableHTML().
			SetReplyAction(hostedAppIdEntered, baseURL).Send()
		return errors.New("Application Id '" + appId + "' is incorrect")
	}
	return c.NewMessage().SetText("Great! Now write me the *Secret* for this application").
		EnableHTML().
		SetReplyAction(hostedAppSecretEntered, baseURL, appId).Send()
}

func mustBeAuthed(c *integram.Context) (bool, error) {

	provider := c.OAuthProvider()

	if !provider.IsSetup() {
		return false, c.NewMessage().SetText(fmt.Sprintf("To be able to use interactive replies in Telegram, first you need to add oauth application on your hosted GitLab instance (admin priveleges required): %s\nAdd application with any name(f.e. Telegram) and specify this *Redirect URI*: \n%s\n\nAfter you press *Submit* you will receive app info. First, send me the *Application Id*", c.ServiceBaseURL.String()+"/admin/applications/new", provider.RedirectURL())).
			SetChat(c.User.ID).
			SetBackupChat(c.Chat.ID).
			EnableHTML().
			EnableForceReply().
			DisableWebPreview().
			SetReplyAction(hostedAppIdEntered, c.ServiceBaseURL.String()).Send()

	}
	if !c.User.OAuthValid() {
		return false, c.NewMessage().SetTextFmt("You need to authorize me to use interactive replies: %s", c.User.OauthInitURL()).
			DisableWebPreview().
			SetChat(c.User.ID).SetBackupChat(c.Chat.ID).Send()
	}

	return true, nil

}
func noteUniqueID(projectId int, noteId string) string {
	return "note_" + strconv.Itoa(projectId) + "_" + noteId
}

func getDomainFromUrl(s string) (string, error) {

	u, err := url.Parse(s)
	if err != nil {
		return "", err
	}
	return u.Host, nil
}

func Api(c *integram.Context) *api.Client {

	client := api.NewClient(c.User.OAuthHttpClient(), "")
	client.SetBaseURL(c.ServiceBaseURL.String() + API_URI_SUFFIX)
	return client
}

func sendIssueComment(c *integram.Context, projectID int, issueID int, text string) error {
	note, _, err := Api(c).Notes.CreateIssueNote(projectID, issueID, &api.CreateIssueNoteOptions{Body: text})

	if note != nil {
		c.Message.UpdateEventsID(c.Db(), "issue_note_"+strconv.Itoa(note.ID))
	}

	return err
}

func sendMRComment(c *integram.Context, projectID int, MergeRequestID int, text string) error {
	note, _, err := Api(c).Notes.CreateMergeRequestNote(projectID, MergeRequestID, &api.CreateMergeRequestNoteOptions{Body: text})

	if note != nil {
		c.Message.UpdateEventsID(c.Db(), noteUniqueID(projectID, strconv.Itoa(note.ID)))
	}

	return err
}

func sendSnippetComment(c *integram.Context, projectID int, SnippetID int, text string) error {
	note, _, err := Api(c).Notes.CreateSnippetNote(projectID, SnippetID, &api.CreateSnippetNoteOptions{Body: text})
	if note != nil {
		c.Message.UpdateEventsID(c.Db(), noteUniqueID(projectID, strconv.Itoa(note.ID)))
	}

	return err
}

func trim(s string, max int) string {
	if len(s) > max {
		return s[:max] + "…"
	} else {
		return s
	}
}

func sendCommitComment(c *integram.Context, projectID int, commitID string, msg *integram.IncomingMessage) error {
	note, _, err := Api(c).Notes.CreateCommitNote(projectID, commitID, &api.CreateCommitNoteOptions{Note: msg.Text})
	if err != nil {
		return err
	}
	// note id not availble for commit comment. So use the date. Collisions are unlikely here...
	c.Message.UpdateEventsID(c.Db(), noteUniqueID(projectID, note.CreatedAt))

	return err
}
func commitsReplied(c *integram.Context, baseURL string, projectID int, commits []Commit) error {
	authorized, err := mustBeAuthed(c)

	if err != nil {
		return err
	}

	if !authorized {
		//todo:bug message lost
		return c.User.SetAfterAuthAction(commitsReplied, baseURL, projectID, commits)
	} else {
		buttons := integram.Buttons{}

		for _, commit := range commits {
			buttons.Append(commit.Id, trim(commit.Message, 40))
		}

		return c.NewMessage().
			SetText(c.User.Mention()+" please specify commit to comment").
			SetKeyboard(buttons.Markup(1), true).
			EnableForceReply().
			SetReplyAction(commitToReplySelected, baseURL, projectID, c.Message).
			Send()
	}
}

// we nee msg param because action c.Message can contains selected commit id from prev state at commitsReplied and not the comment message
func commitToReplySelected(c *integram.Context, baseURL string, projectID int, msg *integram.IncomingMessage) error {

	commitID, _ := c.KeyboardAnswer()

	c.Message.SetReplyAction(commitReplied, baseURL, projectID, commitID)
	c.SetServiceBaseURL(baseURL)

	authorized, err := mustBeAuthed(c)
	if !authorized {
		return c.User.SetAfterAuthAction(sendCommitComment, c, projectID, commitID, msg.Text)
	} else {
		c.Service().DoJob(sendCommitComment, projectID, commitID)
	}
	return err
}

func commitReplied(c *integram.Context, baseURL string, projectID int, commitID string) error {
	c.Message.SetReplyAction(commitReplied, baseURL, projectID, commitID)
	c.SetServiceBaseURL(baseURL)

	authorized, err := mustBeAuthed(c)
	if !authorized {
		return c.User.SetAfterAuthAction(sendCommitComment, c, projectID, commitID, c.Message)
	} else {

		c.Service().DoJob(sendCommitComment, c, projectID, commitID, c.Message)
	}
	return err
}

func issueReplied(c *integram.Context, baseURL string, projectID int, issueID int) error {
	c.SetServiceBaseURL(baseURL)

	authorized, err := mustBeAuthed(c)
	if !authorized {
		c.User.SetAfterAuthAction(issueReplied, baseURL, projectID, issueID)
	} else {
		_, err = c.Service().DoJob(sendIssueComment, c, projectID, issueID, c.Message.Text)
	}

	c.Message.SetReplyAction(issueReplied, baseURL, projectID, issueID)
	return err
}

func mrReplied(c *integram.Context, baseURL string, projectID int, mergeRequestID int) error {
	c.SetServiceBaseURL(baseURL)

	authorized, err := mustBeAuthed(c)
	if !authorized {
		c.User.SetAfterAuthAction(mrReplied, baseURL, projectID, mergeRequestID)
	} else {
		_, err = c.Service().DoJob(sendMRComment, c, baseURL, projectID, mergeRequestID, c.Message.Text)
	}

	c.Message.SetReplyAction(mrReplied, baseURL, projectID, mergeRequestID)
	return err
}

func snippetReplied(c *integram.Context, baseURL string, projectID int, snippetID int) error {
	c.SetServiceBaseURL(baseURL)

	authorized, err := mustBeAuthed(c)
	if !authorized {
		c.User.SetAfterAuthAction(sendSnippetComment, baseURL, projectID, snippetID, c.Message.Text)
	} else {
		_, err = c.Service().DoJob(sendSnippetComment, c, baseURL, projectID, snippetID, c.Message.Text)
	}

	c.Message.SetReplyAction(mrReplied, baseURL, projectID, snippetID)
	return err
}

func WebhookHandler(c *integram.Context, request *integram.WebhookContext) (err error) {
	wh := &Webhook{}

	err=request.JSON(wh)

	if err!=nil{
		return
	}

	msg := c.NewMessage()

	if wh.Repository.Homepage != "" {
		c.SetServiceBaseURL(wh.Repository.Homepage)
	} else if wh.Object_attributes != nil {
		c.SetServiceBaseURL(wh.Object_attributes.Url)
	}

	switch wh.Object_kind {
	case "push":
		s := strings.Split(wh.Ref, "/")
		branch := s[len(s)-1]
		text := ""

		added := 0
		removed := 0
		modified := 0
		anyOherPersonCommits := false
		for _, commit := range wh.Commits {
			if commit.Author.Email != wh.User_email && commit.Author.Name != wh.User_name {
				anyOherPersonCommits = true
			}
		}
		for _, commit := range wh.Commits {

			commit.Message = strings.TrimSuffix(commit.Message, "\n")
			if anyOherPersonCommits {
				text += Mention(c, commit.Author.Name, commit.Author.Email) + ": "
			}
			text += m.URL(commit.Message, commit.Url) + "\n"
			added += len(commit.Added)
			removed += len(commit.Removed)
			modified += len(commit.Modified)
		}
		f := ""
		if modified > 0 {
			f += strconv.Itoa(modified) + " files modified"
		}

		if added > 0 {
			if f == "" {
				f += strconv.Itoa(added) + " files added"
			} else {
				f += " " + strconv.Itoa(added) + " added"
			}
		}

		if removed > 0 {
			if f == "" {
				f += strconv.Itoa(removed) + " files removed"
			} else {
				f += " " + strconv.Itoa(removed) + " removed"
			}
		}
		wp := ""
		if len(wh.Commits) > 1 {
			wp = c.WebPreview(fmt.Sprintf("%d commits", len(wh.Commits)), "@"+wh.Before[0:10]+" ... @"+wh.After[0:10], f, compareURL(wh.Repository.Homepage, wh.Before, wh.After), "")
		} else if len(wh.Commits) == 1 {
			wp = c.WebPreview("Commit", "@"+wh.After[0:10], f, wh.Commits[0].Url, "")
		}

		var err error

		if len(wh.Commits) > 0 {
			if len(wh.Commits) == 1 {
				msg.SetReplyAction(commitReplied, c.ServiceBaseURL.String(), wh.Project_id, wh.Commits[0].Id)
			} else {
				msg.SetReplyAction(commitsReplied, c.ServiceBaseURL.String(), wh.Project_id, wh.Commits)
			}
			_, err = msg.AddEventID("commit_" + wh.Commits[0].Id).SetText(fmt.Sprintf("%s %s to %s\n%s", wh.User_name, m.URL("pushed", wp), m.URL(wh.Repository.Name+"/"+branch, wh.Repository.Homepage+"/tree/"+url.QueryEscape(branch)), text)).
				EnableHTML().
				SendAndGetID()

		} else {
			_, err = msg.SetText(fmt.Sprintf("%s created branch %s\n%s", wh.User_name, m.URL(wh.Repository.Name+"/"+branch, wh.Repository.Homepage+"/tree/"+url.QueryEscape(branch)), text)).
				EnableHTML().
				SendAndGetID()
		}

		return err
	case "tag_push":
		s := strings.Split(wh.Ref, "/")
		itemType := s[len(s)-2]
		if itemType == "tags" {
			itemType = "tag"
		} else if itemType == "heads" {
			itemType = "branch"
		}

		return msg.SetText(fmt.Sprintf("%s pushed new %s at %s", Mention(c, wh.User_name, wh.User_email), m.URL(itemType+" "+s[len(s)-1], wh.Repository.Homepage+"/tree/"+s[len(s)-1]), m.URL(wh.User_name+" / "+wh.Repository.Name, wh.Repository.Homepage))).
			EnableHTML().DisableWebPreview().Send()
	case "issue":
		if wh.Object_attributes.Milestone_id > 0 {
			// Todo: need an API access to fetch milestones
		}

		msg.SetReplyAction(issueReplied, c.ServiceBaseURL.String(), wh.Object_attributes.Project_id, wh.Object_attributes.Id)

		if wh.Object_attributes.Action == "open" {
			err := msg.AddEventID("issue_" + strconv.Itoa(wh.Object_attributes.Id)).SetText(fmt.Sprintf("%s %s %s at %s:\n%s\n%s", Mention(c, wh.User.Username, wh.User_email), wh.Object_attributes.State, m.URL("issue", wh.Object_attributes.Url), m.URL(wh.User.Username+" / "+wh.Repository.Name, wh.Repository.Homepage), m.Bold(wh.Object_attributes.Title), wh.Object_attributes.Description)).
				EnableHTML().DisableWebPreview().Send()

			return err
		} else {
			action := "updated"
			if wh.Object_attributes.Action == "reopen" {
				action = "reopened"
			} else if wh.Object_attributes.Action == "close" {
				action = "closed"
			}

			id := issueMessageID(c, wh.Object_attributes.Id)

			if id > 0 {
				// reply to existing message
				return msg.SetText(fmt.Sprintf("%s by %s", m.Bold(action), Mention(c, wh.User.Username, ""))).
					EnableHTML().DisableWebPreview().SetReplyToMsgID(id).Send()
			} else {
				// original message not found. Send WebPreview
				wp := c.WebPreview("Issue", wh.Object_attributes.Title, wh.User.Username+" / "+wh.Repository.Name, wh.Object_attributes.Url, "")
				return msg.SetText(fmt.Sprintf("%s by %s", m.URL(action, wp), Mention(c, wh.User.Username, ""))).Send()
			}
		}

	case "note":
		wp := ""
		noteType := ""
		originMsg := &integram.Message{}
		noteID := strconv.Itoa(wh.Object_attributes.Id)
		if wh.Object_attributes.Note == "Commit" {
			// collisions by date are unlikely here
			noteID = wh.Object_attributes.Created_at
		}
		if msg, _ := c.FindMessageByEventID(noteUniqueID(wh.Object_attributes.Project_id, noteID)); msg != nil {
			return nil
		}

		switch wh.Object_attributes.Note {
		case "Commit":
			noteType = "commit"
			originMsg, _ = c.FindMessageByEventID(fmt.Sprintf("commit_%d", wh.Commit.Id))
			if originMsg != nil {
				break
			}
			wp = c.WebPreview("Commit", "@"+wh.Commit.Id[0:10], wh.User.Username+" / "+wh.Repository.Name, wh.Object_attributes.Url, "")
		case "MergeRequest":
			noteType = "merge request"
			originMsg, _ = c.FindMessageByEventID(fmt.Sprintf("mr_%d", wh.Merge_request.Id))
			if originMsg != nil {
				break
			}
			wp = c.WebPreview("Merge Request", wh.Merge_request.Title, wh.User.Username+" / "+wh.Repository.Name, wh.Object_attributes.Url, "")
		case "Issue":
			noteType = "issue"
			originMsg, _ = c.FindMessageByEventID(fmt.Sprintf("issue_%d", wh.Issue.Id))
			if originMsg != nil {
				break
			}
			wp = c.WebPreview("Issue", wh.Issue.Title, wh.User.Username+" / "+wh.Repository.Name, wh.Object_attributes.Url, "")
		case "Snippet":
			noteType = "snippet"
			originMsg, _ = c.FindMessageByEventID(fmt.Sprintf("snippet_%d", wh.Snippet.Id))
			if originMsg != nil {
				break
			}
			wp = c.WebPreview("Snippet", wh.Snippet.Title, wh.User.Username+" / "+wh.Repository.Name, wh.Object_attributes.Url, "")
		}

		if originMsg == nil {
			if wp == "" {
				wp = wh.Object_attributes.Url
			}

			if noteType == "" {
				noteType = strings.ToLower(wh.Object_attributes.Noteable_type)
			}

			return msg.SetTextFmt("%s commented on %s: %s", Mention(c, wh.User.Username, ""), m.URL(strings.ToLower(wh.Object_attributes.Noteable_type), wp), wh.Object_attributes.Note).
				EnableHTML().
				Send()
		} else {
			return msg.SetText(fmt.Sprintf("%s: %s", Mention(c, wh.User.Username, ""), wh.Object_attributes.Note)).
				DisableWebPreview().EnableHTML().SetReplyToMsgID(originMsg.MsgID).Send()
		}

	case "merge_request":

		if wh.Object_attributes.Action == "open" {
			if wh.Object_attributes.Description != "" {
				wh.Object_attributes.Description = "\n" + wh.Object_attributes.Description
			}

			err := msg.AddEventID("mr_" + strconv.Itoa(wh.Object_attributes.Id)).SetText(fmt.Sprintf("%s %s %s at %s:\n%s%s", Mention(c, wh.User.Username, wh.User_email), wh.Object_attributes.State, m.URL("merge request", wh.Object_attributes.Url), m.URL(wh.User_name+" / "+wh.Repository.Name, wh.Repository.Homepage), m.Bold(wh.Object_attributes.Title), wh.Object_attributes.Description)).
				EnableHTML().DisableWebPreview().Send()

			return err
		}

	default:

		return
		break
	}
	return
}

func Update(c *integram.Context) error {

	command, param := c.Message.GetCommand()

	if c.Message.IsEventBotAddedToGroup() {
		command = "start"
	}
	if param == "silent" {
		command = ""
	}

	switch command {

	case "start":
		c.Log().Debug("hm")
		return c.NewMessage().EnableAntiFlood().EnableHTML().
			SetText("Hi here! To setup notifications " + m.Bold("for this chat") + " your GitLab project(repo), open Settings -> Web Hooks and add this URL:\n" + m.Fixed(c.Chat.ServiceHookURL())).EnableHTML().Send()

	case "cancel", "clean", "reset":
		return c.NewMessage().SetText("Clean").HideKeyboard().Send()
	}
	return nil
}
