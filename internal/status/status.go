// Package status renders feed items into the text of a post.
package status

import (
	"fmt"
	"strings"
	"text/template"

	"github.com/sj14/feedhorn/internal/feed"
)

// Default is the template of a feed that sets none: the title, a blank line
// and the link.
const Default = "{{.Title}}\n\n{{.Link}}"

// Mastodon's default limit. Other servers may allow more, but none allow less.
const maxCharacters = 500

// Parse compiles a template and renders it once with an example item, so a
// misspelled field such as {{.Tilte}} fails at startup and not at the first
// post.
func Parse(text string) (*template.Template, error) {
	tmpl, err := template.New("status").Parse(text)
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}

	example := feed.Item{Title: "title", Link: "https://example.com"}
	if _, err := execute(tmpl, example); err != nil {
		return nil, err
	}

	return tmpl, nil
}

// Render fills the template with the item. A post longer than the limit is cut
// off at the end with an ellipsis, which can cut the link as well; truncated
// tells the caller, since it means the template needs to be shorter.
func Render(tmpl *template.Template, item feed.Item) (text string, truncated bool, err error) {
	text, err = execute(tmpl, item)
	if err != nil {
		return "", false, err
	}

	runes := []rune(text)
	if len(runes) <= maxCharacters {
		return text, false, nil
	}

	return string(runes[:maxCharacters-1]) + "…", true, nil
}

func execute(tmpl *template.Template, item feed.Item) (string, error) {
	var b strings.Builder
	if err := tmpl.Execute(&b, item); err != nil {
		return "", fmt.Errorf("render template: %w", err)
	}

	// an item without title or link would otherwise start or end with the
	// blank line between them
	return strings.TrimSpace(b.String()), nil
}
