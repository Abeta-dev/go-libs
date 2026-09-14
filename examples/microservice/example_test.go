// SPDX-License-Identifier: MIT

package main_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	microservice "github.com/umesh0492/go-libs/examples/microservice"
)

func ExampleNewApp() {
	app := microservice.NewApp()

	// Perform an in-memory health probe check
	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	rec := httptest.NewRecorder()
	app.Router.ServeHTTP(rec, req)

	fmt.Printf("Health status code: %d\n", rec.Code)
	// Output:
	// Health status code: 200
}
