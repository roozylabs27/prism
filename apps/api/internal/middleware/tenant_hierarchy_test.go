package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/database"
	"github.com/roozylabs/prism/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeHierarchyChecker struct {
	members    map[string]bool
	primaryOrg string
	workspaces map[string]*models.Workspace // key: "org/ws"
	projects   map[string]*models.Project   // key: "org/proj"
	wsMembers  map[string]string            // key: "ws/user" -> role (only enforced when non-nil)
	userPerms  map[string][]string          // key: "org/user" -> perms
}

func (f *fakeHierarchyChecker) IsMember(_ context.Context, userID, orgID string) (bool, error) {
	if f.members[orgID+"/"+userID] {
		return true, nil
	}
	return false, nil
}

func (f *fakeHierarchyChecker) GetPrimaryOrganization(_ context.Context, userID string) (string, error) {
	return f.primaryOrg, nil
}

func (f *fakeHierarchyChecker) FindWorkspaceInOrg(_ context.Context, orgID, workspaceID string) (*models.Workspace, error) {
	if ws, ok := f.workspaces[orgID+"/"+workspaceID]; ok {
		return ws, nil
	}
	return nil, errors.New("workspace not found")
}

func (f *fakeHierarchyChecker) FindProjectInOrg(_ context.Context, orgID, projectID string) (*models.Project, error) {
	if proj, ok := f.projects[orgID+"/"+projectID]; ok {
		return proj, nil
	}
	return nil, errors.New("project not found")
}

func (f *fakeHierarchyChecker) GetUserPermissions(_ context.Context, userID, orgID string) ([]string, string, error) {
	if f.userPerms == nil {
		return []string{"workspace:admin"}, "owner", nil
	}
	return f.userPerms[orgID+"/"+userID], "custom", nil
}

func (f *fakeHierarchyChecker) GetWorkspaceMemberRole(_ context.Context, wsID, userID string) (string, error) {
	if f.wsMembers == nil {
		return "admin", nil
	}
	if role, ok := f.wsMembers[wsID+"/"+userID]; ok && role != "" {
		return role, nil
	}
	return "", errors.New("not a workspace member")
}

func TestResolveCanonicalTenantContext_HierarchyValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	checker := &fakeHierarchyChecker{
		members: map[string]bool{"org_a/user_1": true},
		workspaces: map[string]*models.Workspace{
			"org_a/ws_1": {ID: "ws_1", OrgID: "org_a"},
		},
		projects: map[string]*models.Project{
			"org_a/proj_1": {ID: "proj_1", WorkspaceID: "ws_1"},
			"org_a/proj_2": {ID: "proj_2", WorkspaceID: "ws_2"},
		},
	}

	makeCtx := func(headers map[string]string) *gin.Context {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		c.Request = req
		c.Set("userId", "user_1")
		return c
	}

	t.Run("foreign workspace forbidden", func(t *testing.T) {
		c := makeCtx(map[string]string{
			"X-Prism-Org-ID":       "org_a",
			"X-Prism-Workspace-ID": "ws_other_org",
		})
		_, err := ResolveCanonicalTenantContext(c, nil, checker)
		require.ErrorIs(t, err, ErrCrossWorkspaceForbidden)
	})

	t.Run("project outside selected workspace forbidden", func(t *testing.T) {
		c := makeCtx(map[string]string{
			"X-Prism-Org-ID":       "org_a",
			"X-Prism-Workspace-ID": "ws_1",
			"X-Prism-Project-ID":   "proj_2",
		})
		_, err := ResolveCanonicalTenantContext(c, nil, checker)
		require.ErrorIs(t, err, ErrCrossProjectForbidden)
	})

	t.Run("project header auto-fills parent workspace when omitted", func(t *testing.T) {
		c := makeCtx(map[string]string{
			"X-Prism-Org-ID":     "org_a",
			"X-Prism-Project-ID": "proj_1",
		})
		tc, err := ResolveCanonicalTenantContext(c, nil, checker)
		require.NoError(t, err)
		assert.Equal(t, "org_a", tc.OrgID)
		assert.Equal(t, "ws_1", tc.WorkspaceID)
		assert.Equal(t, "proj_1", tc.ProjectID)

		dbCtx := database.TenantContextFrom(c.Request.Context())
		assert.Equal(t, "user_1", dbCtx.UserID)
		assert.Equal(t, "org_a", dbCtx.OrgID)
		assert.Equal(t, "ws_1", dbCtx.WorkspaceID)
		assert.Equal(t, "proj_1", dbCtx.ProjectID)
	})

	t.Run("non-admin user denied workspace they are not a member of", func(t *testing.T) {
		strictChecker := &fakeHierarchyChecker{
			members: map[string]bool{"org_a/user_1": true},
			workspaces: map[string]*models.Workspace{
				"org_a/ws_1": {ID: "ws_1", OrgID: "org_a"},
				"org_a/ws_2": {ID: "ws_2", OrgID: "org_a"},
			},
			userPerms: map[string][]string{
				"org_a/user_1": {"workspace:read", "logs:read"},
			},
			wsMembers: map[string]string{
				"ws_1/user_1": "developer",
			},
		}
		cAllowed := makeCtx(map[string]string{
			"X-Prism-Org-ID":       "org_a",
			"X-Prism-Workspace-ID": "ws_1",
		})
		tc, err := ResolveCanonicalTenantContext(cAllowed, nil, strictChecker)
		require.NoError(t, err)
		assert.Equal(t, "ws_1", tc.WorkspaceID)

		cDenied := makeCtx(map[string]string{
			"X-Prism-Org-ID":       "org_a",
			"X-Prism-Workspace-ID": "ws_2",
		})
		_, err = ResolveCanonicalTenantContext(cDenied, nil, strictChecker)
		require.ErrorIs(t, err, ErrCrossWorkspaceForbidden)
	})
}

func TestOptionalTenantMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	checker := &fakeHierarchyChecker{
		members: map[string]bool{"org_a/user_1": true},
	}

	runReq := func(userID string, orgHeader string) *httptest.ResponseRecorder {
		r := gin.New()
		r.Use(func(c *gin.Context) {
			c.Set("userId", userID)
		})
		r.Use(OptionalTenantMiddleware(checker))
		r.GET("/user/permissions", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"orgId":   GetTenantContext(c).OrgID,
				"rlsUser": database.TenantContextFrom(c.Request.Context()).UserID,
				"rlsOrg":  database.TenantContextFrom(c.Request.Context()).OrgID,
			})
		})
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/user/permissions", nil)
		if orgHeader != "" {
			req.Header.Set("X-Prism-Org-ID", orgHeader)
		}
		r.ServeHTTP(w, req)
		return w
	}

	t.Run("un-onboarded user with no org proceeds with user-scoped RLS ctx", func(t *testing.T) {
		w := runReq("user_new", "")
		require.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"rlsUser":"user_new"`)
		assert.Contains(t, w.Body.String(), `"rlsOrg":""`)
	})

	t.Run("unauthorized org header still rejected with 403", func(t *testing.T) {
		w := runReq("user_1", "org_foreign")
		require.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("valid org header attaches org and RLS context", func(t *testing.T) {
		w := runReq("user_1", "org_a")
		require.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"orgId":"org_a"`)
		assert.Contains(t, w.Body.String(), `"rlsOrg":"org_a"`)
	})
}
