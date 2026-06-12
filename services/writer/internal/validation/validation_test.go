package validation

import (
	"testing"
	"time"
)

func TestLongURL(t *testing.T) {
	valid := []string{"http://example.com", "https://example.com/path?q=1", "https://sub.example.com:8443/x"}
	for _, u := range valid {
		if err := LongURL(u); err != nil {
			t.Errorf("LongURL(%q) unexpected error: %v", u, err)
		}
	}
	invalid := []string{"", "ftp://example.com", "not a url", "example.com", "//example.com", "javascript:alert(1)"}
	for _, u := range invalid {
		if err := LongURL(u); err == nil {
			t.Errorf("LongURL(%q) expected error", u)
		}
	}
}

func TestAlias(t *testing.T) {
	for _, a := range []string{"my-alias", "Abc_123", "x"} {
		if err := Alias(a); err != nil {
			t.Errorf("Alias(%q) unexpected error: %v", a, err)
		}
	}
	for _, a := range []string{"", "has space", "bad/slash", "emoji🚀", "toolong-" + string(make([]byte, 64))} {
		if err := Alias(a); err == nil {
			t.Errorf("Alias(%q) expected error", a)
		}
	}
}

func TestExpiry(t *testing.T) {
	now := time.Now()
	if err := Expiry(now.Add(time.Hour), now); err != nil {
		t.Errorf("future expiry should be valid: %v", err)
	}
	if err := Expiry(now.Add(-time.Hour), now); err == nil {
		t.Error("past expiry should be invalid")
	}
}
