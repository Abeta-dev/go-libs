// SPDX-License-Identifier: MIT

package sse_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/abeta-dev/go-libs/sse"
)

func ExampleHandler() {
	h := sse.Handler(func(_ context.Context, stream *sse.Stream) error {
		return stream.SendData("token", "hello")
	})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/stream", nil))
	fmt.Print(rec.Body.String())
	// Output:
	// event: token
	// data: hello
}
