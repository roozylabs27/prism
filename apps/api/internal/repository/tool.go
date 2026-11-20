package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/roozylabs/prism/internal/models"
)

type ToolRepository struct {
	db *sql.DB
}

func NewToolRepository(db *sql.DB) *ToolRepository {
	return &ToolRepository{db: db}
}

const toolColumns = `id, COALESCE(user_id, ''), COALESCE(org_id, ''), COALESCE(workspace_id, ''), name, display_name, description, input_schema, enabled, require_approval, created_at, updated_at`

func scanTool(row interface{ Scan(...interface{}) error }, t *models.Tool) error {
	return row.Scan(&t.ID, &t.UserID, &t.OrgID, &t.WorkspaceID, &t.Name, &t.DisplayName, &t.Description, &t.InputSchema, &t.Enabled, &t.RequireApproval, &t.CreatedAt, &t.UpdatedAt)
}

func (r *ToolRepository) ListByUserID(ctx context.Context, userID string) ([]models.Tool, error) {
	ownerClause, args := tenantOwnerClause(ctx, "", userID, nil)
	where := ownerClause
	if wsClause, wsArgs := appendOptionalWorkspaceClause(ctx, "", args); wsClause != "" {
		where = fmt.Sprintf("(%s) AND %s", ownerClause, wsClause)
		args = wsArgs
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+toolColumns+` FROM tools WHERE `+where+` ORDER BY created_at DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var tools []models.Tool
	for rows.Next() {
		var t models.Tool
		if err := scanTool(rows, &t); err != nil {
			return nil, err
		}
		tools = append(tools, t)
	}
	return tools, rows.Err()
}

func (r *ToolRepository) FindByID(ctx context.Context, id, userID string) (*models.Tool, error) {
	var t models.Tool
	ownerClause, args := tenantOwnerClause(ctx, "", userID, []any{id})
	err := scanTool(r.db.QueryRowContext(ctx,
		`SELECT `+toolColumns+` FROM tools WHERE id = $1 AND (`+ownerClause+`)`, args...,
	), &t)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *ToolRepository) FindByUserAndName(ctx context.Context, userID, name string) (*models.Tool, error) {
	var t models.Tool
	ownerClause, args := tenantOwnerClause(ctx, "", userID, []any{name})
	err := scanTool(r.db.QueryRowContext(ctx,
		`SELECT `+toolColumns+` FROM tools WHERE name = $1 AND (`+ownerClause+`)`, args...,
	), &t)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *ToolRepository) Create(ctx context.Context, t *models.Tool) error {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	if t.OrgID == "" {
		t.OrgID = tenantOrgID(ctx)
	}
	if t.WorkspaceID == "" {
		t.WorkspaceID = tenantWorkspaceID(ctx)
	}
	now := time.Now()
	t.CreatedAt = now
	t.UpdatedAt = now
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO tools (id, user_id, org_id, workspace_id, name, display_name, description, input_schema, enabled, require_approval, created_at, updated_at)
		 VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), $5, $6, $7, $8, $9, $10, $11, $12)`,
		t.ID, t.UserID, t.OrgID, t.WorkspaceID, t.Name, t.DisplayName, t.Description, []byte(t.InputSchema), t.Enabled, t.RequireApproval, t.CreatedAt, t.UpdatedAt)
	return err
}

func (r *ToolRepository) CreateWithBackendsTx(ctx context.Context, t *models.Tool, backends []models.ToolBackend) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	if t.OrgID == "" {
		t.OrgID = tenantOrgID(ctx)
	}
	if t.WorkspaceID == "" {
		t.WorkspaceID = tenantWorkspaceID(ctx)
	}
	now := time.Now()
	t.CreatedAt = now
	t.UpdatedAt = now
	_, err = tx.ExecContext(ctx,
		`INSERT INTO tools (id, user_id, org_id, workspace_id, name, display_name, description, input_schema, enabled, require_approval, created_at, updated_at)
		 VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), $5, $6, $7, $8, $9, $10, $11, $12)`,
		t.ID, t.UserID, t.OrgID, t.WorkspaceID, t.Name, t.DisplayName, t.Description, []byte(t.InputSchema), t.Enabled, t.RequireApproval, t.CreatedAt, t.UpdatedAt)
	if err != nil {
		return err
	}

	for i := range backends {
		b := &backends[i]
		if b.ID == "" {
			b.ID = uuid.New().String()
		}
		b.ToolID = t.ID
		b.CreatedAt = now
		b.UpdatedAt = now
		if b.BackendType == "" {
			b.BackendType = "http"
		}
		if b.TimeoutMs == 0 {
			b.TimeoutMs = 10000
		}
		_, err = tx.ExecContext(ctx,
			`INSERT INTO tool_backends (id, tool_id, name, backend_type, endpoint_url, auth_token_encrypted, auth_header_name, auth_header_prefix, timeout_ms, priority, enabled, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
			b.ID, b.ToolID, b.Name, b.BackendType, b.EndpointURL, b.AuthTokenEncrypted, b.AuthHeaderName, b.AuthHeaderPrefix, b.TimeoutMs, b.Priority, b.Enabled, b.CreatedAt, b.UpdatedAt)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *ToolRepository) Update(ctx context.Context, t *models.Tool) error {
	t.UpdatedAt = time.Now()
	args := []any{t.DisplayName, t.Description, []byte(t.InputSchema), t.Enabled, t.RequireApproval, t.UpdatedAt, t.ID}
	ownerClause, args := tenantOwnerClause(ctx, "", t.UserID, args)
	_, err := r.db.ExecContext(ctx,
		`UPDATE tools SET display_name=$1, description=$2, input_schema=$3, enabled=$4, require_approval=$5, updated_at=$6
		 WHERE id=$7 AND (`+ownerClause+`)`,
		args...)
	return err
}

func (r *ToolRepository) Delete(ctx context.Context, id, userID string) error {
	ownerClause, args := tenantOwnerClause(ctx, "", userID, []any{id})
	res, err := r.db.ExecContext(ctx, `DELETE FROM tools WHERE id = $1 AND (`+ownerClause+`)`, args...)
	return requireOneRow(res, err)
}

func (r *ToolRepository) GetToolWithBackends(ctx context.Context, userID, toolName string) (*models.ToolWithBackends, error) {
	tool, err := r.FindByUserAndName(ctx, userID, toolName)
	if err != nil {
		return nil, err
	}
	return r.attachBackends(ctx, tool)
}

func (r *ToolRepository) GetToolWithBackendsByID(ctx context.Context, id, userID string) (*models.ToolWithBackends, error) {
	tool, err := r.FindByID(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	return r.attachBackends(ctx, tool)
}

func (r *ToolRepository) attachBackends(ctx context.Context, tool *models.Tool) (*models.ToolWithBackends, error) {
	backendRepo := NewToolBackendRepository(r.db)
	backends, err := backendRepo.ListEnabledByToolID(ctx, tool.ID)
	if err != nil {
		return nil, err
	}
	return &models.ToolWithBackends{Tool: *tool, Backends: backends}, nil
}
