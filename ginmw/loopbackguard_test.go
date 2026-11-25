// SPDX-License-Identifier: MIT

package ginmw_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/abeta-dev/go-libs/ginmw"
	"github.com/abeta-dev/go-libs/loopbackguard"
	"github.com/abeta-dev/go-libs/securityheaders"
	"github.com/gin-gonic/gin"
)

type errRandReader struct{}

func (errRandReader) Read([]byte) (int, error) { return 0, errors.New("rand error") }

func TestLoopbackGuardAndSecurityHeadersWithOptions_GinAdapters(t *testing.T) {
	gin.SetMode(gin.TestMode)

	guard, mw, err := ginmw.NewLoopbackGuard(loopbackguard.WithCSRFToken("gin-csrf-tok"))
	if err != nil {
		t.Fatalf("NewLoopbackGuard failed: %v", err)
	}

	r := gin.New()
	r.Use(ginmw.SecurityHeadersWithOptions(
		securityheaders.WithDisableHSTS(),
		securityheaders.WithCacheControl("private, no-cache"),
		securityheaders.WithServerName("local-bff"),
	))
	r.Use(mw)

	reached := false
	r.POST("/mutate", func(c *gin.Context) {
		reached = true
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// 1. Missing CSRF aborts Gin pipeline with 403
	reqBad := httptest.NewRequest(http.MethodPost, "/mutate", nil)
	reqBad.Host = "127.0.0.1:8080"
	recBad := httptest.NewRecorder()
	r.ServeHTTP(recBad, reqBad)
	if recBad.Code != http.StatusForbidden || reached {
		t.Fatalf("expected 403 abort, got status=%d reached=%v", recBad.Code, reached)
	}
	if recBad.Header().Get("Strict-Transport-Security") != "" {
		t.Fatalf("expected HSTS disabled")
	}
	if recBad.Header().Get("Cache-Control") != "private, no-cache" {
		t.Fatalf("unexpected Cache-Control: %q", recBad.Header().Get("Cache-Control"))
	}

	// 2. Valid CSRF succeeds
	reqOK := httptest.NewRequest(http.MethodPost, "/mutate", nil)
	reqOK.Host = "127.0.0.1:8080"
	reqOK.Header.Set(loopbackguard.DefaultCSRFHeader, guard.Token())
	recOK := httptest.NewRecorder()
	r.ServeHTTP(recOK, reqOK)
	if recOK.Code != http.StatusOK || !reached {
		t.Fatalf("expected 200, got status=%d reached=%v", recOK.Code, reached)
	}

	// 3. Nil guard is a safe pass-through
	rNil := gin.New()
	rNil.Use(ginmw.LoopbackGuard(nil))
	rNil.GET("/ping", func(c *gin.Context) { c.Status(http.StatusOK) })
	recNil := httptest.NewRecorder()
	rNil.ServeHTTP(recNil, httptest.NewRequest(http.MethodGet, "/ping", nil))
	if recNil.Code != http.StatusOK {
		t.Fatalf("expected nil guard passthrough 200, got %d", recNil.Code)
	}

	// 4. NewLoopbackGuard error propagation when entropy source fails
	if _, _, err := ginmw.NewLoopbackGuard(loopbackguard.WithRandReader(errRandReader{})); err == nil {
		t.Fatal("expected error from NewLoopbackGuard on failing rand reader")
	}
}
