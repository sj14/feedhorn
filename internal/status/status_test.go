package status

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/sj14/feedhorn/internal/feed"
)

func TestRender(t *testing.T) {
	tests := []struct {
		name     string
		template string
		item     feed.Item
		want     string
	}{
		{name: "default", template: Default, item: feed.Item{Title: "Hello", Link: "https://example.com"}, want: "Hello\n\nhttps://example.com"},
		{name: "default without title", template: Default, item: feed.Item{Link: "https://example.com"}, want: "https://example.com"},
		{name: "default without link", template: Default, item: feed.Item{Title: "Hello"}, want: "Hello"},
		{name: "custom", template: "{{.Title}} #ARD\n\n{{.Link}}", item: feed.Item{Title: "Hello", Link: "https://example.com"}, want: "Hello #ARD\n\nhttps://example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := Parse(tt.template)
			if err != nil {
				t.Fatal(err)
			}

			got, truncated, err := Render(tmpl, tt.item)
			if err != nil {
				t.Fatal(err)
			}

			if got != tt.want || truncated {
				t.Fatalf("got %q (truncated %v), want %q", got, truncated, tt.want)
			}
		})
	}
}

func TestRenderTruncatesAtTheEnd(t *testing.T) {
	tmpl, err := Parse(Default)
	if err != nil {
		t.Fatal(err)
	}

	got, truncated, err := Render(tmpl, feed.Item{Title: strings.Repeat("ä", 1000), Link: "https://example.com"})
	if err != nil {
		t.Fatal(err)
	}

	if !truncated {
		t.Fatal("want truncated")
	}

	if !strings.HasSuffix(got, "…") || utf8.RuneCountInString(got) != maxCharacters {
		t.Fatalf("got %d characters ending in %q, want %d ending in an ellipsis", utf8.RuneCountInString(got), got[len(got)-10:], maxCharacters)
	}
}

func TestParseRejects(t *testing.T) {
	for name, text := range map[string]string{
		"syntax":        "{{.Title",
		"unknown field": "{{.Tilte}}",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse(text); err == nil {
				t.Fatal("want error")
			}
		})
	}
}
