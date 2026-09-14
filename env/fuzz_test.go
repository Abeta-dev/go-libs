// SPDX-License-Identifier: MIT

package env_test

import (
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/umesh0492/go-libs/env"
)

func FuzzDuration(f *testing.F) {
	seeds := []string{
		"1s",
		"500ms",
		"10m",
		"2h45m",
		"0s",
		"-5s",
		"invalid",
		"",
		"999999999999999999h",
		"1.5s",
		"100us",
		"100ns",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, val string) {
		const key = "TEST_FUZZ_DURATION_ENV"
		if err := os.Setenv(key, val); err != nil {
			return // Skip inputs rejected by the OS environment table (e.g. null bytes)
		}
		defer os.Unsetenv(key)

		def := 10 * time.Second
		got := env.Duration(key, def)

		expected, err := time.ParseDuration(val)
		if val == "" || err != nil {
			if got != def {
				t.Fatalf("expected default %v for val=%q, got %v", def, val, got)
			}
		} else {
			if got != expected {
				t.Fatalf("expected parsed %v for val=%q, got %v", expected, val, got)
			}
		}
	})
}

func FuzzInt(f *testing.F) {
	seeds := []string{
		"0",
		"42",
		"-100",
		"2147483647",
		"-2147483648",
		"9223372036854775807",
		"abc",
		"",
		"   12  ",
		"12.34",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, val string) {
		const key = "TEST_FUZZ_INT_ENV"
		if err := os.Setenv(key, val); err != nil {
			return // Skip inputs rejected by the OS environment table
		}
		defer os.Unsetenv(key)

		const def = 999
		got := env.Int(key, def)

		expected, err := strconv.Atoi(val)
		if val == "" || err != nil {
			if got != def {
				t.Fatalf("expected default %d for val=%q, got %d", def, val, got)
			}
		} else {
			if got != expected {
				t.Fatalf("expected parsed %d for val=%q, got %d", expected, val, got)
			}
		}
	})
}

func FuzzBool(f *testing.F) {
	seeds := []string{
		"true",
		"false",
		"1",
		"0",
		"t",
		"f",
		"TRUE",
		"FALSE",
		"yes",
		"no",
		"",
		"random",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, val string) {
		const key = "TEST_FUZZ_BOOL_ENV"
		if err := os.Setenv(key, val); err != nil {
			return // Skip inputs rejected by the OS environment table
		}
		defer os.Unsetenv(key)

		const def = false
		got := env.Bool(key, def)

		expected, err := strconv.ParseBool(strings.TrimSpace(val))
		if val == "" || err != nil {
			if got != def {
				t.Fatalf("expected default %v for val=%q, got %v", def, val, got)
			}
		} else {
			if got != expected {
				t.Fatalf("expected parsed %v for val=%q, got %v", expected, val, got)
			}
		}
	})
}
