package middleware

import "net/http"

// SecurityHeaders is a middleware that adds security-related HTTP response headers:
//
//   - X-Content-Type-Options: nosniff — prevents MIME-type sniffing.
//   - X-Frame-Options: DENY — blocks all framing (clickjacking protection).
//   - X-XSS-Protection: 1; mode=block — enables legacy browser XSS filter.
//   - Referrer-Policy: strict-origin-when-cross-origin — limits referrer info.
//   - Content-Security-Policy — restricts resource loading to same-origin.
//   - Strict-Transport-Security — enforces HTTPS for 2 years with preload.
//   - Permissions-Policy — disables camera, microphone, geolocation, and payment APIs.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; form-action 'self'")
		w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")

		next.ServeHTTP(w, r)
	})
}
