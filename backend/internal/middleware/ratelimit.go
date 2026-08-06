package middleware

import (
	"net/http"
	"sync"
	"time"

	"casino-backend/pkg/response"
)

// RateLimiter implements a per-IP token-bucket rate limiter.
//
// Each client IP gets its own bucket that refills at [rate] tokens per second
// up to a maximum of [capacity] tokens. When a bucket is exhausted, subsequent
// requests receive HTTP 429 Too Many Requests.
type RateLimiter struct {
	mu       sync.Mutex
	clients  map[string]*bucket
	rate     int
	capacity int
}

// bucket tracks the token state for a single client IP.
type bucket struct {
	tokens    int
	lastTime  time.Time
	rate      int
	capacity  int
}

// NewRateLimiter creates a rate limiter where each IP is allowed [rate] tokens
// per second with a burst capacity of [capacity] tokens.
func NewRateLimiter(rate, capacity int) *RateLimiter {
	return &RateLimiter{
		clients:  make(map[string]*bucket),
		rate:     rate,
		capacity: capacity,
	}
}

// Limit returns an HTTP middleware that enforces per-IP rate limiting.
//
// The client IP is determined from r.RemoteAddr, falling back to the
// X-Forwarded-For header when present. Returns 429 if the client's bucket
// is empty.
func (rl *RateLimiter) Limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			ip = forwarded
		}

		if !rl.allow(ip) {
			response.Error(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// allow checks whether the given IP has a token available, consuming one if so.
// It refills tokens based on elapsed time since the last request.
func (rl *RateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	b, exists := rl.clients[ip]
	if !exists {
		b = &bucket{
			tokens:   rl.capacity,
			lastTime: time.Now(),
			rate:     rl.rate,
			capacity: rl.capacity,
		}
		rl.clients[ip] = b
	}

	elapsed := time.Since(b.lastTime)
	refill := int(elapsed.Seconds()) * b.rate
	b.tokens += refill
	if b.tokens > b.capacity {
		b.tokens = b.capacity
	}
	b.lastTime = time.Now()

	if b.tokens <= 0 {
		return false
	}

	b.tokens--
	return true
}

// Cleanup starts a background goroutine that periodically removes stale client
// entries that have been inactive for more than 5 minutes. The goroutine runs
// every [interval] and is stopped when the process exits.
func (rl *RateLimiter) Cleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			rl.mu.Lock()
			for ip, b := range rl.clients {
				if time.Since(b.lastTime) > 5*time.Minute {
					delete(rl.clients, ip)
				}
			}
			rl.mu.Unlock()
		}
	}()
}
