// Package ticket talks to the ticketing platform.
package ticket

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"northwick.example/finding-relay/internal/finding"
)

// Creates have been seen to take 14s on the platform side while it stays
// healthy (2026-10-15). Giving up earlier than that just opens a second
// ticket on retry. Search, create and the chat post together stay inside the
// consumer's 30s AckWait.
const (
	searchTimeout = 5 * time.Second
	createTimeout = 15 * time.Second
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
	fmt.Fprintf(&b, "%s:%d\n\n%s\n\nMatch:\n%s\n", f.Path, f.Line, f.Description, f.SafeMatch())
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
	url     string
	token   string
	http    *http.Client
	limiter *limiter
}

// New returns a Client for the platform at baseURL. perSecond caps the
// request rate of this client; 0 disables the cap.
func New(baseURL, token string, perSecond float64) *Client {
	return &Client{
		url:     strings.TrimRight(baseURL, "/"),
		token:   token,
		http:    &http.Client{},
		limiter: newLimiter(perSecond),
	}
}

// FindByExternalRef returns the id of an existing ticket with the given
// external_ref, or "" if there is none.
func (c *Client) FindByExternalRef(ctx context.Context, ref string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, searchTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url+"/tickets?external_ref="+url.QueryEscape(ref), nil)
	if err != nil {
		return "", err
	}
	resp, err := c.do(ctx, req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if err := checkStatus(resp, "search"); err != nil {
		return "", err
	}
	var out []struct {
		ID          string `json:"id"`
		ExternalRef string `json:"external_ref"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("ticket: decode search response: %w", err)
	}
	// Match on the field as well, in case the filter is not applied server side.
	for _, t := range out {
		if t.ExternalRef == ref {
			return t.ID, nil
		}
	}
	return "", nil
}

// Create posts a ticket and returns its id.
func (c *Client) Create(ctx context.Context, t Ticket) (string, error) {
	payload, err := json.Marshal(t)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url+"/tickets", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.do(ctx, req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if err := checkStatus(resp, "create"); err != nil {
		return "", err
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("ticket: decode response: %w", err)
	}
	return out.ID, nil
}

func (c *Client) do(ctx context.Context, req *http.Request) (*http.Response, error) {
	if err := c.limiter.wait(ctx); err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	return c.http.Do(req)
}

func checkStatus(resp *http.Response, op string) error {
	if resp.StatusCode == http.StatusTooManyRequests {
		return ErrRateLimited
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("ticket: %s failed: status %d", op, resp.StatusCode)
	}
	return nil
}
