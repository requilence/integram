package integram

import "testing"

func TestHTMLRichText_Pre(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name string
		hrt  HTMLRichText
		args args
		want string
	}{
		{"test1", HTMLRichText{}, args{"text here"}, "<pre>text here</pre>"},
		{"test2", HTMLRichText{}, args{"</a>text here"}, "<pre>&lt;/a&gt;text here</pre>"},
	}
	for _, tt := range tests {
		hrt := HTMLRichText{}
		if got := hrt.Pre(tt.args.s); got != tt.want {
			t.Errorf("%q. HTMLRichText.Pre() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestHTMLRichText_Fixed(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name string
		hrt  HTMLRichText
		args args
		want string
	}{
		{"test1", HTMLRichText{}, args{"text here"}, "<code>text here</code>"},
		{"test2", HTMLRichText{}, args{"</a>text here"}, "<code>&lt;/a&gt;text here</code>"},
	}
	for _, tt := range tests {
		hrt := HTMLRichText{}
		if got := hrt.Fixed(tt.args.s); got != tt.want {
			t.Errorf("%q. HTMLRichText.Fixed() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestHTMLRichText_EncodeEntities(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name string
		hrt  HTMLRichText
		args args
		want string
	}{
		{"test1", HTMLRichText{}, args{"<a href=\"index.html?a=1&b=2\">abc</a>"}, "&lt;a href=\"index.html?a=1&amp;b=2\"&gt;abc&lt;/a&gt;"},
	}
	for _, tt := range tests {
		hrt := HTMLRichText{}
		if got := hrt.EncodeEntities(tt.args.s); got != tt.want {
			t.Errorf("%q. HTMLRichText.EncodeEntities() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestHTMLRichText_URL(t *testing.T) {
	type args struct {
		text string
		url  string
	}
	tests := []struct {
		name string
		hrt  HTMLRichText
		args args
		want string
	}{
		{"test1", HTMLRichText{}, args{"text here", "https://integram.org"}, "<a href=\"https://integram.org\">text here</a>"},
		{"test2", HTMLRichText{}, args{"</a>text here", "https://integram.org/?a=1&b=2"}, "<a href=\"https://integram.org/?a=1&b=2\">&lt;/a&gt;text here</a>"},
	}
	for _, tt := range tests {
		hrt := HTMLRichText{}
		if got := hrt.URL(tt.args.text, tt.args.url); got != tt.want {
			t.Errorf("%q. HTMLRichText.URL() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestHTMLRichText_Bold(t *testing.T) {
	type args struct {
		text string
	}
	tests := []struct {
		name string
		hrt  HTMLRichText
		args args
		want string
	}{
		{"test1", HTMLRichText{}, args{"text here"}, "<b>text here</b>"},
		{"test2", HTMLRichText{}, args{"</a>text here"}, "<b>&lt;/a&gt;text here</b>"},
	}
	for _, tt := range tests {
		hrt := HTMLRichText{}
		if got := hrt.Bold(tt.args.text); got != tt.want {
			t.Errorf("%q. HTMLRichText.Bold() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestHTMLRichText_Italic(t *testing.T) {
	type args struct {
		text string
	}
	tests := []struct {
		name string
		hrt  HTMLRichText
		args args
		want string
	}{
		{"test1", HTMLRichText{}, args{"text here"}, "<i>text here</i>"},
		{"test2", HTMLRichText{}, args{"</a>text here"}, "<i>&lt;/a&gt;text here</i>"},
	}
	for _, tt := range tests {
		hrt := HTMLRichText{}
		if got := hrt.Italic(tt.args.text); got != tt.want {
			t.Errorf("%q. HTMLRichText.Italic() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestMarkdownRichText_Pre(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name string
		mrt  MarkdownRichText
		args args
		want string
	}{
		{"test1", MarkdownRichText{}, args{"text here"}, "```\ntext here\n```"},
		{"test2", MarkdownRichText{}, args{"```text here"}, "```\n‛‛‛text here\n```"},
	}
	for _, tt := range tests {
		mrt := MarkdownRichText{}
		if got := mrt.Pre(tt.args.s); got != tt.want {
			t.Errorf("%q. MarkdownRichText.Pre() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestMarkdownRichText_Fixed(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name string
		mrt  MarkdownRichText
		args args
		want string
	}{
		{"test1", MarkdownRichText{}, args{"text here"}, "`text here`"},
		{"test2", MarkdownRichText{}, args{"`text here"}, "`‛text here`"},
	}
	for _, tt := range tests {
		mrt := MarkdownRichText{}
		if got := mrt.Fixed(tt.args.s); got != tt.want {
			t.Errorf("%q. MarkdownRichText.Fixed() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestMarkdownRichText_Esc(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name string
		mrt  MarkdownRichText
		args args
		want string
	}{
		{"test1", MarkdownRichText{}, args{"[a](b)"}, "\\[a\\]\\(b\\)"},
		{"test2", MarkdownRichText{}, args{"`here is the*_text"}, "\\`here is the\\*\\_text"},
	}
	for _, tt := range tests {
		mrt := MarkdownRichText{}
		if got := mrt.Esc(tt.args.s); got != tt.want {
			t.Errorf("%q. MarkdownRichText.Esc() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestMarkdownRichText_URL(t *testing.T) {
	type args struct {
		text string
		url  string
	}
	tests := []struct {
		name string
		mrt  MarkdownRichText
		args args
		want string
	}{
		{"test1", MarkdownRichText{}, args{"text here", "https://integram.org"}, "[text here](https://integram.org)"},
		{"test2", MarkdownRichText{}, args{"(text here[])", "https://integram.org/?a=1&b=2"}, "[❨text here⟦⟧❩](https://integram.org/?a=1&b=2)"},
	}
	for _, tt := range tests {
		mrt := MarkdownRichText{}
		if got := mrt.URL(tt.args.text, tt.args.url); got != tt.want {
			t.Errorf("%q. MarkdownRichText.URL() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestMarkdownRichText_Bold(t *testing.T) {
	type args struct {
		text string
	}
	tests := []struct {
		name string
		mrt  MarkdownRichText
		args args
		want string
	}{
		{"test1", MarkdownRichText{}, args{"text here"}, "*text here*"},
		{"test2", MarkdownRichText{}, args{"*text here"}, "*∗text here*"},
	}
	for _, tt := range tests {
		mrt := MarkdownRichText{}
		if got := mrt.Bold(tt.args.text); got != tt.want {
			t.Errorf("%q. MarkdownRichText.Bold() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestMarkdownRichText_Italic(t *testing.T) {
	type args struct {
		text string
	}
	tests := []struct {
		name string
		mrt  MarkdownRichText
		args args
		want string
	}{
		{"test1", MarkdownRichText{}, args{"text here"}, "_text here_"},
		{"test2", MarkdownRichText{}, args{"_text here"}, "_＿text here_"}}
	for _, tt := range tests {
		mrt := MarkdownRichText{}
		if got := mrt.Italic(tt.args.text); got != tt.want {
			t.Errorf("%q. MarkdownRichText.Italic() = %v, want %v", tt.name, got, tt.want)
		}
	}
}
