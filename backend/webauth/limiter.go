package webauth

import (
	"sync"
	"time"
)

type rateLimiter struct {
	mu      sync.Mutex
	hits    map[string][]time.Time
	limit   int
	window  time.Duration
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{hits: map[string][]time.Time{}, limit: limit, window: window}
}

func (l *rateLimiter) Allow(key string) bool {
	if key == "" {
		key = "unknown"
	}
	now := time.Now()
	cutoff := now.Add(-l.window)
	l.mu.Lock()
	defer l.mu.Unlock()
	arr := l.hits[key]
	kept := arr[:0]
	for _, t := range arr {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= l.limit {
		l.hits[key] = kept
		return false
	}
	l.hits[key] = append(kept, now)
	return true
}

var loginLimiter = newRateLimiter(10, 10*time.Minute)
