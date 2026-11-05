// SPDX-License-Identifier: MIT

// Package ginmw — security headers adapter (gin wrapper).
package ginmw

import (
	"github.com/Abeta-dev/go-libs/securityheaders"
	"github.com/gin-gonic/gin"
)

// SecurityHeaders sets defensive HTTP headers on every response.
// serverName replaces the default Server header (e.g. "core-api", "inventory-api").
// Pass an empty string to use the library default ("api").
func SecurityHeaders(serverName ...string) gin.HandlerFunc {
	name := "api"
	if len(serverName) > 0 && serverName[0] != "" {
		name = serverName[0]
	}
	h := securityheaders.New(securityheaders.WithServerName(name))(noopHandler)
	return gin.WrapH(h)
}
