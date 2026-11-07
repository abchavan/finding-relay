// Package finding defines the scanner payload carried on findings.* subjects
// (contract v1) and findings.v2.* subjects (contract v2).
package finding

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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

	// Set from the payload, not part of the v1 wire format.
	Version           int    `json:"-"` // contract version, 1 or 2
	MatchRef          string `json:"-"` // v2: link to the match in the evidence vault
	LegacyFingerprint string `json:"-"` // v2: the v1 fingerprint of the same finding, if sent
}

// ErrNoFingerprint is returned when the payload has no fingerprint.
var ErrNoFingerprint = errors.New("finding: fingerprint is required")

// SafeMatch returns the matched text, masked when it is a leaked credential,
// so the secret is not copied into tickets, chat or logs.
func (f Finding) SafeMatch() string {
	if f.Scanner != "gitleaks" {
		return f.Evidence.Match
	}
	return mask(f.Evidence.Match)
}

func mask(s string) string {
	if len(s) <= 8 {
		return strings.Repeat("*", len(s))
	}
	return s[:4] + strings.Repeat("*", len(s)-4)
}

// ErrBadLine is returned with a usable Finding when the payload's line is not
// an integer. Line is left at 0; the rest of the finding is intact.
var ErrBadLine = errors.New("finding: line is not an integer")

// Parse decodes a v1 or v2 payload into a Finding.
func Parse(data []byte) (Finding, error) {
	var head struct {
		SchemaVersion int `json:"schema_version"`
	}
	if err := json.Unmarshal(data, &head); err == nil && head.SchemaVersion == 2 {
		return parseV2(data)
	}
	return parseV1(data)
}

func parseV1(data []byte) (Finding, error) {
	f := Finding{Version: 1}
	var lineErr error
	if err := json.Unmarshal(data, &f); err != nil {
		var typeErr *json.UnmarshalTypeError
		if !errors.As(err, &typeErr) || typeErr.Field != "line" {
			return Finding{}, fmt.Errorf("finding: decode: %w", err)
		}
		f.Line = 0
		lineErr = ErrBadLine
	}
	if f.Fingerprint == "" {
		return Finding{}, ErrNoFingerprint
	}
	f.Version = 1
	return f, lineErr
}
