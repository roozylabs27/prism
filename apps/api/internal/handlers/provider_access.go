package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/middleware"
	"github.com/roozylabs/prism/internal/models"
)

func isSystemProvider(p *models.Provider) bool {
	return p != nil && p.UserID == "" && p.OrgID == ""
}

func callerOwnsProvider(c *gin.Context, p *models.Provider, userID string) bool {
	if isSystemProvider(p) {
		return false
	}
	orgID := middleware.GetTenantContext(c).OrgID
	if orgID != "" && p.OrgID != "" {
		return p.OrgID == orgID
	}
	return userID != "" && p.UserID == userID
}

func callerCanAccessProvider(c *gin.Context, p *models.Provider, userID string) bool {
	if p == nil {
		return false
	}
	if isSystemProvider(p) {
		return true
	}
	return callerOwnsProvider(c, p, userID)
}
