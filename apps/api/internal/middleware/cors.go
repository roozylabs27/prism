package middleware

import (
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// CORSMiddleware answers the listed origins with credentials. The list comes
// from config (ALLOWED_ORIGINS), which refuses "*" and local origins in
// production.
func CORSMiddleware(allowedOrigins []string) gin.HandlerFunc {
	return cors.New(cors.Config{
		AllowOriginFunc: func(origin string) bool {
			if origin == "" {
				return true
			}
			for _, allowed := range allowedOrigins {
				if allowed == "*" || strings.EqualFold(allowed, origin) {
					return true
				}
			}
			return false
		},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Prism-Agent-ID", "X-Prism-Role", "X-Prism-Org-ID", "X-Prism-Workspace-ID", "X-Prism-Project-ID", "X-Routing-Policy", "X-Prism-Approval-ID", "Idempotency-Key", "X-Request-ID"},
		ExposeHeaders:    []string{"Content-Length", "X-Request-ID", "X-Prism-Model", "X-Prism-Provider", "X-Prism-Routing-Policy", "X-Roozy-Model", "X-Roozy-Provider"},
		AllowCredentials: true,
	})
}
