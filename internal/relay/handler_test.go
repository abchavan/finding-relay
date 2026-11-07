package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"northwick.example/finding-relay/internal/chat"
	"northwick.example/finding-relay/internal/finding"
	"northwick.example/finding-relay/internal/ticket"
)

type fakeMsg struct {
	data    []byte
	subject string
	calls   []string
}

func (m *fakeMsg) Data() []byte    { return m.data }
func (m *fakeMsg) Subject() string { return m.subject }
func (m *fakeMsg) Ack() error      { m.calls = append(m.calls, "ack"); return nil }
func (m *fakeMsg) Nak() error      { m.calls = append(m.calls, "nak"); return nil }
func (m *fakeMsg) NakWithDelay(time.Duration) error {
	m.calls = append(m.calls, "nak-delayed")
	return nil
}
func (m *fakeMsg) Term() error { m.calls = append(m.calls, "term"); return nil }

func (m *fakeMsg) only(t *testing.T, want string) {
	t.Helper()
	if len(m.calls) != 1 || m.calls[0] != want {
		t.Fatalf("calls = %v, want [%s]", m.calls, want)
	}
}

type sinks struct {
	mu         sync.Mutex
	tickets    []ticket.Ticket
	messages   []chat.Message
	status     int
	chatStatus int
}

func newHandler(t *testing.T, suppress string) (*Handler, *sinks) {
	t.Helper()
	s := &sinks{status: http.StatusCreated, chatStatus: http.StatusNoContent}
	tsrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.status != http.StatusCreated {
			w.WriteHeader(s.status)
			return
		}
		if r.Method == http.MethodGet {
			ref := r.URL.Query().Get("external_ref")
			out := []map[string]string{}
			for i, tk := range s.tickets {
				if tk.ExternalRef == ref {
					out = append(out, map[string]string{"id": fmt.Sprintf("T-%d", i+1), "external_ref": ref})
				}
			}
			_ = json.NewEncoder(w).Encode(out)
			return
		}
		var tk ticket.Ticket
		_ = json.NewDecoder(r.Body).Decode(&tk)
		s.tickets = append(s.tickets, tk)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"T-` + string(rune('0'+len(s.tickets))) + `"}`))
	}))
	csrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var m chat.Message
		_ = json.NewDecoder(r.Body).Decode(&m)
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.chatStatus != http.StatusNoContent {
			w.WriteHeader(s.chatStatus)
			return
		}
		s.messages = append(s.messages, m)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(tsrv.Close)
	t.Cleanup(csrv.Close)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewHandler(log, ticket.New(tsrv.URL, "tok", 0), chat.New(csrv.URL), ParseSuppress(suppress)), s
}

func msgFor(t *testing.T, fp string) *fakeMsg {
	t.Helper()
	f := finding.Finding{
		ID: "id-" + fp, Fingerprint: fp, Scanner: "semgrep", Severity: "high", Repo: "northwick/ledger",
		Path: "a.go", Line: 3, Title: "Issue", Description: "desc", Evidence: finding.Evidence{Match: "secret-ish"},
	}
	b, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	return &fakeMsg{data: b, subject: "findings.semgrep"}
}

func TestHappyPath(t *testing.T) {
	h, s := newHandler(t, "")
	m := msgFor(t, "fp1")
	h.Handle(context.Background(), m)
	m.only(t, "ack")
	if len(s.tickets) != 1 || len(s.messages) != 1 {
		t.Fatalf("tickets=%d messages=%d", len(s.tickets), len(s.messages))
	}
	if s.tickets[0].ExternalRef != "fp1" {
		t.Fatalf("ticket = %+v", s.tickets[0])
	}
	if want := "high Issue in northwick/ledger (T-1)"; s.messages[0].Text != want || s.messages[0].Match != "secret-ish" {
		t.Fatalf("message = %+v", s.messages[0])
	}
}

func TestDuplicateFingerprint(t *testing.T) {
	h, s := newHandler(t, "")
	h.Handle(context.Background(), msgFor(t, "fp1"))
	m := msgFor(t, "fp1")
	h.Handle(context.Background(), m)
	m.only(t, "ack")
	if len(s.tickets) != 1 {
		t.Fatalf("tickets = %d, want 1", len(s.tickets))
	}
}

func TestBadJSONTerms(t *testing.T) {
	h, s := newHandler(t, "")
	m := &fakeMsg{data: []byte("{nope"), subject: "findings.semgrep"}
	h.Handle(context.Background(), m)
	m.only(t, "term")
	if len(s.tickets) != 0 {
		t.Fatal("ticket created for bad message")
	}
}

func TestSuppressedAcks(t *testing.T) {
	h, s := newHandler(t, "fp1, fp2")
	m := msgFor(t, "fp2")
	h.Handle(context.Background(), m)
	m.only(t, "ack")
	if len(s.tickets) != 0 || len(s.messages) != 0 {
		t.Fatal("suppressed finding reached a sink")
	}
}

func TestRateLimitedRetries(t *testing.T) {
	h, s := newHandler(t, "")
	s.status = http.StatusTooManyRequests
	m := msgFor(t, "fp1")
	h.Handle(context.Background(), m)
	m.only(t, "nak-delayed")
}

func TestTicketErrorNaks(t *testing.T) {
	h, s := newHandler(t, "")
	s.status = http.StatusServiceUnavailable
	m := msgFor(t, "fp1")
	h.Handle(context.Background(), m)
	m.only(t, "nak-delayed")
}

// A ticket that already exists on the platform (another replica, a restart,
// a replay) is not created again and does not ping chat again.
func TestExistingTicketNotRecreated(t *testing.T) {
	h, s := newHandler(t, "")
	s.tickets = append(s.tickets, ticket.Ticket{ExternalRef: "fp1"})
	m := msgFor(t, "fp1")
	h.Handle(context.Background(), m)
	m.only(t, "ack")
	if len(s.tickets) != 1 || len(s.messages) != 0 {
		t.Fatalf("tickets=%d messages=%d, want 1 and 0", len(s.tickets), len(s.messages))
	}
}

// The ticket was created but chat failed: the retry posts to chat without
// opening a second ticket.
func TestChatFailureRetriesChatOnly(t *testing.T) {
	h, s := newHandler(t, "")
	s.chatStatus = http.StatusBadGateway
	m := msgFor(t, "fp1")
	h.Handle(context.Background(), m)
	m.only(t, "nak-delayed")

	s.chatStatus = http.StatusNoContent
	m = msgFor(t, "fp1")
	h.Handle(context.Background(), m)
	m.only(t, "ack")
	if len(s.tickets) != 1 || len(s.messages) != 1 {
		t.Fatalf("tickets=%d messages=%d, want 1 and 1", len(s.tickets), len(s.messages))
	}
}

// Two relay instances share no memory; the platform lookup keeps them from
// opening two tickets for the same fingerprint.
func TestSecondInstanceDoesNotDuplicate(t *testing.T) {
	h1, s := newHandler(t, "")
	h1.Handle(context.Background(), msgFor(t, "fp1"))

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h2 := NewHandler(log, h1.tickets, h1.chat, nil)
	m := msgFor(t, "fp1")
	h2.Handle(context.Background(), m)
	m.only(t, "ack")
	if len(s.tickets) != 1 || len(s.messages) != 1 {
		t.Fatalf("tickets=%d messages=%d, want 1 and 1", len(s.tickets), len(s.messages))
	}
}

func TestSecretMaskedInChat(t *testing.T) {
	h, s := newHandler(t, "")
	f := finding.Finding{
		ID: "id-1", Fingerprint: "fp1", Scanner: "gitleaks", Severity: "critical", Repo: "northwick/payments-api",
		Title: "Live key", Evidence: finding.Evidence{Match: "sk_live_abcdefghijklmnop"},
	}
	b, _ := json.Marshal(f)
	h.Handle(context.Background(), &fakeMsg{data: b, subject: "findings.gitleaks"})
	if len(s.messages) != 1 || strings.Contains(s.messages[0].Match, "abcdefgh") {
		t.Fatalf("messages = %+v", s.messages)
	}
}

func TestParseSuppress(t *testing.T) {
	set := ParseSuppress(" a ,b,, ")
	if len(set) != 2 || !set["a"] || !set["b"] {
		t.Fatalf("set = %v", set)
	}
}

// The create failed after the platform had opened the ticket. The retry finds
// it, does not open a second one, and still posts to chat once.
func TestCreateErrorAfterCreateStillNotifies(t *testing.T) {
	var mu sync.Mutex
	var refs []string
	creates := 0
	tsrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Method == http.MethodGet {
			out := []map[string]string{}
			for i, ref := range refs {
				if ref == r.URL.Query().Get("external_ref") {
					out = append(out, map[string]string{"id": fmt.Sprintf("T-%d", i+1), "external_ref": ref})
				}
			}
			_ = json.NewEncoder(w).Encode(out)
			return
		}
		var tk ticket.Ticket
		_ = json.NewDecoder(r.Body).Decode(&tk)
		refs = append(refs, tk.ExternalRef)
		creates++
		if creates == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprintf(w, `{"id":"T-%d"}`, len(refs))
	}))
	defer tsrv.Close()
	var posts int
	csrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		posts++
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer csrv.Close()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := NewHandler(log, ticket.New(tsrv.URL, "tok", 0), chat.New(csrv.URL), nil)

	m := msgFor(t, "fp1")
	h.Handle(context.Background(), m)
	m.only(t, "nak-delayed")
	m = msgFor(t, "fp1")
	h.Handle(context.Background(), m)
	m.only(t, "ack")
	if len(refs) != 1 || posts != 1 {
		t.Fatalf("tickets=%d posts=%d, want 1 and 1", len(refs), posts)
	}
}

func v2MsgFor(t *testing.T, fp, legacy string) *fakeMsg {
	t.Helper()
	p := map[string]any{
		"schema_version": 2, "id": "id-" + fp, "fingerprint": fp, "scanner": "semgrep", "rule": "r",
		"severity": 3, "repo": "northwick/ledger", "path": "a.go", "line": 3, "title": "Issue",
		"description": "desc", "evidence": map[string]string{"match_ref": "https://evidence.northwick.example/v1/findings/x/match"},
		"emitted_at": "2026-11-02T00:00:00Z",
	}
	if legacy != "" {
		p["legacy_fingerprint"] = legacy
	}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	return &fakeMsg{data: b, subject: "findings.v2.semgrep"}
}

// While the scanners dual-publish, a relay on contract 1 tickets v1 only.
func TestV2SkippedOnContract1(t *testing.T) {
	h, s := newHandler(t, "")
	m := v2MsgFor(t, "fp2", "")
	h.Handle(context.Background(), m)
	m.only(t, "ack")
	if len(s.tickets) != 0 || len(s.messages) != 0 {
		t.Fatalf("tickets=%d messages=%d", len(s.tickets), len(s.messages))
	}
}

func TestV1SkippedOnContract2(t *testing.T) {
	h, s := newHandler(t, "")
	h.Contract = 2
	m := msgFor(t, "fp1")
	h.Handle(context.Background(), m)
	m.only(t, "ack")
	if len(s.tickets) != 0 {
		t.Fatalf("tickets=%d", len(s.tickets))
	}
}

func TestV2NewFindingTicketedUnderV2Fingerprint(t *testing.T) {
	h, s := newHandler(t, "")
	h.Contract = 2
	m := v2MsgFor(t, "fp2", "fp1")
	h.Handle(context.Background(), m)
	m.only(t, "ack")
	if len(s.tickets) != 1 || s.tickets[0].ExternalRef != "fp2" || s.tickets[0].Severity != "high" {
		t.Fatalf("tickets = %+v", s.tickets)
	}
}

// A v2 finding whose v1 fingerprint already has a ticket is not ticketed again.
func TestV2MatchesV1TicketByLegacyFingerprint(t *testing.T) {
	h, s := newHandler(t, "")
	h.Handle(context.Background(), msgFor(t, "fp1"))
	h.Contract = 2
	m := v2MsgFor(t, "fp2", "fp1")
	h.Handle(context.Background(), m)
	m.only(t, "ack")
	if len(s.tickets) != 1 || len(s.messages) != 1 {
		t.Fatalf("tickets=%d messages=%d, want 1 and 1", len(s.tickets), len(s.messages))
	}
}

func TestV2SuppressedByLegacyFingerprint(t *testing.T) {
	h, s := newHandler(t, "fp1")
	h.Contract = 2
	m := v2MsgFor(t, "fp2", "fp1")
	h.Handle(context.Background(), m)
	m.only(t, "ack")
	if len(s.tickets) != 0 {
		t.Fatalf("tickets=%d", len(s.tickets))
	}
}
