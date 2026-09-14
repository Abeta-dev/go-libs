// SPDX-License-Identifier: MIT

package telemetry_test

import (
	"context"
	"fmt"

	"github.com/umesh0492/go-libs/telemetry"
)

func ExampleStartSpan() {
	ctx := context.Background()
	_, span := telemetry.StartSpan(ctx, "example-service", "handle_request")
	defer span.End()

	fmt.Printf("span started: %t\n", span != nil)
	// Output:
	// span started: true
}
