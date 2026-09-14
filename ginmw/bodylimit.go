// SPDX-License-Identifier: MIT

// Package ginmw provides HTTP body size limiting middleware.
package ginmw

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

// noopHandler is a no-op http.Handler used as the final handler when wrapping
// net/http middleware with gin.WrapH (the middleware calls Next itself).
var noopHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

// LimitBodyDefault applies a 2 MB maximum body to all API requests.
func LimitBodyDefault() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2<<20) // 2 MB
		c.Next()
		// Drain to surface MaxBytesError early if the client keeps sending
		_, _ = io.Copy(io.Discard, c.Request.Body)
	}
}

// LimitBodyAuth applies a strict 4 KB limit for authentication endpoints.
func LimitBodyAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4<<10) // 4 KB
		c.Next()
	}
}
