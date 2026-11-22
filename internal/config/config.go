// Package config holds the Mastodon account and the feeds feedhorn posts from.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/goccy/go-yaml"
)

type Config struct {
	// Server is the base URL of the Mastodon instance, e.g. https://mastodon.social.
	Server string `yaml:"server"`
	// Visibility of the posts: public, unlisted, private or direct.
	Visibility string   `yaml:"visibility"`
	Feeds      []string `yaml:"feeds"`
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

	// the API paths are appended to the server, a trailing slash would double it
	config.Server = strings.TrimSuffix(config.Server, "/")

	if err := config.validate(); err != nil {
		return Config{}, err
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
		if seen[feed] {
			return fmt.Errorf("feed %q is listed twice", feed)
		}
		seen[feed] = true
	}

	return nil
}
