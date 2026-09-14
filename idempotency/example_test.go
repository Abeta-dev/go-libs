// SPDX-License-Identifier: MIT

package idempotency_test

import (
	"context"
	"fmt"

	"github.com/umesh0492/go-libs/idempotency"
)

func ExampleMemoryStore() {
	store := idempotency.NewMemoryStore()
	ctx := context.Background()

	// First execution locks key: exists is false, rec is nil
	exists, _, err := store.Lock(ctx, "req-xyz")
	if err != nil {
		panic(err)
	}
	fmt.Printf("already exists: %v\n", exists)

	// Save completed response
	err = store.Save(ctx, "req-xyz", idempotency.Response{
		StatusCode: 201,
		Body:       []byte(`{"order_id": "123"}`),
	})
	if err != nil {
		panic(err)
	}

	// Subsequent replay returns cached record
	exists2, rec2, _ := store.Lock(ctx, "req-xyz")
	fmt.Printf("replay exists: %v, status: %s, code: %d\n", exists2, rec2.Status, rec2.Response.StatusCode)

	// Output:
	// already exists: false
	// replay exists: true, status: COMPLETED, code: 201
}
