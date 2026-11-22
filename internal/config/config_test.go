package config

import (
	"os"
	"path/filepath"
	"testing"
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
  - https://example.com/feed.xml
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
}

func TestLoadRejects(t *testing.T) {
	tests := map[string]string{
		"unknown field": "server: https://m.example\nfeeds: [https://e.example/f]\ntypo: 1\n",
		"no server":     "feeds: [https://e.example/f]\n",
		"bad server":    "server: mastodon.example\nfeeds: [https://e.example/f]\n",
		"no feeds":      "server: https://m.example\n",
		"duplicate":     "server: https://m.example\nfeeds: [https://e.example/f, https://e.example/f]\n",
		"visibility":    "server: https://m.example\nvisibility: everyone\nfeeds: [https://e.example/f]\n",
	}

	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(write(t, content)); err == nil {
				t.Fatal("want error")
			}
		})
	}
}
