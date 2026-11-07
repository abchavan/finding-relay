package admin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func do(h http.Handler, method, path, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if token != "" {
		req.Header.Set("X-Admin-Token", token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func noReplay(t *testing.T) ReplayFunc {
	return func(context.Context, time.Time) (time.Time, error) {
		t.Fatal("replay ran")
		return time.Time{}, nil
	}
}

func TestHealthz(t *testing.T) {
	rec := do(New("s", nil), http.MethodGet, "/healthz", "")
	if rec.Code != 200 || rec.Body.String() != "ok" {
		t.Fatalf("%d %q", rec.Code, rec.Body.String())
	}
}

func TestReplayRequiresToken(t *testing.T) {
	h := New("s3cret", noReplay(t))
	if rec := do(h, http.MethodPost, "/admin/replay?since=2026-10-01T00:00:00Z", "wrong"); rec.Code != http.StatusForbidden {
		t.Fatalf("code = %d", rec.Code)
	}
}

func TestReplayDisabledWithoutToken(t *testing.T) {
	h := New("", noReplay(t))
	if rec := do(h, http.MethodPost, "/admin/replay?since=2026-10-01T00:00:00Z", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("code = %d", rec.Code)
	}
}

func TestReplayRequiresSince(t *testing.T) {
	h := New("s3cret", noReplay(t))
	if rec := do(h, http.MethodPost, "/admin/replay", "s3cret"); rec.Code != http.StatusBadRequest {
		t.Fatalf("code = %d", rec.Code)
	}
}

func TestReplayAccepted(t *testing.T) {
	var got time.Time
	oldest := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	h := New("s3cret", func(_ context.Context, since time.Time) (time.Time, error) {
		got = since
		return oldest, nil
	})
	rec := do(h, http.MethodPost, "/admin/replay?since=2026-10-01T00:00:00Z", "s3cret")
	if rec.Code != http.StatusAccepted {
		t.Fatalf("code = %d body = %q", rec.Code, rec.Body.String())
	}
	if !got.Equal(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("since = %v", got)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "replaying" || body["warning"] != "" {
		t.Fatalf("body = %v", body)
	}
}

func TestReplayWarnsBeyondRetention(t *testing.T) {
	oldest := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	h := New("s3cret", func(context.Context, time.Time) (time.Time, error) { return oldest, nil })
	rec := do(h, http.MethodPost, "/admin/replay?since=2026-03-01T00:00:00Z", "s3cret")
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["warning"] == "" || body["oldest_available"] != "2026-10-20T00:00:00Z" {
		t.Fatalf("body = %v", body)
	}
}

func TestReplayFailure(t *testing.T) {
	h := New("s", func(context.Context, time.Time) (time.Time, error) { return time.Time{}, errors.New("boom") })
	if rec := do(h, http.MethodPost, "/admin/replay?since=2026-10-01T00:00:00Z", "s"); rec.Code != http.StatusInternalServerError {
		t.Fatalf("code = %d", rec.Code)
	}
}
