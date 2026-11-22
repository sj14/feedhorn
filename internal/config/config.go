// Package config holds the Mastodon account and the feeds feedhorn posts from.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"text/template"

	"github.com/goccy/go-yaml"
	"github.com/sj14/feedhorn/internal/status"
)

type Config struct {
	// Server is the base URL of the Mastodon instance, e.g. https://mastodon.social.
	Server string `yaml:"server"`
	// Visibility of the posts: public, unlisted, private or direct.
	Visibility string `yaml:"visibility"`
	// Template is the text of a post for every feed that sets none itself.
	Template string `yaml:"template"`
	Feeds    []Feed `yaml:"feeds"`
}

type Feed struct {
	URL string `yaml:"url"`
	// Template overrides the global template for this feed.
	Template string `yaml:"template"`

	// Status is the parsed template, set by Load.
	Status *template.Template `yaml:"-"`
}

const defaultVisibility = "public"

// Load reads the config file. Unknown fields are rejected, so a misspelled
// option fails loudly instead of silently doing nothing.
func Load(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	var config Config
	if err := yaml.NewDecoder(bytes.NewReader(b), yaml.DisallowUnknownField()).Decode(&config); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}

	if config.Visibility == "" {
		config.Visibility = defaultVisibility
	}

	if config.Template == "" {
		config.Template = status.Default
	}

	// the API paths are appended to the server, a trailing slash would double it
	config.Server = strings.TrimSuffix(config.Server, "/")

	if err := config.validate(); err != nil {
		return Config{}, err
	}

	for i := range config.Feeds {
		feed := &config.Feeds[i]

		if feed.Template == "" {
			feed.Template = config.Template
		}

		feed.Status, err = status.Parse(feed.Template)
		if err != nil {
			return Config{}, fmt.Errorf("feed %q: %w", feed.URL, err)
		}
	}

	return config, nil
}

func (c Config) validate() error {
	server, err := url.Parse(c.Server)
	if err != nil || server.Host == "" || (server.Scheme != "https" && server.Scheme != "http") {
		return fmt.Errorf("server %q is not an http(s) URL", c.Server)
	}

	switch c.Visibility {
	case "public", "unlisted", "private", "direct":
	default:
		return fmt.Errorf("unknown visibility %q", c.Visibility)
	}

	if len(c.Feeds) == 0 {
		return errors.New("no feeds configured")
	}

	// the feed URL is the key of the stored state, a duplicate would post
	// every new item twice
	seen := make(map[string]bool, len(c.Feeds))
	for _, feed := range c.Feeds {
		if feed.URL == "" {
			return errors.New("feed without url")
		}

		if seen[feed.URL] {
			return fmt.Errorf("feed %q is listed twice", feed.URL)
		}
		seen[feed.URL] = true
	}

	return nil
}
