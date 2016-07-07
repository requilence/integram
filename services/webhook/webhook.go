package webhook

import (
	"errors"
	"github.com/requilence/integram"
	m "github.com/requilence/integram/html"
)

type Config struct {
}

type Webhook struct {
	Text        string
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

func (c Config) Service() *integram.Service {
	return &integram.Service{
		Name:                "webhook",
		NameToPrint:         "Webhook",
		WebhookHandler:      WebhookHandler,
		TGNewMessageHandler: Update,
	}
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
		return c.NewMessage().EnableAntiFlood().EnableHTML().
			SetText("Hi here! You can send Slack-compatible simple webhooks to " + m.Bold("this chat") + " using this URL: \n" + m.Fixed(c.Chat.ServiceHookURL()) + "\n\nExample (JSON payload):\n" + m.Pre("{\"text\":\"So advanced\\nMuch innovations 🙀\"}")).Send()

	}
	return nil
}

func WebhookHandler(c *integram.Context, wc *integram.WebhookContext) (err error) {

	wh := Webhook{}
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
		return c.NewMessage().SetText(wh.Text + " " + wh.Channel).EnableAntiFlood().Send()
	}

	return errors.New("Text and Attachments not found")
}
