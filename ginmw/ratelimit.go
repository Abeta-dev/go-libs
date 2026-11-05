// SPDX-License-Identifier: MIT

// Package ginmw — ratelimit adapters (gin-native wrappers).
package ginmw

import (
	"net/http"
	"strconv"

	"github.com/Abeta-dev/go-libs/ratelimit"
	"github.com/gin-gonic/gin"
)

// GlobalRateLimit applies standard API protection: 200 req/min per IP.
func GlobalRateLimit() gin.HandlerFunc {
	return wrapRateLimitMiddleware(ratelimit.NewGlobal())
}

// AuthRateLimit applies strict brute-force protection: 10 req/min per IP.
func AuthRateLimit() gin.HandlerFunc {
	return wrapRateLimitMiddleware(ratelimit.NewAuth())
}

func wrapRateLimitMiddleware(mw func(http.Handler) http.Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		allowed := false
		h := mw(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			allowed = true
			c.Request = r
		}))
		h.ServeHTTP(c.Writer, c.Request)
		if !allowed {
			c.Abort()
			return
		}
		c.Next()
	}
}

// RateLimitWithLimiter applies rate limiting using any ratelimit.Limiter implementation.
// An optional keyFunc can be supplied; by default, the client IP (c.ClientIP()) is used.
func RateLimitWithLimiter(limiter ratelimit.Limiter, keyFunc ...func(*gin.Context) string) gin.HandlerFunc {
	kf := func(c *gin.Context) string {
		return c.ClientIP()
	}
	if len(keyFunc) > 0 && keyFunc[0] != nil {
		kf = keyFunc[0]
	}
	return func(c *gin.Context) {
		key := kf(c)
		if !limiter.Allow(key) {
			c.Header("RateLimit-Remaining", "0")
			c.Header("X-RateLimit-Remaining", "0")
			c.Header("Retry-After", "1")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "too many requests",
				"code":  "RATE_LIMITED",
			})
			return
		}
		rem := limiter.Remaining(key)
		c.Header("RateLimit-Remaining", strconv.Itoa(rem))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(rem))
		c.Next()
	}
}
