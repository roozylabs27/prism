package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeWorkspaceStore struct {
	workspaces map[string]models.Workspace
	projects   map[string]models.Project
}

func newFakeWorkspaceStore() *fakeWorkspaceStore {
	return &fakeWorkspaceStore{
		workspaces: map[string]models.Workspace{
			"ws-1": {ID: "ws-1", OrgID: "org-1", Name: "Default", Slug: "default"},
		},
		projects: map[string]models.Project{
			"proj-1": {ID: "proj-1", WorkspaceID: "ws-1", Name: "Core", Slug: "core"},
		},
	}
}

func (f *fakeWorkspaceStore) ListWorkspacesByOrg(_ context.Context, orgID string) ([]models.Workspace, error) {
	var out []models.Workspace
	for _, ws := range f.workspaces {
		if ws.OrgID == orgID {
			out = append(out, ws)
		}
	}
	return out, nil
}

func (f *fakeWorkspaceStore) CreateWorkspace(_ context.Context, orgID, _, name, slug string) (*models.Workspace, error) {
	ws := models.Workspace{ID: "ws-2", OrgID: orgID, Name: name, Slug: slug}
	f.workspaces[ws.ID] = ws
	return &ws, nil
}

func (f *fakeWorkspaceStore) UpdateWorkspace(_ context.Context, orgID, workspaceID, name, slug string) (*models.Workspace, error) {
	ws, ok := f.workspaces[workspaceID]
	if !ok || ws.OrgID != orgID {
		return nil, repository.ErrWorkspaceNotFound
	}
	ws.Name = name
	if slug != "" {
		ws.Slug = slug
	}
	f.workspaces[workspaceID] = ws
	return &ws, nil
}

func (f *fakeWorkspaceStore) DeleteWorkspace(_ context.Context, orgID, workspaceID string) error {
	ws, ok := f.workspaces[workspaceID]
	if !ok || ws.OrgID != orgID {
		return repository.ErrWorkspaceNotFound
	}
	delete(f.workspaces, workspaceID)
	return nil
}

func (f *fakeWorkspaceStore) ListProjectsByWorkspace(_ context.Context, orgID, workspaceID string) ([]models.Project, error) {
	ws, ok := f.workspaces[workspaceID]
	if !ok || ws.OrgID != orgID {
		return nil, repository.ErrWorkspaceNotFound
	}
	var out []models.Project
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
	p := models.Project{ID: "proj-2", WorkspaceID: workspaceID, Name: name, Slug: slug}
	f.projects[p.ID] = p
	return &p, nil
}

func (f *fakeWorkspaceStore) UpdateProject(_ context.Context, orgID, workspaceID, projectID, name, slug string) (*models.Project, error) {
	ws, ok := f.workspaces[workspaceID]
	if !ok || ws.OrgID != orgID {
		return nil, repository.ErrWorkspaceNotFound
	}
	p, ok := f.projects[projectID]
	if !ok || p.WorkspaceID != workspaceID {
		return nil, repository.ErrProjectNotFound
	}
	p.Name = name
	if slug != "" {
		p.Slug = slug
	}
	f.projects[projectID] = p
	return &p, nil
}

func (f *fakeWorkspaceStore) DeleteProject(_ context.Context, orgID, workspaceID, projectID string) error {
	ws, ok := f.workspaces[workspaceID]
	if !ok || ws.OrgID != orgID {
		return repository.ErrWorkspaceNotFound
	}
	p, ok := f.projects[projectID]
	if !ok || p.WorkspaceID != workspaceID {
		return repository.ErrProjectNotFound
	}
	delete(f.projects, projectID)
	return nil
}

func TestWorkspaceHandler_UpdateAndDeleteWorkspaceAndProject(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := newFakeWorkspaceStore()
	h := &WorkspaceHandler{repo: store}

	// Update workspace
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/workspaces/ws-1", bytes.NewBufferString(`{"name":"Production","slug":"production"}`))
	c.Params = gin.Params{{Key: "workspaceId", Value: "ws-1"}}
	c.Set("organizationId", "org-1")
	h.UpdateWorkspace(c)
	require.Equal(t, http.StatusOK, w.Code)
	var updatedWS models.Workspace
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &updatedWS))
	assert.Equal(t, "Production", updatedWS.Name)
	assert.Equal(t, "production", updatedWS.Slug)

	// Update project
	wProj := httptest.NewRecorder()
	cProj, _ := gin.CreateTestContext(wProj)
	cProj.Request = httptest.NewRequest(http.MethodPut, "/api/workspaces/ws-1/projects/proj-1", bytes.NewBufferString(`{"name":"Agent Core","slug":"agent-core"}`))
	cProj.Params = gin.Params{{Key: "workspaceId", Value: "ws-1"}, {Key: "projectId", Value: "proj-1"}}
	cProj.Set("organizationId", "org-1")
	h.UpdateProject(cProj)
	require.Equal(t, http.StatusOK, wProj.Code)
	var updatedProj models.Project
	require.NoError(t, json.Unmarshal(wProj.Body.Bytes(), &updatedProj))
	assert.Equal(t, "Agent Core", updatedProj.Name)

	// Delete project then workspace
	wDelProj := httptest.NewRecorder()
	cDelProj, _ := gin.CreateTestContext(wDelProj)
	cDelProj.Request = httptest.NewRequest(http.MethodDelete, "/api/workspaces/ws-1/projects/proj-1", nil)
	cDelProj.Params = gin.Params{{Key: "workspaceId", Value: "ws-1"}, {Key: "projectId", Value: "proj-1"}}
	cDelProj.Set("organizationId", "org-1")
	h.DeleteProject(cDelProj)
	require.Equal(t, http.StatusOK, wDelProj.Code)

	wDelWS := httptest.NewRecorder()
	cDelWS, _ := gin.CreateTestContext(wDelWS)
	cDelWS.Request = httptest.NewRequest(http.MethodDelete, "/api/workspaces/ws-1", nil)
	cDelWS.Params = gin.Params{{Key: "workspaceId", Value: "ws-1"}}
	cDelWS.Set("organizationId", "org-1")
	h.DeleteWorkspace(cDelWS)
	require.Equal(t, http.StatusOK, wDelWS.Code)
}
