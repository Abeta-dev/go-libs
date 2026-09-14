// SPDX-License-Identifier: MIT

package requestid_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/umesh0492/go-libs/requestid"
)

func ExampleMiddleware() {
	handler := requestid.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := requestid.FromContext(r.Context())
		fmt.Printf("has_id=%t\n", reqID != "")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(requestid.Header, "req-xyz-789")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	fmt.Printf("response_header=%s\n", w.Header().Get(requestid.Header))
	// Output:
	// has_id=true
	// response_header=req-xyz-789
}
