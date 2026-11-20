package handlers

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeLogoutAuthService struct {
	deletedTokens []string
	logoutErr     error
}

func (f *fakeLogoutAuthService) Login(_ context.Context, _, _, _, _ string) (*service.LoginResponse, error) {
	return nil, nil
}

func (f *fakeLogoutAuthService) Signup(_ context.Context, _, _, _, _, _ string) (*service.LoginResponse, error) {
	return nil, nil
}

func (f *fakeLogoutAuthService) Logout(_ context.Context, token string) error {
	f.deletedTokens = append(f.deletedTokens, token)
	return f.logoutErr
}

func (f *fakeLogoutAuthService) Me(_ context.Context, _ string) (*models.User, error) {
	return nil, nil
}

func assertClearsAuthCookie(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	cookies := w.Result().Cookies()
	var authCookie *http.Cookie
	for _, ck := range cookies {
		if ck.Name == "auth_token" {
			authCookie = ck
			break
		}
	}
	require.NotNil(t, authCookie, "expected auth_token Set-Cookie header to clear session")
	assert.Empty(t, authCookie.Value)
	assert.Less(t, authCookie.MaxAge, 0)
	assert.True(t, authCookie.HttpOnly)
}

func TestAuthHandler_Logout_IdempotentCookieClear(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("valid cookie revokes session and clears HttpOnly cookie", func(t *testing.T) {
		svc := &fakeLogoutAuthService{}
		h := &AuthHandler{auth: svc}
		r := gin.New()
		r.POST("/api/auth/logout", h.Logout)

		req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
		req.AddCookie(&http.Cookie{Name: "auth_token", Value: "tok_valid"})
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, []string{"tok_valid"}, svc.deletedTokens)
		assertClearsAuthCookie(t, w)
	})

	t.Run("expired or missing session error still returns 200 and clears cookie", func(t *testing.T) {
		for _, svcErr := range []error{sql.ErrNoRows, errors.New("session expired or purged")} {
			svc := &fakeLogoutAuthService{logoutErr: svcErr}
			h := &AuthHandler{auth: svc}
			r := gin.New()
			r.POST("/api/auth/logout", h.Logout)

			req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
			req.AddCookie(&http.Cookie{Name: "auth_token", Value: "tok_expired"})
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			require.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, []string{"tok_expired"}, svc.deletedTokens)
			assertClearsAuthCookie(t, w)
		}
	})

	t.Run("bearer header triggers session delete and cookie clear", func(t *testing.T) {
		svc := &fakeLogoutAuthService{}
		h := &AuthHandler{auth: svc}
		r := gin.New()
		r.POST("/api/auth/logout", h.Logout)

		req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
		req.Header.Set("Authorization", "Bearer tok_header")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, []string{"tok_header"}, svc.deletedTokens)
		assertClearsAuthCookie(t, w)
	})

	t.Run("missing token still returns 200 and clears stale cookie", func(t *testing.T) {
		svc := &fakeLogoutAuthService{}
		h := &AuthHandler{auth: svc}
		r := gin.New()
		r.POST("/api/auth/logout", h.Logout)

		req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		assert.Empty(t, svc.deletedTokens)
		assertClearsAuthCookie(t, w)
	})
}
