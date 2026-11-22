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
