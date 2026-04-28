package middleware

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type rateLimitEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type rateLimitStore struct {
	mu      sync.Mutex
	entries map[string]*rateLimitEntry
	r       rate.Limit
	burst   int
	stop    chan struct{}
}

func newRateLimitStore(r rate.Limit, burst int) *rateLimitStore {
	s := &rateLimitStore{
		entries: make(map[string]*rateLimitEntry),
		r:       r,
		burst:   burst,
		stop:    make(chan struct{}),
	}
	go func() {
		ticker := time.NewTicker(2 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.mu.Lock()
				for k, e := range s.entries {
					if time.Since(e.lastSeen) > 5*time.Minute {
						delete(s.entries, k)
					}
				}
				s.mu.Unlock()
			case <-s.stop:
				return
			}
		}
	}()
	return s
}

func (s *rateLimitStore) get(key string) *rate.Limiter {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entries[key]
	if !ok {
		e = &rateLimitEntry{limiter: rate.NewLimiter(s.r, s.burst)}
		s.entries[key] = e
	}
	e.lastSeen = time.Now()
	return e.limiter
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.SplitN(xff, ",", 2)[0]
	}
	// Strip port from RemoteAddr
	addr := r.RemoteAddr
	if i := strings.LastIndex(addr, ":"); i != -1 {
		addr = addr[:i]
	}
	return addr
}

// RateLimit returns a middleware that limits requests by client IP.
// r is the sustained rate (e.g. rate.Every(600*time.Millisecond) for 100/min).
// burst is the maximum burst size.
func RateLimit(r rate.Limit, burst int) func(http.Handler) http.Handler {
	store := newRateLimitStore(r, burst)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			key := clientIP(req)
			limiter := store.get(key)
			if !limiter.Allow() {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "60")
				http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, req)
		})
	}
}
