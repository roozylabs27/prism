package handlers

import (
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

const oauthInviteCookie = "oauth_invite"

func captureOAuthInviteCookie(c *gin.Context) {
	invite := strings.TrimSpace(c.Query("invite"))
	if invite != "" {
		setOAuthCookie(c, oauthInviteCookie, invite, int(oauthStateTTL.Seconds()), oauthStatePath)
		return
	}
	// Clear any stale invite cookie so an earlier aborted flow does not redirect a fresh sign-in.
	setOAuthCookie(c, oauthInviteCookie, "", -1, oauthStatePath)
}

func consumeOAuthPostLoginRedirect(c *gin.Context) string {
	invite := strings.TrimSpace(c.Query("invite"))
	if invite == "" {
		if cookieVal, err := c.Cookie(oauthInviteCookie); err == nil {
			invite = strings.TrimSpace(cookieVal)
		}
	}
	setOAuthCookie(c, oauthInviteCookie, "", -1, oauthStatePath)
	if invite != "" {
		return "/accept-invite?token=" + url.QueryEscape(invite)
	}
	return "/"
}
