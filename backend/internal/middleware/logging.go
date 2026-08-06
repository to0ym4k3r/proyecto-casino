package middleware

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// Logger is a basic middleware that wraps the response writer to capture the
// HTTP status code. It does not currently emit any output — prefer
// RequestLogger for structured logging.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := &statusResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(wrapped, r)
		_ = start
		_ = wrapped.statusCode
	})
}

// RequestLogger returns a middleware that logs every completed HTTP request
// using the provided zerolog.Logger.
//
// Each log entry includes: method, path, status code, remote address,
// request ID, and duration. If the incoming request has no X-Request-ID
// header, one is generated and set on the response so callers can correlate
// requests.
func RequestLogger(log zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			wrapped := &statusResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			requestID := r.Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = uuid.New().String()
				w.Header().Set("X-Request-ID", requestID)
			}

			next.ServeHTTP(wrapped, r)

			log.Info().
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Int("status", wrapped.statusCode).
				Str("remote", r.RemoteAddr).
				Str("request_id", requestID).
				Dur("duration", time.Since(start)).
				Msg("request completed")
		})
	}
}

// Recoverer is a middleware that recovers from panics in downstream handlers.
//
// When a panic is caught it sends a generic 500 Internal Server Error response
// to the client. The panic value is not logged — pair with RequestLogger or
// a dedicated error reporter for observability.
func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// statusResponseWriter is an http.ResponseWriter wrapper that captures the
// status code written via WriteHeader so logging middleware can read it after
// the handler chain completes.
type statusResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader records the status code before delegating to the underlying writer.
func (w *statusResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}
