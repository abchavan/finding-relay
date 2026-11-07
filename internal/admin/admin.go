// Package admin serves the health check and operator endpoints.
package admin

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

// ReplayFunc recreates the consumer so it redelivers everything published
// since the given time. It returns the time of the oldest message the stream
// still holds, so the caller can tell when the request reaches further back
// than retention allows.
type ReplayFunc func(ctx context.Context, since time.Time) (oldest time.Time, err error)

// New returns the admin HTTP handler. The replay endpoint is disabled when
// token is empty.
func New(token string, replay ReplayFunc) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("POST /admin/replay", func(w http.ResponseWriter, r *http.Request) {
		if token == "" {
			http.Error(w, "replay disabled: ADMIN_TOKEN not set", http.StatusForbidden)
			return
		}
		got := r.Header.Get("X-Admin-Token")
		if subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		since, err := time.Parse(time.RFC3339, r.URL.Query().Get("since"))
		if err != nil {
			http.Error(w, "since is required, RFC 3339 (e.g. 2026-03-01T00:00:00Z)", http.StatusBadRequest)
			return
		}
		oldest, err := replay(r.Context(), since)
		if err != nil {
			slog.Error("replay failed", "since", since, "err", err)
			http.Error(w, "replay failed", http.StatusInternalServerError)
			return
		}
		resp := map[string]string{"status": "replaying", "since": since.UTC().Format(time.RFC3339)}
		if !oldest.IsZero() {
			resp["oldest_available"] = oldest.UTC().Format(time.RFC3339)
		}
		if !oldest.IsZero() && oldest.After(since) {
			resp["warning"] = "stream does not hold messages that old; replay starts at oldest_available"
			slog.Warn("replay requested beyond stream retention", "since", since, "oldest_available", oldest)
		}
		slog.Info("replay requested", "since", since)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(resp)
	})
	return mux
}
