// Package finding defines the scanner payload carried on findings.* subjects.
package finding

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Evidence is the matched text and its surrounding lines.
type Evidence struct {
	Match   string `json:"match"`
	Context string `json:"context"`
}

// Finding is one scanner result.
type Finding struct {
	ID          string    `json:"id"`
	Fingerprint string    `json:"fingerprint"`
	Scanner     string    `json:"scanner"`
	Rule        string    `json:"rule"`
	Severity    string    `json:"severity"`
	Repo        string    `json:"repo"`
	Path        string    `json:"path"`
	Line        int       `json:"line"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Evidence    Evidence  `json:"evidence"`
	EmittedAt   time.Time `json:"emitted_at"`
}

// ErrNoFingerprint is returned when the payload has no fingerprint.
var ErrNoFingerprint = errors.New("finding: fingerprint is required")

// Parse decodes a message payload into a Finding.
func Parse(data []byte) (Finding, error) {
	var f Finding
	if err := json.Unmarshal(data, &f); err != nil {
		return Finding{}, fmt.Errorf("finding: decode: %w", err)
	}
	if f.Fingerprint == "" {
		return Finding{}, ErrNoFingerprint
	}
	return f, nil
}
