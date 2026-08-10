package security

import (
	"fmt"
	"net/http"
	"testing"

	"casino-backend/internal/middleware"
)

func rateLimitProbe() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

// TestRateLimitEnforcesBurstCapacity confirms the token-bucket limiter
// (middleware/ratelimit.go:64-93) allows the configured burst and then returns
// 429 for excess requests from the same client IP.
func TestRateLimitEnforcesBurstCapacity(t *testing.T) {
	rl := middleware.NewRateLimiter(1, 3, "")
	h := rl.Limit(rateLimitProbe())

	for i := 0; i < 3; i++ {
		rec := doJSON(http.MethodGet, "/", h, "", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: code=%d want 200 (burst)", i, rec.Code)
		}
	}

	throttled := 0
	for i := 0; i < 10; i++ {
		rec := doJSON(http.MethodGet, "/", h, "", nil)
		if rec.Code == http.StatusTooManyRequests {
			throttled++
		}
	}
	if throttled == 0 {
		t.Error("no request throttled after the burst — token bucket not enforced")
	}
	t.Logf("after burst of 3: %d/10 requests returned 429", throttled)
}

// TestRateLimitXForwardedForSpoofBypass confirms finding #3 (ALTA) is closed:
// with no trusted header configured (the default) the limiter keys ONLY on
// RemoteAddr and ignores a client-supplied X-Forwarded-For header, so rotating
// the header value can no longer mint a fresh bucket per request.
func TestRateLimitXForwardedForSpoofBypass(t *testing.T) {
	rl := middleware.NewRateLimiter(1, 3, "")
	h := rl.Limit(rateLimitProbe())

	throttled := 0
	for i := 0; i < 25; i++ {
		rec := doJSON(http.MethodGet, "/", h, "",
			map[string]string{"X-Forwarded-For": fmt.Sprintf("203.0.113.%d", i)})
		if rec.Code == http.StatusTooManyRequests {
			throttled++
		}
	}
	if throttled == 0 {
		t.Error("spoofed X-Forwarded-For values bypassed the limiter — RemoteAddr not used as the key")
	}
}

// TestRateLimitSharedXFFSharesBucket confirms the flip side of the same issue:
// with a *constant* X-Forwarded-For, all requests collapse into one bucket.
func TestRateLimitSharedXFFSharesBucket(t *testing.T) {
	rl := middleware.NewRateLimiter(1, 3, "")
	h := rl.Limit(rateLimitProbe())
	xff := map[string]string{"X-Forwarded-For": "203.0.113.77"}

	for i := 0; i < 3; i++ {
		if rec := doJSON(http.MethodGet, "/", h, "", xff); rec.Code != http.StatusOK {
			t.Fatalf("request %d: code=%d want 200 (burst)", i, rec.Code)
		}
	}
	if rec := doJSON(http.MethodGet, "/", h, "", xff); rec.Code != http.StatusTooManyRequests {
		t.Errorf("constant-XFF request code=%d want 429 (shared bucket)", rec.Code)
	}
}
