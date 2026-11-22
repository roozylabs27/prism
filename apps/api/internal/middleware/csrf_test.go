package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func csrfTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CSRFOriginMiddleware([]string{"https://console.prism.roozylabs.com", "https://app.prism.roozylabs.com", "http://localhost:3000"}))
	ok := func(c *gin.Context) { c.Status(http.StatusOK) }
	r.GET("/api/providers", ok)
	r.POST("/api/gateway-keys", ok)
	r.POST("/api/auth/logout", ok)
	return r
}

func TestCSRFOriginMiddleware(t *testing.T) {
	r := csrfTestRouter()

	t.Run("allows safe GET with auth cookie", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/providers", nil)
		req.AddCookie(&http.Cookie{Name: "auth_token", Value: "tok"})
		req.Header.Set("Origin", "https://evil.roozylabs.com")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("allows Bearer CLI POST without cookie even if Origin header absent", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/gateway-keys", nil)
		req.Header.Set("Authorization", "Bearer prism_sk_123")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("blocks sibling subdomain Origin on cookie POST", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/gateway-keys", nil)
		req.AddCookie(&http.Cookie{Name: "auth_token", Value: "tok"})
		req.Header.Set("Origin", "https://evil.roozylabs.com")
		req.Header.Set("Sec-Fetch-Site", "same-site")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Contains(t, w.Body.String(), "CSRF_ORIGIN_REJECTED")
	})

	t.Run("blocks cross-site Sec-Fetch-Site on logout even without cookie", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
		req.Header.Set("Sec-Fetch-Site", "cross-site")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("blocks untrusted Referer when Origin absent", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/gateway-keys", nil)
		req.AddCookie(&http.Cookie{Name: "auth_token", Value: "tok"})
		req.Header.Set("Referer", "https://attacker.example/exploit.html")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("allows trusted dashboard Origin", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/gateway-keys", nil)
		req.AddCookie(&http.Cookie{Name: "auth_token", Value: "tok"})
		req.Header.Set("Origin", "https://app.prism.roozylabs.com")
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}
