package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/roozylabs/prism/internal/models"
)

var (
	ErrWorkspaceNotFound     = errors.New("workspace not found")
	ErrWorkspaceLimitReached = errors.New("organization workspace limit reached")
	ErrWorkspaceSlugExists   = errors.New("workspace slug already exists in organization")
	ErrProjectNotFound       = errors.New("project not found")
	ErrProjectLimitReached   = errors.New("workspace project limit reached")
	ErrProjectSlugExists     = errors.New("project slug already exists in workspace")
)

var nonSlugChars = regexp.MustCompile(`[^a-z0-9]+`)

type WorkspaceRepository struct {
	db *sql.DB
}

func NewWorkspaceRepository(db *sql.DB) *WorkspaceRepository {
	return &WorkspaceRepository{db: db}
}

func (r *WorkspaceRepository) ListWorkspacesByOrg(ctx context.Context, orgID string) ([]models.Workspace, error) {
	if orgID == "" {
		return nil, errors.New("organization ID is required")
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, org_id, name, slug, created_at, updated_at
		 FROM workspaces
		 WHERE org_id = $1
		 ORDER BY created_at ASC, name ASC`, orgID,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []models.Workspace
	for rows.Next() {
		var ws models.Workspace
		if err := rows.Scan(&ws.ID, &ws.OrgID, &ws.Name, &ws.Slug, &ws.CreatedAt, &ws.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, ws)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if out == nil {
		out = []models.Workspace{}
	}
	return out, nil
}

func (r *WorkspaceRepository) FindWorkspaceInOrg(ctx context.Context, orgID, workspaceID string) (*models.Workspace, error) {
	if orgID == "" || workspaceID == "" {
		return nil, ErrWorkspaceNotFound
	}
	var ws models.Workspace
	err := r.db.QueryRowContext(ctx,
		`SELECT id, org_id, name, slug, created_at, updated_at
		 FROM workspaces
		 WHERE org_id = $1 AND id = $2`, orgID, workspaceID,
	).Scan(&ws.ID, &ws.OrgID, &ws.Name, &ws.Slug, &ws.CreatedAt, &ws.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrWorkspaceNotFound
		}
		return nil, err
	}
	return &ws, nil
}

func (r *WorkspaceRepository) CreateWorkspace(ctx context.Context, orgID, userID, name, slug string) (*models.Workspace, error) {
	if orgID == "" {
		return nil, errors.New("organization ID is required")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("workspace name is required")
	}

	wsID := "ws_" + strings.ReplaceAll(uuid.New().String(), "-", "")[:16]
	finalSlug := normalizeSlug(slug, name, "workspace", wsID[3:7])

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var maxWorkspaces int
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(max_workspaces, 0) FROM organizations WHERE id = $1`, orgID).Scan(&maxWorkspaces)
	if err != nil {
		return nil, fmt.Errorf("load organization limits: %w", err)
	}

	var currentCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM workspaces WHERE org_id = $1`, orgID).Scan(&currentCount); err != nil {
		return nil, err
	}
	if maxWorkspaces > 0 && currentCount >= maxWorkspaces {
		return nil, ErrWorkspaceLimitReached
	}

	now := time.Now().UTC()
	ws := &models.Workspace{
		ID:        wsID,
		OrgID:     orgID,
		Name:      name,
		Slug:      finalSlug,
		CreatedAt: now,
		UpdatedAt: now,
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO workspaces (id, org_id, name, slug, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		ws.ID, ws.OrgID, ws.Name, ws.Slug, ws.CreatedAt, ws.UpdatedAt,
	)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return nil, ErrWorkspaceSlugExists
		}
		return nil, err
	}

	if userID != "" {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO workspace_members (id, workspace_id, user_id, role, created_at, updated_at)
			 VALUES ($1, $2, $3, 'admin', $4, $5)
			 ON CONFLICT (workspace_id, user_id) DO NOTHING`,
			uuid.New().String(), ws.ID, userID, now, now,
		)
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return ws, nil
}

func (r *WorkspaceRepository) ListProjectsByWorkspace(ctx context.Context, orgID, workspaceID string) ([]models.Project, error) {
	if _, err := r.FindWorkspaceInOrg(ctx, orgID, workspaceID); err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT p.id, p.workspace_id, p.name, p.slug, p.created_at, p.updated_at
		 FROM projects p
		 INNER JOIN workspaces w ON w.id = p.workspace_id
		 WHERE w.org_id = $1 AND p.workspace_id = $2
		 ORDER BY p.created_at ASC, p.name ASC`, orgID, workspaceID,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []models.Project
	for rows.Next() {
		var p models.Project
		if err := rows.Scan(&p.ID, &p.WorkspaceID, &p.Name, &p.Slug, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if out == nil {
		out = []models.Project{}
	}
	return out, nil
}

func (r *WorkspaceRepository) FindProjectInOrg(ctx context.Context, orgID, projectID string) (*models.Project, error) {
	if orgID == "" || projectID == "" {
		return nil, ErrProjectNotFound
	}
	var p models.Project
	err := r.db.QueryRowContext(ctx,
		`SELECT p.id, p.workspace_id, p.name, p.slug, p.created_at, p.updated_at
		 FROM projects p
		 INNER JOIN workspaces w ON w.id = p.workspace_id
		 WHERE w.org_id = $1 AND p.id = $2`, orgID, projectID,
	).Scan(&p.ID, &p.WorkspaceID, &p.Name, &p.Slug, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrProjectNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (r *WorkspaceRepository) CreateProject(ctx context.Context, orgID, workspaceID, name, slug string) (*models.Project, error) {
	if orgID == "" || workspaceID == "" {
		return nil, ErrWorkspaceNotFound
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("project name is required")
	}

	projID := "proj_" + strings.ReplaceAll(uuid.New().String(), "-", "")[:16]
	finalSlug := normalizeSlug(slug, name, "project", projID[5:9])

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var resolvedWS string
	var maxProjects int
	err = tx.QueryRowContext(ctx,
		`SELECT w.id, COALESCE(o.max_projects_per_workspace, 0)
		 FROM workspaces w
		 INNER JOIN organizations o ON o.id = w.org_id
		 WHERE w.org_id = $1 AND w.id = $2`, orgID, workspaceID,
	).Scan(&resolvedWS, &maxProjects)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrWorkspaceNotFound
		}
		return nil, err
	}

	var currentCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM projects WHERE workspace_id = $1`, workspaceID).Scan(&currentCount); err != nil {
		return nil, err
	}
	if maxProjects > 0 && currentCount >= maxProjects {
		return nil, ErrProjectLimitReached
	}

	now := time.Now().UTC()
	p := &models.Project{
		ID:          projID,
		WorkspaceID: workspaceID,
		Name:        name,
		Slug:        finalSlug,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO projects (id, workspace_id, name, slug, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		p.ID, p.WorkspaceID, p.Name, p.Slug, p.CreatedAt, p.UpdatedAt,
	)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return nil, ErrProjectSlugExists
		}
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return p, nil
}

func normalizeSlug(explicitSlug, fallbackName, defaultPrefix, suffix string) string {
	trimmedExplicit := strings.TrimSpace(explicitSlug)
	source := trimmedExplicit
	if source == "" {
		source = fallbackName
	}
	s := nonSlugChars.ReplaceAllString(strings.ToLower(source), "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = defaultPrefix
	}
	if trimmedExplicit == "" && suffix != "" {
		s = s + "-" + suffix
	}
	return s
}
