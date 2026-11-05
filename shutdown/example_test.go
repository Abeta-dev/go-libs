// SPDX-License-Identifier: MIT

package shutdown_test

import (
	"context"
	"fmt"
	"time"

	"github.com/Abeta-dev/go-libs/shutdown"
)

func ExampleManager() {
	mgr := shutdown.New(500 * time.Millisecond)

	mgr.Register("database", func(ctx context.Context) error {
		return nil
	})

	mgr.Register("cache", func(ctx context.Context) error {
		return nil
	})

	fmt.Println("shutdown hooks registered successfully")
	// Output:
	// shutdown hooks registered successfully
}
