// SPDX-License-Identifier: MIT

package ginmw_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/umesh0492/go-libs/ginmw"
)

func TestRespondSuccess(t *testing.T) {
	r := gin.New()
	r.GET("/ok", func(c *gin.Context) {
		ginmw.RespondSuccess(c, map[string]string{"message": "all good"})
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got: %d", rec.Code)
	}

	var resp ginmw.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}
	if !resp.Success || resp.Data == nil {
		t.Fatalf("unexpected envelope: %+v", resp)
	}
}

func TestRespondCreated(t *testing.T) {
	r := gin.New()
	r.POST("/create", func(c *gin.Context) {
		ginmw.RespondCreated(c, "id-123")
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/create", nil)
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got: %d", rec.Code)
	}
}

func TestRespondError(t *testing.T) {
	r := gin.New()
	r.GET("/err", func(c *gin.Context) {
		ginmw.RespondError(c, http.StatusNotFound, "NOT_FOUND", "Resource missing")
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/err", nil)
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got: %d", rec.Code)
	}

	var resp ginmw.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}
	if resp.Success || resp.Error == nil || resp.Error.Code != "NOT_FOUND" {
		t.Fatalf("unexpected error response: %+v", resp)
	}
}

func TestRespondErrorWithDetail(t *testing.T) {
	r := gin.New()
	r.GET("/err-detail", func(c *gin.Context) {
		ginmw.RespondErrorWithDetail(c, http.StatusBadRequest, "INVALID_PARAM", "Validation error", "Field 'email' is invalid")
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/err-detail", nil)
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got: %d", rec.Code)
	}

	var resp ginmw.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}
	if resp.Error == nil || resp.Error.Detail != "Field 'email' is invalid" {
		t.Fatalf("expected detail, got: %+v", resp.Error)
	}
}
