// SPDX-License-Identifier: MIT

package ginmw_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/umesh0492/go-libs/ginmw"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestRequireTenant_FromHeader(t *testing.T) {
	r := gin.New()
	r.Use(ginmw.RequireTenant())
	r.GET("/test", func(c *gin.Context) {
		tenantID := c.GetString("tenant_id")
		c.String(http.StatusOK, tenantID)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("x-tenant-id", "tenant-123")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got: %d", rec.Code)
	}
	if rec.Body.String() != "tenant-123" {
		t.Fatalf("expected 'tenant-123', got: %s", rec.Body.String())
	}
}

func TestRequireTenant_FromQuery(t *testing.T) {
	r := gin.New()
	r.Use(ginmw.RequireTenant())
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test?tenant_id=tenant-abc", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got: %d", rec.Code)
	}
}

func TestRequireTenant_Missing_Aborts(t *testing.T) {
	r := gin.New()
	r.Use(ginmw.RequireTenant())
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got: %d", rec.Code)
	}
}

func TestRequireTenant_BypassRole(t *testing.T) {
	r := gin.New()
	// Middleware injecting super admin claims
	r.Use(func(c *gin.Context) {
		c.Set("claims", &ginmw.Claims{
			UserID: uuid.New(),
			Roles:  []string{"SUPERADMIN"},
		})
		c.Next()
	})
	r.Use(ginmw.RequireTenant(ginmw.WithBypassRoles("SUPERADMIN")))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "bypassed")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for bypassed superadmin, got: %d", rec.Code)
	}
	if rec.Body.String() != "bypassed" {
		t.Fatalf("expected body 'bypassed', got: %s", rec.Body.String())
	}
}

func TestRequireTenant_CustomOptionsAndContext(t *testing.T) {
	r := gin.New()
	r.Use(ginmw.RequireTenant(
		ginmw.WithTenantContextKey("custom_tenant_key"),
		ginmw.WithTenantHeader("x-custom-tenant"),
		ginmw.WithTenantQuery("custom_tid"),
	))
	r.GET("/custom", func(c *gin.Context) {
		val := c.GetString("custom_tenant_key")
		c.String(http.StatusOK, val)
	})

	// Test custom header
	req := httptest.NewRequest(http.MethodGet, "/custom", nil)
	req.Header.Set("x-custom-tenant", "cust-1")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "cust-1" {
		t.Fatalf("expected cust-1 from custom header, got %d %s", rec.Code, rec.Body.String())
	}

	// Test custom query
	req = httptest.NewRequest(http.MethodGet, "/custom?custom_tid=cust-2", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "cust-2" {
		t.Fatalf("expected cust-2 from custom query, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestExtractTenantID_ContextAndClaims(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request = req

	// 1. Context key lookup
	c.Set(ginmw.DefaultTenantContextKey, "ctx-tenant")
	if tid := ginmw.ExtractTenantID(c, "", "", ""); tid != "ctx-tenant" {
		t.Fatalf("expected ctx-tenant, got %s", tid)
	}

	// 2. Claims lookup
	c.Set(ginmw.DefaultTenantContextKey, "")
	claimsID := uuid.New()
	c.Set("claims", &ginmw.Claims{
		TenantID: claimsID,
	})
	if tid := ginmw.ExtractTenantID(c, "", "", ""); tid != claimsID.String() {
		t.Fatalf("expected %s from claims, got %s", claimsID.String(), tid)
	}
}
