// SPDX-License-Identifier: MIT

package ginmw

import (
	"net/http"

	"github.com/abeta-dev/go-libs/sse"
	"github.com/gin-gonic/gin"
)

// SSE adapts sse.NewStream for Gin route handlers, managing canonical SSE
// headers, flushing, heartbeat keepalives, Last-Event-ID, and clean teardown.
func SSE(fn func(c *gin.Context, stream *sse.Stream) error, opts ...sse.Option) gin.HandlerFunc {
	return func(c *gin.Context) {
		stream, err := sse.NewStream(c.Writer, c.Request, opts...)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
				"code":  "SSE_UNSUPPORTED",
			})
			return
		}
		defer func() { _ = stream.Close() }()
		if fn != nil {
			if err := fn(c, stream); err != nil {
				_ = c.Error(err)
			}
		}
	}
}
