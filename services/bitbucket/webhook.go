package bitbucket

import (
	"errors"
	"github.com/requilence/integram"
	"time"

	"encoding/json"
	"fmt"
	api "github.com/ktrysmt/go-bitbucket"
	m "github.com/requilence/integram/html"
	"regexp"
	"strconv"
	"strings"
)

type OldWebhook struct {
	CanonURL string `json:"canon_url"`
	Commits  []struct {
		Author string `json:"author"`
		Branch string `json:"branch"`
		Files  []struct {
			File string `json:"file"`
			Type string `json:"type"`
		} `json:"files"`
		Message      string   `json:"message"`
		Node         string   `json:"node"`
		Parents      []string `json:"parents"`
		RawAuthor    string   `json:"raw_author"`
		RawNode      string   `json:"raw_node"`
		Revision     int      `json:"revision"`
		Size         int      `json:"size"`
		Timestamp    string   `json:"timestamp"`
		Utctimestamp string   `json:"utctimestamp"`
	} `json:"commits"`
	Repository struct {
		AbsoluteURL string `json:"absolute_url"`
		Fork        bool   `json:"fork"`
		IsPrivate   bool   `json:"is_private"`
		Name        string `json:"name"`
		Owner       string `json:"owner"`
		Scm         string `json:"scm"`
		Slug        string `json:"slug"`
		Website     string `json:"website"`
	} `json:"repository"`
	User string `json:"user"`
}

// map of webhook events to the payload type
var eventTypeMap = map[string]interface{}{
	"repo:push":                    api.RepoPushEvent{},
	"repo:fork":                    api.RepoForkEvent{},
	"repo:commit_comment_created":  api.RepoCommitCommentCreatedEvent{},
	"repo:commit_status_created":   api.RepoCommitStatusCreatedEvent{},
	"repo:commit_status_updated":   api.RepoCommitStatusUpdatedEvent{},
	"issue:created":                api.IssueCreatedEvent{},
	"issue:updated":                api.IssueUpdatedEvent{},
	"issue:comment_created":        api.IssueCommentCreatedEvent{},
	"pullrequest:created":          api.PullRequestCreatedEvent{},
	"pullrequest:updated":          api.PullRequestUpdatedEvent{},
	"pullrequest:approved":         api.PullRequestApprovedEvent{},
	"pullrequest:unapproved":       api.PullRequestApprovalRemovedEvent{},
	"pullrequest:fulfilled":        api.PullRequestMergedEvent{},
	"pullrequest:rejected":         api.PullRequestDeclinedEvent{},
	"pullrequest:comment_created":  api.PullRequestCommentCreatedEvent{},
	"pullrequest:comment_updated":  api.PullRequestCommentUpdatedEvent{},
	"pull_request:comment_deleted": api.PullRequestCommentDeletedEvent{},
}

func commitUniqueID(commitHash string) string {
	return "commit_" + commitHash
}
func commitCommentUniqueID(commitHash string, commentID int) string {
	return "commit_" + commitHash + "_" + strconv.Itoa(commentID)
}

func issueUniqueID(fullRepo string, issueID int) string {
	return "issue_" + fullRepo + "_" + strconv.Itoa(issueID)
}

func prUniqueID(fullRepo string, prID int) string {
	return "pr_" + fullRepo + "_" + strconv.Itoa(prID)
}

func prCommentUniqueID(fullRepo string, prID int, commentID int) string {
	return "pr_" + fullRepo + "_" + strconv.Itoa(prID) + "_" + strconv.Itoa(commentID)
}
func issueCommentUniqueID(fullRepo string, issueID int, commentID int) string {
	return "issue_" + fullRepo + "_" + strconv.Itoa(issueID) + "_" + strconv.Itoa(commentID)
}

var issueStates = map[string]string{"new": "set as new", "open": "opened", "on hold": "put on hold", "resolved": "marked as resolved", "duplicate": "marked as duplicate", "invalid": "marked as invalid", "wontfix": "marked as won't fix", "closed": "closed"}

func issueDecentState(state string) string {
	if v, exists := issueStates[state]; exists {
		return v
	}
	return state
}

var reRepoFullNameFromURL = regexp.MustCompile("repositories/([^/]*/[^/]*)")

func prText(c *integram.Context, pr *api.PullRequest) string {

	r := reRepoFullNameFromURL.FindStringSubmatch(pr.Links.Self.Href)

	repo := ""
	if len(r) == 2 {
		repo = r[1]
	}

	if pr.Description == pr.Title {
		pr.Description = ""
	}
	text := fmt.Sprintf("%s %s\n%s",
		m.Bold(pr.Title),
		m.URL("➔", c.WebPreview("Pull Request", repo, "by "+pr.Author.DisplayName, pr.Links.HTML.String(), "")),
		pr.Description)

	if len(pr.Reviewers) > 0 {
		text += "\n  👤 "

		for i, reviewer := range pr.Reviewers {
			text += mention(c, &reviewer)
			if i < len(pr.Reviewers)-1 {
				text += ", "
			}
		}
	}
	return text
}

func issueText(c *integram.Context, issue *api.Issue) string {

	r := reRepoFullNameFromURL.FindStringSubmatch(issue.Links.Self.Href)

	repo := ""
	if len(r) == 2 {
		repo = r[1]
	}

	return fmt.Sprintf("%s %s\n%s\n%s",
		m.Bold(issue.Title),
		m.URL("➔", c.WebPreview("by "+issue.Reporter.DisplayName, repo, "", issue.Links.HTML.String(), "")),
		issue.Content.Raw,
		"#"+issue.Priority+" #"+issue.Type)
}

func prInlineKeyboard(pr *api.PullRequest) integram.InlineKeyboard {
	but := integram.InlineButtons{}

	but.Append("assign", "Assign")
	but.Append("commits", "Commits")

	return but.Markup(4, "actions")
}

func commitInlineKeyboard(commit *api.Commit) integram.InlineKeyboard {
	but := integram.InlineButtons{}
	// wating for api endpoints..
	//but.Append("assign", "Assign")
	//but.Append("status", "⬆︎ "+strings.ToUpper(issue.State[0:1])+issue.State[1:])

	if len(commit.ApprovedActorsUUID) > 0 {
		but.Append("vote", fmt.Sprintf("✅ Approved (%d)", len(commit.ApprovedActorsUUID)))
	} else {
		but.Append("vote", "✅ Approve")
	}

	return but.Markup(4, "actions")
}

func issueInlineKeyboard(issue *api.Issue) integram.InlineKeyboard {
	but := integram.InlineButtons{}
	// wating for api endpoints..
	//but.Append("assign", "Assign")
	//but.Append("status", "⬆︎ "+strings.ToUpper(issue.State[0:1])+issue.State[1:])

	if issue.Votes > 0 {
		but.Append("vote", fmt.Sprintf("👍 %d", issue.Votes))
	} else {
		but.Append("vote", "👍")
	}

	return but.Markup(4, "actions")
}
func commitShort(hash string) string {
	if len(hash) > 10 {
		return hash[0:10]
	}
	return hash
}

func oldWebhookHandler(c *integram.Context, wc *integram.WebhookContext) (err error) {

	wh := OldWebhook{}

	payload := wc.FormValue("payload")

	if payload == "" {
		return errors.New("X-Event-Key header missed and old-style webhook data not found")
	}

	json.Unmarshal([]byte(payload), &wh)

	if wh.CanonURL == "" {
		return errors.New("Error decoding payload for old webhook")
	}

	msg := c.NewMessage()
	commits := 0
	text := ""
	wp := ""

	if len(wh.Commits) == 0 {
		return nil
	}

	headCommit := wh.Commits[len(wh.Commits)-1]

	if len(wh.Commits) > 1 {

		wp = c.WebPreview(fmt.Sprintf("%d commits", len(wh.Commits)), "@"+wh.Commits[0].Node[0:10]+" ... @"+headCommit.Node[0:10], "", wh.CanonURL+wh.Repository.AbsoluteURL+"/compare/"+headCommit.Node+".."+wh.Commits[0].Parents[0], "")

		anyOherPersonCommits := false
		for _, commit := range wh.Commits {
			if commit.Author != wh.User {
				anyOherPersonCommits = true
				break
			}
		}
		for _, commit := range wh.Commits {
			commits++
			if anyOherPersonCommits {
				text += m.Bold(commit.Author) + ": "
			}
			text += m.URL(commit.Message, wh.CanonURL+wh.Repository.AbsoluteURL+"commits/"+headCommit.Node[0:10]) + "\n"
		}

	} else if len(wh.Commits) == 1 {
		wp = c.WebPreview("Commit", "@"+headCommit.Node[0:10], "", wh.CanonURL+wh.Repository.AbsoluteURL+"commits/"+headCommit.Node[0:10], "")

		commit := &wh.Commits[0]
		if commit.Author != wh.User {
			text += m.Bold(commit.Author) + ": "
		}

		text += commit.Message + "\n"
	}

	if len(wh.Commits) > 0 {

		return msg.SetTextFmt("%s %s to %s/%s\n%s",
			m.Bold(wh.User),
			m.URL("pushed", wp),
			m.URL(wh.Repository.Name, wh.CanonURL+wh.Repository.AbsoluteURL),
			m.URL(headCommit.Branch, wh.CanonURL+wh.Repository.AbsoluteURL+"branch/"+headCommit.Branch),
			text).
			EnableHTML().
			Send()
	}
	return nil

}
func WebhookHandler(c *integram.Context, wc *integram.WebhookContext) (err error) {
	eventKey := wc.Header("X-Event-Key")

	if eventKey == "" {
		// try the old one Bitbucket POST service

		return oldWebhookHandler(c, wc)
	}

	if _, ok := eventTypeMap[eventKey]; !ok {

		return errors.New("Bad X-Event-Key: " + eventKey)
	}

	c.Log().Debugf("eventKey=%v", eventKey)

	if err != nil {
		return errors.New("JSON deserialization error: " + err.Error())
	}

	switch eventKey {
	case "repo:push":
		event := api.RepoPushEvent{}
		wc.JSON(&event)

		for _, change := range event.Push.Changes {

			msg := c.NewMessage()
			commits := 0
			text := ""

			if len(change.Commits) > 1 {
				anyOherPersonCommits := false
				for _, commit := range change.Commits {
					if commit.Author.User.UUID != event.Actor.UUID {
						anyOherPersonCommits = true
						break
					}
				}
				for _, commit := range change.Commits {
					commits++
					if anyOherPersonCommits {
						text += mention(c, &commit.Author.User) + ": "
					}
					text += m.URL(commit.Message, commit.Links.HTML.Href) + "\n"
				}
				if change.Truncated {
					text += m.URL("... See all", change.Links.Commits.Href) + "\n"
				}
			} else if len(change.Commits) == 1 {
				commits++
				commit := &change.Commits[0]
				if commit.Author.User.UUID != event.Actor.UUID {
					text += mention(c, &commit.Author.User) + ": "
				}

				text += commit.Message + "\n"
			}

			change.New.Target.Author.User.Links.Avatar.Href = strings.Replace(change.New.Target.Author.User.Links.Avatar.Href, "/32/", "/128/", 1)
			wp := ""
			if change.Truncated {
				wp = c.WebPreview("> 5 commits", "@"+commitShort(change.Old.Target.Hash)+" ... @"+commitShort(change.New.Target.Hash), "", change.Links.HTML.Href, change.New.Target.Author.User.Links.Avatar.Href)
			} else if commits > 1 {
				wp = c.WebPreview(fmt.Sprintf("%d commits", commits), "@"+commitShort(change.Old.Target.Hash)+" ... @"+commitShort(change.New.Target.Hash), "", change.Links.HTML.Href, change.New.Target.Author.User.Links.Avatar.Href)
			} else if commits == 1 {
				wp = c.WebPreview("Commit", "@"+commitShort(change.New.Target.Hash), "", change.Commits[0].Links.HTML.Href, change.New.Target.Author.User.Links.Avatar.Href)
			}

			if commits > 0 {
				pushedText := ""
				if change.Forced {
					pushedText = m.URL("❗️ forcibly pushed", wp)
				} else {
					pushedText = m.URL("pushed", wp)
				}
				_, err = msg.SetTextFmt("%s %s to %s/%s\n%s",
					mention(c, &change.New.Target.Author.User),
					pushedText,
					m.URL(event.Repository.Name, event.Repository.Links.HTML.Href),
					m.URL(change.New.Name, change.New.Links.HTML.Href),
					text).
					AddEventID(commitUniqueID(change.Commits[0].Hash)).
					EnableHTML().
					SendAndGetID()
			}
		}
	case "issue:created":
		event := api.IssueCreatedEvent{}
		err := wc.JSON(&event)
		if err != nil {
			return err
		}
		event.Issue.Repository = &event.Repository

		c.SetServiceCache(issueUniqueID(event.Repository.FullName, event.Issue.ID), event.Issue, time.Hour*24*365)

		return c.NewMessage().AddEventID(issueUniqueID(event.Repository.FullName, event.Issue.ID)).
			SetInlineKeyboard(issueInlineKeyboard(&event.Issue)).
			SetText(issueText(c, &event.Issue)).
			SetCallbackAction(issueInlineButtonPressed, event.Repository.FullName, event.Issue.ID).
			EnableHTML().Send()
	case "issue:comment_created":
		event := api.IssueCommentCreatedEvent{}
		err := wc.JSON(&event)
		if err != nil {
			return err
		}

		var rm *integram.Message
		if event.Comment.Parent.ID > 0 {
			// actually bitbucket doesn't provide parent id for issue comments for now
			rm, _ = c.FindMessageByEventID(issueCommentUniqueID(event.Repository.FullName, event.Issue.ID, event.Comment.Parent.ID))
		}
		if rm == nil {
			rm, _ = c.FindMessageByEventID(issueUniqueID(event.Repository.FullName, event.Issue.ID))
		}

		msg := c.NewMessage().AddEventID(issueCommentUniqueID(event.Repository.FullName, event.Issue.ID, event.Comment.ID)).EnableHTML()

		if rm != nil {
			return msg.SetReplyToMsgID(rm.MsgID).SetText(fmt.Sprintf("%s: %s", mention(c, &event.Actor), event.Comment.Content.Raw)).Send()
		} else {
			wp := c.WebPreview("Issue", event.Issue.Title, event.Repository.FullName, event.Comment.Links.HTML.Href, "")
			return msg.SetText(fmt.Sprintf("%s %s: %s", m.URL("💬", wp), mention(c, &event.Actor), event.Comment.Content.Raw)).Send()
		}
	case "repo:commit_comment_created":
		event := api.RepoCommitCommentCreatedEvent{}
		err := wc.JSON(&event)
		if err != nil {
			return err
		}

		var rm *integram.Message
		if event.Comment.Parent.ID > 0 {
			// actually bitbucket doesn't provide parent id for issue comments for now
			rm, _ = c.FindMessageByEventID(commitCommentUniqueID(event.Commit.Hash, event.Comment.Parent.ID))
		}
		if rm == nil {
			rm, _ = c.FindMessageByEventID(commitUniqueID(event.Commit.Hash))
		}

		msg := c.NewMessage().AddEventID(commitCommentUniqueID(event.Commit.Hash, event.Comment.Parent.ID)).EnableHTML()

		if rm != nil {
			return msg.SetReplyToMsgID(rm.MsgID).SetText(fmt.Sprintf("%s: %s", mention(c, &event.Actor), event.Comment.Content.Raw)).Send()
		} else {
			wp := c.WebPreview("Commit", "@"+event.Commit.Hash[0:10], event.Repository.FullName, event.Comment.Links.HTML.Href, "")
			return msg.SetText(fmt.Sprintf("%s %s: %s", m.URL("💬", wp), mention(c, &event.Actor), event.Comment.Content.Raw)).Send()
		}
	case "issue:updated":
		event := api.IssueUpdatedEvent{}
		err := wc.JSON(&event)
		if err != nil {
			return err
		}

		rm, _ := c.FindMessageByEventID(issueUniqueID(event.Repository.FullName, event.Issue.ID))

		msg := c.NewMessage().AddEventID(issueCommentUniqueID(event.Repository.FullName, event.Issue.ID, event.Comment.ID)).EnableHTML()

		if rm != nil {
			return msg.SetReplyToMsgID(rm.MsgID).SetText(fmt.Sprintf("%s: %s", mention(c, &event.Actor), event.Comment.Content.Raw)).Send()
		} else {
			wp := c.WebPreview("Issue", event.Issue.Title, event.Repository.FullName, event.Comment.Links.HTML.Href, "")
			return msg.SetText(fmt.Sprintf("%s %s an issue: %s", mention(c, &event.Actor), m.URL(issueDecentState(event.Issue.State), wp), event.Comment.Content.Raw)).Send()
		}

	case "pullrequest:updated":
		event := api.PullRequestCreatedEvent{}
		err := wc.JSON(&event)
		if err != nil {
			return err
		}

		prText := prText(c, &event.PullRequest)

		eventID := prUniqueID(event.Repository.FullName, event.PullRequest.ID)
		rm, _ := c.FindMessageByEventID(eventID)

		msg := c.NewMessage()

		if rm != nil {
			c.EditMessagesTextWithEventID(c.Bot().ID, eventID, prText)
			// if last PR message just posted
			if err == nil && time.Now().Sub(rm.Date).Seconds() < 60 {
				return nil
			}

			msg.SetReplyToMsgID(rm.MsgID)
		} else {
			msg.AddEventID(prUniqueID(event.Repository.FullName, event.PullRequest.ID))
		}

		return msg.
			SetText("✏️ " + prText).
			EnableHTML().Send()
	case "pullrequest:created":
		event := api.PullRequestCreatedEvent{}
		err := wc.JSON(&event)
		if err != nil {
			return err
		}

		c.SetServiceCache(prUniqueID(event.Repository.FullName, event.PullRequest.ID), event.PullRequest, time.Hour*24*365)

		return c.NewMessage().AddEventID(prUniqueID(event.Repository.FullName, event.PullRequest.ID)).
			//SetInlineKeyboard(prInlineKeyboard(&event.PullRequest)).
			SetText(prText(c, &event.PullRequest)).
			//SetCallbackAction(prInlineButtonPressed, event.Repository.FullName, event.PullRequest.ID).
			EnableHTML().Send()

	case "pullrequest:approved":
		event := api.PullRequestApprovedEvent{}
		err := wc.JSON(&event)
		if err != nil {
			return err
		}

		rm, _ := c.FindMessageByEventID(prUniqueID(event.Repository.FullName, event.PullRequest.ID))
		msg := c.NewMessage().EnableHTML()

		if rm != nil {
			return msg.SetReplyToMsgID(rm.MsgID).SetText(fmt.Sprintf("✅ Approved by %s", mention(c, &event.Actor))).Send()
		} else {
			wp := c.WebPreview("Pull Request", event.PullRequest.Title, "by "+event.PullRequest.Author.DisplayName+" in "+event.Repository.FullName, event.PullRequest.Links.HTML.Href, "")
			return msg.SetText(fmt.Sprintf("✅ %s by %s", m.URL("Approved", wp), mention(c, &event.Actor))).Send()
		}

	case "pullrequest:unapproved":
		event := api.PullRequestApprovalRemovedEvent{}
		err := wc.JSON(&event)
		if err != nil {
			return err
		}

		rm, _ := c.FindMessageByEventID(prUniqueID(event.Repository.FullName, event.PullRequest.ID))
		msg := c.NewMessage().EnableHTML()

		if rm != nil {
			return msg.SetReplyToMsgID(rm.MsgID).SetText(fmt.Sprintf("❌ %s removed approval", mention(c, &event.Actor))).Send()
		} else {
			wp := c.WebPreview("Pull Request", event.PullRequest.Title, "by "+event.PullRequest.Author.DisplayName+" in "+event.Repository.FullName, event.PullRequest.Links.HTML.Href, "")
			return msg.SetText(fmt.Sprintf("❌ %s %s", mention(c, &event.Actor), m.URL("removed approval", wp))).Send()
		}

	case "pullrequest:fulfilled":
		event := api.PullRequestMergedEvent{}
		err := wc.JSON(&event)
		if err != nil {
			return err
		}

		rm, _ := c.FindMessageByEventID(prUniqueID(event.Repository.FullName, event.PullRequest.ID))
		msg := c.NewMessage().EnableHTML()

		if rm != nil {
			return msg.SetReplyToMsgID(rm.MsgID).SetText(fmt.Sprintf("✅ Merged by %s", mention(c, &event.Actor))).Send()
		} else {
			wp := c.WebPreview("Pull Request", event.PullRequest.Title, "by "+event.PullRequest.Author.DisplayName+" in "+event.Repository.FullName, event.PullRequest.Links.HTML.Href, "")
			return msg.SetText(fmt.Sprintf("✅ %s by %s", m.URL("Merged", wp), mention(c, &event.Actor))).Send()
		}
	case "pullrequest:rejected":
		event := api.PullRequestDeclinedEvent{}
		err := wc.JSON(&event)
		if err != nil {
			return err
		}

		rm, _ := c.FindMessageByEventID(prUniqueID(event.Repository.FullName, event.PullRequest.ID))
		msg := c.NewMessage().EnableHTML()

		if rm != nil {
			return msg.SetReplyToMsgID(rm.MsgID).SetText(fmt.Sprintf("❌ Declined by %s: %s", mention(c, &event.Actor), event.PullRequest.Reason)).Send()
		} else {
			wp := c.WebPreview("Pull Request", event.PullRequest.Title, "by "+event.PullRequest.Author.DisplayName+" in "+event.Repository.FullName, event.PullRequest.Links.HTML.Href, "")
			return msg.SetText(fmt.Sprintf("❌ %s by %s", m.URL("Declined", wp), mention(c, &event.Actor))).Send()
		}
	case "pullrequest:comment_created":
		event := api.PullRequestCommentCreatedEvent{}
		err := wc.JSON(&event)
		if err != nil {
			return err
		}

		rm, _ := c.FindMessageByEventID(prUniqueID(event.Repository.FullName, event.PullRequest.ID))

		msg := c.NewMessage().AddEventID(prCommentUniqueID(event.Repository.FullName, event.PullRequest.ID, event.Comment.ID)).EnableHTML()

		if rm != nil {
			return msg.SetReplyToMsgID(rm.MsgID).SetText(fmt.Sprintf("%s: %s", mention(c, &event.Actor), event.Comment.Content.Raw)).Send()
		} else {
			wp := c.WebPreview("Pull Request", event.PullRequest.Title, event.Repository.FullName, event.PullRequest.Links.HTML.Href, "")
			return msg.SetText(fmt.Sprintf("%s %s: %s", m.URL("💬", wp), mention(c, &event.Actor), event.Comment.Content.Raw)).Send()
		}
	}
	return err

}
