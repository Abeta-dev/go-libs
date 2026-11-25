// SPDX-License-Identifier: MIT

package ginmw_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/abeta-dev/go-libs/ginmw"
	"github.com/abeta-dev/go-libs/sse"
	"github.com/gin-gonic/gin"
)

type failingSSEWriter struct {
	gin.ResponseWriter
}

func (failingSSEWriter) Write([]byte) (int, error) { return 0, errors.New("write fail") }
func (failingSSEWriter) Flush()                    {}

func TestSSE_GinAdapter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.GET("/stream", ginmw.SSE(func(_ *gin.Context, stream *sse.Stream) error {
		return stream.SendJSON("token", gin.H{"delta": "hi", "resume": stream.LastEventID()})
	}))

	var recordedErr error
	r.GET("/stream-err", func(c *gin.Context) {
		c.Next()
		if len(c.Errors) > 0 {
			recordedErr = c.Errors.Last().Err
		}
	}, ginmw.SSE(func(_ *gin.Context, _ *sse.Stream) error {
		return errors.New("stream aborted")
	}))

	// 1. Success stream
	req := httptest.NewRequest(http.MethodGet, "/stream", nil)
	req.Header.Set("Last-Event-ID", "99")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"resume":"99"`) {
		t.Fatalf("unexpected SSE output: code=%d body=%q", rec.Code, rec.Body.String())
	}

	// 2. Callback error attached to c.Errors
	recErr := httptest.NewRecorder()
	r.ServeHTTP(recErr, httptest.NewRequest(http.MethodGet, "/stream-err", nil))
	if recordedErr == nil || recordedErr.Error() != "stream aborted" {
		t.Fatalf("expected recorded error, got %v", recordedErr)
	}

	// 3. NewStream failure (initial retry write error) aborts the Gin context
	recFail := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recFail)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/fail", nil)
	ctx.Writer = failingSSEWriter{ResponseWriter: ctx.Writer}
	ginmw.SSE(nil, sse.WithRetry(time.Second))(ctx)
	if !ctx.IsAborted() {
		t.Fatalf("expected aborted context on NewStream failure")
	}
}
