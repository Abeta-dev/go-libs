// SPDX-License-Identifier: MIT

package httpmw_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/abeta-dev/go-libs/httpmw"
	"github.com/abeta-dev/go-libs/securityheaders"
)

func ExampleChain() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ping", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("pong"))
	})

	handler := httpmw.Chain(
		mux,
		httpmw.RequestID(),
		httpmw.SecurityHeaders(securityheaders.WithDisableHSTS()),
		httpmw.Recovery(),
	)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ping", nil))
	fmt.Println(rec.Code, rec.Body.String())
	// Output:
	// 200 pong
}
