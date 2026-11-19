package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeWorkspaceStore struct {
	orgsMaxWS   map[string]int
	orgsMaxProj map[string]int
	workspaces  map[string]models.Workspace
	projects    map[string]models.Project
}

func newFakeWorkspaceStore() *fakeWorkspaceStore {
	return &fakeWorkspaceStore{
		orgsMaxWS:   map[string]int{"org_agency": 5},
		orgsMaxProj: map[string]int{"org_agency": 3},
		workspaces: map[string]models.Workspace{
			"ws_other": {ID: "ws_other", OrgID: "org_other", Name: "Foreign Client", Slug: "foreign"},
		},
		projects: map[string]models.Project{},
	}
}

func (f *fakeWorkspaceStore) ListWorkspacesByOrg(_ context.Context, orgID string) ([]models.Workspace, error) {
	out := []models.Workspace{}
	for _, ws := range f.workspaces {
		if ws.OrgID == orgID {
			out = append(out, ws)
		}
	}
	return out, nil
}

func (f *fakeWorkspaceStore) CreateWorkspace(_ context.Context, orgID, _, name, slug string) (*models.Workspace, error) {
	count := 0
	for _, ws := range f.workspaces {
		if ws.OrgID == orgID {
			count++
			if slug != "" && ws.Slug == slug {
				return nil, repository.ErrWorkspaceSlugExists
			}
		}
	}
	if max := f.orgsMaxWS[orgID]; max > 0 && count >= max {
		return nil, repository.ErrWorkspaceLimitReached
	}
	if slug == "" {
		slug = strings.ToLower(strings.ReplaceAll(name, " ", "-"))
	}
	id := "ws_" + slug
	ws := models.Workspace{ID: id, OrgID: orgID, Name: name, Slug: slug}
	f.workspaces[id] = ws
	return &ws, nil
}

func (f *fakeWorkspaceStore) ListProjectsByWorkspace(_ context.Context, orgID, workspaceID string) ([]models.Project, error) {
	ws, ok := f.workspaces[workspaceID]
	if !ok || ws.OrgID != orgID {
		return nil, repository.ErrWorkspaceNotFound
	}
	out := []models.Project{}
	for _, p := range f.projects {
		if p.WorkspaceID == workspaceID {
			out = append(out, p)
		}
	}
	return out, nil
}

func (f *fakeWorkspaceStore) CreateProject(_ context.Context, orgID, workspaceID, name, slug string) (*models.Project, error) {
	ws, ok := f.workspaces[workspaceID]
	if !ok || ws.OrgID != orgID {
		return nil, repository.ErrWorkspaceNotFound
	}
	count := 0
	for _, p := range f.projects {
		if p.WorkspaceID == workspaceID {
			count++
			if slug != "" && p.Slug == slug {
				return nil, repository.ErrProjectSlugExists
			}
		}
	}
	if max := f.orgsMaxProj[orgID]; max > 0 && count >= max {
		return nil, repository.ErrProjectLimitReached
	}
	if slug == "" {
		slug = strings.ToLower(strings.ReplaceAll(name, " ", "-"))
	}
	id := "proj_" + slug
	p := models.Project{ID: id, WorkspaceID: workspaceID, Name: name, Slug: slug}
	f.projects[id] = p
	return &p, nil
}

func TestWorkspaceHandler_OrgIsolationLimitsAndProjectHierarchy(t *testing.T) {
	store := newFakeWorkspaceStore()
	h := &WorkspaceHandler{repo: store}

	register := func(r *gin.Engine) {
		r.GET("/api/workspaces", h.ListWorkspaces)
		r.POST("/api/workspaces", h.CreateWorkspace)
		r.GET("/api/workspaces/:workspaceId/projects", h.ListProjects)
		r.POST("/api/workspaces/:workspaceId/projects", h.CreateProject)
	}

	agencyTenant := &models.TenantContext{OrgID: "org_agency", Role: "owner"}
	r := asUser("alice", agencyTenant, register)

	// 1. Missing name -> 400
	wBad := doJSON(r, http.MethodPost, "/api/workspaces", `{"name":"   "}`)
	requireStatus(t, wBad, http.StatusBadRequest)

	// 2. Create two isolated client workspaces cleanly
	wC1 := doJSON(r, http.MethodPost, "/api/workspaces", `{"name":"Client Alpha","slug":"client-alpha"}`)
	requireStatus(t, wC1, http.StatusCreated)
	var ws1 models.Workspace
	require.NoError(t, json.Unmarshal(wC1.Body.Bytes(), &ws1))
	assert.Equal(t, "org_agency", ws1.OrgID)

	wC2 := doJSON(r, http.MethodPost, "/api/workspaces", `{"name":"Client Beta","slug":"client-beta"}`)
	requireStatus(t, wC2, http.StatusCreated)

	// 3. Duplicate slug -> 409 conflict
	wDup := doJSON(r, http.MethodPost, "/api/workspaces", `{"name":"Dup","slug":"client-alpha"}`)
	requireStatus(t, wDup, http.StatusConflict)
	assert.Contains(t, wDup.Body.String(), "WORKSPACE_SLUG_EXISTS")

	// 4. Listing excludes foreign organization workspaces
	wList := doJSON(r, http.MethodGet, "/api/workspaces", "")
	requireStatus(t, wList, http.StatusOK)
	assert.Contains(t, wList.Body.String(), "Client Alpha")
	assert.Contains(t, wList.Body.String(), "Client Beta")
	assert.NotContains(t, wList.Body.String(), "Foreign Client")

	// 5. Creating project inside foreign workspace fails closed (404)
	wForeignProj := doJSON(r, http.MethodPost, "/api/workspaces/ws_other/projects", `{"name":"Sneaky"}`)
	requireStatus(t, wForeignProj, http.StatusNotFound)
	assert.Contains(t, wForeignProj.Body.String(), "WORKSPACE_NOT_FOUND")

	// 6. Create project inside Client Alpha & verify max project limit (max=3 -> 4th fails 409)
	for _, pName := range []string{"Support Bot", "Invoice Extractor", "Sales Triage"} {
		wP := doJSON(r, http.MethodPost, "/api/workspaces/"+ws1.ID+"/projects", `{"name":"`+pName+`"}`)
		requireStatus(t, wP, http.StatusCreated)
	}
	wLimit := doJSON(r, http.MethodPost, "/api/workspaces/"+ws1.ID+"/projects", `{"name":"Fourth Bot"}`)
	requireStatus(t, wLimit, http.StatusConflict)
	assert.Contains(t, wLimit.Body.String(), "PROJECT_LIMIT_REACHED")

	wListProj := doJSON(r, http.MethodGet, "/api/workspaces/"+ws1.ID+"/projects", "")
	requireStatus(t, wListProj, http.StatusOK)
	assert.Contains(t, wListProj.Body.String(), "Support Bot")

	// 7. Missing tenant org context fails closed (403)
	rNoTenant := asUser("alice", nil, register)
	wNoOrg := doJSON(rNoTenant, http.MethodGet, "/api/workspaces", "")
	requireStatus(t, wNoOrg, http.StatusForbidden)
}
