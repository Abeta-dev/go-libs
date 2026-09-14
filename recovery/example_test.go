// SPDX-License-Identifier: MIT

package recovery_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	"github.com/umesh0492/go-libs/recovery"
)

func ExampleMiddleware() {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(recovery.Middleware())

	router.GET("/panic", func(c *gin.Context) {
		panic("unexpected system failure")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	fmt.Println("Status:", rec.Code)
	// Output:
	// Status: 500
}
