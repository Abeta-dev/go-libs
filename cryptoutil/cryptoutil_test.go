// SPDX-License-Identifier: MIT

package cryptoutil_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/umesh0492/go-libs/cryptoutil"
	"github.com/umesh0492/go-libs/stringutil"
	"golang.org/x/crypto/bcrypt"
)

type errorReader struct{}

func (errorReader) Read(p []byte) (int, error) {
	return 0, errors.New("rand error")
}

func TestGenerateTempPassword(t *testing.T) {
	tests := []struct {
		name    string
		length  int
		wantErr error
	}{
		{"valid length", 12, nil},
		{"minimum valid length", 8, nil},
		{"too short", 7, cryptoutil.ErrPasswordTooShort},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := cryptoutil.GenerateTempPassword(tt.length)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("GenerateTempPassword() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && len(got) != tt.length {
				t.Errorf("GenerateTempPassword() returned length %d, want %d", len(got), tt.length)
			}
		})
	}
}

func TestGenerateTempPassword_RandError(t *testing.T) {
	cryptoutil.SetRandReader(errorReader{})
	defer cryptoutil.ResetRandReader()

	pwd, err := cryptoutil.GenerateTempPassword(10)
	assert.Error(t, err)
	assert.Equal(t, "", pwd)
	assert.Contains(t, err.Error(), "rand error")
}

func TestHashAndComparePassword(t *testing.T) {
	password := "superSecret123!"

	hash, err := cryptoutil.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	if hash == password {
		t.Error("HashPassword() returned plain text password")
	}

	err = cryptoutil.ComparePassword(hash, password)
	if err != nil {
		t.Errorf("ComparePassword() failed for valid password: %v", err)
	}

	err = cryptoutil.ComparePassword(hash, "wrongpassword")
	if err == nil {
		t.Error("ComparePassword() succeeded for invalid password")
	} else if !errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		t.Errorf("ComparePassword() error = %v, want %v", err, bcrypt.ErrMismatchedHashAndPassword)
	}

	if !cryptoutil.IsCorrectPassword(hash, password) {
		t.Error("IsCorrectPassword() returned false for valid password")
	}

	if cryptoutil.IsCorrectPassword(hash, "wrongpassword") {
		t.Error("IsCorrectPassword() returned true for invalid password")
	}

	t.Run("password too long for bcrypt", func(t *testing.T) {
		longPwd := make([]byte, 100)
		for i := range longPwd {
			longPwd[i] = 'a'
		}
		_, err := cryptoutil.HashPassword(string(longPwd))
		if err == nil {
			t.Error("HashPassword() succeeded for > 72 byte password")
		}
	})
}

func TestHashAndComparePassword_WithRandomAlphanumeric(t *testing.T) {
	pwd, err := stringutil.RandomAlphanumeric(16)
	assert.NoError(t, err)
	assert.Len(t, pwd, 16)

	hash, err := cryptoutil.HashPassword(pwd)
	assert.NoError(t, err)

	assert.NoError(t, cryptoutil.ComparePassword(hash, pwd))
	assert.True(t, cryptoutil.IsCorrectPassword(hash, pwd))
}

func TestComparePassword_InvalidHash(t *testing.T) {
	tests := []struct {
		name string
		hash string
	}{
		{"empty hash", ""},
		{"too short", "short"},
		{"invalid prefix", "invalid-prefix-hash-that-is-long-enough-to-pass-length-checks"},
		{"not bcrypt", "$1$randomsalt$abcdefghijklmn"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cryptoutil.ComparePassword(tt.hash, "password123")
			assert.Error(t, err)
			assert.True(t, errors.Is(err, cryptoutil.ErrInvalidHash), "expected ErrInvalidHash, got: %v", err)
			assert.False(t, cryptoutil.IsCorrectPassword(tt.hash, "password123"))
		})
	}
}
