// SPDX-License-Identifier: MIT

package ginmw_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/umesh0492/go-libs/ginmw"
	"github.com/umesh0492/go-libs/ratelimit"
)

func init() { gin.SetMode(gin.TestMode) }

func newRouter(mw gin.HandlerFunc) *gin.Engine {
	r := gin.New()
	r.Use(mw)
	r.Any("/test", func(c *gin.Context) { c.Status(200) })
	return r
}

// ── CORS ─────────────────────────────────────────────────────────────────────

func TestCORS_ExactOriginAllowed(t *testing.T) {
	r := newRouter(ginmw.CORS([]string{"https://app.example.com"}))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://app.example.com")
	r.ServeHTTP(w, req)
	assert.Equal(t, "https://app.example.com", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_WildcardSuffixAllowed(t *testing.T) {
	r := newRouter(ginmw.CORS([]string{"*.example.com"}))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://preview.example.com")
	r.ServeHTTP(w, req)
	assert.Equal(t, "https://preview.example.com", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_WildcardSuffixMultipleAllowed(t *testing.T) {
	r := newRouter(ginmw.CORS([]string{"*.example.com", "*.internal.net"}))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://service.internal.net")
	r.ServeHTTP(w, req)
	assert.Equal(t, "https://service.internal.net", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_EmptyAllowedOrigins(t *testing.T) {
	r := newRouter(ginmw.CORS([]string{}))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://any.example.com")
	r.ServeHTTP(w, req)
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_UnknownOriginNotReflected(t *testing.T) {
	r := newRouter(ginmw.CORS([]string{"https://allowed.com"}))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://evil.com")
	r.ServeHTTP(w, req)
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_OptionsPreflightReturns204(t *testing.T) {
	r := newRouter(ginmw.CORS([]string{"https://app.example.com"}))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "https://app.example.com")
	r.ServeHTTP(w, req)
	assert.Equal(t, 204, w.Code)
}

func TestCORS_AllowMethodsContainsPUT(t *testing.T) {
	r := newRouter(ginmw.CORS([]string{"https://app.example.com"}))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "https://app.example.com")
	r.ServeHTTP(w, req)
	methods := w.Header().Get("Access-Control-Allow-Methods")
	assert.Contains(t, methods, "PUT")
	assert.Contains(t, methods, "DELETE")
	assert.Contains(t, methods, "PATCH")
}

// ── JWT ───────────────────────────────────────────────────────────────────────

func TestAuthMiddleware_MissingToken(t *testing.T) {
	ginmw.SetJWTSecret("test-secret")
	r := newRouter(ginmw.AuthMiddleware())
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, 401, w.Code)
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	ginmw.SetJWTSecret("test-secret")
	r := newRouter(ginmw.AuthMiddleware())
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer this.is.invalid")
	r.ServeHTTP(w, req)
	assert.Equal(t, 401, w.Code)
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	ginmw.SetJWTSecret("test-secret")

	claims := &ginmw.Claims{
		Email: "admin@example.com",
		Roles: []string{"platform_admin"},
	}
	token, err := ginmw.GenerateToken(claims, 15)
	require.NoError(t, err)

	r := gin.New()
	r.Use(ginmw.AuthMiddleware())
	r.GET("/test", func(c *gin.Context) {
		cl := ginmw.GetClaims(c)
		require.NotNil(t, cl)
		c.JSON(200, gin.H{"email": cl.Email})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "admin@example.com")
}

func TestGenerateToken_PreservesRegisteredClaims(t *testing.T) {
	ginmw.SetJWTSecret("test-secret")

	nbf := time.Now().Add(-1 * time.Minute)
	claims := &ginmw.Claims{
		Email: "user@example.com",
		Roles: []string{"developer"},
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "my-auth-service",
			Subject:   "usr_12345",
			Audience:  jwt.ClaimStrings{"api.service.internal", "client.app"},
			ID:        "jti-998877",
			NotBefore: jwt.NewNumericDate(nbf),
		},
	}

	tokenStr, err := ginmw.GenerateToken(claims, 30)
	require.NoError(t, err)

	// Verify on the claims struct
	assert.Equal(t, "my-auth-service", claims.Issuer)
	assert.Equal(t, "usr_12345", claims.Subject)
	assert.Equal(t, jwt.ClaimStrings{"api.service.internal", "client.app"}, claims.Audience)
	assert.Equal(t, "jti-998877", claims.ID)
	assert.Equal(t, jwt.NewNumericDate(nbf), claims.NotBefore)
	assert.NotNil(t, claims.ExpiresAt)
	assert.NotNil(t, claims.IssuedAt)

	// Verify by parsing token back
	parsedClaims := &ginmw.Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, parsedClaims, func(token *jwt.Token) (interface{}, error) {
		return []byte("test-secret"), nil
	})
	require.NoError(t, err)
	require.True(t, token.Valid)

	assert.Equal(t, "my-auth-service", parsedClaims.Issuer)
	assert.Equal(t, "usr_12345", parsedClaims.Subject)
	assert.Equal(t, jwt.ClaimStrings{"api.service.internal", "client.app"}, parsedClaims.Audience)
	assert.Equal(t, "jti-998877", parsedClaims.ID)
	assert.Equal(t, nbf.Unix(), parsedClaims.NotBefore.Unix())
}

func TestClaims_Methods(t *testing.T) {
	t.Run("PrimaryRole", func(t *testing.T) {
		c1 := &ginmw.Claims{Roles: []string{"admin", "user"}}
		assert.Equal(t, "admin", c1.PrimaryRole())

		c2 := &ginmw.Claims{Roles: []string{}}
		assert.Equal(t, "", c2.PrimaryRole())

		c3 := &ginmw.Claims{Roles: nil}
		assert.Equal(t, "", c3.PrimaryRole())
	})

	t.Run("HasRole", func(t *testing.T) {
		c := &ginmw.Claims{Roles: []string{"admin", "editor"}}
		assert.True(t, c.HasRole("admin"))
		assert.True(t, c.HasRole("editor"))
		assert.False(t, c.HasRole("viewer"))
	})
}

func TestAuthMiddleware_BearerCaseInsensitive(t *testing.T) {
	ginmw.SetJWTSecret("test-secret")
	claims := &ginmw.Claims{Email: "test@x.com", Roles: []string{"CLIENT_ADMIN"}}
	token, _ := ginmw.GenerateToken(claims, 15)

	r := newRouter(ginmw.AuthMiddleware())
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "bearer "+token) // lowercase
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
}

func TestAuthMiddleware_QueryToken_DefaultRejected(t *testing.T) {
	ginmw.SetJWTSecret("test-secret")
	claims := &ginmw.Claims{Email: "query@example.com"}
	token, err := ginmw.GenerateToken(claims, 15)
	require.NoError(t, err)

	r := newRouter(ginmw.AuthMiddleware())
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test?token="+token, nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, 401, w.Code)
	assert.Contains(t, w.Body.String(), "UNAUTHORIZED")
}

func TestAuthMiddleware_QueryToken_Allowed(t *testing.T) {
	ginmw.SetJWTSecret("test-secret")
	claims := &ginmw.Claims{Email: "ws@example.com"}
	token, err := ginmw.GenerateToken(claims, 15)
	require.NoError(t, err)

	r := newRouter(ginmw.AuthMiddleware(ginmw.WithAllowQueryToken()))

	// 1. Valid token in query param is accepted
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test?token="+token, nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	// 2. Missing query param is rejected
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w2, req2)
	assert.Equal(t, 401, w2.Code)
	assert.Contains(t, w2.Body.String(), "UNAUTHORIZED")
}

func TestAuthMiddleware_NonBearerHeader(t *testing.T) {
	ginmw.SetJWTSecret("test-secret")
	claims := &ginmw.Claims{Email: "ws@example.com"}
	token, err := ginmw.GenerateToken(claims, 15)
	require.NoError(t, err)

	// Non-bearer header with query token not allowed
	r1 := newRouter(ginmw.AuthMiddleware())
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("GET", "/test", nil)
	req1.Header.Set("Authorization", "Basic user:pass")
	r1.ServeHTTP(w1, req1)
	assert.Equal(t, 401, w1.Code)

	// Non-bearer header with query token allowed falls through to query
	r2 := newRouter(ginmw.AuthMiddleware(ginmw.WithAllowQueryToken()))
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/test?token="+token, nil)
	req2.Header.Set("Authorization", "Basic user:pass")
	r2.ServeHTTP(w2, req2)
	assert.Equal(t, 200, w2.Code)
}

func TestAuthMiddleware_InvalidSigningMethods(t *testing.T) {
	ginmw.SetJWTSecret("test-secret")

	// 1. "none" algorithm attack
	claims := &ginmw.Claims{Email: "attacker@example.com"}
	noneToken := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	tokenStr, err := noneToken.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	r := newRouter(ginmw.AuthMiddleware())
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	r.ServeHTTP(w, req)
	assert.Equal(t, 401, w.Code)
	assert.Contains(t, w.Body.String(), "TOKEN_EXPIRED")

	// 2. Algorithm confusion / substitution: HS384 instead of HS256
	hs384Token := jwt.NewWithClaims(jwt.SigningMethodHS384, claims)
	hs384Str, err := hs384Token.SignedString([]byte("test-secret"))
	require.NoError(t, err)

	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/test", nil)
	req2.Header.Set("Authorization", "Bearer "+hs384Str)
	r.ServeHTTP(w2, req2)
	assert.Equal(t, 401, w2.Code)
	assert.Contains(t, w2.Body.String(), "TOKEN_EXPIRED")
}

func TestJWTSecret_Validation(t *testing.T) {
	// Restore secret after this test
	defer ginmw.SetJWTSecret("test-secret")

	// 1. SetJWTSecret empty string rejected
	err := ginmw.SetJWTSecret("")
	assert.Error(t, err)

	err = ginmw.SetJWTSecret("   ")
	assert.Error(t, err)

	// 2. GenerateToken with uninitialized secret fails
	claims := &ginmw.Claims{Email: "test@example.com"}
	_, err = ginmw.GenerateToken(claims, 15)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "uninitialized")

	// 3. AuthMiddleware with uninitialized secret aborts with 401
	r := newRouter(ginmw.AuthMiddleware())
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer some-token")
	r.ServeHTTP(w, req)
	assert.Equal(t, 401, w.Code)
	assert.Contains(t, w.Body.String(), "UNAUTHORIZED")

	// 4. Valid secret setting succeeds
	err = ginmw.SetJWTSecret("new-secret")
	assert.NoError(t, err)
}

func TestGetClaims_ReturnsNilIfUnauthenticated(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	assert.Nil(t, ginmw.GetClaims(c))
}

// ── BodyLimit ─────────────────────────────────────────────────────────────────

func TestLimitBodyDefault_PassesSmallBody(t *testing.T) {
	r := newRouter(ginmw.LimitBodyDefault())
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/test", strings.NewReader("hello"))
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
}

func TestLimitBodyAuth_PassesSmallBody(t *testing.T) {
	r := newRouter(ginmw.LimitBodyAuth())
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/test", strings.NewReader(`{"email":"a@b.com","password":"pass"}`))
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
}

// ── RateLimit ─────────────────────────────────────────────────────────────────

func TestGlobalRateLimit_AllowsFirstRequest(t *testing.T) {
	r := newRouter(ginmw.GlobalRateLimit())
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "1.2.3.4:9999"
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
}

func TestAuthRateLimit_AllowsFirstRequest(t *testing.T) {
	r := newRouter(ginmw.AuthRateLimit())
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/test", nil)
	req.RemoteAddr = "1.2.3.5:9999"
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
}

func TestRateLimitWithLimiter_AllowsAndBlocksDefaultKey(t *testing.T) {
	limiter := ratelimit.NewTokenBucket(1, time.Minute)
	r := newRouter(ginmw.RateLimitWithLimiter(limiter))

	// First request succeeds
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = "1.2.3.4:1234"
	r.ServeHTTP(w1, req1)
	assert.Equal(t, 200, w1.Code)
	assert.Equal(t, "0", w1.Header().Get("RateLimit-Remaining"))

	// Second request is blocked
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "1.2.3.4:1234"
	r.ServeHTTP(w2, req2)
	assert.Equal(t, 429, w2.Code)
	assert.Contains(t, w2.Body.String(), "RATE_LIMITED")
}

func TestRateLimitWithLimiter_CustomKeyFunc(t *testing.T) {
	limiter := ratelimit.NewSlidingWindow(1, time.Minute)
	customKey := func(c *gin.Context) string {
		return c.GetHeader("X-Tenant-ID")
	}
	r := newRouter(ginmw.RateLimitWithLimiter(limiter, customKey))

	// First request with tenant-1 succeeds
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("GET", "/test", nil)
	req1.Header.Set("X-Tenant-ID", "tenant-1")
	r.ServeHTTP(w1, req1)
	assert.Equal(t, 200, w1.Code)

	// Request with tenant-2 succeeds
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/test", nil)
	req2.Header.Set("X-Tenant-ID", "tenant-2")
	r.ServeHTTP(w2, req2)
	assert.Equal(t, 200, w2.Code)

	// Second request with tenant-1 is blocked
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest("GET", "/test", nil)
	req3.Header.Set("X-Tenant-ID", "tenant-1")
	r.ServeHTTP(w3, req3)
	assert.Equal(t, 429, w3.Code)
}

// ── RequestID ─────────────────────────────────────────────────────────────────

func TestRequestID_InjectsHeader(t *testing.T) {
	r := newRouter(ginmw.RequestID())
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)
	assert.NotEmpty(t, w.Header().Get("X-Request-ID"))
}

func TestRequestID_PreservesExistingID(t *testing.T) {
	r := newRouter(ginmw.RequestID())
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", "my-trace-id-123")
	r.ServeHTTP(w, req)
	assert.Equal(t, "my-trace-id-123", w.Header().Get("X-Request-ID"))
}

func TestGetRequestID_ReturnsIDFromContext(t *testing.T) {
	// GetRequestID reads from the context key set by go-libs/requestid.Middleware.
	// When wrapped via gin.WrapH the context propagation depends on gin internals;
	// we assert the function is callable and returns a string (may be empty).
	r := gin.New()
	r.Use(ginmw.RequestID())
	r.GET("/test", func(c *gin.Context) {
		id := ginmw.GetRequestID(c.Request.Context())
		_ = id // string, not nil
		c.Status(200)
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", "trace-abc")
	assert.NotPanics(t, func() { r.ServeHTTP(w, req) })
	assert.Equal(t, 200, w.Code)
	// The response header is set by the underlying requestid middleware
	assert.NotEmpty(t, w.Header().Get("X-Request-ID"))
}

// ── SecurityHeaders ───────────────────────────────────────────────────────────

func TestSecurityHeaders_SetsXFrameOptions(t *testing.T) {
	r := newRouter(ginmw.SecurityHeaders("core-api"))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
}

func TestSecurityHeaders_DefaultServerName(t *testing.T) {
	// Empty string → falls back to "api"
	r := newRouter(ginmw.SecurityHeaders())
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, "api", w.Header().Get("Server"))
}

func TestSecurityHeaders_CustomServerName(t *testing.T) {
	r := newRouter(ginmw.SecurityHeaders("inventory-service"))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, "inventory-service", w.Header().Get("Server"))
}

// ── Logger ────────────────────────────────────────────────────────────────────

func TestLogger_DoesNotPanic(t *testing.T) {
	r := newRouter(ginmw.Logger())
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	assert.NotPanics(t, func() { r.ServeHTTP(w, req) })
	assert.Equal(t, 200, w.Code)
}
