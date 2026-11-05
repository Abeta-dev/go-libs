# `httpclient` — Resilient Outbound HTTP Client & RoundTripper

Package `httpclient` wraps `net/http` with composable circuit breaking (`circuitbreaker.Breaker`), exponential-backoff retries (`retry.Config`), deterministic clock injection (`clock.Clock`), and RED metrics (`metrics.Counter` / `metrics.Histogram`).

## Usage

```go
package main

import (
	"time"

	"github.com/Abeta-dev/go-libs/circuitbreaker"
	"github.com/Abeta-dev/go-libs/httpclient"
	"github.com/Abeta-dev/go-libs/retry"
)

func main() {
	cb := circuitbreaker.NewConsecutive(5, 30*time.Second)
	client := httpclient.New(
		httpclient.WithBreaker(cb),
		httpclient.WithRetry(retry.Config{
			MaxAttempts: 3,
			InitialWait: 100 * time.Millisecond,
			MaxWait:     2 * time.Second,
			Strategy:    retry.ExponentialJitter,
		}),
	)
	_ = client
}
```
