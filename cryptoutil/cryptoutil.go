// SPDX-License-Identifier: MIT

// Package cryptoutil provides domain-agnostic cryptographic helpers.
//
// Extracted from core-service/internal/service/auth_service.go to avoid
// duplication across services. Has zero knowledge of any business domain.
//
// Dependencies: golang.org/x/crypto/bcrypt and crypto/rand.
package cryptoutil

import (
	"crypto/rand"
	"errors"
	"fmt"

	"github.com/umesh0492/go-libs/stringutil"
	"golang.org/x/crypto/bcrypt"
)

const (
	// DefaultPasswordLength is the length of generated temporary passwords.
	DefaultPasswordLength = 12

	// BcryptCost is the work factor for password hashing.
	// 12 is the OWASP-recommended minimum for 2024+.
	BcryptCost = 12

	// passwordChars is the character set for temporary passwords.
	// Excludes ambiguous characters (0, O, I, l) to avoid transcription errors.
	passwordChars = "ABCDEFGHJKLMNPQRSTUVWXYZ" +
		"abcdefghijkmnpqrstuvwxyz" +
		"23456789" +
		"!@#$%^&*"
)

// Error variables returned by cryptoutil operations.
var (
	// ErrPasswordTooShort is returned when requesting a temporary password shorter than 8 characters.
	ErrPasswordTooShort = errors.New("cryptoutil: password length must be >= 8")
	// ErrInvalidHash is returned when a supplied password hash is malformed.
	ErrInvalidHash = errors.New("cryptoutil: invalid password hash")
)

var randReader = rand.Reader

// GenerateTempPassword generates a cryptographically secure random password
// of the given length using the safe character set.
// Returns a plain-text password that must be hashed before persistence.
func GenerateTempPassword(length int) (string, error) {
	if length < 8 {
		return "", ErrPasswordTooShort
	}
	return stringutil.RandomStringFromReader(randReader, length, passwordChars)
}

var bcryptGenerateFromPassword = bcrypt.GenerateFromPassword

// HashPassword returns the bcrypt hash of the plain-text password.
// Uses BcryptCost (12) work factor.
func HashPassword(password string) (string, error) {
	b, err := bcryptGenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// ComparePassword returns nil if the plain-text password matches the hash.
// Returns bcrypt.ErrMismatchedHashAndPassword on mismatch.
// Returns ErrInvalidHash if the hash is malformed.
func ComparePassword(hash, password string) error {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return err
		}
		return fmt.Errorf("%w: %w", ErrInvalidHash, err)
	}
	return nil
}

// IsCorrectPassword is a convenience bool wrapper around ComparePassword.
func IsCorrectPassword(hash, password string) bool {
	return ComparePassword(hash, password) == nil
}
