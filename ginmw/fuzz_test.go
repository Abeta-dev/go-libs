// SPDX-License-Identifier: MIT

package ginmw_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/umesh0492/go-libs/ginmw"
)

func FuzzJWTExtract(f *testing.F) {
	gin.SetMode(gin.TestMode)

	seeds := []struct {
		authHeader string
		tokenQuery string
	}{
		{"Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...", ""},
		{"bearer valid.jwt.token", ""},
		{"BEARER uppercase", ""},
		{"Basic dXNlcjpwYXNz", ""},
		{"", "my-query-token"},
		{"malformed header without space", ""},
		{"Bearer ", ""},
		{"\x00\xff", "\r\n"},
		{"Bearer " + string([]byte{0, 1, 2, 3}), ""},
		{"", ""},
	}

	for _, s := range seeds {
		f.Add(s.authHeader, s.tokenQuery)
	}

	r := gin.New()
	r.Use(ginmw.AuthMiddleware())
	r.GET("/protected", func(c *gin.Context) {
		claims := ginmw.GetClaims(c)
		_ = claims
		c.Status(http.StatusOK)
	})

	f.Fuzz(func(t *testing.T, authHeader, tokenQuery string) {
		target := "/protected"
		if tokenQuery != "" {
			target += "?token=" + tokenQuery
		}

		req, err := http.NewRequest(http.MethodGet, target, nil)
		if err != nil {
			return // Malformed URL from fuzzing query param is rejected by http.NewRequest
		}
		if authHeader != "" {
			req.Header.Set("Authorization", authHeader)
		}

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		// Must NEVER panic or return 500
		if rec.Code != http.StatusOK && rec.Code != http.StatusUnauthorized {
			t.Fatalf("unexpected HTTP status code %d for authHeader=%q tokenQuery=%q", rec.Code, authHeader, tokenQuery)
		}
	})
}
