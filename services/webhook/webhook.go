package webhook

import (
	"errors"

	"github.com/requilence/integram"
)

var m = integram.HTMLRichText{}

// Config is empty because no need in it here
type Config struct{}

type webhook struct {
	Text        string
	Mrkdwn      bool
	Channel     string
	Attachments []struct {
		Pretext    string `json:"pretext"`
		AuthorName string `json:"author_name"`
		AuthorLink string `json:"author_link"`
		Title      string `json:"title"`
		TitleLink  string `json:"title_link"`
		Text       string `json:"text"`
		ImageURL   string `json:"image_url"`
		ThumbURL   string `json:"thumb_url"`
		Ts         int    `json:"ts"`
	} `json:"attachments"`
}

// Service returns *integram.Service
func (c Config) Service() *integram.Service {
	return &integram.Service{
		Name:                "webhook",
		NameToPrint:         "Webhook",
		WebhookHandler:      webhookHandler,
		TGNewMessageHandler: update,
	}
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
			SetText("Hi here! You can send " + m.URL("Slack-compatible", "https://api.slack.com/docs/message-formatting#message_formatting") + " simple webhooks to " + m.Bold("this chat") + " using this URL: \n" + m.Fixed(c.Chat.ServiceHookURL()) + "\n\nExample (JSON payload):\n" + m.Pre("{\"text\":\"So _advanced_\\nMuch *innovations* 🙀\"}")).Send()

	}
	return nil
}

func webhookHandler(c *integram.Context, wc *integram.WebhookContext) (err error) {

	wh := webhook{Mrkdwn: true}
	err = wc.JSON(&wh)

	if err != nil {
		return
	}

	if len(wh.Attachments) > 0 {
		if wh.Text != "" {
			wh.Text += "\n"
		}
		wp := c.WebPreview(wh.Attachments[0].Title, wh.Attachments[0].AuthorName, wh.Attachments[0].Pretext, wh.Attachments[0].TitleLink, wh.Attachments[0].ThumbURL)
		text := m.URL(" ", wp) + " " + wh.Text
		for i, attachment := range wh.Attachments {
			if i > 0 {
				text += "\n"
			}
			text += m.URL(attachment.Title, attachment.TitleLink) + " " + attachment.Pretext
		}
		return c.NewMessage().SetText(text).EnableAntiFlood().EnableHTML().Send()
	}

	if wh.Text != "" {
		m := c.NewMessage().SetText(wh.Text + " " + wh.Channel).EnableAntiFlood()
		if wh.Mrkdwn {
			m.EnableMarkdown()
		}
		return m.Send()
	}

	return errors.New("Text and Attachments not found")
}
