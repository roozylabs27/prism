package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/httputil"
	"github.com/roozylabs/prism/internal/middleware"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/repository"
)

type workspaceStore interface {
	ListWorkspacesByOrg(ctx context.Context, orgID string) ([]models.Workspace, error)
	CreateWorkspace(ctx context.Context, orgID, userID, name, slug string) (*models.Workspace, error)
	ListProjectsByWorkspace(ctx context.Context, orgID, workspaceID string) ([]models.Project, error)
	CreateProject(ctx context.Context, orgID, workspaceID, name, slug string) (*models.Project, error)
}

type WorkspaceHandler struct {
	repo  workspaceStore
	audit *AdminAudit
}

func NewWorkspaceHandler(repo *repository.WorkspaceRepository) *WorkspaceHandler {
	var store workspaceStore
	if repo != nil {
		store = repo
	}
	return &WorkspaceHandler{repo: store}
}

func (h *WorkspaceHandler) SetAuditLog(a *AdminAudit) {
	h.audit = a
}

type createWorkspaceRequest struct {
	Name string `json:"name" binding:"required"`
	Slug string `json:"slug"`
}

type createProjectRequest struct {
	Name string `json:"name" binding:"required"`
	Slug string `json:"slug"`
}

func tenantOrgID(c *gin.Context) string {
	if orgID := middleware.GetTenantContext(c).OrgID; orgID != "" {
		return orgID
	}
	if orgID := c.GetString("organizationId"); orgID != "" {
		return orgID
	}
	return c.GetString("organization_id")
}

func (h *WorkspaceHandler) ListWorkspaces(c *gin.Context) {
	orgID := tenantOrgID(c)
	if orgID == "" {
		httputil.RespondForbidden(c, "Organization context is required", nil, "TENANT_REQUIRED")
		return
	}
	if h.repo == nil {
		httputil.RespondInternalError(c, "Workspace store unavailable", nil, "WORKSPACE_STORE_UNAVAILABLE")
		return
	}
	workspaces, err := h.repo.ListWorkspacesByOrg(c.Request.Context(), orgID)
	if err != nil {
		httputil.RespondInternalError(c, "Failed to list workspaces", err, "WORKSPACES_LIST_FAILED")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   workspaces,
	})
}

func (h *WorkspaceHandler) CreateWorkspace(c *gin.Context) {
	orgID := tenantOrgID(c)
	if orgID == "" {
		httputil.RespondForbidden(c, "Organization context is required", nil, "TENANT_REQUIRED")
		return
	}
	if h.repo == nil {
		httputil.RespondInternalError(c, "Workspace store unavailable", nil, "WORKSPACE_STORE_UNAVAILABLE")
		return
	}

	var req createWorkspaceRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		httputil.RespondBadRequest(c, "Workspace name is required", err, "WORKSPACE_NAME_REQUIRED")
		return
	}

	userID := c.GetString("userId")
	ws, err := h.repo.CreateWorkspace(c.Request.Context(), orgID, userID, req.Name, req.Slug)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrWorkspaceLimitReached):
			httputil.RespondError(c, http.StatusConflict, "Organization maximum workspace limit reached", err, "WORKSPACE_LIMIT_REACHED")
		case errors.Is(err, repository.ErrWorkspaceSlugExists):
			httputil.RespondError(c, http.StatusConflict, "Workspace slug already exists in organization", err, "WORKSPACE_SLUG_EXISTS")
		default:
			httputil.RespondInternalError(c, "Failed to create workspace", err, "WORKSPACE_CREATE_FAILED")
		}
		return
	}

	h.audit.Record(c, "workspace.create", "workspace", ws.ID, map[string]any{"name": ws.Name, "slug": ws.Slug})
	c.JSON(http.StatusCreated, ws)
}

func (h *WorkspaceHandler) ListProjects(c *gin.Context) {
	orgID := tenantOrgID(c)
	if orgID == "" {
		httputil.RespondForbidden(c, "Organization context is required", nil, "TENANT_REQUIRED")
		return
	}
	if h.repo == nil {
		httputil.RespondInternalError(c, "Workspace store unavailable", nil, "WORKSPACE_STORE_UNAVAILABLE")
		return
	}
	wsID := strings.TrimSpace(c.Param("workspaceId"))
	projects, err := h.repo.ListProjectsByWorkspace(c.Request.Context(), orgID, wsID)
	if err != nil {
		if errors.Is(err, repository.ErrWorkspaceNotFound) {
			httputil.RespondNotFound(c, "Workspace not found in this organization", err, "WORKSPACE_NOT_FOUND")
			return
		}
		httputil.RespondInternalError(c, "Failed to list workspace projects", err, "PROJECTS_LIST_FAILED")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   projects,
	})
}

func (h *WorkspaceHandler) CreateProject(c *gin.Context) {
	orgID := tenantOrgID(c)
	if orgID == "" {
		httputil.RespondForbidden(c, "Organization context is required", nil, "TENANT_REQUIRED")
		return
	}
	if h.repo == nil {
		httputil.RespondInternalError(c, "Workspace store unavailable", nil, "WORKSPACE_STORE_UNAVAILABLE")
		return
	}
	wsID := strings.TrimSpace(c.Param("workspaceId"))
	var req createProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		httputil.RespondBadRequest(c, "Project name is required", err, "PROJECT_NAME_REQUIRED")
		return
	}

	proj, err := h.repo.CreateProject(c.Request.Context(), orgID, wsID, req.Name, req.Slug)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrWorkspaceNotFound):
			httputil.RespondNotFound(c, "Workspace not found in this organization", err, "WORKSPACE_NOT_FOUND")
		case errors.Is(err, repository.ErrProjectLimitReached):
			httputil.RespondError(c, http.StatusConflict, "Workspace maximum project limit reached", err, "PROJECT_LIMIT_REACHED")
		case errors.Is(err, repository.ErrProjectSlugExists):
			httputil.RespondError(c, http.StatusConflict, "Project slug already exists in workspace", err, "PROJECT_SLUG_EXISTS")
		default:
			httputil.RespondInternalError(c, "Failed to create project", err, "PROJECT_CREATE_FAILED")
		}
		return
	}

	h.audit.Record(c, "project.create", "project", proj.ID, map[string]any{"workspaceId": proj.WorkspaceID, "name": proj.Name, "slug": proj.Slug})
	c.JSON(http.StatusCreated, proj)
}
