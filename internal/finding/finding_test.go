package finding

import (
	"errors"
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
