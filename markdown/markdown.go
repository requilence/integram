package markdown

import (
	"fmt"
	"strings"
)

func Pre(s string) string {
	return "``` " + s + "```"
}

func Fixed(s string) string {
	return "`" + s + "`"
}

func URLEsc(s string) string {
	repalcer := strings.NewReplacer("[", "\\[", "]", "\\]", "(", "\\(", ")", "\\)")
	return repalcer.Replace(s)
}

func URL(text string, url string) string {
	text = fmt.Sprintf("[%s](%s)", URLEsc(text), URLEsc(url))
	return text
}

func Bold(text string) string {
	if text == "" {
		return ""
	}
	repalcer := strings.NewReplacer("*", "\\*")
	text = repalcer.Replace(text)
	text = fmt.Sprintf("*%s*", text)
	return text
}

func Italic(text string) string {
	if text == "" {
		return ""
	}
	repalcer := strings.NewReplacer("_", "\\_")
	text = repalcer.Replace(text)
	text = fmt.Sprintf("_%s_", text)
	return text
}
