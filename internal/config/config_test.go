package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sj14/feedhorn/internal/feed"
	"github.com/sj14/feedhorn/internal/status"
)

func write(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	return path
}

func TestLoadDefaults(t *testing.T) {
	conf, err := Load(write(t, `
server: https://mastodon.example/
feeds:
  - url: https://example.com/feed.xml
`))
	if err != nil {
		t.Fatal(err)
	}

	if conf.Server != "https://mastodon.example" {
		t.Errorf("server %q still has its trailing slash", conf.Server)
	}

	if conf.Visibility != "public" {
		t.Errorf("visibility %q, want public", conf.Visibility)
	}

	if conf.Feeds[0].Template != status.Default || conf.Feeds[0].Status == nil {
		t.Errorf("feed template %q, want the parsed default", conf.Feeds[0].Template)
	}
}

func TestLoadTemplates(t *testing.T) {
	conf, err := Load(write(t, `
server: https://mastodon.example
template: "{{.Link}}"
feeds:
  - url: https://example.com/global.xml
  - url: https://example.com/own.xml
    template: "{{.Title}} #own"
`))
	if err != nil {
		t.Fatal(err)
	}

	if got := conf.Feeds[0].Template; got != "{{.Link}}" {
		t.Errorf("feed without template got %q, want the global one", got)
	}

	if got := conf.Feeds[1].Template; got != "{{.Title}} #own" {
		t.Errorf("feed with template got %q, want its own", got)
	}
}

func TestLoadRejects(t *testing.T) {
	tests := map[string]string{
		"unknown field":  "server: https://m.example\nfeeds: [{url: https://e.example/f}]\ntypo: 1\n",
		"no server":      "feeds: [{url: https://e.example/f}]\n",
		"bad server":     "server: mastodon.example\nfeeds: [{url: https://e.example/f}]\n",
		"no feeds":       "server: https://m.example\n",
		"no url":         "server: https://m.example\nfeeds: [{template: x}]\n",
		"plain url list": "server: https://m.example\nfeeds: [https://e.example/f]\n",
		"duplicate":      "server: https://m.example\nfeeds: [{url: https://e.example/f}, {url: https://e.example/f}]\n",
		"visibility":     "server: https://m.example\nvisibility: everyone\nfeeds: [{url: https://e.example/f}]\n",
		"bad template":   "server: https://m.example\nfeeds: [{url: https://e.example/f, template: '{{.Tilte}}'}]\n",
	}

	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(write(t, content)); err == nil {
				t.Fatal("want error")
			}
		})
	}
}

func TestIgnores(t *testing.T) {
	conf, err := Load(write(t, `
server: https://mastodon.example
feeds:
  - url: https://example.com/feed.xml
    ignore:
      - link: "/shorts/"
      - title: "(?i)trailer"
        link: "^https://example\\.com/"
`))
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		item feed.Item
		want bool
	}{
		{item: feed.Item{Title: "A film", Link: "https://www.youtube.com/watch?v=1"}, want: false},
		{item: feed.Item{Title: "A short", Link: "https://www.youtube.com/shorts/1"}, want: true},
		// a rule with title and link needs both to match
		{item: feed.Item{Title: "Official Trailer", Link: "https://example.com/1"}, want: true},
		{item: feed.Item{Title: "Official Trailer", Link: "https://other.example/1"}, want: false},
	}

	for _, tt := range tests {
		if got := conf.Feeds[0].Ignores(tt.item); got != tt.want {
			t.Errorf("Ignores(%+v) = %v, want %v", tt.item, got, tt.want)
		}
	}
}

func TestLoadRejectsIgnoreRules(t *testing.T) {
	tests := map[string]string{
		"empty rule":  "server: https://m.example\nfeeds: [{url: https://e.example/f, ignore: [{}]}]\n",
		"bad title":   "server: https://m.example\nfeeds: [{url: https://e.example/f, ignore: [{title: '('}]}]\n",
		"bad link":    "server: https://m.example\nfeeds: [{url: https://e.example/f, ignore: [{link: '('}]}]\n",
		"typo in key": "server: https://m.example\nfeeds: [{url: https://e.example/f, ignore: [{titel: x}]}]\n",
	}

	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(write(t, content)); err == nil {
				t.Fatal("want error")
			}
		})
	}
}
