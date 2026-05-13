// Package chat posts notifications to the chat webhook.
package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"northwick.example/finding-relay/internal/finding"
)

// Message is the webhook payload.
type Message struct {
	Text  string `json:"text"`
	Match string `json:"match"`
}

// FromFinding builds the notification for a ticketed finding.
func FromFinding(f finding.Finding, ticketID string) Message {
	return Message{
		Text:  fmt.Sprintf("%s %s in %s (%s)", f.Severity, f.Title, f.Repo, ticketID),
		Match: f.Evidence.Match,
	}
}

// Client posts messages to a webhook URL.
type Client struct {
	url  string
	http *http.Client
}

// New returns a Client for the webhook at url.
func New(url string) *Client {
	return &Client{url: url, http: &http.Client{Timeout: 5 * time.Second}}
}

// Post sends a message.
func (c *Client) Post(ctx context.Context, m Message) error {
	payload, err := json.Marshal(m)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("chat: webhook failed: status %d", resp.StatusCode)
	}
	return nil
}
