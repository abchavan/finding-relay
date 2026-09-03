// Command chat-sink is an in-memory stand-in for the Northwick chat webhook.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

type message struct {
	ReceivedAt time.Time       `json:"received_at"`
	Token      string          `json:"token"`
	Body       json.RawMessage `json:"body"`
}

type server struct {
	mu        sync.Mutex
	messages  []message
	posts     int
	failEvery int
	status    int
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /hooks/{token}", s.hook)
	mux.HandleFunc("GET /messages", func(w http.ResponseWriter, _ *http.Request) {
		s.mu.Lock()
		out := append([]message{}, s.messages...)
		s.mu.Unlock()
		writeJSON(w, http.StatusOK, out)
	})
	mux.HandleFunc("POST /reset", func(w http.ResponseWriter, _ *http.Request) {
		s.mu.Lock()
		s.messages, s.posts, s.failEvery, s.status = nil, 0, 0, 0
		s.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /chaos", func(w http.ResponseWriter, r *http.Request) {
		var c struct {
			FailEvery int `json:"fail_every"`
			Status    int `json:"status"`
		}
		if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		s.mu.Lock()
		s.failEvery, s.status, s.posts = c.FailEvery, c.Status, 0
		s.mu.Unlock()
		writeJSON(w, http.StatusOK, c)
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	return mux
}

func (s *server) hook(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil || !json.Valid(raw) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	s.mu.Lock()
	s.posts++
	if s.failEvery > 0 && s.posts%s.failEvery == 0 {
		status := s.status
		s.mu.Unlock()
		if status == 0 {
			status = http.StatusBadGateway
		}
		writeJSON(w, status, map[string]string{"error": "injected failure"})
		return
	}
	s.messages = append(s.messages, message{
		ReceivedAt: time.Now().UTC(), Token: r.PathValue("token"), Body: raw,
	})
	s.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}
	if len(os.Args) > 1 && os.Args[1] == "-healthcheck" {
		c := http.Client{Timeout: 2 * time.Second}
		resp, err := c.Get("http://127.0.0.1:" + port + "/healthz")
		if err != nil || resp.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		return
	}
	fmt.Fprintf(os.Stderr, "chat-sink listening on :%s\n", port)
	if err := http.ListenAndServe(":"+port, (&server{}).routes()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
