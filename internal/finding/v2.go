package finding

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// severities maps the v2 integer severity to the v1 names the ticket API and
// chat already use.
var severities = []string{"info", "low", "medium", "high", "critical"}

// findingV2 is the payload of contracts/findings.v2.asyncapi.yaml (draft of
// 2026-10-22). legacy_fingerprint is not in the draft; it has been asked for so
// v2 findings can be matched to tickets opened under v1.
type findingV2 struct {
	SchemaVersion int    `json:"schema_version"`
	ID            string `json:"id"`
	Fingerprint   string `json:"fingerprint"`
	Scanner       string `json:"scanner"`
	Rule          string `json:"rule"`
	Severity      *int   `json:"severity"`
	Repo          string `json:"repo"`
	Path          string `json:"path"`
	Line          int    `json:"line"`
	Title         string `json:"title"`
	Description   string `json:"description"`
	Evidence      struct {
		MatchRef string `json:"match_ref"`
	} `json:"evidence"`
	LegacyFingerprint string    `json:"legacy_fingerprint"`
	EmittedAt         time.Time `json:"emitted_at"`
}

func parseV2(data []byte) (Finding, error) {
	var v findingV2
	var lineErr error
	if err := json.Unmarshal(data, &v); err != nil {
		var typeErr *json.UnmarshalTypeError
		if !errors.As(err, &typeErr) || typeErr.Field != "line" {
			return Finding{}, fmt.Errorf("finding: decode v2: %w", err)
		}
		v.Line = 0
		lineErr = ErrBadLine
	}
	if v.Fingerprint == "" {
		return Finding{}, ErrNoFingerprint
	}
	if v.Severity == nil || *v.Severity < 0 || *v.Severity >= len(severities) {
		return Finding{}, fmt.Errorf("finding: v2 severity out of range")
	}
	return Finding{
		ID:                v.ID,
		Fingerprint:       v.Fingerprint,
		Scanner:           v.Scanner,
		Rule:              v.Rule,
		Severity:          severities[*v.Severity],
		Repo:              v.Repo,
		Path:              v.Path,
		Line:              v.Line,
		Title:             v.Title,
		Description:       v.Description,
		EmittedAt:         v.EmittedAt,
		Version:           2,
		MatchRef:          v.Evidence.MatchRef,
		LegacyFingerprint: v.LegacyFingerprint,
	}, lineErr
}
