package relay

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
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
	mu       sync.Mutex
	tickets  []ticket.Ticket
	messages []chat.Message
	status   int
}

func newHandler(t *testing.T, suppress string) (*Handler, *sinks) {
	t.Helper()
	s := &sinks{status: http.StatusCreated}
	tsrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.status != http.StatusCreated {
			w.WriteHeader(s.status)
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
		s.messages = append(s.messages, m)
		s.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(tsrv.Close)
	t.Cleanup(csrv.Close)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewHandler(log, ticket.New(tsrv.URL, "tok"), chat.New(csrv.URL), ParseSuppress(suppress)), s
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

func TestBadJSONNaks(t *testing.T) {
	h, s := newHandler(t, "")
	m := &fakeMsg{data: []byte("{nope"), subject: "findings.semgrep"}
	h.Handle(context.Background(), m)
	m.only(t, "nak-delayed")
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

func TestRateLimitedTerms(t *testing.T) {
	h, s := newHandler(t, "")
	s.status = http.StatusTooManyRequests
	m := msgFor(t, "fp1")
	h.Handle(context.Background(), m)
	m.only(t, "term")
}

func TestTicketErrorNaks(t *testing.T) {
	h, s := newHandler(t, "")
	s.status = http.StatusServiceUnavailable
	m := msgFor(t, "fp1")
	h.Handle(context.Background(), m)
	m.only(t, "nak")
}

func TestParseSuppress(t *testing.T) {
	set := ParseSuppress(" a ,b,, ")
	if len(set) != 2 || !set["a"] || !set["b"] {
		t.Fatalf("set = %v", set)
	}
}
