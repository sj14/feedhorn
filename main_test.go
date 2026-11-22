package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sj14/feedhorn/internal/mastodon"
	"github.com/sj14/feedhorn/internal/state"
)

// fakeFeed serves an RSS feed whose items can be changed between rounds.
type fakeFeed struct {
	mu    sync.Mutex
	links []string // newest first, like a real feed
}

func (f *fakeFeed) set(links ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.links = links
}

func (f *fakeFeed) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	var items strings.Builder
	for _, link := range f.links {
		fmt.Fprintf(&items, "<item><title>title of %s</title><link>%s</link></item>", link, link)
	}

	fmt.Fprintf(w, `<?xml version="1.0"?><rss version="2.0"><channel><title>test</title>%s</channel></rss>`, items.String())
}

// fakeMastodon records the posted statuses and fails while failing is set.
type fakeMastodon struct {
	mu       sync.Mutex
	statuses []string
	failing  bool
}

func (m *fakeMastodon) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.failing {
		http.Error(w, `{"error":"unavailable"}`, http.StatusServiceUnavailable)
		return
	}

	m.statuses = append(m.statuses, r.FormValue("status"))
	fmt.Fprint(w, `{}`)
}

func (m *fakeMastodon) setFailing(failing bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failing = failing
}

func (m *fakeMastodon) posted() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.statuses...)
}

func TestProcessFeed(t *testing.T) {
	feedSrv := &fakeFeed{}
	feedHTTP := httptest.NewServer(feedSrv)
	defer feedHTTP.Close()

	masto := &fakeMastodon{}
	mastoHTTP := httptest.NewServer(masto)
	defer mastoHTTP.Close()

	statePath := filepath.Join(t.TempDir(), "state.json")

	newRelay := func() relay {
		known, err := state.Load(statePath)
		if err != nil {
			t.Fatal(err)
		}

		return relay{
			client:    http.DefaultClient,
			statePath: statePath,
			retention: time.Hour,
			known:     known,
			poster: &mastodon.Client{
				HTTP:       http.DefaultClient,
				Server:     mastoHTTP.URL,
				Token:      "token",
				Visibility: "public",
			},
		}
	}

	r := newRelay()

	// first round only remembers the existing items
	feedSrv.set("https://example.com/2", "https://example.com/1")
	if err := r.processFeed(t.Context(), feedHTTP.URL); err != nil {
		t.Fatal(err)
	}

	if got := masto.posted(); len(got) != 0 {
		t.Fatalf("first round posted %q, want nothing", got)
	}

	// two new items are posted oldest first
	feedSrv.set("https://example.com/4", "https://example.com/3", "https://example.com/2", "https://example.com/1")
	if err := r.processFeed(t.Context(), feedHTTP.URL); err != nil {
		t.Fatal(err)
	}

	want := []string{
		"title of https://example.com/3\n\nhttps://example.com/3",
		"title of https://example.com/4\n\nhttps://example.com/4",
	}
	assertPosted(t, masto.posted(), want)

	// a failed post is retried in the next round
	feedSrv.set("https://example.com/5", "https://example.com/4", "https://example.com/3")
	masto.setFailing(true)
	if err := r.processFeed(t.Context(), feedHTTP.URL); err == nil {
		t.Fatal("want error while mastodon fails")
	}

	masto.setFailing(false)

	// a restart reads the state from disk and posts only the missing item
	r = newRelay()
	if err := r.processFeed(t.Context(), feedHTTP.URL); err != nil {
		t.Fatal(err)
	}

	want = append(want, "title of https://example.com/5\n\nhttps://example.com/5")
	assertPosted(t, masto.posted(), want)
}

func assertPosted(t *testing.T, got, want []string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("posted %q, want %q", got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("post %d is %q, want %q", i, got[i], want[i])
		}
	}
}
