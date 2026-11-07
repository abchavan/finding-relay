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

const sampleV2 = `{"schema_version":2,"id":"0c7a9e52-1d4b-4f8e-a6c3-5e2f9b7d1a40","fingerprint":"5d41402abc4b2a76b9719d911017c592e99f0a3bd2c4a7e8f1b6c5d4a3b2c1d0","scanner":"semgrep","rule":"go.lang.security.audit.net.use-tls.use-tls","severity":4,"repo":"northwick/merchant-portal","path":"internal/server/server.go","line":88,"title":"HTTP server started without TLS","description":"d","evidence":{"match_ref":"https://evidence.northwick.example/v1/findings/0c7a9e52-1d4b-4f8e-a6c3-5e2f9b7d1a40/match"},"emitted_at":"2026-11-02T02:14:07Z"}`

func TestParseV1HasVersion1(t *testing.T) {
	f, err := Parse([]byte(sample))
	if err != nil || f.Version != 1 {
		t.Fatalf("version=%d err=%v", f.Version, err)
	}
}

func TestParseV2(t *testing.T) {
	f, err := Parse([]byte(sampleV2))
	if err != nil {
		t.Fatal(err)
	}
	if f.Version != 2 || f.Severity != "critical" || f.Line != 88 || f.Evidence.Match != "" ||
		f.MatchRef != "https://evidence.northwick.example/v1/findings/0c7a9e52-1d4b-4f8e-a6c3-5e2f9b7d1a40/match" {
		t.Fatalf("unexpected finding: %+v", f)
	}
	if f.LegacyFingerprint != "" {
		t.Fatalf("legacy = %q", f.LegacyFingerprint)
	}
}

func TestParseV2Legacy(t *testing.T) {
	data := strings.Replace(sampleV2, `"schema_version":2,`, `"schema_version":2,"legacy_fingerprint":"0123456789abcdef0123456789abcdef01234567",`, 1)
	f, err := Parse([]byte(data))
	if err != nil || f.LegacyFingerprint != "0123456789abcdef0123456789abcdef01234567" {
		t.Fatalf("legacy=%q err=%v", f.LegacyFingerprint, err)
	}
}

func TestParseV2SeverityOutOfRange(t *testing.T) {
	for _, sev := range []string{`"severity":5`, `"severity":-1`, `"severity":"high"`} {
		data := strings.Replace(sampleV2, `"severity":4`, sev, 1)
		if _, err := Parse([]byte(data)); err == nil {
			t.Errorf("%s: expected error", sev)
		}
	}
	data := strings.Replace(sampleV2, `"severity":4,`, ``, 1)
	if _, err := Parse([]byte(data)); err == nil {
		t.Error("missing severity: expected error")
	}
}
