// Package ticket talks to the ticketing platform.
package ticket

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"northwick.example/finding-relay/internal/finding"
)

// ErrRateLimited is returned when the platform answers 429.
var ErrRateLimited = errors.New("ticket: rate limited")

// Ticket is the create request body.
type Ticket struct {
	Title       string   `json:"title"`
	Body        string   `json:"body"`
	Severity    string   `json:"severity"`
	ExternalRef string   `json:"external_ref"`
	Labels      []string `json:"labels"`
}

// FromFinding maps a finding to a ticket.
func FromFinding(f finding.Finding) Ticket {
	var b strings.Builder
	fmt.Fprintf(&b, "%s:%d\n\n%s\n\nMatch:\n%s\n", f.Path, f.Line, f.Description, f.Evidence.Match)
	return Ticket{
		Title:       f.Title,
		Body:        b.String(),
		Severity:    f.Severity,
		ExternalRef: f.Fingerprint,
		Labels:      []string{"scanner:" + f.Scanner, "repo:" + f.Repo},
	}
}

// Client creates tickets.
type Client struct {
	url   string
	token string
	http  *http.Client
}

// New returns a Client for the platform at url.
func New(url, token string) *Client {
	return &Client{url: strings.TrimRight(url, "/"), token: token, http: &http.Client{Timeout: 5 * time.Second}}
}

// Create posts a ticket and returns its id.
func (c *Client) Create(ctx context.Context, t Ticket) (string, error) {
	payload, err := json.Marshal(t)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url+"/tickets", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		return "", ErrRateLimited
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("ticket: create failed: status %d", resp.StatusCode)
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("ticket: decode response: %w", err)
	}
	return out.ID, nil
}
