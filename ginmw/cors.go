// SPDX-License-Identifier: MIT

// Package ginmw — CORS adapter for gin.
package ginmw

import (
	"strings"

	"github.com/gin-gonic/gin"
)

func isOriginAllowed(origin string, allowedOrigins []string) bool {
	for _, o := range allowedOrigins {
		trimmed := strings.TrimSpace(o)
		if trimmed == origin {
			return true
		}
		// Wildcard suffix: "*.example.com"
		if strings.HasPrefix(trimmed, "*.") {
			suffix := trimmed[1:] // ".example.com"
			if strings.HasSuffix(origin, suffix) {
				return true
			}
		}
	}
	return false
}

// CORS returns a gin middleware that handles cross-origin requests.
// allowedOrigins may contain exact origins or wildcard-prefix entries like "*.example.com".
func CORS(allowedOrigins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		if origin != "" && isOriginAllowed(origin, allowedOrigins) {
			// Reflect the exact requesting origin — never set a mismatched value
			c.Header("Access-Control-Allow-Origin", origin)
		}
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Expose-Headers", "Content-Disposition, Content-Type, Content-Length")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
