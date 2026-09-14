// SPDX-License-Identifier: MIT

package metrics_test

import (
	"fmt"

	"github.com/umesh0492/go-libs/metrics"
)

// testCounter is a simple in-memory counter for illustrating how to implement
// the metrics.Counter interface with your own backend (e.g. Prometheus).
type testCounter struct{ n float64 }

func (c *testCounter) Inc()          { c.n++ }
func (c *testCounter) Add(v float64) { c.n += v }

func ExampleCounter() {
	// Swap NoopCounter for any real backend that satisfies metrics.Counter.
	var c metrics.Counter = &testCounter{}
	c.Inc()
	c.Add(4)
	fmt.Println(c.(*testCounter).n)
	// Output:
	// 5
}
