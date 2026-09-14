// SPDX-License-Identifier: MIT

package health_test

import (
	"context"
	"fmt"
	"net/http/httptest"
	"time"

	"github.com/umesh0492/go-libs/health"
)

func ExampleNew() {
	svc := health.New(
		health.WithStartTime(time.Now().Add(-10*time.Minute)),
		health.WithChecker("database", func(ctx context.Context) error {
			return nil
		}),
	)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/healthz", nil)
	svc.Handler(rec, req)

	fmt.Printf("health status code: %d\n", rec.Code)

	// Output:
	// health status code: 200
}
