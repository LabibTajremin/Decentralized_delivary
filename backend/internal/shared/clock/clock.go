// Package clock abstracts the system clock so time-dependent behaviour is
// testable without sleeping. Production wiring passes System; tests pass a
// Fixed clock and advance it explicitly.
package clock

import (
	"sync"
	"time"
)

// Clock reports the current time.
type Clock interface {
	Now() time.Time
}

// System reads the real clock, normalised to UTC. Every timestamp the backend
// stores or compares is UTC; local time exists only at the presentation edge.
type System struct{}

// Now returns the current UTC time.
func (System) Now() time.Time { return time.Now().UTC() }

// Fixed is a controllable clock for tests. It is safe for concurrent use.
type Fixed struct {
	mu sync.Mutex
	t  time.Time
}

// NewFixed returns a clock pinned to t, normalised to UTC.
func NewFixed(t time.Time) *Fixed { return &Fixed{t: t.UTC()} }

// Now returns the pinned time.
func (f *Fixed) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.t
}

// Advance moves the clock forward by d and returns the new time.
func (f *Fixed) Advance(d time.Duration) time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.t = f.t.Add(d)
	return f.t
}

// Set moves the clock to t exactly.
func (f *Fixed) Set(t time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.t = t.UTC()
}
