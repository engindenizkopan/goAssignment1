package transporthttp

import (
	"io"
	"net/http"
	"strings"
	"time"
)

// BodyLimit limits request bodies to maxBytes.
func BodyLimit(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if maxBytes > 0 {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireJSON ensures Content-Type is application/json for POST endpoints.
func RequireJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ct := r.Header.Get("Content-Type")
		if r.Method == http.MethodPost && !strings.HasPrefix(strings.ToLower(ct), "application/json") {
			WriteProblem(w, http.StatusUnsupportedMediaType, "unsupported media type", "expected application/json", nil)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// APIKeyAuth allows an optional list of API keys; if the list is empty, auth is bypassed.
// Keys are expected in header: X-API-Key.
func APIKeyAuth(allowed map[string]struct{}) func(http.Handler) http.Handler {
	if len(allowed) == 0 {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("X-API-Key")
			if _, ok := allowed[key]; !ok {
				WriteProblem(w, http.StatusUnauthorized, "unauthorized", "invalid or missing API key", nil)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Simple global leaky bucket for GET /metrics (20 req/min by default).
type rateState struct {
	tokens         float64
	lastRefillNano int64
}

func RateLimitPerMinute(limitPerMin int, clock func() time.Time) func(http.Handler) http.Handler {
	if limitPerMin <= 0 {
		return func(next http.Handler) http.Handler { return next }
	}
	state := &rateState{tokens: float64(limitPerMin), lastRefillNano: clock().UnixNano()}
	capacity := float64(limitPerMin)
	refillPerSec := float64(limitPerMin) / 60.0

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || !strings.HasPrefix(r.URL.Path, "/metrics") {
				next.ServeHTTP(w, r)
				return
			}
			now := clock()
			elapsed := float64(now.UnixNano()-state.lastRefillNano) / 1e9
			state.lastRefillNano = now.UnixNano()

			state.tokens += elapsed * refillPerSec
			if state.tokens > capacity {
				state.tokens = capacity
			}
			if state.tokens < 1.0 {
				w.Header().Set("Retry-After", "3")
				WriteProblem(w, http.StatusTooManyRequests, "rate limit exceeded", "try again later", nil)
				return
			}
			state.tokens -= 1.0
			next.ServeHTTP(w, r)
		})
	}
}

// DrainBody fully reads and closes request bodies (handler helper).
func DrainBody(r *http.Request) {
	if r.Body != nil {
		_, _ = io.Copy(io.Discard, r.Body)
		_ = r.Body.Close()
	}
}
