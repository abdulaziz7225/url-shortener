// Package model holds the persistency-controller domain types.
package model

import (
	"errors"
	"time"
)

// ErrNotFound is returned when a code has no mapping.
var ErrNotFound = errors.New("mapping not found")

// Mapping is a stored short-code -> long-URL record.
type Mapping struct {
	Code      string
	LongURL   string
	ExpiresAt *time.Time
	CreatedAt time.Time
}
