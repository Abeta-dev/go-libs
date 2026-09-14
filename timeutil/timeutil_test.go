// SPDX-License-Identifier: MIT

package timeutil_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/umesh0492/go-libs/timeutil"
)

func TestNowIn(t *testing.T) {
	utc := time.UTC
	now := timeutil.NowIn(utc)
	assert.Equal(t, "UTC", now.Location().String())
}

func TestFormatIn(t *testing.T) {
	tm := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	got := timeutil.FormatIn(tm, time.UTC, "")
	want := "2024-01-01T12:00:00Z"
	assert.Equal(t, want, got)

	got2 := timeutil.FormatIn(tm, time.UTC, "2006-01-02")
	assert.Equal(t, "2024-01-01", got2)
}

func TestStartOfDay(t *testing.T) {
	tm := time.Date(2024, 6, 15, 22, 30, 0, 0, time.UTC)
	start := timeutil.StartOfDay(tm, time.UTC)
	assert.Equal(t, 0, start.Hour())
	assert.Equal(t, 0, start.Minute())
	assert.Equal(t, 0, start.Second())
	assert.Equal(t, 0, start.Nanosecond())
	assert.Equal(t, 2024, start.Year())
	assert.Equal(t, time.June, start.Month())
	assert.Equal(t, 15, start.Day())
}

func TestEndOfDay(t *testing.T) {
	tm := time.Date(2024, 6, 15, 1, 0, 0, 0, time.UTC)
	end := timeutil.EndOfDay(tm, time.UTC)
	assert.Equal(t, 23, end.Hour())
	assert.Equal(t, 59, end.Minute())
	assert.Equal(t, 59, end.Second())
	assert.Equal(t, 999999999, end.Nanosecond())
	assert.Equal(t, 2024, end.Year())
	assert.Equal(t, time.June, end.Month())
	assert.Equal(t, 15, end.Day())
}

func TestAddBusinessDays(t *testing.T) {
	tm := time.Date(2024, 5, 3, 12, 0, 0, 0, time.UTC) // Friday
	got := timeutil.AddBusinessDays(tm, 2)             // Should be Tuesday
	assert.Equal(t, time.Tuesday, got.Weekday())
}

func TestNilLocationDefaults(t *testing.T) {
	now := timeutil.NowIn(nil)
	assert.Equal(t, "UTC", now.Location().String())

	tm := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	assert.Equal(t, "2024-01-01T12:00:00Z", timeutil.FormatIn(tm, nil, ""))

	start := timeutil.StartOfDay(tm, nil)
	assert.Equal(t, 0, start.Hour())

	end := timeutil.EndOfDay(tm, nil)
	assert.Equal(t, 23, end.Hour())

	// AddBusinessDays edge cases
	assert.Equal(t, tm, timeutil.AddBusinessDays(tm, 0))
	clamped := timeutil.AddBusinessDays(tm, 2000000)
	assert.True(t, clamped.After(tm))
	clampedNeg := timeutil.AddBusinessDays(tm, -2000000)
	assert.True(t, clampedNeg.Before(tm))
	subDays := timeutil.AddBusinessDays(tm, -2)
	assert.Equal(t, time.Thursday, subDays.Weekday())

	// AddBusinessDays starting on weekend days
	sat := time.Date(2024, 5, 4, 12, 0, 0, 0, time.UTC) // Saturday
	sun := time.Date(2024, 5, 5, 12, 0, 0, 0, time.UTC) // Sunday
	assert.Equal(t, time.Monday, timeutil.AddBusinessDays(sat, 1).Weekday())
	assert.Equal(t, time.Monday, timeutil.AddBusinessDays(sun, 1).Weekday())
	assert.Equal(t, time.Friday, timeutil.AddBusinessDays(sat, -1).Weekday())
	assert.Equal(t, time.Friday, timeutil.AddBusinessDays(sun, -1).Weekday())
	assert.Equal(t, time.Tuesday, timeutil.AddBusinessDays(sat, 7).Weekday())
	assert.Equal(t, time.Thursday, timeutil.AddBusinessDays(sat, -7).Weekday())
}

func TestParseTime(t *testing.T) {
	_, err := timeutil.ParseTime("")
	assert.Error(t, err)

	_, err = timeutil.ParseTime("invalid-date-string")
	assert.Error(t, err)

	parsed, err := timeutil.ParseTime("2024-05-01", "2006-01-02")
	assert.NoError(t, err)
	assert.Equal(t, 2024, parsed.Year())
	assert.Equal(t, time.May, parsed.Month())
	assert.Equal(t, 1, parsed.Day())
}

func TestBusinessDays(t *testing.T) {
	t1 := time.Date(2024, 5, 3, 10, 0, 0, 0, time.UTC) // Friday
	t2 := time.Date(2024, 5, 7, 15, 0, 0, 0, time.UTC) // Tuesday

	assert.Equal(t, 2, timeutil.BusinessDays(t1, t2))
	assert.Equal(t, 2, timeutil.BusinessDaysBetween(t1, t2))
	assert.Equal(t, -2, timeutil.BusinessDaysBetween(t2, t1))
	assert.Equal(t, 0, timeutil.BusinessDaysBetween(t1, t1))

	// Same calendar day, different hour
	t3 := time.Date(2024, 5, 3, 18, 0, 0, 0, time.UTC)
	assert.Equal(t, 0, timeutil.BusinessDaysBetween(t1, t3))

	// Multi-week span
	t4 := time.Date(2024, 5, 3, 10, 0, 0, 0, time.UTC)
	t5 := time.Date(2024, 5, 24, 10, 0, 0, 0, time.UTC) // 3 weeks later
	assert.Equal(t, 15, timeutil.BusinessDaysBetween(t4, t5))

	// Dates with negative years (BC) to exercise civil calendar era arithmetic
	bc1 := time.Date(-50, 1, 1, 0, 0, 0, 0, time.UTC)
	bc2 := time.Date(50, 1, 1, 0, 0, 0, 0, time.UTC)
	assert.True(t, timeutil.BusinessDaysBetween(bc1, bc2) > 0)
}
