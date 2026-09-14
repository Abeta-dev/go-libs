// SPDX-License-Identifier: MIT

package ginmw_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	"github.com/umesh0492/go-libs/ginmw"
)

func ExampleRequestID() {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(ginmw.RequestID())

	router.GET("/ping", func(c *gin.Context) {
		reqID := c.Writer.Header().Get("X-Request-ID")
		c.String(http.StatusOK, "has_id: %t", reqID != "")
	})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	fmt.Println(rec.Body.String())
	// Output:
	// has_id: true
}

func ExampleSecurityHeaders() {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(ginmw.SecurityHeaders())

	router.GET("/secure", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/secure", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	fmt.Println("X-Frame-Options:", rec.Header().Get("X-Frame-Options"))
	fmt.Println("X-Content-Type-Options:", rec.Header().Get("X-Content-Type-Options"))
	// Output:
	// X-Frame-Options: DENY
	// X-Content-Type-Options: nosniff
}
