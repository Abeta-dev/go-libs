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
