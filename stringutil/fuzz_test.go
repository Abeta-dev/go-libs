// SPDX-License-Identifier: MIT

package stringutil_test

import (
	"testing"
	"unicode/utf8"

	"github.com/umesh0492/go-libs/stringutil"
)

func FuzzMaskEmail(f *testing.F) {
	seeds := []string{
		"john.doe@example.com",
		"a@example.com",
		"ab@example.com",
		"abc@example.com",
		"invalidemail",
		"",
		"@@@",
		"user+tag@sub.domain.co.uk",
		"josé.silva@empresa.com.br",
		"李雷@example.com",
		"user@localhost",
		"extremely.long.email.address.that.exceeds.normal.limits@domain.with.many.subdomains.example.com",
		"\x00\x01\x02@test.com",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, email string) {
		got := stringutil.MaskEmail(email)
		if utf8.ValidString(email) && !utf8.ValidString(got) {
			t.Fatalf("MaskEmail(%q) produced invalid UTF-8: %q", email, got)
		}
	})
}

func FuzzMaskPhone(f *testing.F) {
	seeds := []string{
		"+919876543210",
		"123",
		"1234",
		"12345",
		"1234567",
		"+1 (555) 019-2834",
		"",
		"9999999999",
		"phone-number-with-letters",
		"📞1234567890",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, phone string) {
		got := stringutil.MaskPhone(phone)
		if utf8.ValidString(phone) && !utf8.ValidString(got) {
			t.Fatalf("MaskPhone(%q) produced invalid UTF-8: %q", phone, got)
		}
	})
}

func FuzzTruncate(f *testing.F) {
	f.Add("hello world", 5)
	f.Add("hi", 5)
	f.Add("日本語のテスト", 3)
	f.Add("", 0)
	f.Add("testing", -1)
	f.Add("testing", -100)
	f.Add("testing", 0)
	f.Add("testing", 100)
	f.Add("🚀🌟🎉🔥", 2)

	f.Fuzz(func(t *testing.T, s string, n int) {
		got := stringutil.Truncate(s, n)
		if utf8.ValidString(s) && !utf8.ValidString(got) {
			t.Fatalf("Truncate(%q, %d) produced invalid UTF-8: %q", s, n, got)
		}
		if n <= 0 {
			if got != "" {
				t.Fatalf("Truncate with n <= 0 must return empty string: got %q", got)
			}
			return
		}
		if utf8.ValidString(got) {
			runeCount := utf8.RuneCountInString(got)
			if runeCount > n {
				t.Fatalf("Truncate rune count %d > requested %d for %q", runeCount, n, s)
			}
		}
	})
}
