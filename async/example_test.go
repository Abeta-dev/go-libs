// SPDX-License-Identifier: MIT

package async_test

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/Abeta-dev/go-libs/async"
)

func ExampleParallel() {
	var sum atomic.Int64

	err := async.Parallel(context.Background(),
		func(ctx context.Context) error {
			sum.Add(10)
			return nil
		},
		func(ctx context.Context) error {
			sum.Add(20)
			return nil
		},
	)
	fmt.Printf("err=%v sum=%d\n", err, sum.Load())
	// Output:
	// err=<nil> sum=30
}
