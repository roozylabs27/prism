package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/httputil"
	"github.com/roozylabs/prism/internal/models"
)

// GatewayKeyRoutes are the dashboard routes a gateway API key may call: the
// ones the CLI and TypeScript SDK use. A session can call every route.
var GatewayKeyRoutes = map[string]bool{
	"GET /api/agents":                                       true,
	"POST /api/agents":                                      true,
	"GET /api/agents/:id":                                   true,
	"PUT /api/agents/:id":                                   true,
	"DELETE /api/agents/:id":                                true,
	"GET /api/agents/:id/stats":                             true,
	"GET /api/mcp/servers":                                  true,
	"POST /api/mcp/servers":                                 true,
	"GET /api/mcp/servers/:id":                              true,
	"PUT /api/mcp/servers/:id":                              true,
	"DELETE /api/mcp/servers/:id":                           true,
	"POST /api/mcp/servers/:id/sync":                        true,
	"GET /api/mcp/servers/:id/tools":                        true,
	"GET /api/mcp/servers/:id/stats":                        true,
	"GET /api/mcp/registry":                                 true,
	"GET /api/mcp/registry/:slug":                           true,
	"GET /api/resources":                                    true,
	"GET /api/tools":                                        true,
	"GET /api/credentials":                                  true,
	"DELETE /api/credentials/:credId":                       true,
	"POST /api/credentials/:credId/reset-cooldown":          true,
	"POST /api/routing/simulate":                            true,
	"GET /api/workspaces":                                   true,
	"POST /api/workspaces":                                  true,
	"PUT /api/workspaces/:workspaceId":                      true,
	"DELETE /api/workspaces/:workspaceId":                   true,
	"GET /api/workspaces/:workspaceId/projects":             true,
	"POST /api/workspaces/:workspaceId/projects":            true,
	"PUT /api/workspaces/:workspaceId/projects/:projectId":  true,
	"DELETE /api/workspaces/:workspaceId/projects/:projectId": true,
	"GET /api/analytics/pilot-outcomes":                     true,
	"GET /api/budgets":                                      true,
	"GET /api/quotas":                                       true,
	"PUT /api/quotas/:target_type/:target_id":               true,
	"GET /api/approvals":                                    true,
}

// GatewayKeyV1Routes are /v1 (and /api/v1) gateway routes that also evaluate
// the key owner's RBAC permission via RequirePermission.
var GatewayKeyV1Routes = map[string]bool{
	"POST /v1/adapters/paperclip/agent/register":     true,
	"POST /api/v1/adapters/paperclip/agent/register": true,
}

// IsGatewayKeyRouteAllowed reports whether a gateway API key may invoke a
// permission-guarded endpoint using its owner's organization permissions.
func IsGatewayKeyRouteAllowed(method, fullPath string) bool {
	key := method + " " + fullPath
	return GatewayKeyRoutes[key] || GatewayKeyV1Routes[key]
}

// RestrictGatewayKeys answers 403 when a request authenticated with a
// gateway API key reaches a route outside allowed. A leaked inference key
// then cannot read logs, change routing or manage members, while the CLI
// keeps working. It must run after AuthMiddleware.
func RestrictGatewayKeys(allowed map[string]bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, viaKey := c.Get("gatewayKey"); viaKey && !allowed[c.Request.Method+" "+c.FullPath()] {
			httputil.RespondError(c, http.StatusForbidden, "This endpoint requires signing in; gateway API keys cannot call it", nil, "GATEWAY_KEY_NOT_ALLOWED")
			return
		}
		c.Next()
	}
}

// GatewayKeyOwner lets permission checks on gateway routes judge the key's
// owner: it sets userId from the key GatewayAuthMiddleware resolved.
func GatewayKeyOwner() gin.HandlerFunc {
	return func(c *gin.Context) {
		if v, ok := c.Get("gatewayKey"); ok {
			if key, isKey := v.(*models.GatewayAPIKey); isKey && key != nil {
				c.Set("userId", key.UserID)
			}
		}
		c.Next()
	}
}
