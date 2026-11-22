package config

import (
	"os"
	"path/filepath"
	"testing"

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
