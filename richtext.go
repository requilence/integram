package integram

import (
	"fmt"
	"strings"
)

// MarkdownRichText produce Markdown that can be sent to Telegram. Not recommended to use because of tricky escaping
// Use HTMLRichText instead
type MarkdownRichText struct{}

// HTMLRichText produce HTML that can be sent to Telegram
type HTMLRichText struct{}

// Pre generates <pre>text</pre>
func (hrt HTMLRichText) Pre(s string) string {
	return "<pre>" + hrt.EncodeEntities(s) + "</pre>"
}

// Fixed generates <code>text</code>
func (hrt HTMLRichText) Fixed(s string) string {
	return "<code>" + hrt.EncodeEntities(s) + "</code>"
}

// EncodeEntities encodes '<', '>' and '&'
func (hrt HTMLRichText) EncodeEntities(s string) string {
	repalcer := strings.NewReplacer("<", "&lt;", ">", "&gt;", "&", "&amp;")
	return repalcer.Replace(s)
}

// URL generates <a href="URL>text</a>
func (hrt HTMLRichText) URL(text string, url string) string {
	text = fmt.Sprintf("<a href=\"%s\">%s</a>", url, hrt.EncodeEntities(text))
	return text
}

// Bold generates <b>text</b>
func (hrt HTMLRichText) Bold(text string) string {
	if text == "" {
		return ""
	}

	text = fmt.Sprintf("<b>%s</b>", hrt.EncodeEntities(text))
	return text
}

// Italic generates <i>text</I>
func (hrt HTMLRichText) Italic(text string) string {
	if text == "" {
		return ""
	}

	text = fmt.Sprintf("<i>%s</i>", hrt.EncodeEntities(text))
	return text
}

// Pre generates```text```
func (mrt MarkdownRichText) Pre(text string) string {
	if text == "" {
		return ""
	}
	return "```\n" + mrt.Esc(text) + "\n```"
}

// Fixed generates`text`
func (mrt MarkdownRichText) Fixed(text string) string {
	if text == "" {
		return ""
	}
	return "`" + mrt.Esc(text) + "`"
}

// Esc replace '[', ']', '(', ')', "`", "_", "*" with similar symbols
func (mrt MarkdownRichText) Esc(s string) string {
	repalcer := strings.NewReplacer("[", "\\[", "]", "\\]", "(", "\\(", ")", "\\)", "`", "\\`", "_", "\\_", "*", "\\*")
	return repalcer.Replace(s)
}

// URL generates [text](URL)
func (mrt MarkdownRichText) URL(text string, url string) string {
	text = fmt.Sprintf("[%s](%s)", mrt.Esc(text), mrt.Esc(url))
	return text
}

// Bold generates *text*
func (mrt MarkdownRichText) Bold(text string) string {
	if text == "" {
		return ""
	}
	return "*" + mrt.Esc(text) + "*"
}

// Italic generates _text_
func (mrt MarkdownRichText) Italic(text string) string {
	if text == "" {
		return ""
	}

	return "_" + mrt.Esc(text) + "_"
}
