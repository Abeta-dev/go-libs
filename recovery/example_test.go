// SPDX-License-Identifier: MIT

package recovery_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/umesh0492/go-libs/recovery"
)

func ExampleMiddleware() {
	mw := recovery.Middleware()
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("unexpected system failure")
	}))

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	fmt.Println("Status:", rec.Code)
	// Output:
	// Status: 500
}
