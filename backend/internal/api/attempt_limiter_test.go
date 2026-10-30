package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time { return c.t }

func limitedGate(clock *fakeClock) http.Handler {
	a := lanAuth{token: "s3cret"}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth/status", authStatusHandler(a))
	mux.HandleFunc("/api/auth/session", createAuthSessionHandler(a))
	mux.HandleFunc("/api/healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	return requireLANToken(a, newAttemptLimiter(clock.now), mux)
}

func do(h http.Handler, from, method, path, body string, hdr map[string]string, cookie string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.RemoteAddr = from + ":4000"
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: accessCookieName, Value: cookie})
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func login(h http.Handler, from, token string) *httptest.ResponseRecorder {
	return do(h, from, http.MethodPost, "/api/auth/session", `{"token":"`+token+`"}`, nil, "")
}

func TestAttemptLimit_BlocksSixthAttemptThenUnblocks(t *testing.T) {
	clock := &fakeClock{t: time.Unix(1000, 0)}
	h := limitedGate(clock)
	for i := 0; i < attemptLimit; i++ {
		if got := login(h, "10.0.0.1", "wrong").Code; got != http.StatusUnauthorized {
			t.Fatalf("attempt %d: got %d", i+1, got)
		}
	}
	rec := login(h, "10.0.0.1", "s3cret")
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("sixth attempt: got %d, Retry-After %q", rec.Code, rec.Header().Get("Retry-After"))
	}
	if got := do(h, "10.0.0.1", http.MethodGet, "/api/healthz", "", nil, "").Code; got != http.StatusTooManyRequests {
		t.Fatalf("unauthenticated gated request while blocked: got %d", got)
	}

	clock.t = clock.t.Add(attemptBaseDelay + time.Second)
	if got := login(h, "10.0.0.1", "s3cret").Code; got != http.StatusNoContent {
		t.Fatalf("after delay: got %d", got)
	}
	// The counter was cleared: five more failures are needed to block again.
	for i := 0; i < attemptLimit-1; i++ {
		login(h, "10.0.0.1", "wrong")
	}
	if got := login(h, "10.0.0.1", "s3cret").Code; got != http.StatusNoContent {
		t.Fatalf("counter not reset: got %d", got)
	}
}

func TestAttemptLimit_DelayGrowsWithEachFailure(t *testing.T) {
	clock := &fakeClock{t: time.Unix(1000, 0)}
	h := limitedGate(clock)
	for i := 0; i < attemptLimit; i++ {
		login(h, "10.0.0.1", "wrong")
	}
	clock.t = clock.t.Add(attemptBaseDelay + time.Second)
	login(h, "10.0.0.1", "wrong") // sixth failure
	rec := login(h, "10.0.0.1", "wrong")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("got %d", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got != "60" {
		t.Fatalf("Retry-After = %q, want 60", got)
	}
}

func TestAttemptLimit_OtherAddressUntouched(t *testing.T) {
	clock := &fakeClock{t: time.Unix(1000, 0)}
	h := limitedGate(clock)
	for i := 0; i < attemptLimit; i++ {
		login(h, "10.0.0.1", "wrong")
	}
	if got := login(h, "10.0.0.2", "s3cret").Code; got != http.StatusNoContent {
		t.Fatalf("other address: got %d", got)
	}
}

func TestAttemptLimit_AuthenticatedAndStatusNotCounted(t *testing.T) {
	clock := &fakeClock{t: time.Unix(1000, 0)}
	h := limitedGate(clock)
	cookie := accessCookieValue("s3cret")
	for i := 0; i < attemptLimit*3; i++ {
		if got := do(h, "10.0.0.1", http.MethodGet, "/api/healthz", "", nil, cookie).Code; got != http.StatusOK {
			t.Fatalf("cookie request: got %d", got)
		}
		if got := do(h, "10.0.0.1", http.MethodGet, "/api/auth/status", "", nil, "").Code; got != http.StatusOK {
			t.Fatalf("status: got %d", got)
		}
	}
	if got := login(h, "10.0.0.1", "s3cret").Code; got != http.StatusNoContent {
		t.Fatalf("login after those: got %d", got)
	}
}

func TestAttemptLimit_WrongHeaderTokenCounts(t *testing.T) {
	clock := &fakeClock{t: time.Unix(1000, 0)}
	h := limitedGate(clock)
	for i := 0; i < attemptLimit; i++ {
		do(h, "10.0.0.1", http.MethodGet, "/api/healthz", "", map[string]string{lanAuthHeader: "nope"}, "")
	}
	if got := login(h, "10.0.0.1", "s3cret").Code; got != http.StatusTooManyRequests {
		t.Fatalf("got %d", got)
	}
}

func TestAttemptLimit_FailuresExpireAfterWindow(t *testing.T) {
	clock := &fakeClock{t: time.Unix(1000, 0)}
	h := limitedGate(clock)
	for i := 0; i < attemptLimit-1; i++ {
		login(h, "10.0.0.1", "wrong")
	}
	clock.t = clock.t.Add(attemptWindow + time.Second)
	login(h, "10.0.0.1", "wrong")
	if got := login(h, "10.0.0.1", "s3cret").Code; got != http.StatusNoContent {
		t.Fatalf("got %d", got)
	}
}
