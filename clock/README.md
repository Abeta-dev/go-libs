# `clock` — Deterministic Time & Fake Clock Abstraction

Package `clock` provides a drop-in `Clock` interface (`RealClock` and thread-safe `FakeClock`) for deterministic, zero-sleep testing of rate limiters, caches, circuit breakers, and retry loops.

## Usage

```go
package main

import (
	"fmt"
	"time"

	"github.com/abeta-dev/go-libs/clock"
)

func main() {
	fake := clock.NewFake(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	timer := fake.NewTimer(5 * time.Second)

	fake.Advance(5 * time.Second)
	firedAt := <-timer.C()
	fmt.Println("Fired at:", firedAt.Format(time.RFC3339))
}
```
