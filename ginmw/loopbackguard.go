// SPDX-License-Identifier: MIT

package ginmw

import (
	"net/http"

	"github.com/abeta-dev/go-libs/loopbackguard"
	"github.com/gin-gonic/gin"
)

// LoopbackGuard wraps an existing loopbackguard.Guard as a Gin middleware,
// aborting the Gin pipeline on 403 DNS-rebinding, Same-Origin, or CSRF failures.
func LoopbackGuard(g *loopbackguard.Guard) gin.HandlerFunc {
	if g == nil {
		return func(c *gin.Context) {
			c.Next()
		}
	}
	mw := g.Middleware
	return func(c *gin.Context) {
		called := false
		handler := mw(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			called = true
			c.Request = r
			c.Next()
		}))
		handler.ServeHTTP(c.Writer, c.Request)
		if !called {
			c.Abort()
		}
	}
}

// NewLoopbackGuard constructs a loopbackguard.Guard with a per-boot ephemeral
// CSRF token and returns both the guard and its Gin middleware adapter.
func NewLoopbackGuard(opts ...loopbackguard.Option) (*loopbackguard.Guard, gin.HandlerFunc, error) {
	g, err := loopbackguard.New(opts...)
	if err != nil {
		return nil, nil, err
	}
	return g, LoopbackGuard(g), nil
}
