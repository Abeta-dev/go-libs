// SPDX-License-Identifier: MIT

// Package stringutil provides domain-agnostic string helper functions.
//
// Has zero knowledge of any business domain. Suitable for use in any service
// that imports go-libs.
package stringutil

import (
	"crypto/rand"
	"errors"
	"io"
	"math/big"
	"strings"
)

// OrDefault returns s if non-empty, otherwise returns the defaultVal.
// Replaces ad-hoc ternary patterns across the codebase.
func OrDefault(s, defaultVal string) string {
	if s == "" {
		return defaultVal
	}
	return s
}

// Truncate returns the first n runes of s. If s is shorter than n, returns s unchanged.
// If n <= 0, returns an empty string. Safe for multi-byte UTF-8 strings.
func Truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}

// MaskEmail masks an email address for safe logging.
// "john.doe@example.com" → "jo******@example.com"
func MaskEmail(email string) string {
	at := strings.LastIndex(email, "@")
	if at < 1 {
		return "**@**"
	}
	local := email[:at]
	domain := email[at:]
	runes := []rune(local)
	visible := 2
	if len(runes) < visible {
		visible = len(runes)
	}
	return string(runes[:visible]) + strings.Repeat("*", len(runes)-visible) + domain
}

// MaskPhone masks a phone number for safe logging.
// Preserves leading '+' if present and masks all characters except the last 4 digits.
func MaskPhone(phone string) string {
	runes := []rune(phone)
	if len(runes) < 4 {
		return "****"
	}
	hasPlus := len(runes) > 0 && runes[0] == '+'
	if hasPlus {
		if len(runes) < 5 {
			return "****"
		}
		suffix := string(runes[len(runes)-4:])
		maskLen := len(runes) - 5
		return "+" + strings.Repeat("*", maskLen) + suffix
	}

	suffix := string(runes[len(runes)-4:])
	maskLen := len(runes) - 4
	return strings.Repeat("*", maskLen) + suffix
}

// TrimAndLower trims whitespace and lowercases a string.
// Useful for normalising email addresses before DB lookup.
func TrimAndLower(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// IsBlank returns true if the string is empty or contains only whitespace.
func IsBlank(s string) bool {
	return strings.TrimSpace(s) == ""
}

// ContainsAny returns true if s contains any of the given substrings.
func ContainsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

var randReader = rand.Reader

// RandomStringFromReader returns a random string of length n drawn from charset using reader r.
func RandomStringFromReader(r io.Reader, length int, charset string) (string, error) {
	if charset == "" {
		return "", errors.New("stringutil: charset cannot be empty")
	}
	password := make([]byte, length)
	for i := range password {
		n, err := rand.Int(r, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		password[i] = charset[n.Int64()]
	}
	return string(password), nil
}

// RandomStringFromCharset returns a cryptographically secure random string of length n drawn from charset.
func RandomStringFromCharset(length int, charset string) (string, error) {
	return RandomStringFromReader(randReader, length, charset)
}

// RandomSecureString returns a random string of the specified length containing alphanumeric characters and symbols (!@#$).
// Uses crypto/rand for cryptographic security.
func RandomSecureString(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$"
	return RandomStringFromCharset(length, charset)
}

// RandomAlphanumeric returns a cryptographically secure random string of the specified length
// strictly consisting of alphanumeric characters ([a-zA-Z0-9]).
func RandomAlphanumeric(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	return RandomStringFromCharset(length, charset)
}
