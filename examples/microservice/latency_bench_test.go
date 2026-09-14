//go:build scale

// SPDX-License-Identifier: MIT
package main_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/umesh0492/go-libs/cache"
	"github.com/umesh0492/go-libs/circuitbreaker"
	"github.com/umesh0492/go-libs/ginmw"
	"github.com/umesh0492/go-libs/recovery"
	"github.com/umesh0492/go-libs/workerpool"
)

func init() {
	gin.SetMode(gin.ReleaseMode)
}

type staticPermissionProvider struct {
	perms []string
}

func (p *staticPermissionProvider) GetPermissionsForUser(ctx context.Context, userID, tenantID uuid.UUID) ([]string, error) {
	return p.perms, nil
}

func Benchmark_01_Raw_Baseline_Handler(b *testing.B) {
	r := gin.New()
	r.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	w := httptest.NewRecorder()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		r.ServeHTTP(w, req)
	}
}

func Benchmark_02_RequestID(b *testing.B) {
	r := gin.New()
	r.Use(ginmw.RequestID())
	r.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	w := httptest.NewRecorder()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		r.ServeHTTP(w, req)
	}
}

func Benchmark_03_SecurityHeaders(b *testing.B) {
	r := gin.New()
	r.Use(ginmw.SecurityHeaders())
	r.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	w := httptest.NewRecorder()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		r.ServeHTTP(w, req)
	}
}

func Benchmark_04_CORS(b *testing.B) {
	r := gin.New()
	r.Use(ginmw.CORS([]string{"*"}))
	r.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	w := httptest.NewRecorder()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		r.ServeHTTP(w, req)
	}
}

func Benchmark_05_Recovery(b *testing.B) {
	r := gin.New()
	r.Use(recovery.Middleware())
	r.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	w := httptest.NewRecorder()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		r.ServeHTTP(w, req)
	}
}

func Benchmark_06_Telemetry_Tracing(b *testing.B) {
	r := gin.New()
	r.Use(ginmw.Telemetry("order-service"))
	r.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	w := httptest.NewRecorder()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		r.ServeHTTP(w, req)
	}
}

func Benchmark_07_RateLimit(b *testing.B) {
	r := gin.New()
	r.Use(ginmw.GlobalRateLimit())
	r.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	w := httptest.NewRecorder()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		r.ServeHTTP(w, req)
	}
}

func Benchmark_08_BodyLimit(b *testing.B) {
	r := gin.New()
	r.Use(ginmw.LimitBodyDefault())
	r.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	w := httptest.NewRecorder()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		r.ServeHTTP(w, req)
	}
}

func Benchmark_09_RBAC_Authorization(b *testing.B) {
	provider := &staticPermissionProvider{perms: []string{"order:read", "order:create"}}
	r := gin.New()
	r.Use(ginmw.RBAC(provider))
	r.GET("/ping", ginmw.RequirePermission("order:read"), func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	w := httptest.NewRecorder()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		r.ServeHTTP(w, req)
	}
}

func Benchmark_10_JWT_Authentication(b *testing.B) {
	secret := "test-secret-key-32-chars-long!"
	ginmw.SetJWTSecret(secret)
	tokenStr, _ := ginmw.GenerateToken(&ginmw.Claims{
		UserID: uuid.New(),
		Roles:  []string{"admin"},
	}, 15)

	r := gin.New()
	r.Use(ginmw.AuthMiddleware())
	r.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	w := httptest.NewRecorder()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		r.ServeHTTP(w, req)
	}
}

func Benchmark_11_Cache_GetOrFetch_Hit(b *testing.B) {
	c := cache.NewTypedCache[string]()
	defer c.Close()

	ctx := context.Background()
	_, _ = c.GetOrFetch(ctx, "key", time.Hour, func(ctx context.Context) (string, error) {
		return "cached-value", nil
	})

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = c.GetOrFetch(ctx, "key", time.Hour, func(ctx context.Context) (string, error) {
			return "cached-value", nil
		})
	}
}

func Benchmark_12_CircuitBreaker_Execute(b *testing.B) {
	cb := circuitbreaker.NewConsecutiveBreaker(5, time.Minute)
	ctx := context.Background()
	noop := func() error { return nil }

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = cb.Execute(ctx, noop)
	}
}

func Benchmark_13_WorkerPool_Submit(b *testing.B) {
	pool := workerpool.New(8, 100_000)
	defer pool.Stop()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = pool.Submit(func() {})
	}
}

func Benchmark_14_Composed_Full_Pipeline(b *testing.B) {
	provider := &staticPermissionProvider{perms: []string{"order:read"}}
	secret := "test-secret-key-32-chars-long!"
	ginmw.SetJWTSecret(secret)
	tokenStr, _ := ginmw.GenerateToken(&ginmw.Claims{
		UserID: uuid.New(),
		Roles:  []string{"admin"},
	}, 15)

	r := gin.New()
	r.Use(
		ginmw.RequestID(),
		ginmw.SecurityHeaders(),
		ginmw.CORS([]string{"*"}),
		recovery.Middleware(),
		ginmw.Telemetry("order-service"),
		ginmw.GlobalRateLimit(),
		ginmw.LimitBodyDefault(),
		ginmw.AuthMiddleware(),
		ginmw.RBAC(provider),
	)

	r.GET("/api/v1/orders", ginmw.RequirePermission("order:read"), func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	w := httptest.NewRecorder()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		r.ServeHTTP(w, req)
	}
}
