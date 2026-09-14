// SPDX-License-Identifier: MIT
package clock_test

import (
	"fmt"
	"time"

	"github.com/umesh0492/go-libs/clock"
)

func ExampleFakeClock() {
	start := time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC)
	fake := clock.NewFakeAt(start)

	timer := fake.NewTimer(5 * time.Second)

	// In real time, this would sleep. With FakeClock, we advance time immediately.
	fake.Add(5 * time.Second)

	firedTime := <-timer.C()
	fmt.Println("Timer fired at:", firedTime.Format(time.RFC3339))
	// Output:
	// Timer fired at: 2026-01-01T12:00:05Z
}
