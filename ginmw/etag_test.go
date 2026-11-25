// SPDX-License-Identifier: MIT

package ginmw_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/abeta-dev/go-libs/etag"
	"github.com/abeta-dev/go-libs/ginmw"
	"github.com/gin-gonic/gin"
)

func TestETag_GinMiddlewareAndHelper(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(ginmw.ETag(
		etag.WithAppVersion("2.0.0"),
		etag.WithIgnoreJSONKeys("cached_at_iso"),
		etag.WithCacheControl("private, no-cache"),
	))

	calls := 0
	r.GET("/screen", func(c *gin.Context) {
		calls++
		if u, ok := c.Writer.(interface{ Unwrap() http.ResponseWriter }); !ok || u.Unwrap() == nil {
			t.Errorf("expected ginETagWriter to implement Unwrap()")
		}
		c.JSON(http.StatusOK, gin.H{"screen": "home", "cached_at_iso": calls})
	})
	r.GET("/flush", func(c *gin.Context) {
		c.Header("Content-Type", "text/plain")
		c.Status(http.StatusOK)
		_, _ = c.Writer.WriteString("streamed")
		c.Writer.Flush()
	})
	r.GET("/direct", func(c *gin.Context) {
		if err := ginmw.WriteVersionedJSON(c, http.StatusOK, gin.H{"v": 1, "ts": calls}, etag.WithIgnoreJSONKeys("ts")); err != nil {
			t.Errorf("WriteVersionedJSON failed: %v", err)
		}
	})

	// 1. First request -> 200 OK with ETag
	rec1 := httptest.NewRecorder()
	r.ServeHTTP(rec1, httptest.NewRequest(http.MethodGet, "/screen", nil))
	tag := rec1.Header().Get(etag.HeaderETag)
	if rec1.Code != http.StatusOK || tag == "" || rec1.Header().Get(etag.HeaderAppVersion) != "2.0.0" {
		t.Fatalf("first request failed: status=%d etag=%q", rec1.Code, tag)
	}

	// 2. Second request with If-None-Match -> 304 Not Modified (0 bytes)
	req2 := httptest.NewRequest(http.MethodGet, "/screen", nil)
	req2.Header.Set(etag.HeaderIfNoneMatch, tag)
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusNotModified || rec2.Body.Len() != 0 {
		t.Fatalf("expected 304 with 0 bytes, got status=%d len=%d", rec2.Code, rec2.Body.Len())
	}

	// 3. Flush endpoint works cleanly
	recFlush := httptest.NewRecorder()
	r.ServeHTTP(recFlush, httptest.NewRequest(http.MethodGet, "/flush", nil))
	if recFlush.Code != http.StatusOK || recFlush.Body.String() != "streamed" {
		t.Fatalf("flush endpoint failed: status=%d body=%q", recFlush.Code, recFlush.Body.String())
	}

	// 4. Direct WriteVersionedJSON helper
	recDirect := httptest.NewRecorder()
	r.ServeHTTP(recDirect, httptest.NewRequest(http.MethodGet, "/direct", nil))
	if recDirect.Code != http.StatusOK || recDirect.Header().Get(etag.HeaderETag) == "" {
		t.Fatalf("direct helper failed: status=%d etag=%q", recDirect.Code, recDirect.Header().Get(etag.HeaderETag))
	}
}
