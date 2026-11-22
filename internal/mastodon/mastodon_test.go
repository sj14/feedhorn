package mastodon

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestStatus(t *testing.T) {
	tests := []struct {
		name  string
		title string
		link  string
		want  string
	}{
		{name: "title and link", title: "Hello", link: "https://example.com", want: "Hello\n\nhttps://example.com"},
		{name: "link only", title: "", link: "https://example.com", want: "https://example.com"},
		{name: "title only", title: "Hello", link: "", want: "Hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Status(tt.title, tt.link); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStatusTruncatesLongTitle(t *testing.T) {
	link := "https://example.com/" + strings.Repeat("x", 200)
	got := Status(strings.Repeat("ä", 1000), link)

	title, gotLink, _ := strings.Cut(got, "\n\n")

	if gotLink != link {
		t.Fatalf("link changed to %q", gotLink)
	}

	if !strings.HasSuffix(title, "…") {
		t.Fatalf("truncated title %q does not end with an ellipsis", title)
	}

	// the link counts as 23 characters, whatever its length
	if counted := utf8.RuneCountInString(title) + len("\n\n") + linkCharacters; counted != maxCharacters {
		t.Fatalf("status counts %d characters, want %d", counted, maxCharacters)
	}
}
