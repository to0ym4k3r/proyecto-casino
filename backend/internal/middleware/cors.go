package middleware

import (
	"net/http"
	"strings"
)

// CORS returns a middleware that handles Cross-Origin Resource Sharing (CORS).
//
// allowedOrigins is a comma-separated list of permitted origins. Requests from
// origins not in the list are passed through without CORS headers. For preflight
// OPTIONS requests from an allowed origin, the middleware responds with 204 No
// Content immediately without calling the next handler. Allowed methods are
// GET, POST, PUT, DELETE, OPTIONS; allowed headers are Accept, Authorization,
// and Content-Type; credentials are permitted.
func CORS(allowedOrigins string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}

			allowed := false
			for _, o := range strings.Split(allowedOrigins, ",") {
				if strings.TrimSpace(o) == origin {
					allowed = true
					break
				}
			}

			if !allowed {
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type")
			w.Header().Set("Access-Control-Expose-Headers", "Link")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Max-Age", "300")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
