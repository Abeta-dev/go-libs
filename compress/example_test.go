// SPDX-License-Identifier: MIT

package compress_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/abeta-dev/go-libs/compress"
	"github.com/andybalholm/brotli"
)

func ExampleNegotiate() {
	fmt.Println(compress.Negotiate("gzip, deflate, br"))
	fmt.Println(compress.Negotiate("br;q=0.5, gzip;q=0.9"))
	fmt.Println(compress.Negotiate("identity"))
	// Output:
	// br
	// gzip
	//
}

func ExampleMiddleware() {
	handler := compress.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(strings.Repeat(`{"ok":true}`, 200)))
	}))

	req := httptest.NewRequest(http.MethodGet, "/items", nil)
	req.Header.Set("Accept-Encoding", "gzip, br")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	raw, _ := io.ReadAll(brotli.NewReader(rec.Body))
	fmt.Println(rec.Header().Get("Content-Encoding"), len(raw))
	// Output:
	// br 2200
}
