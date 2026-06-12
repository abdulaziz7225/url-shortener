package httpapi

import (
	"testing"
	"time"
)

func TestIPRateLimiterPerIP(t *testing.T) {
	l := NewIPRateLimiter(1, 1, time.Minute)

	if !l.Allow("1.1.1.1") {
		t.Fatal("first request for an IP should be allowed")
	}
	if l.Allow("1.1.1.1") {
		t.Fatal("second immediate request for the same IP should be denied")
	}
	if !l.Allow("2.2.2.2") {
		t.Fatal("a different IP has its own bucket and should be allowed")
	}
}

func TestIPRateLimiterEviction(t *testing.T) {
	now := time.Unix(0, 0)
	l := NewIPRateLimiter(1, 1, time.Minute)
	l.nowFn = func() time.Time { return now }

	l.Allow("1.1.1.1")
	if len(l.buckets) != 1 {
		t.Fatalf("expected 1 bucket, got %d", len(l.buckets))
	}

	now = now.Add(2 * time.Minute)
	l.evictStale()
	if len(l.buckets) != 0 {
		t.Fatalf("stale bucket should have been evicted, got %d", len(l.buckets))
	}
}
