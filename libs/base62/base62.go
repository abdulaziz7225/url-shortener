// Package base62 encodes unsigned integers into compact, URL-safe short codes.
package base62

import (
	"errors"
	"strings"
)

const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

const base = uint64(len(alphabet))

// ErrInvalidCharacter is returned by Decode when the input contains a byte
// outside the base62 alphabet.
var ErrInvalidCharacter = errors.New("base62: invalid character")

// Encode renders n in base62. The zero value encodes to "0".
func Encode(n uint64) string {
	if n == 0 {
		return string(alphabet[0])
	}
	var b [11]byte // ceil(log62(2^64)) == 11
	i := len(b)
	for n > 0 {
		i--
		b[i] = alphabet[n%base]
		n /= base
	}
	return string(b[i:])
}

// Decode parses a base62 string back into its numeric value.
func Decode(s string) (uint64, error) {
	if s == "" {
		return 0, ErrInvalidCharacter
	}
	var n uint64
	for i := 0; i < len(s); i++ {
		idx := strings.IndexByte(alphabet, s[i])
		if idx < 0 {
			return 0, ErrInvalidCharacter
		}
		n = n*base + uint64(idx)
	}
	return n, nil
}
