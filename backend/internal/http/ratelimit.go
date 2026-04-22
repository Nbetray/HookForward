package http

import (
	"net"
	"net/http"
	"sync"
	"time"
)

type ipEntry struct {
	count    int
	resetAt  time.Time
}

type rateLimiter struct {
	mu       sync.Mutex
	entries  map[string]*ipEntry
	limit    int
	window   time.Duration
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	rl := &rateLimiter{
		entries: make(map[string]*ipEntry),
		limit:   limit,
		window:  window,
	}
	go rl.cleanup()
	return rl
}

func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	entry, ok := rl.entries[ip]
	if !ok || now.After(entry.resetAt) {
		rl.entries[ip] = &ipEntry{count: 1, resetAt: now.Add(rl.window)}
		return true
	}

	entry.count++
	return entry.count <= rl.limit
}

func (rl *rateLimiter) cleanup() {
	ticker := time.NewTicker(rl.window)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for ip, entry := range rl.entries {
			if now.After(entry.resetAt) {
				delete(rl.entries, ip)
			}
		}
		rl.mu.Unlock()
	}
}

func withRateLimit(limiter *rateLimiter, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := extractIP(r)
		if !limiter.allow(ip) {
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many requests, try again later"})
			return
		}
		next.ServeHTTP(w, r)
	}
}

func extractIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if parts := net.ParseIP(xff); parts != nil {
			return parts.String()
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
