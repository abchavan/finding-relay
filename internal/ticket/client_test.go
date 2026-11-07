package ticket

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"northwick.example/finding-relay/internal/finding"
)

func sampleFinding() finding.Finding {
	return finding.Finding{
		Fingerprint: "fp1", Scanner: "gitleaks", Severity: "critical", Repo: "northwick/ledger",
		Path: "cfg/app.yaml", Line: 7, Title: "Hardcoded key", Description: "a key",
		Evidence: finding.Evidence{Match: "AKIAEXAMPLE"},
	}
}

func TestFromFinding(t *testing.T) {
	tk := FromFinding(sampleFinding())
	if tk.ExternalRef != "fp1" || tk.Labels[0] != "scanner:gitleaks" || tk.Labels[1] != "repo:northwick/ledger" {
		t.Fatalf("unexpected ticket: %+v", tk)
	}
	for _, want := range []string{"cfg/app.yaml:7", "a key", "Match:", "AKIA*******"} {
		if !strings.Contains(tk.Body, want) {
			t.Errorf("body missing %q: %q", want, tk.Body)
		}
	}
	if strings.Contains(tk.Body, "AKIAEXAMPLE") {
		t.Errorf("body contains the unmasked secret: %q", tk.Body)
	}
}

func TestCreate(t *testing.T) {
	var got Ticket
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tickets" || r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("bad request: %s %v", r.URL.Path, r.Header)
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"T-1"}`))
	}))
	defer srv.Close()
	id, err := New(srv.URL, "tok", 0).Create(context.Background(), FromFinding(sampleFinding()))
	if err != nil || id != "T-1" {
		t.Fatalf("id=%q err=%v", id, err)
	}
	if got.ExternalRef != "fp1" {
		t.Fatalf("server saw %+v", got)
	}
}

func TestCreateRateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()
	_, err := New(srv.URL, "tok", 0).Create(context.Background(), Ticket{})
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("got %v", err)
	}
}

func TestCreateServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	_, err := New(srv.URL, "tok", 0).Create(context.Background(), Ticket{})
	if err == nil || errors.Is(err, ErrRateLimited) || !strings.Contains(err.Error(), "503") {
		t.Fatalf("got %v", err)
	}
}

func TestFindByExternalRef(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Query().Get("external_ref") != "fp1" || r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("bad request: %s %s %v", r.Method, r.URL, r.Header)
		}
		_, _ = w.Write([]byte(`[{"id":"T-9","external_ref":"other"},{"id":"T-1","external_ref":"fp1"}]`))
	}))
	defer srv.Close()
	id, err := New(srv.URL, "tok", 0).FindByExternalRef(context.Background(), "fp1")
	if err != nil || id != "T-1" {
		t.Fatalf("id=%q err=%v", id, err)
	}
}

func TestFindByExternalRefNone(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"T-9","external_ref":"other"}]`))
	}))
	defer srv.Close()
	id, err := New(srv.URL, "tok", 0).FindByExternalRef(context.Background(), "fp1")
	if err != nil || id != "" {
		t.Fatalf("id=%q err=%v", id, err)
	}
}

func TestClientRespectsRate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()
	c := New(srv.URL, "tok", 20)
	start := time.Now()
	for i := 0; i < 5; i++ {
		if _, err := c.FindByExternalRef(context.Background(), "fp"); err != nil {
			t.Fatal(err)
		}
	}
	// 5 requests at 20/s: the first goes at once, the next four 50ms apart.
	if d := time.Since(start); d < 190*time.Millisecond {
		t.Fatalf("5 requests took %v, limiter not applied", d)
	}
}

func TestFromFindingV2LinksEvidence(t *testing.T) {
	f := sampleFinding()
	f.Evidence.Match = ""
	f.MatchRef = "https://evidence.northwick.example/v1/findings/x/match"
	tk := FromFinding(f)
	if !strings.Contains(tk.Body, "Evidence: https://evidence.northwick.example/v1/findings/x/match") || strings.Contains(tk.Body, "Match:") {
		t.Fatalf("body = %q", tk.Body)
	}
}
