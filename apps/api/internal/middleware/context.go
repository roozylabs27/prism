package middleware

import "github.com/gin-gonic/gin"

// UserID returns the canonical authenticated user ID from the Gin context,
// checking both "userId" and legacy "user_id".
func UserID(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if id := c.GetString("userId"); id != "" {
		return id
	}
	return c.GetString("user_id")
}

// OrgID returns the canonical organization ID from the Gin context,
// checking TenantContext first, then "organizationId" and "organization_id".
func OrgID(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if tc := GetTenantContext(c); tc.OrgID != "" {
		return tc.OrgID
	}
	if id := c.GetString("organizationId"); id != "" {
		return id
	}
	return c.GetString("organization_id")
}

// WorkspaceID returns the canonical workspace ID from the Gin context.
func WorkspaceID(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if tc := GetTenantContext(c); tc.WorkspaceID != "" {
		return tc.WorkspaceID
	}
	return c.GetString("workspace_id")
}

// ProjectID returns the canonical project ID from the Gin context.
func ProjectID(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if tc := GetTenantContext(c); tc.ProjectID != "" {
		return tc.ProjectID
	}
	return c.GetString("project_id")
}
