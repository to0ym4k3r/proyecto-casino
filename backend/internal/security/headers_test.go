package security

import (
	"net/http"
	"testing"

	"casino-backend/internal/middleware"
)

// TestSecurityHeadersPresent confirms every header set by the
// SecurityHeaders middleware (middleware/security.go:14-25) is present on a
// response: CSP, X-Frame-Options DENY, nosniff, HSTS, Referrer-Policy and
// Permissions-Policy.
func TestSecurityHeadersPresent(t *testing.T) {
	probe := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	h := middleware.SecurityHeaders(probe)
	rec := doJSON(http.MethodGet, "/", h, "", nil)

	want := map[string]string{
		"Content-Security-Policy":   "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; form-action 'self'",
		"X-Frame-Options":           "DENY",
		"X-Content-Type-Options":    "nosniff",
		"Strict-Transport-Security": "max-age=63072000; includeSubDomains; preload",
		"Referrer-Policy":           "strict-origin-when-cross-origin",
		"Permissions-Policy":        "camera=(), microphone=(), geolocation=(), payment=()",
	}
	for k, v := range want {
		if got := rec.Header().Get(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
}

// TestSecurityHeadersThroughFullRouter confirms the headers survive the whole
// production middleware chain (RequestID, logger, recoverer, security headers,
// CORS, rate limiters) on a real route.
func TestSecurityHeadersThroughFullRouter(t *testing.T) {
	r := newTestRouter(t, testSecret)
	rec := doJSON(http.MethodGet, "/api/v1/games/", r, "", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d want 200", rec.Code)
	}
	for _, h := range []string{
		"Content-Security-Policy",
		"X-Frame-Options",
		"X-Content-Type-Options",
		"Strict-Transport-Security",
		"Referrer-Policy",
		"Permissions-Policy",
	} {
		if rec.Header().Get(h) == "" {
			t.Errorf("full-router response missing security header %q", h)
		}
	}
}
