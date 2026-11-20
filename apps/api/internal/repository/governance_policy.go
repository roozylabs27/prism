package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/roozylabs/prism/internal/models"
)

type GovernancePolicyRepository struct {
	db *sql.DB
}

func NewGovernancePolicyRepository(db *sql.DB) *GovernancePolicyRepository {
	return &GovernancePolicyRepository{db: db}
}

const policyColumns = `id, COALESCE(user_id, ''), COALESCE(org_id, ''), COALESCE(workspace_id, ''), name, description, role, effect, agent_pattern, model_pattern, tool_pattern, resource_pattern, priority, enabled, created_at, updated_at`

func scanPolicy(row interface{ Scan(...interface{}) error }, p *models.GovernancePolicy) error {
	return row.Scan(
		&p.ID, &p.UserID, &p.OrgID, &p.WorkspaceID, &p.Name, &p.Description, &p.Role, &p.Effect,
		&p.AgentPattern, &p.ModelPattern, &p.ToolPattern, &p.ResourcePattern,
		&p.Priority, &p.Enabled, &p.CreatedAt, &p.UpdatedAt,
	)
}

func (r *GovernancePolicyRepository) ListByUserID(ctx context.Context, userID string) ([]models.GovernancePolicy, error) {
	ownerClause, args := tenantOwnerClause(ctx, "", userID, nil)
	where := ownerClause
	if wsClause, wsArgs := appendOptionalWorkspaceClause(ctx, "", args); wsClause != "" {
		where = fmt.Sprintf("(%s) AND %s", ownerClause, wsClause)
		args = wsArgs
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+policyColumns+` FROM governance_policies WHERE `+where+` ORDER BY priority ASC, created_at DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var policies []models.GovernancePolicy
	for rows.Next() {
		var p models.GovernancePolicy
		if err := scanPolicy(rows, &p); err != nil {
			return nil, err
		}
		policies = append(policies, p)
	}
	return policies, rows.Err()
}

func (r *GovernancePolicyRepository) FindByID(ctx context.Context, id, userID string) (*models.GovernancePolicy, error) {
	var p models.GovernancePolicy
	ownerClause, args := tenantOwnerClause(ctx, "", userID, []any{id})
	err := scanPolicy(r.db.QueryRowContext(ctx,
		`SELECT `+policyColumns+` FROM governance_policies WHERE id = $1 AND (`+ownerClause+`)`, args...), &p)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *GovernancePolicyRepository) Create(ctx context.Context, p *models.GovernancePolicy) error {
	now := time.Now()
	if p.OrgID == "" {
		p.OrgID = tenantOrgID(ctx)
	}
	if p.WorkspaceID == "" {
		p.WorkspaceID = tenantWorkspaceID(ctx)
	}
	if p.Role == "" {
		p.Role = "developer"
	}
	if p.Effect == "" {
		p.Effect = "allow"
	}
	if p.AgentPattern == "" {
		p.AgentPattern = "*"
	}
	if p.ModelPattern == "" {
		p.ModelPattern = "*"
	}
	if p.ToolPattern == "" {
		p.ToolPattern = "*"
	}
	if p.ResourcePattern == "" {
		p.ResourcePattern = "*"
	}
	if p.Priority <= 0 {
		p.Priority = 100
	}
	return r.db.QueryRowContext(ctx,
		`INSERT INTO governance_policies (user_id, org_id, workspace_id, name, description, role, effect, agent_pattern, model_pattern, tool_pattern, resource_pattern, priority, enabled, created_at, updated_at)
		 VALUES (NULLIF($1, ''), NULLIF($2, ''), NULLIF($3, ''), $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		 RETURNING id, created_at, updated_at`,
		p.UserID, p.OrgID, p.WorkspaceID, p.Name, p.Description, p.Role, p.Effect,
		p.AgentPattern, p.ModelPattern, p.ToolPattern, p.ResourcePattern,
		p.Priority, p.Enabled, now, now,
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
}

func (r *GovernancePolicyRepository) Update(ctx context.Context, p *models.GovernancePolicy) error {
	p.UpdatedAt = time.Now()
	args := []any{
		p.Name, p.Description, p.Role, p.Effect,
		p.AgentPattern, p.ModelPattern, p.ToolPattern, p.ResourcePattern,
		p.Priority, p.Enabled, p.UpdatedAt, p.ID,
	}
	ownerClause, args := tenantOwnerClause(ctx, "", p.UserID, args)
	_, err := r.db.ExecContext(ctx,
		`UPDATE governance_policies SET name=$1, description=$2, role=$3, effect=$4, agent_pattern=$5, model_pattern=$6, tool_pattern=$7, resource_pattern=$8, priority=$9, enabled=$10, updated_at=$11
		 WHERE id = $12 AND (`+ownerClause+`)`,
		args...,
	)
	return err
}

func (r *GovernancePolicyRepository) Delete(ctx context.Context, id, userID string) error {
	ownerClause, args := tenantOwnerClause(ctx, "", userID, []any{id})
	res, err := r.db.ExecContext(ctx, `DELETE FROM governance_policies WHERE id = $1 AND (`+ownerClause+`)`, args...)
	return requireOneRow(res, err)
}
