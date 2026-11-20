package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/roozylabs/prism/internal/models"
)

var (
	ErrApprovalNotFound       = errors.New("approval request not found")
	ErrApprovalNotDecidable   = errors.New("approval request is no longer pending or has expired")
	ErrApprovalInvalidConsume = errors.New("approval request is not approved, expired, already consumed, or arguments do not match")
)

type ApprovalRepository struct {
	db *sql.DB
}

func NewApprovalRepository(db *sql.DB) *ApprovalRepository {
	return &ApprovalRepository{db: db}
}

const approvalColumns = `id, COALESCE(org_id, ''), user_id, COALESCE(agent_id, ''), agent_name, tool_name, mcp_server_name,
	arguments_json, arguments_hash, policy_id, policy_name, reason, status,
	COALESCE(decided_by, ''), COALESCE(decision_note, ''), decided_at, expires_at, consumed_at, created_at, updated_at`

func scanApproval(row interface{ Scan(...interface{}) error }, a *models.ApprovalRequest) error {
	var argsRaw []byte
	var decidedAt, consumedAt sql.NullTime
	err := row.Scan(
		&a.ID, &a.OrgID, &a.UserID, &a.AgentID, &a.AgentName, &a.ToolName, &a.MCPServerName,
		&argsRaw, &a.ArgumentsHash, &a.PolicyID, &a.PolicyName, &a.Reason, &a.Status,
		&a.DecidedBy, &a.DecisionNote, &decidedAt, &a.ExpiresAt, &consumedAt, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return err
	}
	if len(argsRaw) == 0 {
		a.ArgumentsJSON = json.RawMessage(`{}`)
	} else {
		a.ArgumentsJSON = json.RawMessage(argsRaw)
	}
	if decidedAt.Valid {
		t := decidedAt.Time
		a.DecidedAt = &t
	}
	if consumedAt.Valid {
		t := consumedAt.Time
		a.ConsumedAt = &t
	}
	return nil
}

// HashToolArguments computes a deterministic SHA-256 hex digest over canonicalized JSON arguments.
func HashToolArguments(args map[string]interface{}) (json.RawMessage, string) {
	if args == nil {
		args = map[string]interface{}{}
	}
	raw, err := json.Marshal(args)
	if err != nil || len(raw) == 0 {
		raw = []byte(`{}`)
	}
	sum := sha256.Sum256(raw)
	return json.RawMessage(raw), hex.EncodeToString(sum[:])
}

func (r *ApprovalRepository) Create(ctx context.Context, req *models.ApprovalRequest) error {
	if req.ID == "" {
		req.ID = uuid.New().String()
	}
	if req.OrgID == "" {
		req.OrgID = tenantOrgID(ctx)
	}
	if req.Status == "" {
		req.Status = models.ApprovalStatusPending
	}
	if len(req.ArgumentsJSON) == 0 {
		req.ArgumentsJSON = json.RawMessage(`{}`)
	}
	if req.ArgumentsHash == "" {
		sum := sha256.Sum256(req.ArgumentsJSON)
		req.ArgumentsHash = hex.EncodeToString(sum[:])
	}
	now := time.Now()
	if req.CreatedAt.IsZero() {
		req.CreatedAt = now
	}
	req.UpdatedAt = now
	if req.ExpiresAt.IsZero() {
		req.ExpiresAt = now.Add(24 * time.Hour)
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO approval_requests (
			id, org_id, user_id, agent_id, agent_name, tool_name, mcp_server_name,
			arguments_json, arguments_hash, policy_id, policy_name, reason, status,
			expires_at, created_at, updated_at
		) VALUES (
			$1, NULLIF($2, ''), $3, NULLIF($4, ''), $5, $6, $7,
			$8, $9, $10, $11, $12, $13,
			$14, $15, $16
		)`,
		req.ID, req.OrgID, req.UserID, req.AgentID, req.AgentName, req.ToolName, req.MCPServerName,
		[]byte(req.ArgumentsJSON), req.ArgumentsHash, req.PolicyID, req.PolicyName, req.Reason, req.Status,
		req.ExpiresAt, req.CreatedAt, req.UpdatedAt,
	)
	return err
}

func (r *ApprovalRepository) FindByID(ctx context.Context, id, userID string) (*models.ApprovalRequest, error) {
	ownerClause, args := tenantOwnerClause(ctx, "", userID, []any{id})
	var out models.ApprovalRequest
	err := scanApproval(r.db.QueryRowContext(ctx,
		`SELECT `+approvalColumns+` FROM approval_requests WHERE id = $1 AND (`+ownerClause+`)`,
		args...,
	), &out)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrApprovalNotFound
		}
		return nil, err
	}
	return &out, nil
}

func (r *ApprovalRepository) List(ctx context.Context, userID, status string, limit, offset int) ([]models.ApprovalRequest, int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	// Mark overdue pending items as expired inside the current tenant boundary.
	expClause, expArgs := tenantOwnerClause(ctx, "", userID, nil)
	_, _ = r.db.ExecContext(ctx,
		`UPDATE approval_requests SET status = 'expired', updated_at = NOW()
		 WHERE status = 'pending' AND expires_at <= NOW() AND (`+expClause+`)`,
		expArgs...,
	)

	ownerClause, args := tenantOwnerClause(ctx, "", userID, nil)
	where := "(" + ownerClause + ")"
	if status != "" && status != "all" {
		args = append(args, status)
		where += fmt.Sprintf(" AND status = $%d", len(args))
	}

	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM approval_requests WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	argsWithPage := append(args, limit, offset)
	query := fmt.Sprintf(
		`SELECT `+approvalColumns+` FROM approval_requests WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		where, len(argsWithPage)-1, len(argsWithPage),
	)
	rows, err := r.db.QueryContext(ctx, query, argsWithPage...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	items := []models.ApprovalRequest{}
	for rows.Next() {
		var a models.ApprovalRequest
		if err := scanApproval(rows, &a); err != nil {
			return nil, 0, err
		}
		items = append(items, a)
	}
	return items, total, rows.Err()
}

func (r *ApprovalRepository) Decide(ctx context.Context, id, userID, actorID, decisionStatus, note string) (*models.ApprovalRequest, error) {
	args := []any{decisionStatus, actorID, note, id}
	ownerClause, args := tenantOwnerClause(ctx, "", userID, args)
	var out models.ApprovalRequest
	err := scanApproval(r.db.QueryRowContext(ctx,
		`UPDATE approval_requests
		 SET status = $1,
		     decided_by = $2,
		     decision_note = $3,
		     decided_at = NOW(),
		     updated_at = NOW()
		 WHERE id = $4
		   AND (`+ownerClause+`)
		   AND status = 'pending'
		   AND expires_at > NOW()
		 RETURNING `+approvalColumns,
		args...,
	), &out)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrApprovalNotDecidable
		}
		return nil, err
	}
	return &out, nil
}

// VerifyAndConsume atomically transitions an approved, non-expired approval request whose
// tool name, MCP server name, and arguments SHA-256 hash match to status "consumed".
func (r *ApprovalRepository) VerifyAndConsume(ctx context.Context, id, userID, toolName, mcpServerName, argumentsHash string) (*models.ApprovalRequest, error) {
	args := []any{id, toolName, mcpServerName, argumentsHash}
	ownerClause, args := tenantOwnerClause(ctx, "", userID, args)
	var out models.ApprovalRequest
	err := scanApproval(r.db.QueryRowContext(ctx,
		`UPDATE approval_requests
		 SET status = 'consumed',
		     consumed_at = NOW(),
		     updated_at = NOW()
		 WHERE id = $1
		   AND (`+ownerClause+`)
		   AND tool_name = $2
		   AND mcp_server_name = $3
		   AND arguments_hash = $4
		   AND status = 'approved'
		   AND expires_at > NOW()
		 RETURNING `+approvalColumns,
		args...,
	), &out)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrApprovalInvalidConsume
		}
		return nil, err
	}
	return &out, nil
}
