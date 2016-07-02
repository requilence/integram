package html

import (
	"fmt"
	"strings"
)

func Pre(s string) string {
	return "<pre> " + EncodeEntities(s) + "</pre>"
}

func Fixed(s string) string {
	return "<code>" + EncodeEntities(s) + "</code>"
}

func EncodeEntities(s string) string {
	repalcer := strings.NewReplacer("<", "&lt;", ">", "&gt;", "&", "&amp;")
	return repalcer.Replace(s)
}

func URL(text string, url string) string {
	text = fmt.Sprintf("<a href=\"%s\">%s</a>", url, EncodeEntities(text))
	return text
}

func Bold(text string) string {
	if text == "" {
		return ""
	}

	text = fmt.Sprintf("<b>%s</b>", EncodeEntities(text))
	return text
}

func Italic(text string) string {
	if text == "" {
		return ""
	}

	text = fmt.Sprintf("<i>%s</i>", EncodeEntities(text))
	return text
}
