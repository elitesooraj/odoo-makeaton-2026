// Package ratelimit is a small in-process fixed-window limiter.
//
// It is deliberately not distributed: the deployment is a single API instance.
// If the API is ever scaled horizontally this must move to Redis or Postgres.
package ratelimit

import (
	"sync"
	"time"
)

type entry struct {
	count   int
	resetAt time.Time
}

type Limiter struct {
	mu      sync.Mutex
	entries map[string]entry
	limit   int
	window  time.Duration
	now     func() time.Time
}

func New(limit int, window time.Duration) *Limiter {
	return &Limiter{
		entries: make(map[string]entry),
		limit:   limit,
		window:  window,
		now:     time.Now,
	}
}

// Allow records an attempt for key and reports whether it is within the limit.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	if len(l.entries) > 10_000 {
		for k, e := range l.entries {
			if now.After(e.resetAt) {
				delete(l.entries, k)
			}
		}
	}

	e, ok := l.entries[key]
	if !ok || now.After(e.resetAt) {
		l.entries[key] = entry{count: 1, resetAt: now.Add(l.window)}
		return true
	}
	if e.count >= l.limit {
		return false
	}
	e.count++
	l.entries[key] = e
	return true
}
