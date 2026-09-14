// SPDX-License-Identifier: MIT

package httputil_test

import (
	"fmt"
	"net/http/httptest"
	"strings"

	"github.com/umesh0492/go-libs/httputil"
)

func ExampleOK() {
	rec := httptest.NewRecorder()
	httputil.OK(rec, map[string]string{"status": "ready"})

	fmt.Printf("status: %d\n", rec.Code)
	fmt.Printf("body: %s\n", strings.TrimSpace(rec.Body.String()))

	// Output:
	// status: 200
	// body: {"status":"ready"}
}

func ExampleValidationError() {
	rec := httptest.NewRecorder()
	httputil.ValidationError(rec, "email is required")

	fmt.Printf("status: %d\n", rec.Code)
	fmt.Printf("body: %s\n", strings.TrimSpace(rec.Body.String()))

	// Output:
	// status: 400
	// body: {"error":"email is required","code":"VALIDATION_ERROR"}
}
