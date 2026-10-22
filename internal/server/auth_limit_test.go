package restapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time          { return c.now }
func (c *fakeClock) Advance(d time.Duration) { c.now = c.now.Add(d) }

func limitedRouter(clock *fakeClock) *gin.Engine {
	gin.SetMode(gin.TestMode)
	limiter := newAuthFailureLimiter()
	limiter.now = clock.Now
	r := gin.New()
	_ = r.SetTrustedProxies(nil)
	r.Use(apiKeyMiddleware("secret", limiter))
	r.GET("/ping", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func send(r *gin.Engine, remote, key string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.RemoteAddr = remote + ":4000"
	if key != "" {
		req.Header.Set(APIKeyHeader, key)
	}
	r.ServeHTTP(w, req)
	return w
}

func TestAPIKeyMiddlewareLocksOutAClientAfterRepeatedWrongKeys(t *testing.T) {
	clock := &fakeClock{now: time.Unix(1_700_000_000, 0)}
	r := limitedRouter(clock)

	for i := range maxAuthFailures {
		if w := send(r, "203.0.113.5", "guess"); w.Code != http.StatusUnauthorized {
			t.Fatalf("wrong key %d: got %d, want 401", i+1, w.Code)
		}
	}

	// Locked out: even the right key gets 429 with Retry-After.
	w := send(r, "203.0.113.5", "secret")
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("right key while locked out: got %d, want 429", w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Error("429 has no Retry-After header")
	}

	// Another client is not affected.
	if w := send(r, "203.0.113.6", "secret"); w.Code != http.StatusOK {
		t.Errorf("other client: got %d, want 200", w.Code)
	}

	// The lockout ends.
	clock.Advance(authLockout + time.Second)
	if w := send(r, "203.0.113.5", "secret"); w.Code != http.StatusOK {
		t.Errorf("after the lockout: got %d, want 200", w.Code)
	}
}

func TestAPIKeyMiddlewareForgetsOldFailures(t *testing.T) {
	clock := &fakeClock{now: time.Unix(1_700_000_000, 0)}
	r := limitedRouter(clock)

	for range 3 {
		for range maxAuthFailures - 1 {
			send(r, "203.0.113.5", "guess")
		}
		clock.Advance(authFailureWindow + time.Second)
	}

	if w := send(r, "203.0.113.5", "secret"); w.Code != http.StatusOK {
		t.Errorf("failures spread over several windows locked the client: got %d, want 200", w.Code)
	}
}

func TestAPIKeyMiddlewareMissingKeyDoesNotCountAsAGuess(t *testing.T) {
	clock := &fakeClock{now: time.Unix(1_700_000_000, 0)}
	r := limitedRouter(clock)

	for i := range maxAuthFailures * 2 {
		if w := send(r, "203.0.113.5", ""); w.Code != http.StatusUnauthorized {
			t.Fatalf("missing key %d: got %d, want 401", i+1, w.Code)
		}
	}
	if w := send(r, "203.0.113.5", "secret"); w.Code != http.StatusOK {
		t.Errorf("right key after missing-key requests: got %d, want 200", w.Code)
	}
}

func TestAuthFailureLimiterTableStaysBounded(t *testing.T) {
	clock := &fakeClock{now: time.Unix(1_700_000_000, 0)}
	limiter := newAuthFailureLimiter()
	limiter.now = clock.Now

	for i := range maxTrackedClients + 500 {
		limiter.fail("client-" + time.Duration(i).String())
	}
	if got := len(limiter.clients); got > maxTrackedClients {
		t.Errorf("tracked clients = %d, want at most %d", got, maxTrackedClients)
	}
}
