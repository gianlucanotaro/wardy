package http

import (
	"sync"
	"time"
)

// rateLimiter is a simple fixed-window limiter keyed by arbitrary strings
// (IP, email, ...). A two-user household app doesn't need Redis or a
// sliding-window algorithm for this — an in-memory map is plenty.
type rateLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	max      int
	window   time.Duration
}

func newRateLimiter(max int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		attempts: make(map[string][]time.Time),
		max:      max,
		window:   window,
	}
}

// Allow records an attempt against each key and reports whether every key
// is still under its limit. Empty keys are ignored so callers can pass an
// unknown email without it counting against anything.
func (l *rateLimiter) Allow(keys ...string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-l.window)

	for _, key := range keys {
		if key == "" {
			continue
		}

		kept := l.attempts[key][:0]
		for _, t := range l.attempts[key] {
			if t.After(cutoff) {
				kept = append(kept, t)
			}
		}

		if len(kept) >= l.max {
			l.attempts[key] = kept
			return false
		}

		l.attempts[key] = append(kept, now)
	}

	return true
}

// Reset clears attempt history for each key. Called after a successful
// login so a legitimate user isn't penalized for earlier typos.
func (l *rateLimiter) Reset(keys ...string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, key := range keys {
		delete(l.attempts, key)
	}
}
