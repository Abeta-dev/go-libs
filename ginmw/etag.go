// SPDX-License-Identifier: MIT

package ginmw

import (
	"net/http"

	"github.com/abeta-dev/go-libs/etag"
	"github.com/gin-gonic/gin"
)

type ginETagWriter struct {
	gin.ResponseWriter
	ew http.ResponseWriter
}

func (w *ginETagWriter) Write(b []byte) (int, error) {
	return w.ew.Write(b)
}

func (w *ginETagWriter) WriteString(s string) (int, error) {
	return w.ew.Write([]byte(s))
}

func (w *ginETagWriter) WriteHeader(code int) {
	w.ResponseWriter.WriteHeader(code)
	w.ew.WriteHeader(code)
}

func (w *ginETagWriter) Flush() {
	if f, ok := w.ew.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *ginETagWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// ETag returns a Gin middleware that computes deterministic ETags, stamps
// X-App-Version and X-Page-Version headers, auto-busts on X-Client-App-Version
// mismatch, and returns 304 Not Modified (0-byte body) on If-None-Match hits.
func ETag(opts ...etag.Option) gin.HandlerFunc {
	mw := etag.Middleware(opts...)
	return func(c *gin.Context) {
		origWriter := c.Writer
		handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c.Request = r
			c.Writer = &ginETagWriter{
				ResponseWriter: origWriter,
				ew:             w,
			}
			defer func() {
				c.Writer = origWriter
			}()
			c.Next()
		}))
		handler.ServeHTTP(origWriter, c.Request)
	}
}

// WriteVersionedJSON writes payload as version-gated JSON with ETag / 304 support
// directly from a Gin handler.
func WriteVersionedJSON(c *gin.Context, status int, payload any, opts ...etag.Option) error {
	return etag.WriteVersionedJSON(c.Writer, c.Request, status, payload, opts...)
}
