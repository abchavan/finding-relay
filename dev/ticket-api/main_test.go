package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type harness struct {
	t   *testing.T
	s   *server
	h   http.Handler
	now time.Time
}

func newHarness(t *testing.T) *harness {
	hn := &harness{t: t, now: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}
	hn.s = newServer()
	hn.s.now = func() time.Time { return hn.now }
	hn.s.sleep = func(time.Duration) {}
	hn.h = hn.s.routes()
	return hn
}

func (hn *harness) do(method, path, body string, hdr map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	hn.h.ServeHTTP(rec, req)
	return rec
}

func (hn *harness) create(ref, key string) *httptest.ResponseRecorder {
	hdr := map[string]string{"Authorization": "Bearer tok"}
	if key != "" {
		hdr["Idempotency-Key"] = key
	}
	return hn.do("POST", "/tickets", `{"title":"t","severity":"high","external_ref":"`+ref+`"}`, hdr)
}

func (hn *harness) count() int {
	var out []ticket
	_ = json.Unmarshal(hn.do("GET", "/tickets", "", nil).Body.Bytes(), &out)
	return len(out)
}

func TestAuth(t *testing.T) {
	hn := newHarness(t)
	for _, auth := range []string{"", "Bearer ", "Basic abc"} {
		rec := hn.do("POST", "/tickets", `{}`, map[string]string{"Authorization": auth})
		if rec.Code != 401 {
			t.Errorf("auth %q: got %d, want 401", auth, rec.Code)
		}
	}
}

func TestIdempotency(t *testing.T) {
	tests := []struct {
		name       string
		keys       []string
		wantCodes  []int
		wantTicket int
		advance    time.Duration
	}{
		{"same key twice", []string{"k1", "k1"}, []int{201, 200}, 1, 0},
		{"distinct keys", []string{"k1", "k2"}, []int{201, 201}, 2, 0},
		{"no key", []string{"", ""}, []int{201, 201}, 2, 0},
		{"key expires after 24h", []string{"k1", "k1"}, []int{201, 201}, 2, 25 * time.Hour},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hn := newHarness(t)
			var ids []string
			for i, k := range tc.keys {
				if i == 1 {
					hn.now = hn.now.Add(tc.advance)
				}
				rec := hn.create("ref", k)
				if rec.Code != tc.wantCodes[i] {
					t.Fatalf("request %d: got %d, want %d", i, rec.Code, tc.wantCodes[i])
				}
				var tk ticket
				_ = json.Unmarshal(rec.Body.Bytes(), &tk)
				ids = append(ids, tk.ID)
			}
			if got := hn.count(); got != tc.wantTicket {
				t.Errorf("tickets: got %d, want %d", got, tc.wantTicket)
			}
			if tc.wantTicket == 1 && ids[0] != ids[1] {
				t.Errorf("replay returned %s, want original %s", ids[1], ids[0])
			}
		})
	}
}

func TestRateLimit(t *testing.T) {
	hn := newHarness(t)
	for i := 0; i < rateBurst; i++ {
		if rec := hn.create("r", ""); rec.Code != 201 {
			t.Fatalf("request %d: got %d", i+1, rec.Code)
		}
	}
	rec := hn.create("r", "")
	if rec.Code != 429 || rec.Header().Get("Retry-After") != "1" || !strings.Contains(rec.Body.String(), `"error":"rate limited"`) {
		t.Fatalf("request over burst: %d %v %s", rec.Code, rec.Header(), rec.Body.String())
	}
	hn.now = hn.now.Add(time.Second)
	if rec := hn.create("r", ""); rec.Code != 201 {
		t.Fatalf("after refill: got %d", rec.Code)
	}
	var st struct {
		RateLimited int `json:"rate_limited"`
	}
	_ = json.Unmarshal(hn.do("GET", "/stats", "", nil).Body.Bytes(), &st)
	if st.RateLimited != 1 {
		t.Errorf("rate_limited: got %d, want 1", st.RateLimited)
	}
}

func TestChaos(t *testing.T) {
	tests := []struct {
		name        string
		afterCreate bool
		wantTickets int
	}{
		{"before create", false, 0},
		{"after create", true, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hn := newHarness(t)
			cfg := `{"fail_every":1,"status":503,"after_create":` + map[bool]string{true: "true", false: "false"}[tc.afterCreate] + `}`
			if rec := hn.do("POST", "/chaos", cfg, nil); rec.Code != 200 {
				t.Fatalf("chaos: %d", rec.Code)
			}
			if rec := hn.create("c", "k"); rec.Code != 503 {
				t.Fatalf("got %d, want 503", rec.Code)
			}
			if got := hn.count(); got != tc.wantTickets {
				t.Errorf("tickets: got %d, want %d", got, tc.wantTickets)
			}
		})
	}
}

func TestChaosEveryNth(t *testing.T) {
	hn := newHarness(t)
	hn.do("POST", "/chaos", `{"fail_every":2,"status":500}`, nil)
	var codes []int
	for i := 0; i < 4; i++ {
		hn.now = hn.now.Add(time.Second)
		codes = append(codes, hn.create("e", "").Code)
	}
	want := []int{201, 500, 201, 500}
	for i := range want {
		if codes[i] != want[i] {
			t.Fatalf("codes %v, want %v", codes, want)
		}
	}
}

func TestStatsAndReset(t *testing.T) {
	hn := newHarness(t)
	hn.create("a", "")
	hn.create("a", "")
	hn.create("b", "")
	var st struct {
		Tickets    int `json:"tickets"`
		Unique     int `json:"unique_external_refs"`
		Duplicates []struct {
			Ref   string `json:"external_ref"`
			Count int    `json:"count"`
		} `json:"duplicates"`
	}
	_ = json.Unmarshal(hn.do("GET", "/stats", "", nil).Body.Bytes(), &st)
	if st.Tickets != 3 || st.Unique != 2 || len(st.Duplicates) != 1 || st.Duplicates[0].Ref != "a" || st.Duplicates[0].Count != 2 {
		t.Fatalf("stats: %+v", st)
	}

	hn.do("POST", "/chaos", `{"fail_every":1,"status":503}`, nil)
	hn.create("k", "key")
	if rec := hn.do("POST", "/reset", "", nil); rec.Code != 204 {
		t.Fatalf("reset: %d", rec.Code)
	}
	if hn.count() != 0 {
		t.Error("tickets not cleared")
	}
	rec := hn.create("a", "key")
	if rec.Code != 201 || !strings.Contains(rec.Body.String(), "NWK-1001") {
		t.Errorf("after reset: %d %s (idempotency keys, chaos, ids should be cleared)", rec.Code, rec.Body.String())
	}
	if rec := hn.do("GET", "/tickets/NWK-1001", "", nil); rec.Code != 200 {
		t.Errorf("get by id: %d", rec.Code)
	}
	if rec := hn.do("GET", "/tickets/NWK-9", "", nil); rec.Code != 404 {
		t.Errorf("get missing: %d", rec.Code)
	}
}
