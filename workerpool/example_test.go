// SPDX-License-Identifier: MIT

package workerpool_test

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/umesh0492/go-libs/workerpool"
)

func ExamplePool() {
	p := workerpool.New(2, 4)
	defer p.StopWait()

	var counter atomic.Int32
	var wg sync.WaitGroup
	for i := 1; i <= 3; i++ {
		wg.Add(1)
		_ = p.Submit(func() {
			defer wg.Done()
			counter.Add(1)
		})
	}

	wg.Wait()
	fmt.Printf("completed tasks: %d\n", counter.Load())
	// Output:
	// completed tasks: 3
}
