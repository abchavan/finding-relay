package finding

import (
	"errors"
	"strings"
	"testing"
)

const sample = `{"id":"6f1c2a9e-0000-4000-8000-000000000001","fingerprint":"0123456789abcdef0123456789abcdef01234567","scanner":"semgrep","rule":"go.sql.injection","severity":"high","repo":"northwick/ledger","path":"db/query.go","line":42,"title":"SQL built from user input","description":"d","evidence":{"match":"m","context":"c"},"emitted_at":"2026-10-01T12:00:00Z"}`

func TestParse(t *testing.T) {
	f, err := Parse([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	if f.Repo != "northwick/ledger" || f.Line != 42 || f.Evidence.Match != "m" {
		t.Fatalf("unexpected finding: %+v", f)
	}
	if f.EmittedAt.Year() != 2026 {
		t.Fatalf("emitted_at not parsed: %v", f.EmittedAt)
	}
}

func TestParseBadJSON(t *testing.T) {
	if _, err := Parse([]byte("{nope")); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseRequiresFingerprint(t *testing.T) {
	_, err := Parse([]byte(`{"id":"x","severity":"low"}`))
	if !errors.Is(err, ErrNoFingerprint) {
		t.Fatalf("got %v", err)
	}
}

func TestParseToleratesBadLine(t *testing.T) {
	data := strings.Replace(sample, `"line":42`, `"line":"forty-two"`, 1)
	f, err := Parse([]byte(data))
	if !errors.Is(err, ErrBadLine) {
		t.Fatalf("got %v", err)
	}
	if f.Fingerprint != "0123456789abcdef0123456789abcdef01234567" || f.Line != 0 || f.Severity != "high" {
		t.Fatalf("unexpected finding: %+v", f)
	}
}

func TestSafeMatch(t *testing.T) {
	f := Finding{Scanner: "gitleaks", Evidence: Evidence{Match: "sk_live_abcdefgh"}}
	if got := f.SafeMatch(); got != "sk_l************" {
		t.Fatalf("got %q", got)
	}
	f.Scanner = "semgrep"
	if got := f.SafeMatch(); got != "sk_live_abcdefgh" {
		t.Fatalf("got %q", got)
	}
}
