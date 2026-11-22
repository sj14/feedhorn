// Package feed fetches RSS, Atom and JSON feeds and reduces their entries to
// what feedhorn posts.
package feed

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
)

// Item is one feed entry. ID identifies it across fetches: the GUID if the
// feed has one, the link otherwise.
type Item struct {
	ID    string
	Title string
	Link  string
}

// Fetch downloads and parses a feed. The items come oldest first, the order
// in which they are posted.
func Fetch(ctx context.Context, client *http.Client, userAgent, url string) ([]Item, error) {
	parser := gofeed.NewParser()
	parser.Client = client
	parser.UserAgent = userAgent

	parsed, err := parser.ParseURLWithContext(url, ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch feed: %w", err)
	}

	sortOldestFirst(parsed.Items)

	items := make([]Item, 0, len(parsed.Items))
	for _, entry := range parsed.Items {
		item := Item{
			ID:    entry.GUID,
			Title: strings.TrimSpace(entry.Title),
			Link:  strings.TrimSpace(entry.Link),
		}

		if item.ID == "" {
			item.ID = item.Link
		}

		// without an identity there is no way to tell whether it was posted
		if item.ID == "" {
			continue
		}

		items = append(items, item)
	}

	return items, nil
}

// sortOldestFirst sorts by date when every entry has one. Otherwise it only
// reverses the list, since feeds put their newest entry first.
func sortOldestFirst(entries []*gofeed.Item) {
	for _, entry := range entries {
		if date(entry) == nil {
			slices.Reverse(entries)
			return
		}
	}

	slices.SortStableFunc(entries, func(a, b *gofeed.Item) int {
		return date(a).Compare(*date(b))
	})
}

func date(entry *gofeed.Item) *time.Time {
	if entry.PublishedParsed != nil {
		return entry.PublishedParsed
	}
	return entry.UpdatedParsed
}
