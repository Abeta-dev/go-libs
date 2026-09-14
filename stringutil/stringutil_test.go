// SPDX-License-Identifier: MIT

package stringutil_test

import (
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/umesh0492/go-libs/stringutil"
)

type errorReader struct{}

func (errorReader) Read(p []byte) (int, error) {
	return 0, errors.New("rand error")
}

func TestOrDefault(t *testing.T) {
	if got := stringutil.OrDefault("", "default"); got != "default" {
		t.Errorf("OrDefault() = %v, want %v", got, "default")
	}
	if got := stringutil.OrDefault("value", "default"); got != "value" {
		t.Errorf("OrDefault() = %v, want %v", got, "value")
	}
}

func TestTruncate(t *testing.T) {
	if got := stringutil.Truncate("hello world", 5); got != "hello" {
		t.Errorf("Truncate() = %v, want %v", got, "hello")
	}
	if got := stringutil.Truncate("hi", 5); got != "hi" {
		t.Errorf("Truncate() = %v, want %v", got, "hi")
	}
	if got := stringutil.Truncate("日本語のテスト", 3); got != "日本語" {
		t.Errorf("Truncate() = %v, want %v", got, "日本語")
	}
}

func TestMaskEmail(t *testing.T) {
	tests := []struct {
		email string
		want  string
	}{
		{"john.doe@example.com", "jo******@example.com"},
		{"a@example.com", "a@example.com"},
		{"ab@example.com", "ab@example.com"},
		{"invalidemail", "**@**"},
		{"", "**@**"},
	}

	for _, tt := range tests {
		got := stringutil.MaskEmail(tt.email)
		assert.Equal(t, tt.want, got, "MaskEmail(%q)", tt.email)
	}
}

func TestMaskPhone(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"+919876543210", "+********3210"},
		{"+14155552671", "+*******2671"},
		{"+447911123456", "+********3456"},
		{"+1234", "+1234"},
		{"+123", "****"},
		{"123", "****"},
		{"1234", "1234"},
		{"1234567", "***4567"},
		{"9876543210", "******3210"},
		{"", "****"},
	}

	for _, tt := range tests {
		got := stringutil.MaskPhone(tt.input)
		assert.Equal(t, tt.want, got, "MaskPhone(%q)", tt.input)
	}
}

func TestTrimAndLower(t *testing.T) {
	if got := stringutil.TrimAndLower("  Hello World  "); got != "hello world" {
		t.Errorf("TrimAndLower() = %v, want %v", got, "hello world")
	}
}

func TestIsBlank(t *testing.T) {
	if !stringutil.IsBlank("   ") {
		t.Errorf("IsBlank() returned false for blank string")
	}
	if stringutil.IsBlank(" a ") {
		t.Errorf("IsBlank() returned true for non-blank string")
	}
}

func TestContainsAny(t *testing.T) {
	if !stringutil.ContainsAny("hello world", "world", "planet") {
		t.Errorf("ContainsAny() returned false")
	}
	if stringutil.ContainsAny("hello world", "planet", "galaxy") {
		t.Errorf("ContainsAny() returned true")
	}
}

func TestRandomSecureString(t *testing.T) {
	got, err := stringutil.RandomSecureString(12)
	if err != nil {
		t.Errorf("RandomSecureString() error = %v", err)
	}
	if len(got) != 12 {
		t.Errorf("RandomSecureString() length = %v, want %v", len(got), 12)
	}
}

func TestRandomSecureString_Error(t *testing.T) {
	stringutil.SetRandReader(errorReader{})
	defer stringutil.ResetRandReader()

	str, err := stringutil.RandomSecureString(10)
	assert.Error(t, err)
	assert.Equal(t, "", str)
	assert.Contains(t, err.Error(), "rand error")
}

func TestRandomAlphanumeric(t *testing.T) {
	got, err := stringutil.RandomAlphanumeric(32)
	assert.NoError(t, err)
	assert.Len(t, got, 32)

	// Verify all characters are in [a-zA-Z0-9]
	matched, err := regexp.MatchString(`^[a-zA-Z0-9]{32}$`, got)
	assert.NoError(t, err)
	assert.True(t, matched, "expected alphanumeric string, got: %s", got)

	// Verify it contains no symbols (!@#$)
	assert.False(t, strings.ContainsAny(got, "!@#$"))
}

func TestRandomAlphanumeric_Error(t *testing.T) {
	stringutil.SetRandReader(errorReader{})
	defer stringutil.ResetRandReader()

	str, err := stringutil.RandomAlphanumeric(10)
	assert.Error(t, err)
	assert.Equal(t, "", str)
	assert.Contains(t, err.Error(), "rand error")
}

func TestRandomStringFromCharset_EmptyCharset(t *testing.T) {
	str, err := stringutil.RandomStringFromCharset(10, "")
	assert.Error(t, err)
	assert.Equal(t, "", str)
	assert.Contains(t, err.Error(), "charset cannot be empty")
}
