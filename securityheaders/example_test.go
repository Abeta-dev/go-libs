// SPDX-License-Identifier: MIT

package securityheaders_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/umesh0492/go-libs/securityheaders"
)

func ExampleDefault() {
	handler := securityheaders.Default(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	fmt.Println("X-Frame-Options:", rec.Header().Get("X-Frame-Options"))
	fmt.Println("X-Content-Type-Options:", rec.Header().Get("X-Content-Type-Options"))
	// Output:
	// X-Frame-Options: DENY
	// X-Content-Type-Options: nosniff
}
