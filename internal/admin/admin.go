// Package admin serves the health check and operator endpoints.
package admin

import (
	"context"
	"log/slog"
	"net/http"
)

// New returns the admin HTTP handler. replay recreates the consumer from the
// start of the stream.
func New(token string, replay func(ctx context.Context) error) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("POST /admin/replay", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Admin-Token") != token {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if err := replay(r.Context()); err != nil {
			slog.Error("replay failed", "err", err)
			http.Error(w, "replay failed", http.StatusInternalServerError)
			return
		}
		slog.Info("replay requested")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"status":"replaying"}`))
	})
	return mux
}
