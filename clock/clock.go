// SPDX-License-Identifier: MIT

package clock

import "time"

// Clock represents an abstract time source.
// It mirrors the standard library time package API shapes for drop-in replacement.
type Clock interface {
	// Now returns the current time.
	Now() time.Time

	// Since returns the time elapsed since t.
	Since(t time.Time) time.Duration

	// Until returns the duration until t.
	Until(t time.Time) time.Duration

	// Sleep pauses the current goroutine for at least duration d.
	Sleep(d time.Duration)

	// After waits for the duration to elapse and then sends the current time on the returned channel.
	After(d time.Duration) <-chan time.Time

	// NewTimer creates a new Timer that will send the current time on its channel after at least duration d.
	NewTimer(d time.Duration) Timer

	// NewTicker returns a new Ticker containing a channel that will send the current time with a period specified by d.
	NewTicker(d time.Duration) Ticker
}

// Timer represents an abstract event timer.
type Timer interface {
	// C returns the channel on which the ticks are delivered.
	C() <-chan time.Time

	// Reset changes the timer to expire after duration d.
	// It returns true if the timer had been active, false if the timer had
	// expired or been stopped.
	Reset(d time.Duration) bool

	// Stop prevents the Timer from firing.
	// It returns true if the call stops the timer, false if the timer has already
	// expired or been stopped.
	Stop() bool
}

// Ticker represents an abstract periodic ticker.
type Ticker interface {
	// C returns the channel on which the ticks are delivered.
	C() <-chan time.Time

	// Reset stops a ticker and resets its period to the specified duration.
	Reset(d time.Duration)

	// Stop turns off a ticker. After Stop, no more ticks will be sent.
	Stop()
}
