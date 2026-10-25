package auth

import (
	"net"
	"sync"
	"time"
)

// Failed attempts are counted per client address, shared by the REST API (bad
// API keys), the Web UI login form and the x3270 logon (#89). After
// maxFailures failures within failureWindow the address is blocked for
// blockTime: REST answers 429, the Web UI and x3270 report the usual failed
// login, and the log records the real reason.
const (
	maxFailures   = 5
	failureWindow = 5 * time.Minute
	blockTime     = 5 * time.Minute
)

type failureRecord struct {
	count        int
	windowStart  time.Time
	blockedUntil time.Time
}

// failureLimiter is the shared per-address failure counter.
type failureLimiter struct {
	mu        sync.Mutex
	addrs     map[string]*failureRecord
	lastPrune time.Time
	now       func() time.Time
}

func newFailureLimiter() *failureLimiter {
	return &failureLimiter{addrs: make(map[string]*failureRecord), now: time.Now}
}

// failures is the limiter used by all interfaces. Tests replace it.
var failures = newFailureLimiter()

// hostOnly drops a port, if any, so "10.0.0.5:1234" and "10.0.0.5" count together.
func hostOnly(addr string) string {
	if h, _, err := net.SplitHostPort(addr); err == nil {
		return h
	}
	return addr
}

// blocked reports whether addr is blocked, and for how much longer.
func (l *failureLimiter) blocked(addr string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	r, ok := l.addrs[hostOnly(addr)]
	if !ok {
		return false, 0
	}
	left := r.blockedUntil.Sub(l.now())
	return left > 0, left
}

// fail records a failed attempt from addr.
func (l *failureLimiter) fail(addr string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	l.prune(now)
	a := hostOnly(addr)
	r, ok := l.addrs[a]
	if !ok {
		r = &failureRecord{windowStart: now}
		l.addrs[a] = r
	}
	if now.Sub(r.windowStart) > failureWindow {
		r.count = 0
		r.windowStart = now
	}
	r.count++
	if r.count >= maxFailures {
		r.blockedUntil = now.Add(blockTime)
		r.count = 0
		r.windowStart = now
	}
}

// prune drops addresses with nothing recent, at most once a minute. Called with mu held.
func (l *failureLimiter) prune(now time.Time) {
	if now.Sub(l.lastPrune) < time.Minute {
		return
	}
	l.lastPrune = now
	for a, r := range l.addrs {
		if now.Sub(r.windowStart) > failureWindow && now.After(r.blockedUntil) {
			delete(l.addrs, a)
		}
	}
}

// hourlyOnce answers "first time this hour?" for a key, so that successful
// REST key use is logged once per user and address per hour (#89), not on
// every request.
type hourlyOnce struct {
	mu        sync.Mutex
	seen      map[string]time.Time
	lastPrune time.Time
	now       func() time.Time
}

func newHourlyOnce() *hourlyOnce {
	return &hourlyOnce{seen: make(map[string]time.Time), now: time.Now}
}

// hourly limits repeated log entries (successful key use, refused requests)
// to one per hour per user and address. Tests replace it.
var hourly = newHourlyOnce()

// first reports whether key has not been seen in the last hour, and marks it seen.
func (h *hourlyOnce) first(key string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := h.now()
	if now.Sub(h.lastPrune) > time.Minute {
		h.lastPrune = now
		for k, t := range h.seen {
			if now.Sub(t) >= time.Hour {
				delete(h.seen, k)
			}
		}
	}
	if t, ok := h.seen[key]; ok && now.Sub(t) < time.Hour {
		return false
	}
	h.seen[key] = now
	return true
}
