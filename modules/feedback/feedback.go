// ability to add the feedback

package feedback

import (
	"fmt"
	"github.com/requilence/integram"
	"time"

	"github.com/kelseyhightower/envconfig"
	"math/rand"
	"errors"
)

var FeedbackModule = integram.Module{
	Actions: []interface{}{
		askForFeedbackReplied,
		feedbackEdited,
	},
}

var m = integram.HTMLRichText{}

const (
	langFeedbackCmdText               = "Tell what we can improve to make this Youtube bot better"
	langFeedbackOkText                = "Thanks for your feedback 👍 It was forwarded to developers. If you have something to add you can just edit your message"
	langFeedbackOnlyTextSupportedText = "For now only the text feedback is accepted. If you want to send some screenshots, please note this in the text"
)

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyz1234567890-_")

type FeedbackConfig struct {
	ChatID int64 `envconfig:"CHAT_ID" required:"true"`
}

var config FeedbackConfig

func init() {
	envconfig.Process("FEEDBACK", &config)
}

func randStr(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

type feedbackMsg struct {
	ChatID int64
	MsgID  int

	Text string
}

func feedbackEdited(c *integram.Context, feedbackID string) error {
	// handle feedback message edited
	if feedbackID == "" {
		return nil
	}

	var texts []feedbackMsg

	c.User.Cache(feedbackID, &texts)
	joinedText := ""
	found := false

	for i, msg := range texts {
		if msg.MsgID == c.Message.MsgID && msg.ChatID == c.Message.ChatID {
			msg.Text = c.Message.Text
			texts[i] = msg
			found = true
		}
		joinedText += msg.Text + "\n"
	}

	if found {
		c.User.SetCache(feedbackID, texts, time.Hour*48)
		_, err := c.EditMessagesTextWithEventID("fb_"+feedbackID, formatFeedbackMessage(joinedText, c))

		return err
	}

	return nil

}

func formatFeedbackMessage(text string, ctx *integram.Context) string {
	var userMention string

	if ctx.User.UserName != "" {
		userMention = "@" + ctx.User.UserName
	} else {
		userMention = fmt.Sprintf(`<a href="tg://user?id=%d">%s</a>`, ctx.User.ID, ctx.User.FirstName+" "+ctx.User.LastName)
	}

	text = m.EncodeEntities(text)

	suffix := fmt.Sprintf("\n#%s • by %s • ver #%s", ctx.ServiceName, userMention, integram.GetShortVersion())

	if len(text) > (4096 - len(suffix)) {
		text = text[0:(4096-len(suffix)-3)] + "..."
	}
	return text + suffix
}

func askForFeedbackReplied(c *integram.Context) error {

	if c.Message.Text == "" {
		return c.NewMessage().
			SetReplyToMsgID(c.Message.MsgID).
			SetText(langFeedbackOnlyTextSupportedText).
			EnableForceReply().
			SetReplyAction(askForFeedbackReplied).
			Send()
	}

	feedbackID := ""

	c.User.Cache("feedbackID", &feedbackID)

	var texts = []feedbackMsg{}
	if feedbackID == "" {
		feedbackID = randStr(10)
		c.User.SetCache("feedbackID", feedbackID, time.Hour*24)
	} else {
		c.User.Cache(feedbackID, &texts)
	}

	texts = append(texts, feedbackMsg{c.Chat.ID, c.Message.MsgID, c.Message.Text})
	c.User.SetCache(feedbackID, texts, time.Hour*48)

	joinedText := ""

	for _, msg := range texts {
		joinedText += msg.Text + "\n"
	}

	if len(texts) > 1 {
		_, err := c.EditMessagesTextWithEventID("fb_"+feedbackID, formatFeedbackMessage(joinedText, c))

		if err != nil {
			c.Log().WithError(err).Error("askForFeedbackReplied EditMessagesTextWithEventID error")
		}
	}

	c.NewMessage().SetReplyToMsgID(c.Message.MsgID).SetText(langFeedbackOkText).Send()
	c.Message.SetEditAction(feedbackEdited, feedbackID)

	if len(texts) == 1 {
		return c.NewMessage().
			AddEventID("fb_" + feedbackID).
			SetChat(config.ChatID).
			EnableHTML().
			SetText(formatFeedbackMessage(joinedText, c)).
			Send()
	}

	return nil
}

func SendAskForFeedbackMessage(c *integram.Context) error {
	if config.ChatID == 0 {
		return errors.New("Received /feedback but env FEEDBACK_CHAT_ID not set")
	}
	return c.NewMessage().SetText(langFeedbackCmdText).EnableForceReply().SetReplyAction(askForFeedbackReplied).Send()
}
