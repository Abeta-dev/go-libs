// SPDX-License-Identifier: MIT

package timeutil_test

import (
	"fmt"
	"time"

	"github.com/umesh0492/go-libs/timeutil"
)

func ExampleAddBusinessDays() {
	friday := time.Date(2024, 5, 3, 10, 0, 0, 0, time.UTC)
	tuesday := timeutil.AddBusinessDays(friday, 2)
	fmt.Println(tuesday.Weekday())
	// Output:
	// Tuesday
}

func ExampleStartOfDay() {
	tm := time.Date(2024, 5, 1, 15, 30, 0, 0, time.UTC)
	start := timeutil.StartOfDay(tm, time.UTC)
	fmt.Printf("%02d:%02d:%02d\n", start.Hour(), start.Minute(), start.Second())
	// Output:
	// 00:00:00
}
