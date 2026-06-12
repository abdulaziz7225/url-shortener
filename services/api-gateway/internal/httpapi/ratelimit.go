package httpapi

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// IPRateLimiter applies an independent token bucket per client IP.
type IPRateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rps     rate.Limit
	burst   int
	ttl     time.Duration
	nowFn   func() time.Time
}

type bucket struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewIPRateLimiter builds a limiter allowing rps requests/second with the given
// burst per IP. Idle buckets are evicted after ttl.
func NewIPRateLimiter(rps float64, burst int, ttl time.Duration) *IPRateLimiter {
	return &IPRateLimiter{
		buckets: make(map[string]*bucket),
		rps:     rate.Limit(rps),
		burst:   burst,
		ttl:     ttl,
		nowFn:   time.Now,
	}
}

// Allow reports whether a request from ip may proceed.
func (l *IPRateLimiter) Allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.nowFn()
	b, ok := l.buckets[ip]
	if !ok {
		b = &bucket{limiter: rate.NewLimiter(l.rps, l.burst)}
		l.buckets[ip] = b
	}
	b.lastSeen = now
	return b.limiter.Allow()
}

// evictStale removes buckets idle longer than ttl.
func (l *IPRateLimiter) evictStale() {
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := l.nowFn().Add(-l.ttl)
	for ip, b := range l.buckets {
		if b.lastSeen.Before(cutoff) {
			delete(l.buckets, ip)
		}
	}
}

// RunEviction periodically prunes idle buckets until ctx-equivalent stop chan
// is closed.
func (l *IPRateLimiter) RunEviction(stop <-chan struct{}) {
	ticker := time.NewTicker(l.ttl)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			l.evictStale()
		}
	}
}
