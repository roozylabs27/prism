package middleware

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/httputil"
)

func isMutatingMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func isOriginInList(origin string, allowedOrigins []string) bool {
	origin = strings.TrimRight(strings.TrimSpace(origin), "/")
	if origin == "" || strings.EqualFold(origin, "null") {
		return false
	}
	for _, allowed := range allowedOrigins {
		allowedTrimmed := strings.TrimRight(strings.TrimSpace(allowed), "/")
		if allowedTrimmed == "*" || strings.EqualFold(allowedTrimmed, origin) {
			return true
		}
	}
	return false
}

func extractOriginFromReferer(referer string) string {
	u, err := url.Parse(strings.TrimSpace(referer))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

// CSRFOriginMiddleware defends cookie-authenticated state-changing endpoints
// (and logout) against cross-origin and sibling-subdomain request forgery.
func CSRFOriginMiddleware(allowedOrigins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !isMutatingMethod(c.Request.Method) {
			c.Next()
			return
		}

		_, cookieErr := c.Cookie("auth_token")
		hasAuthCookie := cookieErr == nil
		isLogout := strings.HasSuffix(c.Request.URL.Path, "/auth/logout")
		if !hasAuthCookie && !isLogout {
			c.Next()
			return
		}

		secFetchSite := strings.TrimSpace(c.GetHeader("Sec-Fetch-Site"))
		if strings.EqualFold(secFetchSite, "cross-site") {
			httputil.RespondForbidden(c, "Cross-site request blocked by CSRF policy", nil, "CSRF_ORIGIN_REJECTED")
			return
		}

		if origin := c.GetHeader("Origin"); origin != "" {
			if !isOriginInList(origin, allowedOrigins) {
				httputil.RespondForbidden(c, "Untrusted Origin blocked by CSRF policy", nil, "CSRF_ORIGIN_REJECTED")
				return
			}
			c.Next()
			return
		}

		if referer := c.GetHeader("Referer"); referer != "" {
			refOrigin := extractOriginFromReferer(referer)
			if !isOriginInList(refOrigin, allowedOrigins) {
				httputil.RespondForbidden(c, "Untrusted Referer blocked by CSRF policy", nil, "CSRF_ORIGIN_REJECTED")
				return
			}
			c.Next()
			return
		}

		c.Next()
	}
}
