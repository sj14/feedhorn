// Package config holds the Mastodon account and the feeds feedhorn posts from.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"text/template"

	"github.com/goccy/go-yaml"
	"github.com/sj14/feedhorn/internal/feed"
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
	// Ignore lists the items that are not posted.
	Ignore []Rule `yaml:"ignore"`

	// Status is the parsed template, set by Load.
	Status *template.Template `yaml:"-"`
}

// Rule matches items by regular expressions on their title and link. A rule
// that sets both only matches items where both match.
type Rule struct {
	Title string `yaml:"title"`
	Link  string `yaml:"link"`

	// the compiled expressions, set by Load; nil for a field the rule leaves out
	title *regexp.Regexp
	link  *regexp.Regexp
}

// Ignores reports whether one of the feed's ignore rules matches the item.
func (f Feed) Ignores(item feed.Item) bool {
	for _, rule := range f.Ignore {
		if rule.matches(item) {
			return true
		}
	}

	return false
}

func (r Rule) matches(item feed.Item) bool {
	if r.title != nil && !r.title.MatchString(item.Title) {
		return false
	}

	if r.link != nil && !r.link.MatchString(item.Link) {
		return false
	}

	return true
}

// compile parses the expressions. A rule without any would match every item,
// which is far more likely a mistake than intended.
func (r *Rule) compile() error {
	if r.Title == "" && r.Link == "" {
		return errors.New("ignore rule without title or link")
	}

	var err error

	if r.Title != "" {
		if r.title, err = regexp.Compile(r.Title); err != nil {
			return fmt.Errorf("ignore title: %w", err)
		}
	}

	if r.Link != "" {
		if r.link, err = regexp.Compile(r.Link); err != nil {
			return fmt.Errorf("ignore link: %w", err)
		}
	}

	return nil
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
		feedConf := &config.Feeds[i]

		if feedConf.Template == "" {
			feedConf.Template = config.Template
		}

		feedConf.Status, err = status.Parse(feedConf.Template)
		if err != nil {
			return Config{}, fmt.Errorf("feed %q: %w", feedConf.URL, err)
		}

		for j := range feedConf.Ignore {
			if err := feedConf.Ignore[j].compile(); err != nil {
				return Config{}, fmt.Errorf("feed %q: %w", feedConf.URL, err)
			}
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
	for _, feedConf := range c.Feeds {
		if feedConf.URL == "" {
			return errors.New("feed without url")
		}

		if seen[feedConf.URL] {
			return fmt.Errorf("feed %q is listed twice", feedConf.URL)
		}
		seen[feedConf.URL] = true
	}

	return nil
}
