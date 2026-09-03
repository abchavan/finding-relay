package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func post(h http.Handler, path, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", path, strings.NewReader(body)))
	return rec
}

func TestMessagesStored(t *testing.T) {
	h := (&server{}).routes()
	if rec := post(h, "/hooks/tok1", `{"text":"hi","match":"x"}`); rec.Code != 204 {
		t.Fatalf("got %d", rec.Code)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/messages", nil))
	var got []message
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Token != "tok1" || string(got[0].Body) != `{"text":"hi","match":"x"}` || got[0].ReceivedAt.IsZero() {
		t.Fatalf("messages: %+v", got)
	}
	if rec := post(h, "/hooks/tok1", `not json`); rec.Code != 400 {
		t.Errorf("bad json: %d", rec.Code)
	}
}

func TestChaosCounter(t *testing.T) {
	tests := []struct {
		name      string
		failEvery int
		want      []int
	}{
		{"every third", 3, []int{204, 204, 502, 204, 204, 502, 204, 204, 502}},
		{"disabled", 0, []int{204, 204, 204, 204}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := (&server{}).routes()
			cfg, _ := json.Marshal(map[string]int{"fail_every": tc.failEvery, "status": 502})
			post(h, "/chaos", string(cfg))
			for i, want := range tc.want {
				if rec := post(h, "/hooks/t", `{}`); rec.Code != want {
					t.Fatalf("request %d: got %d, want %d", i+1, rec.Code, want)
				}
			}
		})
	}
}

func TestReset(t *testing.T) {
	s := &server{}
	h := s.routes()
	post(h, "/chaos", `{"fail_every":1,"status":502}`)
	post(h, "/hooks/t", `{}`)
	if rec := post(h, "/reset", ""); rec.Code != 204 {
		t.Fatalf("reset: %d", rec.Code)
	}
	if rec := post(h, "/hooks/t", `{}`); rec.Code != 204 || len(s.messages) != 1 {
		t.Fatalf("after reset: %d, %d messages", rec.Code, len(s.messages))
	}
}
