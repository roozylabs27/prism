package middleware

import (
	"context"
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/database"
	"github.com/roozylabs/prism/internal/httputil"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/repository"
)

// TenantHierarchyValidator validates that requested workspace and project IDs
// belong to the canonical organization and matching parent workspace.
type TenantHierarchyValidator interface {
	FindWorkspaceInOrg(ctx context.Context, orgID, workspaceID string) (*models.Workspace, error)
	FindProjectInOrg(ctx context.Context, orgID, projectID string) (*models.Project, error)
}

// CompositeTenantChecker combines organization membership/primary-org lookup
// with workspace/project hierarchy validation.
type CompositeTenantChecker struct {
	OrgMemberChecker
	OrgPrimaryResolver
	TenantHierarchyValidator
}

// NewCompositeTenantChecker builds a CompositeTenantChecker backed by the
// account and workspace repositories (gracefully handling nil inputs in tests).
func NewCompositeTenantChecker(
	accountRepo *repository.AccountRepository,
	workspaceRepo *repository.WorkspaceRepository,
) *CompositeTenantChecker {
	checker := &CompositeTenantChecker{}
	if accountRepo != nil {
		checker.OrgMemberChecker = accountRepo
		checker.OrgPrimaryResolver = accountRepo
	}
	if workspaceRepo != nil {
		checker.TenantHierarchyValidator = workspaceRepo
	}
	return checker
}

func validateSessionHierarchy(
	c *gin.Context,
	checker OrgMemberChecker,
	orgID, wsID, projID string,
	gatewayKey *models.GatewayAPIKey,
) (string, string, error) {
	if c == nil || c.Request == nil || checker == nil {
		return wsID, projID, nil
	}
	hv, ok := checker.(TenantHierarchyValidator)
	if !ok || hv == nil {
		return wsID, projID, nil
	}

	ctx := c.Request.Context()
	keyPinnedWs := gatewayKey != nil && gatewayKey.WorkspaceID != nil && *gatewayKey.WorkspaceID != ""
	keyPinnedProj := gatewayKey != nil && gatewayKey.ProjectID != nil && *gatewayKey.ProjectID != ""

	if wsID != "" && !keyPinnedWs {
		ws, err := hv.FindWorkspaceInOrg(ctx, orgID, wsID)
		if err != nil || ws == nil {
			return "", "", ErrCrossWorkspaceForbidden
		}
	}

	if projID != "" && !keyPinnedProj {
		proj, err := hv.FindProjectInOrg(ctx, orgID, projID)
		if err != nil || proj == nil {
			return "", "", ErrCrossProjectForbidden
		}
		if wsID != "" && proj.WorkspaceID != wsID {
			return "", "", ErrCrossProjectForbidden
		}
		if wsID == "" {
			wsID = proj.WorkspaceID
		}
	}

	return wsID, projID, nil
}

func attachTenantDBContext(c *gin.Context, gatewayKey *models.GatewayAPIKey, tc models.TenantContext) {
	if c == nil || c.Request == nil {
		return
	}
	userID := c.GetString("userId")
	if userID == "" {
		userID = c.GetString("user_id")
	}
	if userID == "" && gatewayKey != nil {
		userID = gatewayKey.UserID
	}
	ctx := database.WithTenantContext(c.Request.Context(), database.TenantDBContext{
		UserID:      userID,
		OrgID:       tc.OrgID,
		WorkspaceID: tc.WorkspaceID,
		ProjectID:   tc.ProjectID,
	})
	c.Request = c.Request.WithContext(ctx)
}

// ContextWithTenant returns c.Request.Context() guaranteed to carry database.TenantDBContext
// derived from either existing RLS context or Gin's TenantContextKey / userId (preserving
// behavior in unit tests that bypass TenantMiddleware).
func ContextWithTenant(c *gin.Context) context.Context {
	if c == nil || c.Request == nil {
		return context.Background()
	}
	base := c.Request.Context()
	cur := database.TenantContextFrom(base)
	tc := GetTenantContext(c)
	userID := c.GetString("userId")
	if userID == "" {
		userID = c.GetString("user_id")
	}
	if cur.OrgID == "" && tc.OrgID != "" {
		cur.OrgID = tc.OrgID
	}
	if cur.WorkspaceID == "" && tc.WorkspaceID != "" {
		cur.WorkspaceID = tc.WorkspaceID
	}
	if cur.ProjectID == "" && tc.ProjectID != "" {
		cur.ProjectID = tc.ProjectID
	}
	if cur.UserID == "" && userID != "" {
		cur.UserID = userID
	}
	return database.WithTenantContext(base, cur)
}

// OptionalTenantMiddleware resolves tenant context when available (e.g. for
// /api/user/permissions before a new user completes org onboarding), while still
// rejecting explicit spoofed or unauthorized X-Prism-* headers with 403 Forbidden.
func OptionalTenantMiddleware(orgChecker ...OrgMemberChecker) gin.HandlerFunc {
	var checker OrgMemberChecker
	if len(orgChecker) > 0 {
		checker = orgChecker[0]
	}

	return func(c *gin.Context) {
		var gwKey *models.GatewayAPIKey
		if val, exists := c.Get("gatewayKey"); exists {
			if keyObj, ok := val.(*models.GatewayAPIKey); ok {
				gwKey = keyObj
			}
		}

		_, err := ResolveCanonicalTenantContext(c, gwKey, checker)
		if err != nil {
			if errors.Is(err, ErrMissingTenantContext) {
				attachTenantDBContext(c, gwKey, models.TenantContext{})
				c.Next()
				return
			}
			httputil.RespondForbidden(c, err.Error(), err, "TENANT_SECURITY_ERROR")
			c.Abort()
			return
		}
		c.Next()
	}
}
