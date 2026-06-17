package admin

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
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

func TestHealthz(t *testing.T) {
	rec := do(New("s", nil), http.MethodGet, "/healthz", "")
	if rec.Code != 200 || rec.Body.String() != "ok" {
		t.Fatalf("%d %q", rec.Code, rec.Body.String())
	}
}

func TestReplayRequiresToken(t *testing.T) {
	called := false
	h := New("s3cret", func(context.Context) error { called = true; return nil })
	if rec := do(h, http.MethodPost, "/admin/replay", "wrong"); rec.Code != http.StatusForbidden {
		t.Fatalf("code = %d", rec.Code)
	}
	if called {
		t.Fatal("replay ran without a valid token")
	}
}

func TestReplayAccepted(t *testing.T) {
	called := false
	h := New("s3cret", func(context.Context) error { called = true; return nil })
	rec := do(h, http.MethodPost, "/admin/replay", "s3cret")
	if rec.Code != http.StatusAccepted || rec.Body.String() != `{"status":"replaying"}` || !called {
		t.Fatalf("%d %q called=%v", rec.Code, rec.Body.String(), called)
	}
}

func TestReplayFailure(t *testing.T) {
	h := New("s", func(context.Context) error { return errors.New("boom") })
	if rec := do(h, http.MethodPost, "/admin/replay", "s"); rec.Code != http.StatusInternalServerError {
		t.Fatalf("code = %d", rec.Code)
	}
}
