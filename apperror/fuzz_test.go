// SPDX-License-Identifier: MIT

package apperror_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/umesh0492/go-libs/apperror"
)

func FuzzErrorChain(f *testing.F) {
	seeds := []struct {
		code    string
		msg     string
		wrapMsg string
	}{
		{"BAD_REQUEST", "invalid input parameter", "validation failed"},
		{"NOT_FOUND", "user record 42", "sql: no rows in result set"},
		{"INTERNAL", "something crashed", "connection reset by peer"},
		{"UNAUTHORIZED", "token expired", "jwt signature invalid"},
		{"FORBIDDEN", "access denied", "insufficient role"},
		{"CONFLICT", "email already registered", "unique constraint violation"},
		{"UNPROCESSABLE", "negative balance", "business rule failure"},
		{"UNAVAILABLE", "circuit breaker open", "upstream timeout"},
		{"CUSTOM_CODE", "unknown issue", "underlying cause"},
		{"", "", ""},
		{"\x00\xff", "special chars\n\r\t", "nested \x00"},
	}

	for _, s := range seeds {
		f.Add(s.code, s.msg, s.wrapMsg)
	}

	f.Fuzz(func(t *testing.T, codeStr, msg, wrapMsg string) {
		code := apperror.Code(codeStr)
		var baseErr error
		if wrapMsg != "" {
			baseErr = errors.New(wrapMsg)
		}

		err := apperror.Wrap(baseErr, code, msg)
		if err == nil {
			t.Fatal("Wrap must return non-nil error")
		}

		// Verify Error() formatting never crashes
		errStr := err.Error()
		if !strings.Contains(errStr, string(code)) {
			t.Fatalf("expected Error() to contain code %q, got %q", code, errStr)
		}
		if !strings.Contains(errStr, msg) {
			t.Fatalf("expected Error() to contain message %q, got %q", msg, errStr)
		}

		// Verify Unwrap
		if baseErr != nil {
			if !errors.Is(err, baseErr) {
				t.Fatalf("expected errors.Is to match baseErr %v", baseErr)
			}
		}

		// Verify HTTP status is a valid 4xx or 5xx code
		status := err.GetStatus()
		if status < 400 || status > 599 {
			t.Fatalf("GetStatus() returned unexpected status: %d", status)
		}

		// Verify GetCode satisfies interface
		if err.GetCode() != string(code) {
			t.Fatalf("GetCode() = %q, want %q", err.GetCode(), string(code))
		}
	})
}
