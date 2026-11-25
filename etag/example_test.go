// SPDX-License-Identifier: MIT

package etag_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/abeta-dev/go-libs/etag"
)

func ExampleMiddleware() {
	mw := etag.Middleware(
		etag.WithAppVersion("1.2.0"),
		etag.WithIgnoreJSONKeys("cached_at_iso"),
		etag.WithCacheControl("private, no-cache"),
	)

	h := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"title":"Dashboard","cached_at_iso":"2026-11-25T00:00:00Z"}`))
	}))

	rec1 := httptest.NewRecorder()
	h.ServeHTTP(rec1, httptest.NewRequest(http.MethodGet, "/dashboard", nil))

	req2 := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	req2.Header.Set(etag.HeaderIfNoneMatch, rec1.Header().Get(etag.HeaderETag))
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)

	fmt.Println(rec1.Code, rec2.Code, rec2.Body.Len())
	// Output:
	// 200 304 0
}
