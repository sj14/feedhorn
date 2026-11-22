package feed

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func serve(t *testing.T, body string) string {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, body)
	}))
	t.Cleanup(srv.Close)

	return srv.URL
}

func ids(items []Item) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		result = append(result, item.ID)
	}
	return result
}

func TestFetchSortsByDate(t *testing.T) {
	url := serve(t, `<?xml version="1.0"?><rss version="2.0"><channel><title>t</title>
		<item><guid>b</guid><link>https://example.com/b</link><pubDate>Tue, 03 Nov 2026 10:00:00 GMT</pubDate></item>
		<item><guid>c</guid><link>https://example.com/c</link><pubDate>Wed, 04 Nov 2026 10:00:00 GMT</pubDate></item>
		<item><guid>a</guid><link>https://example.com/a</link><pubDate>Mon, 02 Nov 2026 10:00:00 GMT</pubDate></item>
	</channel></rss>`)

	items, err := Fetch(t.Context(), http.DefaultClient, "test", url)
	if err != nil {
		t.Fatal(err)
	}

	if got := fmt.Sprint(ids(items)); got != "[a b c]" {
		t.Fatalf("got order %s, want [a b c]", got)
	}
}

func TestFetchWithoutDatesReversesOrder(t *testing.T) {
	url := serve(t, `<?xml version="1.0"?><rss version="2.0"><channel><title>t</title>
		<item><link>https://example.com/new</link></item>
		<item><link>https://example.com/old</link></item>
		<item><title>neither guid nor link</title></item>
	</channel></rss>`)

	items, err := Fetch(t.Context(), http.DefaultClient, "test", url)
	if err != nil {
		t.Fatal(err)
	}

	// the link stands in for the missing guid, an item without either is skipped
	if got := fmt.Sprint(ids(items)); got != "[https://example.com/old https://example.com/new]" {
		t.Fatalf("got %s", got)
	}
}
