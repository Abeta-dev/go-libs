// SPDX-License-Identifier: MIT

package uuidutil

import (
	"strings"

	"github.com/google/uuid"
)

// UUID is an alias for uuid.UUID.
type UUID = uuid.UUID

// Nil is the zero UUID value.
var Nil = uuid.Nil

// New generates a new random v4 UUID.
func New() UUID {
	return uuid.New()
}

// NewString generates a new random v4 UUID string.
func NewString() string {
	return uuid.New().String()
}

// NewV7 generates a new time-ordered v7 UUID (RFC 9562), falling back to v4 only if the CSPRNG fails.
func NewV7() UUID {
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.New()
	}
	return id
}

// NewV7String generates a new time-ordered v7 UUID string.
func NewV7String() string {
	return NewV7().String()
}

// Parse parses a UUID string, returning an error on failure.
func Parse(s string) (UUID, error) {
	return uuid.Parse(s)
}

// ParsePtr parses a UUID string, returning a pointer to the UUID or nil if empty.
func ParsePtr(s string) (*UUID, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	u, err := uuid.Parse(s)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// MustParse parses a UUID string and panics on failure.
func MustParse(s string) UUID {
	return uuid.MustParse(s)
}

// IsValid reports whether s is a valid UUID string.
func IsValid(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}

// IsNil reports whether the UUID is the zero value.
func IsNil(id UUID) bool {
	return id == uuid.Nil
}
