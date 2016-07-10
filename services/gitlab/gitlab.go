package gitlab

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/requilence/integram"
	api "github.com/xanzy/go-gitlab"
	"golang.org/x/oauth2"
)

var m = integram.HTMLRichText{}

//Config contains OAuth data only
type Config struct {
	integram.OAuthProvider
}

const apiSuffixURL = "/api/v3/"

// Service returns integram.Service from gitlab.Config
func (c Config) Service() *integram.Service {
	return &integram.Service{
		Name:                "gitlab",
		NameToPrint:         "GitLab",
		TGNewMessageHandler: update,
		WebhookHandler:      webhookHandler,
		JobsPool:            1,
		Jobs: []integram.Job{
			{sendIssueComment, 10, integram.JobRetryFibonacci},
			{sendSnippetComment, 10, integram.JobRetryFibonacci},
			{sendMRComment, 10, integram.JobRetryFibonacci},
			{sendCommitComment, 10, integram.JobRetryFibonacci},
			{cacheNickMap, 10, integram.JobRetryFibonacci},
		},

		Actions: []interface{}{
			hostedAppIDEntered,
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
		OAuthSuccessful: oAuthSuccessful,
	}

}

func oAuthSuccessful(c *integram.Context) error {
	c.Service().SheduleJob(cacheNickMap, 0, time.Now().Add(time.Second*5), c)
	return c.NewMessage().SetText("Great! Now you can reply issues, commits, merge requests and snippets").Send()
}

func me(c *integram.Context) (*api.User, error) {
	user := &api.User{}

	c.User.Cache("me", user)
	if user.ID > 0 {
		return user, nil
	}

	user, _, err := client(c).Users.CurrentUser()

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

type repository struct {
	Name        string
	URL         string `json:"url"`
	Description string
	Homepage    string
}

type author struct {
	Name  string
	Email string
}

type user struct {
	Name      string
	Username  string
	AvatarURL string `json:"avatar_url"`
}

type commit struct {
	ID        string `json:"id"`
	Message   string
	Timestamp time.Time
	Author    author
	URL       string `json:"url"`
	Added     []string
	Modified  []string
	Removed   []string
}

type attributes struct {
	ID           int `json:"id"`
	Title        string
	Note         string
	NoteableType string `json:"noteable_type"`
	AssigneeID   int    `json:"assignee_id"`
	AuthorID     int    `json:"author_id"`
	ProjectID    int    `json:"project_id"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
	CommitID     string `json:"commit_id"`
	Position     int
	BranchName   string `json:"branch_name"`
	Description  string
	MilestoneID  int `json:"milestone_id"`
	NoteableID   int `json:"noteable_id"`
	State        string
	Iid          int
	URL          string
	Action       string
}

type mergeRequest struct {
	ID           int    `json:"id"`
	TargetBranch string `json:"target_branch"`
	SourceBranch string `json:"source_branch"`
	AssigneeID   int    `json:"assignee_id"`
	AuthorID     int    `json:"author_id"`
	State        string
	Title        string
	MergeStatus  string `json:"merge_status"`
	Description  string
}

type issue struct {
	ID    int `json:"id"`
	Title string
	State string
	Iid   int
}

type snippet struct {
	ID       int `json:"id"`
	Title    string
	FileName string `json:"file_name"`
}

type webhook struct {
	ObjectKind       string `json:"object_kind"`
	Ref              string
	Before           string
	User             user
	UserID           int         `json:"user_id"`
	UserName         string      `json:"user_name"`
	UserEmail        string      `json:"user_email"`
	UserAvatar       string      `json:"user_avatar"`
	ObjectAttributes *attributes `json:"object_attributes"`
	//ProjectID        int
	Repository   repository
	ProjectID    int `json:"project_id"`
	Issue        *issue
	Snippet      *snippet
	After        string
	Commits      []commit
	Commit       *commit
	MergeRequest *mergeRequest `json:"merge_request"`
}

func mention(c *integram.Context, name string, email string) string {
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

func commitMessageID(c *integram.Context, commitID string) int {
	msg, err := c.FindMessageByEventID("commit_" + commitID)

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

func hostedAppSecretEntered(c *integram.Context, baseURL string, appID string) error {
	c.SetServiceBaseURL(baseURL)

	appSecret := strings.TrimSpace(c.Message.Text)
	if len(appSecret) != 64 {
		c.NewMessage().SetText("Looks like this *Application Secret* is incorrect. Must be a 64 HEX symbols. Please try again").EnableHTML().DisableWebPreview().SetReplyAction(hostedAppSecretEntered, baseURL).Send()
		return errors.New("Application Secret '" + appSecret + "' is incorrect")
	}
	conf := integram.OAuthProvider{BaseURL: c.ServiceBaseURL, ID: appID, Secret: appSecret}
	_, err := conf.OAuth2Client(c).Exchange(oauth2.NoContext, "-")

	if strings.Contains(err.Error(), `"error":"invalid_grant"`) {
		// means the app is exists
		c.SaveOAuthProvider(c.ServiceBaseURL, appID, appSecret)
		_, err := mustBeAuthed(c)

		return err
	}
	return c.NewMessage().SetText("Application ID or Secret is incorrect. Please try again. Enter *Application ID*").
		EnableHTML().
		SetReplyAction(hostedAppIDEntered, baseURL).Send()
}

func hostedAppIDEntered(c *integram.Context, baseURL string) error {
	c.SetServiceBaseURL(baseURL)

	appID := strings.TrimSpace(c.Message.Text)
	if len(appID) != 64 {
		c.NewMessage().SetText("Looks like this *Application ID* is incorrect. Must be a 64 HEX symbols. Please try again").
			EnableHTML().
			SetReplyAction(hostedAppIDEntered, baseURL).Send()
		return errors.New("Application ID '" + appID + "' is incorrect")
	}
	return c.NewMessage().SetText("Great! Now write me the *Secret* for this application").
		EnableHTML().
		SetReplyAction(hostedAppSecretEntered, baseURL, appID).Send()
}

func mustBeAuthed(c *integram.Context) (bool, error) {

	provider := c.OAuthProvider()

	if !provider.IsSetup() {
		return false, c.NewMessage().SetText(fmt.Sprintf("To be able to use interactive replies in Telegram, first you need to add oauth application on your hosted GitLab instance (admin priveleges required): %s\nAdd application with any name(f.e. Telegram) and specify this *Redirect URI*: \n%s\n\nAfter you press *Submit* you will receive app info. First, send me the *Application ID*", c.ServiceBaseURL.String()+"/admin/applications/new", provider.RedirectURL())).
			SetChat(c.User.ID).
			SetBackupChat(c.Chat.ID).
			EnableHTML().
			EnableForceReply().
			DisableWebPreview().
			SetReplyAction(hostedAppIDEntered, c.ServiceBaseURL.String()).Send()

	}
	if !c.User.OAuthValid() {
		return false, c.NewMessage().SetTextFmt("You need to authorize me to use interactive replies: %s", c.User.OauthInitURL()).
			DisableWebPreview().
			SetChat(c.User.ID).SetBackupChat(c.Chat.ID).Send()
	}

	return true, nil

}

func noteUniqueID(projectID int, noteID string) string {
	return "note_" + strconv.Itoa(projectID) + "_" + noteID
}

func client(c *integram.Context) *api.Client {

	client := api.NewClient(c.User.OAuthHTTPClient(), "")
	client.SetBaseURL(c.ServiceBaseURL.String() + apiSuffixURL)
	return client
}

func sendIssueComment(c *integram.Context, projectID int, issueID int, text string) error {
	note, _, err := client(c).Notes.CreateIssueNote(projectID, issueID, &api.CreateIssueNoteOptions{Body: text})

	if note != nil {
		c.Message.UpdateEventsID(c.Db(), "issue_note_"+strconv.Itoa(note.ID))
	}

	return err
}

func sendMRComment(c *integram.Context, projectID int, MergeRequestID int, text string) error {
	note, _, err := client(c).Notes.CreateMergeRequestNote(projectID, MergeRequestID, &api.CreateMergeRequestNoteOptions{Body: text})

	if note != nil {
		c.Message.UpdateEventsID(c.Db(), noteUniqueID(projectID, strconv.Itoa(note.ID)))
	}

	return err
}

func sendSnippetComment(c *integram.Context, projectID int, SnippetID int, text string) error {
	note, _, err := client(c).Notes.CreateSnippetNote(projectID, SnippetID, &api.CreateSnippetNoteOptions{Body: text})
	if note != nil {
		c.Message.UpdateEventsID(c.Db(), noteUniqueID(projectID, strconv.Itoa(note.ID)))
	}

	return err
}

func trim(s string, max int) string {
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}

func sendCommitComment(c *integram.Context, projectID int, commitID string, msg *integram.IncomingMessage) error {
	note, _, err := client(c).Notes.CreateCommitNote(projectID, commitID, &api.CreateCommitNoteOptions{Note: msg.Text})
	if err != nil {
		return err
	}
	// note id not availble for commit comment. So use the date. Collisions are unlikely here...
	c.Message.UpdateEventsID(c.Db(), noteUniqueID(projectID, note.CreatedAt))

	return err
}
func commitsReplied(c *integram.Context, baseURL string, projectID int, commits []commit) error {
	authorized, err := mustBeAuthed(c)

	if err != nil {
		return err
	}

	if !authorized {
		//todo:bug message lost
		return c.User.SetAfterAuthAction(commitsReplied, baseURL, projectID, commits)
	}
	buttons := integram.Buttons{}

	for _, commit := range commits {
		buttons.Append(commit.ID, trim(commit.Message, 40))
	}

	return c.NewMessage().
		SetText(c.User.Mention()+" please specify commit to comment").
		SetKeyboard(buttons.Markup(1), true).
		EnableForceReply().
		SetReplyAction(commitToReplySelected, baseURL, projectID, c.Message).
		Send()

}

// we nee msg param because action c.Message can contains selected commit id from prev state at commitsReplied and not the comment message
func commitToReplySelected(c *integram.Context, baseURL string, projectID int, msg *integram.IncomingMessage) error {

	commitID, _ := c.KeyboardAnswer()

	c.Message.SetReplyAction(commitReplied, baseURL, projectID, commitID)
	c.SetServiceBaseURL(baseURL)

	authorized, err := mustBeAuthed(c)
	if !authorized {
		return c.User.SetAfterAuthAction(sendCommitComment, c, projectID, commitID, msg.Text)
	}

	c.Service().DoJob(sendCommitComment, projectID, commitID)

	return err
}

func commitReplied(c *integram.Context, baseURL string, projectID int, commitID string) error {
	c.Message.SetReplyAction(commitReplied, baseURL, projectID, commitID)
	c.SetServiceBaseURL(baseURL)

	authorized, err := mustBeAuthed(c)
	if !authorized {
		return c.User.SetAfterAuthAction(sendCommitComment, c, projectID, commitID, c.Message)
	}
	c.Service().DoJob(sendCommitComment, c, projectID, commitID, c.Message)
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

func webhookHandler(c *integram.Context, request *integram.WebhookContext) (err error) {
	wh := &webhook{}

	err = request.JSON(wh)

	if err != nil {
		return
	}

	msg := c.NewMessage()

	if wh.Repository.Homepage != "" {
		c.SetServiceBaseURL(wh.Repository.Homepage)
	} else if wh.ObjectAttributes != nil {
		c.SetServiceBaseURL(wh.ObjectAttributes.URL)
	}

	switch wh.ObjectKind {
	case "push":
		s := strings.Split(wh.Ref, "/")
		branch := s[len(s)-1]
		text := ""

		added := 0
		removed := 0
		modified := 0
		anyOherPersonCommits := false
		for _, commit := range wh.Commits {
			if commit.Author.Email != wh.UserEmail && commit.Author.Name != wh.UserName {
				anyOherPersonCommits = true
			}
		}
		for _, commit := range wh.Commits {

			commit.Message = strings.TrimSuffix(commit.Message, "\n")
			if anyOherPersonCommits {
				text += mention(c, commit.Author.Name, commit.Author.Email) + ": "
			}
			text += m.URL(commit.Message, commit.URL) + "\n"
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
			wp = c.WebPreview("Commit", "@"+wh.After[0:10], f, wh.Commits[0].URL, "")
		}

		var err error

		if len(wh.Commits) > 0 {
			if len(wh.Commits) == 1 {
				msg.SetReplyAction(commitReplied, c.ServiceBaseURL.String(), wh.ProjectID, wh.Commits[0].ID)
			} else {
				msg.SetReplyAction(commitsReplied, c.ServiceBaseURL.String(), wh.ProjectID, wh.Commits)
			}
			_, err = msg.AddEventID("commit_" + wh.Commits[0].ID).SetText(fmt.Sprintf("%s %s to %s\n%s", wh.UserName, m.URL("pushed", wp), m.URL(wh.Repository.Name+"/"+branch, wh.Repository.Homepage+"/tree/"+url.QueryEscape(branch)), text)).
				EnableHTML().
				SendAndGetID()

		} else {
			_, err = msg.SetText(fmt.Sprintf("%s created branch %s\n%s", wh.UserName, m.URL(wh.Repository.Name+"/"+branch, wh.Repository.Homepage+"/tree/"+url.QueryEscape(branch)), text)).
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

		return msg.SetText(fmt.Sprintf("%s pushed new %s at %s", mention(c, wh.UserName, wh.UserEmail), m.URL(itemType+" "+s[len(s)-1], wh.Repository.Homepage+"/tree/"+s[len(s)-1]), m.URL(wh.UserName+" / "+wh.Repository.Name, wh.Repository.Homepage))).
			EnableHTML().DisableWebPreview().Send()
	case "issue":
		if wh.ObjectAttributes.MilestoneID > 0 {
			// Todo: need an API access to fetch milestones
		}

		msg.SetReplyAction(issueReplied, c.ServiceBaseURL.String(), wh.ObjectAttributes.ProjectID, wh.ObjectAttributes.ID)

		if wh.ObjectAttributes.Action == "open" {
			err := msg.AddEventID("issue_" + strconv.Itoa(wh.ObjectAttributes.ID)).SetText(fmt.Sprintf("%s %s %s at %s:\n%s\n%s", mention(c, wh.User.Username, wh.UserEmail), wh.ObjectAttributes.State, m.URL("issue", wh.ObjectAttributes.URL), m.URL(wh.User.Username+" / "+wh.Repository.Name, wh.Repository.Homepage), m.Bold(wh.ObjectAttributes.Title), wh.ObjectAttributes.Description)).
				EnableHTML().DisableWebPreview().Send()

			return err
		}
		action := "updated"
		if wh.ObjectAttributes.Action == "reopen" {
			action = "reopened"
		} else if wh.ObjectAttributes.Action == "close" {
			action = "closed"
		}

		id := issueMessageID(c, wh.ObjectAttributes.ID)

		if id > 0 {
			// reply to existing message
			return msg.SetText(fmt.Sprintf("%s by %s", m.Bold(action), mention(c, wh.User.Username, ""))).
				EnableHTML().DisableWebPreview().SetReplyToMsgID(id).Send()
		}
		// original message not found. Send WebPreview
		wp := c.WebPreview("Issue", wh.ObjectAttributes.Title, wh.User.Username+" / "+wh.Repository.Name, wh.ObjectAttributes.URL, "")

		return msg.SetText(fmt.Sprintf("%s by %s", m.URL(action, wp), mention(c, wh.User.Username, ""))).EnableHTML().Send()

	case "note":
		wp := ""
		noteType := ""
		originMsg := &integram.Message{}
		noteID := strconv.Itoa(wh.ObjectAttributes.ID)
		if wh.ObjectAttributes.Note == "Commit" {
			// collisions by date are unlikely here
			noteID = wh.ObjectAttributes.CreatedAt
		}
		if msg, _ := c.FindMessageByEventID(noteUniqueID(wh.ObjectAttributes.ProjectID, noteID)); msg != nil {
			return nil
		}

		switch wh.ObjectAttributes.Note {
		case "Commit":
			noteType = "commit"
			originMsg, _ = c.FindMessageByEventID(fmt.Sprintf("commit_%d", wh.Commit.ID))
			if originMsg != nil {
				break
			}
			wp = c.WebPreview("Commit", "@"+wh.Commit.ID[0:10], wh.User.Username+" / "+wh.Repository.Name, wh.ObjectAttributes.URL, "")
		case "MergeRequest":
			noteType = "merge request"
			originMsg, _ = c.FindMessageByEventID(fmt.Sprintf("mr_%d", wh.MergeRequest.ID))
			if originMsg != nil {
				break
			}
			wp = c.WebPreview("Merge Request", wh.MergeRequest.Title, wh.User.Username+" / "+wh.Repository.Name, wh.ObjectAttributes.URL, "")
		case "Issue":
			noteType = "issue"
			originMsg, _ = c.FindMessageByEventID(fmt.Sprintf("issue_%d", wh.Issue.ID))
			if originMsg != nil {
				break
			}
			wp = c.WebPreview("Issue", wh.Issue.Title, wh.User.Username+" / "+wh.Repository.Name, wh.ObjectAttributes.URL, "")
		case "Snippet":
			noteType = "snippet"
			originMsg, _ = c.FindMessageByEventID(fmt.Sprintf("snippet_%d", wh.Snippet.ID))
			if originMsg != nil {
				break
			}
			wp = c.WebPreview("Snippet", wh.Snippet.Title, wh.User.Username+" / "+wh.Repository.Name, wh.ObjectAttributes.URL, "")
		}

		if originMsg == nil {
			if wp == "" {
				wp = wh.ObjectAttributes.URL
			}

			if noteType == "" {
				noteType = strings.ToLower(wh.ObjectAttributes.NoteableType)
			}

			return msg.SetTextFmt("%s commented on %s: %s", mention(c, wh.User.Username, ""), m.URL(strings.ToLower(wh.ObjectAttributes.NoteableType), wp), wh.ObjectAttributes.Note).
				EnableHTML().
				Send()
		}
		return msg.SetText(fmt.Sprintf("%s: %s", mention(c, wh.User.Username, ""), wh.ObjectAttributes.Note)).
			DisableWebPreview().EnableHTML().SetReplyToMsgID(originMsg.MsgID).Send()
	case "merge_request":

		if wh.ObjectAttributes.Action == "open" {
			if wh.ObjectAttributes.Description != "" {
				wh.ObjectAttributes.Description = "\n" + wh.ObjectAttributes.Description
			}

			err := msg.AddEventID("mr_" + strconv.Itoa(wh.ObjectAttributes.ID)).SetText(fmt.Sprintf("%s %s %s at %s:\n%s%s", mention(c, wh.User.Username, wh.UserEmail), wh.ObjectAttributes.State, m.URL("merge request", wh.ObjectAttributes.URL), m.URL(wh.UserName+" / "+wh.Repository.Name, wh.Repository.Homepage), m.Bold(wh.ObjectAttributes.Title), wh.ObjectAttributes.Description)).
				EnableHTML().DisableWebPreview().Send()

			return err
		}

	default:
		return
		break
	}
	return
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
		c.Log().Debug("hm")
		return c.NewMessage().EnableAntiFlood().EnableHTML().
			SetText("Hi here! To setup notifications " + m.Bold("for this chat") + " your GitLab project(repo), open Settings -> Web Hooks and add this URL:\n" + m.Fixed(c.Chat.ServiceHookURL())).EnableHTML().Send()

	case "cancel", "clean", "reset":
		return c.NewMessage().SetText("Clean").HideKeyboard().Send()
	}
	return nil
}
