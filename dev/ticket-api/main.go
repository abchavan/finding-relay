// Command ticket-api is an in-memory stand-in for the Northwick ticketing
// platform API. See API.md.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	rateBurst      = 30
	rateRefill     = 5.0 // tokens per second
	idempotencyTTL = 24 * time.Hour
)

type ticket struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Body        string    `json:"body,omitempty"`
	Severity    string    `json:"severity"`
	ExternalRef string    `json:"external_ref"`
	Labels      []string  `json:"labels,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type bucket struct {
	tokens float64
	last   time.Time
}

type idemEntry struct {
	id string
	at time.Time
}

type chaosConfig struct {
	FailEvery   int  `json:"fail_every"`
	Status      int  `json:"status"`
	DelayMS     int  `json:"delay_ms"`
	AfterCreate bool `json:"after_create"`
}

type server struct {
	now   func() time.Time
	sleep func(time.Duration)

	mu          sync.Mutex
	tickets     []ticket
	nextID      int
	idem        map[string]idemEntry
	buckets     map[string]*bucket
	rateLimited int
	creates     int
	chaos       chaosConfig
}

func newServer() *server {
	s := &server{now: time.Now, sleep: time.Sleep}
	s.reset()
	return s
}

func (s *server) reset() {
	s.tickets = nil
	s.nextID = 1001
	s.idem = map[string]idemEntry{}
	s.buckets = map[string]*bucket{}
	s.rateLimited = 0
	s.creates = 0
	s.chaos = chaosConfig{}
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /tickets", s.create)
	mux.HandleFunc("GET /tickets", s.list)
	mux.HandleFunc("GET /tickets/{id}", s.get)
	mux.HandleFunc("GET /stats", s.stats)
	mux.HandleFunc("POST /reset", s.doReset)
	mux.HandleFunc("POST /chaos", s.setChaos)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	return mux
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func bearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if len(h) > 7 && strings.EqualFold(h[:7], "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}

// allow takes one token from the bucket of the given caller. Caller holds s.mu.
func (s *server) allow(token string) bool {
	now := s.now()
	b := s.buckets[token]
	if b == nil {
		b = &bucket{tokens: rateBurst, last: now}
		s.buckets[token] = b
	}
	b.tokens += now.Sub(b.last).Seconds() * rateRefill
	if b.tokens > rateBurst {
		b.tokens = rateBurst
	}
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

func (s *server) create(w http.ResponseWriter, r *http.Request) {
	token := bearer(r)
	if token == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	var in ticket
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	s.mu.Lock()
	if !s.allow(token) {
		s.rateLimited++
		s.mu.Unlock()
		w.Header().Set("Retry-After", "1")
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate limited"})
		return
	}
	key := r.Header.Get("Idempotency-Key")
	if key != "" {
		if e, ok := s.idem[key]; ok && s.now().Sub(e.at) < idempotencyTTL {
			t := s.find(e.id)
			s.mu.Unlock()
			writeJSON(w, http.StatusOK, t)
			return
		}
	}
	s.creates++
	n := s.creates
	chaos := s.chaos
	fail := chaos.FailEvery > 0 && n%chaos.FailEvery == 0
	delay := time.Duration(chaos.DelayMS) * time.Millisecond

	var t ticket
	if !fail || chaos.AfterCreate {
		t = ticket{
			ID: fmt.Sprintf("NWK-%d", s.nextID), Title: in.Title, Body: in.Body,
			Severity: in.Severity, ExternalRef: in.ExternalRef, Labels: in.Labels,
			CreatedAt: s.now().UTC(),
		}
		s.nextID++
		s.tickets = append(s.tickets, t)
		if key != "" {
			s.idem[key] = idemEntry{id: t.ID, at: s.now()}
		}
	}
	s.mu.Unlock()

	if delay > 0 {
		s.sleep(delay)
	}
	if fail {
		status := chaos.Status
		if status == 0 {
			status = http.StatusServiceUnavailable
		}
		writeJSON(w, status, map[string]string{"error": "injected failure"})
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

func (s *server) find(id string) ticket {
	for _, t := range s.tickets {
		if t.ID == id {
			return t
		}
	}
	return ticket{}
}

func (s *server) list(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	out := append([]ticket{}, s.tickets...)
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, out)
}

func (s *server) get(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	t := s.find(r.PathValue("id"))
	s.mu.Unlock()
	if t.ID == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *server) stats(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	counts := map[string]int{}
	for _, t := range s.tickets {
		counts[t.ExternalRef]++
	}
	total, limited := len(s.tickets), s.rateLimited
	s.mu.Unlock()

	type dup struct {
		ExternalRef string `json:"external_ref"`
		Count       int    `json:"count"`
	}
	dups := []dup{}
	for ref, c := range counts {
		if c > 1 {
			dups = append(dups, dup{ref, c})
		}
	}
	sort.Slice(dups, func(i, j int) bool { return dups[i].ExternalRef < dups[j].ExternalRef })
	writeJSON(w, http.StatusOK, map[string]any{
		"tickets": total, "unique_external_refs": len(counts),
		"duplicates": dups, "rate_limited": limited,
	})
}

func (s *server) doReset(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	s.reset()
	s.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) setChaos(w http.ResponseWriter, r *http.Request) {
	var c chaosConfig
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	s.mu.Lock()
	s.chaos = c
	s.creates = 0
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, c)
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}
	if len(os.Args) > 1 && os.Args[1] == "-healthcheck" {
		c := http.Client{Timeout: 2 * time.Second}
		resp, err := c.Get("http://127.0.0.1:" + port + "/healthz")
		if err != nil || resp.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		return
	}
	fmt.Fprintf(os.Stderr, "ticket-api listening on :%s\n", port)
	if err := http.ListenAndServe(":"+port, newServer().routes()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
