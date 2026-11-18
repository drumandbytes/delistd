package web

import (
	"sync"
	"time"
)

// RateLimiter allows each key at most `limit` hits per sliding window. The
// keys are the handful of endpoint names handlers pass in, so the map stays
// small and is never pruned.
type RateLimiter struct {
	mu     sync.Mutex
	hits   map[string][]time.Time
	limit  int
	window time.Duration
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{hits: map[string][]time.Time{}, limit: limit, window: window}
}

// Allow records a hit for key and reports whether it is within the limit. A
// refused hit is not recorded, so a client that keeps trying is let back in
// as soon as its oldest hit leaves the window.
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)
	live := rl.hits[key][:0]
	for _, t := range rl.hits[key] {
		if t.After(cutoff) {
			live = append(live, t)
		}
	}
	if len(live) >= rl.limit {
		rl.hits[key] = live
		return false
	}
	rl.hits[key] = append(live, now)
	return true
}
