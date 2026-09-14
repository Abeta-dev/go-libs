// SPDX-License-Identifier: MIT

package timeutil

import (
	"errors"
	"fmt"
	"time"
)

// ErrInvalidTimeFormat indicates that the input string could not be parsed with any supported layout.
var ErrInvalidTimeFormat = errors.New("timeutil: invalid time format")

// CommonLayouts contains standard date and time layouts used for heuristic parsing.
var CommonLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	time.DateTime,
	time.DateOnly,
	time.TimeOnly,
	"2006-01-02 15:04:05.999999999",
	"2006-01-02 15:04:05",
	"2006/01/02 15:04:05",
	"2006/01/02",
	"02-01-2006",
	"02/01/2006",
	time.RFC1123Z,
	time.RFC1123,
	time.RFC822Z,
	time.RFC822,
	time.ANSIC,
	time.UnixDate,
	time.RubyDate,
}

// ParseTime parses a time string using the specified layouts, or CommonLayouts if none are provided.
// It returns the parsed time.Time or ErrInvalidTimeFormat if parsing fails.
func ParseTime(s string, layouts ...string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("%w: empty input", ErrInvalidTimeFormat)
	}
	if len(layouts) == 0 {
		layouts = CommonLayouts
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("%w: %q", ErrInvalidTimeFormat, s)
}

// NowIn returns the current time in the given location.
func NowIn(loc *time.Location) time.Time {
	if loc == nil {
		loc = time.UTC
	}
	return time.Now().In(loc)
}

// FormatIn formats a time.Time in the given location using the given layout.
// Uses time.RFC3339 layout by default if layout is empty.
func FormatIn(t time.Time, loc *time.Location, layout string) string {
	if loc == nil {
		loc = time.UTC
	}
	if layout == "" {
		layout = time.RFC3339
	}
	return t.In(loc).Format(layout)
}

// StartOfDay returns 00:00:00 in the given location for the given date.
func StartOfDay(t time.Time, loc *time.Location) time.Time {
	if loc == nil {
		loc = time.UTC
	}
	y, m, d := t.In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, loc)
}

// EndOfDay returns 23:59:59.999999999 in the given location for the given date.
func EndOfDay(t time.Time, loc *time.Location) time.Time {
	if loc == nil {
		loc = time.UTC
	}
	y, m, d := t.In(loc).Date()
	return time.Date(y, m, d, 23, 59, 59, 999999999, loc)
}

// daysFromCivil returns the number of days since 1970-01-01 for any Gregorian date (y, m, d).
// Uses the Howard Hinnant civil day algorithm (O(1) with zero allocations).
func daysFromCivil(y int, m time.Month, d int) int {
	yInt := y
	mInt := int(m)
	if mInt <= 2 {
		yInt--
	}
	era := yInt / 400
	if yInt < 0 && yInt%400 != 0 {
		era--
	}
	yoe := yInt - era*400
	doy := mInt + 9
	if mInt > 2 {
		doy = mInt - 3
	}
	doy = (153*doy+2)/5 + d - 1
	doe := yoe*365 + yoe/4 - yoe/100 + doy
	return era*146097 + doe - 719468
}

func addPositiveBusinessDays(d time.Time, n int) time.Time {
	if d.Weekday() == time.Saturday {
		d = d.AddDate(0, 0, 2)
		n--
	} else if d.Weekday() == time.Sunday {
		d = d.AddDate(0, 0, 1)
		n--
	}
	if n > 0 {
		weeks := n / 5
		rem := n % 5
		if weeks > 0 {
			d = d.AddDate(0, 0, weeks*7)
		}
		for rem > 0 {
			d = d.AddDate(0, 0, 1)
			if d.Weekday() != time.Saturday && d.Weekday() != time.Sunday {
				rem--
			}
		}
	}
	return d
}

func subtractBusinessDays(d time.Time, n int) time.Time {
	if d.Weekday() == time.Sunday {
		d = d.AddDate(0, 0, -2)
		n++
	} else if d.Weekday() == time.Saturday {
		d = d.AddDate(0, 0, -1)
		n++
	}
	if n < 0 {
		pos := -n
		weeks := pos / 5
		rem := pos % 5
		if weeks > 0 {
			d = d.AddDate(0, 0, -weeks*7)
		}
		for rem > 0 {
			d = d.AddDate(0, 0, -1)
			if d.Weekday() != time.Saturday && d.Weekday() != time.Sunday {
				rem--
			}
		}
	}
	return d
}

// AddBusinessDays adds n business days to t, skipping Saturdays and Sundays.
// If n is negative, business days are subtracted.
// Does not account for public holidays.
func AddBusinessDays(t time.Time, n int) time.Time {
	if n == 0 {
		return t
	}
	const maxDays = 1000000
	if n > maxDays {
		n = maxDays
	} else if n < -maxDays {
		n = -maxDays
	}

	if n > 0 {
		return addPositiveBusinessDays(t, n)
	}
	return subtractBusinessDays(t, n)
}

// BusinessDays returns the number of business days between start and end (exclusive of end).
// If end is before start, a negative count is returned.
// Does not account for public holidays.
func BusinessDays(start, end time.Time) int {
	return BusinessDaysBetween(start, end)
}

// BusinessDaysBetween returns the number of business days between start and end (exclusive of end).
// If end is before start, a negative count is returned.
// Does not account for public holidays.
func BusinessDaysBetween(start, end time.Time) int {
	if start.Equal(end) {
		return 0
	}
	if end.Before(start) {
		return -BusinessDaysBetween(end, start)
	}

	d1 := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	d2 := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC)
	if d1.Equal(d2) {
		return 0
	}

	day1 := daysFromCivil(d1.Year(), d1.Month(), d1.Day())
	day2 := daysFromCivil(d2.Year(), d2.Month(), d2.Day())
	totalDays := day2 - day1

	weeks := totalDays / 7
	rem := totalDays % 7
	bDays := weeks * 5

	cur := d1
	for i := 0; i < rem; i++ {
		if cur.Weekday() != time.Saturday && cur.Weekday() != time.Sunday {
			bDays++
		}
		cur = cur.AddDate(0, 0, 1)
	}
	return bDays
}
