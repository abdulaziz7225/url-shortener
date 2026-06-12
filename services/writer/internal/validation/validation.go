// Package validation checks user-supplied shorten inputs.
package validation

import (
	"errors"
	"net/url"
	"regexp"
	"time"
)

const maxURLLength = 2048

var aliasPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// LongURL accepts only absolute http(s) URLs of bounded length.
func LongURL(raw string) error {
	if raw == "" {
		return errors.New("long_url is required")
	}
	if len(raw) > maxURLLength {
		return errors.New("long_url exceeds maximum length")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return errors.New("long_url is not a valid URL")
	}
	if !u.IsAbs() || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return errors.New("long_url must be an absolute http(s) URL")
	}
	return nil
}

// Alias enforces a URL-safe, bounded custom alias.
func Alias(alias string) error {
	if !aliasPattern.MatchString(alias) {
		return errors.New("custom_alias must be 1-64 characters of letters, digits, '-' or '_'")
	}
	return nil
}

// Expiry requires the expiration to be in the future relative to now.
func Expiry(expiresAt, now time.Time) error {
	if !expiresAt.After(now) {
		return errors.New("expires_at must be in the future")
	}
	return nil
}
