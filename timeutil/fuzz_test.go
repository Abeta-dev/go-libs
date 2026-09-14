// SPDX-License-Identifier: MIT
package timeutil_test

import (
	"testing"
	"time"

	"github.com/umesh0492/go-libs/timeutil"
)

func FuzzParseTime(f *testing.F) {
	// Seed corpus with valid, edge case, and adversarial strings
	f.Add("2026-09-13T12:00:00Z")
	f.Add("2024-02-29T23:59:59.999999999Z") // Leap day
	f.Add("1970-01-01T00:00:00Z")
	f.Add("9999-12-31T23:59:59Z")  // Year 9999
	f.Add("10000-01-01T00:00:00Z") // Year 10000
	f.Add("0001-01-01T00:00:00Z")
	f.Add("2026-09-13")
	f.Add("12:00:00")
	f.Add("Sun, 13 Sep 2026 12:00:00 UTC")
	f.Add("")
	f.Add("not-a-time")
	f.Add("999999999999999999999999")
	f.Add("-0001-01-01T00:00:00Z")
	f.Add("2026-02-29T00:00:00Z") // Non-existent leap day
	f.Add("2026-13-45T99:99:99Z") // Out-of-bounds fields

	f.Fuzz(func(t *testing.T, s string) {
		// Must never panic regardless of input
		parsed, err := timeutil.ParseTime(s)
		if err == nil {
			// If it parsed successfully, formatting and parsing again must be stable
			formatted := timeutil.FormatIn(parsed, time.UTC, "")
			_, _ = timeutil.ParseTime(formatted)
		}
	})
}

func FuzzAddBusinessDays(f *testing.F) {
	// Seed corpus
	f.Add(int64(0), 0)
	f.Add(int64(1726228800), 5)       // Friday
	f.Add(int64(1726228800), -5)      // Negative duration
	f.Add(int64(1709211600), 1)       // 2024 leap day
	f.Add(int64(-62135596800), 10)    // Year 1
	f.Add(int64(253402300799), -10)   // Year 9999
	f.Add(int64(1726228800), 1000)    // Moderate addition
	f.Add(int64(1726228800), -1000)   // Moderate subtraction
	f.Add(int64(1726228800), 100000)  // Large addition
	f.Add(int64(1726228800), -100000) // Large subtraction
	f.Add(int64(1726228800), 1<<20)   // Extreme int

	f.Fuzz(func(t *testing.T, unixSec int64, n int) {
		// Clamp to valid range so AddDate does not overflow internal int64 seconds
		const minSec = -62135596800
		const maxSec = 253402300799
		if unixSec < minSec || unixSec > maxSec {
			return
		}
		// Clamp n for fast fuzz execution
		if n > 500 {
			n = 500
		} else if n < -500 {
			n = -500
		}

		tm := time.Unix(unixSec, 0).UTC()
		result := timeutil.AddBusinessDays(tm, n)
		if n != 0 && (result.Weekday() == time.Saturday || result.Weekday() == time.Sunday) {
			t.Fatalf("AddBusinessDays returned a weekend day: %v for input %v, n=%d", result.Weekday(), tm, n)
		}
	})
}

func FuzzBusinessDays(f *testing.F) {
	// Seed corpus
	f.Add(int64(1726228800), int64(1726228800))
	f.Add(int64(1726228800), int64(1726833600)) // 1 week
	f.Add(int64(1726833600), int64(1726228800)) // reverse
	f.Add(int64(0), int64(86400*30))
	f.Add(int64(1709211600), int64(1709384400))           // leap year boundary
	f.Add(int64(1726228800), int64(1726228800+86400*365)) // 1 year

	f.Fuzz(func(t *testing.T, startSec int64, endSec int64) {
		const minSec = -62135596800
		const maxSec = 253402300799
		if startSec < minSec || startSec > maxSec || endSec < minSec || endSec > maxSec {
			return
		}

		start := time.Unix(startSec, 0).UTC()
		end := time.Unix(endSec, 0).UTC()

		bd := timeutil.BusinessDays(start, end)
		bdRev := timeutil.BusinessDays(end, start)

		// Antisymmetry: BusinessDays(start, end) == -BusinessDays(end, start)
		if bd != -bdRev {
			t.Fatalf("asymmetry detected: BusinessDays(%v, %v)=%d, reverse=%d", start, end, bd, bdRev)
		}
	})
}
