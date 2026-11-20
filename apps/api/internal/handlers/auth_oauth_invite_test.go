package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestInitiateOAuthSetsAndClearsInviteCookie(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "google-client-id")
	t.Setenv("GOOGLE_CLIENT_SECRET", "google-client-secret")

	h := &OAuthHandler{
		authService: &fakeOAuthSessions{},
		endpoints:   defaultOAuthEndpoints,
		client:      http.DefaultClient,
	}
	r := oauthRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/oauth/google?invite=inv-tok-123", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected 307 redirect, got %d", w.Code)
	}
	var inviteCookie *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == oauthInviteCookie {
			inviteCookie = c
		}
	}
	if inviteCookie == nil || inviteCookie.Value != "inv-tok-123" || !inviteCookie.HttpOnly || inviteCookie.Path != oauthStatePath {
		t.Fatalf("expected HttpOnly oauth_invite cookie with path %q and token, got %+v", oauthStatePath, inviteCookie)
	}
}

func TestDevLoginPreservesInviteRedirect(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "")
	t.Setenv("GOOGLE_CLIENT_SECRET", "")

	h := &OAuthHandler{
		authService:   &fakeOAuthSessions{},
		endpoints:     defaultOAuthEndpoints,
		client:        http.DefaultClient,
		allowDevLogin: true,
	}
	r := oauthRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/oauth/google?invite=inv%2Bdev", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected 307, got %d: %s", w.Code, w.Body.String())
	}
	loc := w.Header().Get("Location")
	if loc != "/accept-invite?token=inv%2Bdev" {
		t.Fatalf("expected redirect to /accept-invite?token=inv%%2Bdev, got %q", loc)
	}
	if !setsAuthToken(w) {
		t.Fatalf("expected auth_token cookie to be set")
	}
}

func TestOAuthCallbackConsumesInviteCookieAndRedirectsToAcceptInvite(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "google-cid")
	t.Setenv("GOOGLE_CLIENT_SECRET", "google-csec")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "token") {
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "tok"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"email":          "invited@example.com",
			"verified_email": true,
			"name":           "Invited User",
		})
	}))
	defer srv.Close()

	ep := defaultOAuthEndpoints
	ep.googleToken = srv.URL + "/token"
	ep.googleUserInfo = srv.URL + "/userinfo"

	h := &OAuthHandler{
		authService: &fakeOAuthSessions{},
		endpoints:   ep,
		client:      srv.Client(),
	}
	gin.SetMode(gin.TestMode)
	r := oauthRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/oauth/google/callback?code=abc&state=st-1", nil)
	req.AddCookie(&http.Cookie{Name: oauthStateCookie, Value: "st-1"})
	req.AddCookie(&http.Cookie{Name: oauthInviteCookie, Value: "invite-xyz"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected 307 redirect, got %d: %s", w.Code, w.Body.String())
	}
	if loc := w.Header().Get("Location"); loc != "/accept-invite?token=invite-xyz" {
		t.Fatalf("expected /accept-invite?token=invite-xyz redirect, got %q", loc)
	}
	clearedInvite := false
	for _, c := range w.Result().Cookies() {
		if c.Name == oauthInviteCookie && c.MaxAge < 0 {
			clearedInvite = true
		}
	}
	if !clearedInvite {
		t.Fatalf("expected oauth_invite cookie to be cleared after consumption")
	}
}
