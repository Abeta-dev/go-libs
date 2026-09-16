// SPDX-License-Identifier: MIT

// Package ginmw — request-ID adapter (gin wrapper).
package ginmw

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/umesh0492/go-libs/requestid"
)

// RequestID injects a unique X-Request-ID into every request context and response.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		var newCtx context.Context
		wrapper := requestid.Middleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			newCtx = r.Context()
		}))

		wrapper.ServeHTTP(c.Writer, c.Request)

		c.Request = c.Request.WithContext(newCtx)
		c.Next()
	}
}

// GetRequestID retrieves the request ID from the context.
func GetRequestID(ctx context.Context) string {
	return requestid.FromContext(ctx)
}

// RequestIDHeader is the canonical header name: "X-Request-ID".
const RequestIDHeader = requestid.Header
