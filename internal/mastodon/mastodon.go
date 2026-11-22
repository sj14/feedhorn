// Package mastodon posts statuses through the Mastodon API.
package mastodon

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Mastodon's default limit. Other servers may allow more, but none allow less.
const maxCharacters = 500

// Mastodon counts every link as this many characters, whatever its length.
const linkCharacters = 23

type Client struct {
	HTTP       *http.Client
	Server     string
	Token      string
	UserAgent  string
	Visibility string
}

// Post publishes a status. The idempotency key makes Mastodon ignore a repeat
// of the same request, e.g. a retry after the response got lost, for an hour.
func (c *Client) Post(ctx context.Context, status, idempotencyKey string) error {
	form := url.Values{
		"status":     {status},
		"visibility": {c.Visibility},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Server+"/api/v1/statuses", strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Idempotency-Key", idempotencyKey)
	req.Header.Set("User-Agent", c.UserAgent)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("send status: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// the body holds Mastodon's error message, a short excerpt is enough
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("send status: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	return nil
}

// Status builds the text of a post: the title, a blank line and the link. A
// title too long for the limit is cut off with an ellipsis; the link is kept
// whole since it always counts the same.
func Status(title, link string) string {
	if link == "" {
		return truncate(title, maxCharacters)
	}

	if title == "" {
		return link
	}

	separator := "\n\n"
	budget := maxCharacters - linkCharacters - len(separator)

	return truncate(title, budget) + separator + link
}

func truncate(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}

	return string(runes[:limit-1]) + "…"
}
