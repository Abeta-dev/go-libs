// SPDX-License-Identifier: MIT

package cache_test

import (
	"context"
	"fmt"
	"time"

	"github.com/umesh0492/go-libs/cache"
)

func ExampleTypedCache() {
	c := cache.NewTypedCache[string]()
	ctx := context.Background()

	// First fetch: executes fetchFn
	val, _ := c.GetOrFetch(ctx, "session:user1", time.Minute, func(ctx context.Context) (string, error) {
		return "authenticated", nil
	})

	// Second fetch: served immediately from memory
	cachedVal, _ := c.GetOrFetch(ctx, "session:user1", time.Minute, func(ctx context.Context) (string, error) {
		return "re-fetched", nil
	})

	fmt.Println(val)
	fmt.Println(cachedVal)

	// Output:
	// authenticated
	// authenticated
}
