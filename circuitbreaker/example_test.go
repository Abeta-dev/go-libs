// SPDX-License-Identifier: MIT

package circuitbreaker_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/umesh0492/go-libs/circuitbreaker"
)

func ExampleConsecutiveBreaker() {
	cb := circuitbreaker.NewConsecutiveBreaker(2, 100*time.Millisecond)
	ctx := context.Background()

	// Successful call
	err := cb.Execute(ctx, func() error {
		return nil
	})
	fmt.Printf("success call error: %v\n", err)

	// Failing calls trip the breaker
	_ = cb.Execute(ctx, func() error { return errors.New("downstream error") })
	_ = cb.Execute(ctx, func() error { return errors.New("downstream error") })

	// Breaker is now open, fails immediately without calling function
	err = cb.Execute(ctx, func() error {
		return nil
	})
	fmt.Printf("circuit open: %v\n", errors.Is(err, circuitbreaker.ErrCircuitOpen))

	// Output:
	// success call error: <nil>
	// circuit open: true
}
