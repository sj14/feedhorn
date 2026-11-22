package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sj14/feedhorn/internal/config"
	"github.com/sj14/feedhorn/internal/feed"
	"github.com/sj14/feedhorn/internal/mastodon"
	"github.com/sj14/feedhorn/internal/state"
	"github.com/sj14/feedhorn/internal/status"
)

const userAgent = "feedhorn (+https://github.com/sj14/feedhorn)"

func lookupEnvString(key string, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return defaultVal
}

func lookupEnvDuration(key string, defaultVal time.Duration) time.Duration {
	if val, ok := os.LookupEnv(key); ok {
		duration, err := time.ParseDuration(val)
		if err != nil {
			slog.Error("failed parsing duration", "value", val, "key", key, "err", err)
			os.Exit(1)
		}
		return duration
	}
	return defaultVal
}

func main() {
	var (
		configPath     = flag.String("config", lookupEnvString("CONFIG", "config.yaml"), "path to the config file")
		statePath      = flag.String("state", lookupEnvString("STATE", "state.json"), "path to the state file")
		stateRetention = flag.Duration("state-retention", lookupEnvDuration("STATE_RETENTION", 90*24*time.Hour), "how long to remember items a feed stopped listing")
		updateInterval = flag.Duration("interval", lookupEnvDuration("INTERVAL", 15*time.Minute), "update interval")
		logLevel       = flag.String("log-level", lookupEnvString("LOG_LEVEL", "info"), "log level (debug, info, warn, error)")
	)
	flag.Parse()

	var level slog.Level
	if err := level.UnmarshalText([]byte(*logLevel)); err != nil {
		slog.Error("failed parsing log level", "value", *logLevel, "err", err)
		os.Exit(1)
	}

	slog.SetLogLoggerLevel(level)

	// the token is a secret, so it is only taken from the environment and
	// never from a flag, which would show up in the process list
	token := os.Getenv("MASTODON_TOKEN")
	if token == "" {
		slog.Error("MASTODON_TOKEN is not set")
		os.Exit(1)
	}

	conf, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed loading config", "path", *configPath, "err", err)
		os.Exit(1)
	}

	known, err := state.Load(*statePath)
	if err != nil {
		slog.Error("failed loading state", "path", *statePath, "err", err)
		os.Exit(1)
	}

	// a feed that was removed from the config and added back later should
	// count as new, instead of posting everything it published meanwhile
	configured := make(map[string]bool, len(conf.Feeds))
	for _, feedConf := range conf.Feeds {
		configured[feedConf.URL] = true
	}

	for feedURL := range known {
		if !configured[feedURL] {
			delete(known, feedURL)
		}
	}

	client := &http.Client{Timeout: 30 * time.Second}

	r := relay{
		client:    client,
		statePath: *statePath,
		retention: *stateRetention,
		known:     known,
		poster: &mastodon.Client{
			HTTP:       client,
			Server:     conf.Server,
			Token:      token,
			UserAgent:  userAgent,
			Visibility: conf.Visibility,
		},
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	slog.Info("started", "feeds", len(conf.Feeds), "interval", *updateInterval)

	for {
		for _, feedConf := range conf.Feeds {
			if err := r.processFeed(ctx, feedConf); err != nil {
				if ctx.Err() != nil {
					break
				}

				slog.Error("failed processing feed", "feed", feedConf.URL, "err", err)
			}
		}

		// the interval starts once every feed is done, so a slow feed does not
		// end up being processed twice at the same time
		timer := time.NewTimer(*updateInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			slog.Info("shut down")
			return
		case <-timer.C:
		}
	}
}

type relay struct {
	client    *http.Client
	poster    *mastodon.Client
	statePath string
	retention time.Duration
	known     state.State
}

// processFeed posts the items of a feed that are not known yet, oldest first.
//
// The first time a feed is seen nothing is posted, only its current items are
// remembered; otherwise adding a feed would flood the account with its whole
// history. A failed post ends the feed for this round, so the remaining items
// are retried in the next one and still come in order.
//
// An ignored item is remembered like a posted one, so removing the rule later
// does not post every ignored item the feed still lists.
func (r *relay) processFeed(ctx context.Context, feedConf config.Feed) error {
	feedURL := feedConf.URL

	items, err := feed.Fetch(ctx, r.client, userAgent, feedURL)
	if err != nil {
		return err
	}

	now := time.Now().UTC()

	listed := make(map[string]bool, len(items))
	for _, item := range items {
		listed[item.ID] = true
	}

	entries, seenBefore := r.known[feedURL]
	if !seenBefore {
		entries = make([]state.Entry, 0, len(items))
		for _, item := range items {
			entries = append(entries, state.Entry{ID: item.ID, LastSeen: now})
		}

		r.known[feedURL] = entries
		slog.Info("new feed, remembering its items without posting them", "feed", feedURL, "items", len(items))

		return state.Save(r.statePath, r.known)
	}

	knownIDs := make(map[string]bool, len(entries))
	for _, entry := range entries {
		knownIDs[entry.ID] = true
	}

	for _, item := range items {
		if knownIDs[item.ID] {
			continue
		}

		if feedConf.Ignores(item) {
			slog.Info("ignored", "feed", feedURL, "title", item.Title, "link", item.Link)

			knownIDs[item.ID] = true
			entries = append(entries, state.Entry{ID: item.ID, LastSeen: now})
			r.known[feedURL] = entries

			continue
		}

		text, truncated, err := status.Render(feedConf.Status, item)
		if err != nil {
			return fmt.Errorf("render %q: %w", item.ID, err)
		}

		if truncated {
			slog.Warn("post too long, cut off at the end; shorten the template", "feed", feedURL, "link", item.Link)
		}

		if err := r.poster.Post(ctx, text, idempotencyKey(feedURL, item.ID)); err != nil {
			return fmt.Errorf("post %q: %w", item.ID, err)
		}

		slog.Info("posted", "feed", feedURL, "title", item.Title, "link", item.Link)

		// saved after every post, so a restart in the middle of a batch does
		// not post the first ones again
		knownIDs[item.ID] = true
		entries = append(entries, state.Entry{ID: item.ID, LastSeen: now})
		r.known[feedURL] = entries

		if err := state.Save(r.statePath, r.known); err != nil {
			return err
		}
	}

	r.known[feedURL] = state.Refresh(entries, listed, now, r.retention)

	return state.Save(r.statePath, r.known)
}

func idempotencyKey(feedURL, itemID string) string {
	sum := sha256.Sum256([]byte(feedURL + "\n" + itemID))
	return hex.EncodeToString(sum[:])
}
