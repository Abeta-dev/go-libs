// SPDX-License-Identifier: MIT

package loopbackguard_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/abeta-dev/go-libs/loopbackguard"
)

func ExampleGuard_Middleware() {
	if err := loopbackguard.ValidateLoopbackAddr("127.0.0.1:8080"); err != nil {
		panic(err)
	}
	guard, err := loopbackguard.New(loopbackguard.WithCSRFToken("boot-token-123"))
	if err != nil {
		panic(err)
	}

	h := guard.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/save", nil)
	req.Host = "127.0.0.1:8080"
	req.Header.Set(loopbackguard.DefaultCSRFHeader, guard.Token())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	fmt.Println(rec.Code)
	// Output:
	// 200
}
