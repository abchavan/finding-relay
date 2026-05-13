package ticket

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
	for _, want := range []string{"cfg/app.yaml:7", "a key", "Match:", "AKIAEXAMPLE"} {
		if !strings.Contains(tk.Body, want) {
			t.Errorf("body missing %q: %q", want, tk.Body)
		}
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
	id, err := New(srv.URL, "tok").Create(context.Background(), FromFinding(sampleFinding()))
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
	_, err := New(srv.URL, "tok").Create(context.Background(), Ticket{})
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("got %v", err)
	}
}

func TestCreateServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	_, err := New(srv.URL, "tok").Create(context.Background(), Ticket{})
	if err == nil || errors.Is(err, ErrRateLimited) || !strings.Contains(err.Error(), "503") {
		t.Fatalf("got %v", err)
	}
}
