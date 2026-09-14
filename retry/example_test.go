// SPDX-License-Identifier: MIT

package retry_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/umesh0492/go-libs/retry"
)

func ExampleDo() {
	attempts := 0
	err := retry.Do(context.Background(), retry.Config{
		Attempts:    3,
		InitialWait: time.Millisecond,
		Strategy:    retry.Exponential,
	}, func(_ context.Context) error {
		attempts++
		if attempts < 3 {
			return errors.New("not ready")
		}
		return nil
	})
	fmt.Printf("err=%v attempts=%d\n", err, attempts)
	// Output:
	// err=<nil> attempts=3
}

func ExampleDoWithResult() {
	val, err := retry.DoWithResult(context.Background(), retry.Config{
		Attempts: 1,
	}, func(_ context.Context) (string, error) {
		return "hello", nil
	})
	fmt.Printf("val=%s err=%v\n", val, err)
	// Output:
	// val=hello err=<nil>
}
