package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type RoutingDecisionLog struct {
	ID               string          `json:"id" db:"id"`
	RequestID        string          `json:"requestId" db:"request_id"`
	UserID           string          `json:"userId" db:"user_id"`
	OrgID            string          `json:"orgId,omitempty" db:"org_id"`
	PromptPreview    string          `json:"promptPreview,omitempty" db:"prompt_preview"`
	TaskType         string          `json:"taskType,omitempty" db:"task_type"`
	Complexity       string          `json:"complexity,omitempty" db:"complexity"`
	PolicyName       string          `json:"policyName,omitempty" db:"policy_name"`
	Candidates       json.RawMessage `json:"candidates,omitempty" db:"candidates"`
	SelectedModel    string          `json:"selectedModel,omitempty" db:"selected_model"`
	SelectedProvider string          `json:"selectedProvider,omitempty" db:"selected_provider"`
	BudgetStatus     string          `json:"budgetStatus,omitempty" db:"budget_status"`
	EstimatedCost    float64         `json:"estimatedCost" db:"estimated_cost"`
	ActualCost       float64         `json:"actualCost" db:"actual_cost"`
	DowngradeReason  string          `json:"downgradeReason,omitempty" db:"downgrade_reason"`
	ScoresBreakdown  json.RawMessage `json:"scoresBreakdown,omitempty" db:"scores_breakdown"`
	StatusCode       *int            `json:"statusCode,omitempty" db:"status_code"`
	LatencyMs        *int            `json:"latencyMs,omitempty" db:"latency_ms"`
	TTFTMs           *int            `json:"ttftMs,omitempty" db:"ttft_ms"`
	RetryCount       int             `json:"retryCount" db:"retry_count"`
	ErrorMessage     string          `json:"errorMessage,omitempty" db:"error_message"`
	CreatedAt        time.Time       `json:"createdAt" db:"created_at"`
}

type RoutingDecisionRepository struct {
	db *sql.DB
}

func NewRoutingDecisionRepository(db *sql.DB) *RoutingDecisionRepository {
	return &RoutingDecisionRepository{db: db}
}

func (r *RoutingDecisionRepository) Create(ctx context.Context, decision *RoutingDecisionLog) error {
	if decision.ID == "" {
		decision.ID = uuid.New().String()
	}
	if decision.OrgID == "" {
		decision.OrgID = tenantOrgID(ctx)
	}
	if decision.CreatedAt.IsZero() {
		decision.CreatedAt = time.Now()
	}

	var candidates interface{}
	if len(decision.Candidates) > 0 {
		candidates = []byte(decision.Candidates)
	} else {
		candidates = nil
	}

	var scoresBreakdown interface{}
	if len(decision.ScoresBreakdown) > 0 {
		scoresBreakdown = []byte(decision.ScoresBreakdown)
	} else {
		scoresBreakdown = nil
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO routing_decisions (id, request_id, user_id, org_id, prompt_preview, task_type, complexity, policy_name,
		     candidates, selected_model, selected_provider, budget_status, estimated_cost, actual_cost,
		     downgrade_reason, scores_breakdown, status_code, latency_ms, ttft_ms, retry_count, error_message, created_at)
		 VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22)`,
		decision.ID, decision.RequestID, decision.UserID, decision.OrgID,
		sql.NullString{String: decision.PromptPreview, Valid: decision.PromptPreview != ""},
		sql.NullString{String: decision.TaskType, Valid: decision.TaskType != ""},
		sql.NullString{String: decision.Complexity, Valid: decision.Complexity != ""},
		sql.NullString{String: decision.PolicyName, Valid: decision.PolicyName != ""},
		candidates,
		sql.NullString{String: decision.SelectedModel, Valid: decision.SelectedModel != ""},
		sql.NullString{String: decision.SelectedProvider, Valid: decision.SelectedProvider != ""},
		sql.NullString{String: decision.BudgetStatus, Valid: decision.BudgetStatus != ""},
		decision.EstimatedCost, decision.ActualCost,
		sql.NullString{String: decision.DowngradeReason, Valid: decision.DowngradeReason != ""},
		scoresBreakdown,
		nullIntPtr(decision.StatusCode),
		nullIntPtr(decision.LatencyMs),
		nullIntPtr(decision.TTFTMs),
		decision.RetryCount,
		sql.NullString{String: decision.ErrorMessage, Valid: decision.ErrorMessage != ""},
		decision.CreatedAt,
	)
	return err
}

func nullIntPtr(v *int) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*v), Valid: true}
}

func (r *RoutingDecisionRepository) UpdateActualCostByRequestID(ctx context.Context, requestID string, cost float64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE routing_decisions SET actual_cost = $1 WHERE request_id = $2`,
		cost, requestID,
	)
	return err
}

// UpdateOutcomeByRequestID backfills execution status, timing, retries, error details, and final cost
// onto the routing decision created at route selection time.
func (r *RoutingDecisionRepository) UpdateOutcomeByRequestID(
	ctx context.Context,
	requestID string,
	actualCost float64,
	statusCode int,
	latencyMs int,
	ttftMs int,
	retryCount int,
	errMsg string,
) error {
	var ttftVal sql.NullInt64
	if ttftMs > 0 {
		ttftVal = sql.NullInt64{Int64: int64(ttftMs), Valid: true}
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE routing_decisions
		 SET actual_cost = CASE WHEN $1 > 0 THEN $1 ELSE actual_cost END,
		     status_code = $2,
		     latency_ms = $3,
		     ttft_ms = COALESCE($4, ttft_ms),
		     retry_count = $5,
		     error_message = NULLIF($6, '')
		 WHERE request_id = $7`,
		actualCost, statusCode, latencyMs, ttftVal, retryCount, errMsg, requestID,
	)
	return err
}

func (r *RoutingDecisionRepository) ListWithFilter(ctx context.Context, userID string, limit, offset int) ([]RoutingDecisionLog, int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	query := `SELECT id, request_id, user_id, COALESCE(org_id, ''), prompt_preview, task_type, complexity, policy_name,
		        candidates, selected_model, selected_provider, budget_status,
		        estimated_cost, actual_cost, downgrade_reason, scores_breakdown,
		        status_code, latency_ms, ttft_ms, retry_count, error_message, created_at
		 FROM routing_decisions WHERE 1=1`
	countQuery := `SELECT COUNT(*) FROM routing_decisions WHERE 1=1`

	var args []interface{}
	var countArgs []interface{}

	if userID != "" || tenantOrgID(ctx) != "" {
		ownerClause, nextArgs := tenantOwnerClause(ctx, "", userID, args)
		_, countArgs = tenantOwnerClause(ctx, "", userID, countArgs)
		args = nextArgs
		filter := " AND (" + ownerClause + ")"
		query += filter
		countQuery += filter
	}

	var total int64
	if err := r.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query += " ORDER BY created_at DESC"

	args = append(args, limit)
	query += fmt.Sprintf(" LIMIT $%d", len(args))

	args = append(args, offset)
	query += fmt.Sprintf(" OFFSET $%d", len(args))

	hasPromptCol := true
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		// Fallback for VPS before migration 032 columns exist
		hasPromptCol = false
		fallbackQuery := `SELECT id, request_id, user_id, COALESCE(org_id, ''), task_type, complexity, policy_name,
			        candidates, selected_model, selected_provider, budget_status,
			        estimated_cost, actual_cost, downgrade_reason, created_at
			 FROM routing_decisions WHERE 1=1`
		argsFallback := []interface{}{}
		if userID != "" || tenantOrgID(ctx) != "" {
			ownerClause, nextArgs := tenantOwnerClause(ctx, "", userID, argsFallback)
			argsFallback = nextArgs
			fallbackQuery += " AND (" + ownerClause + ")"
		}
		fallbackQuery += " ORDER BY created_at DESC"
		argsFallback = append(argsFallback, limit, offset)
		fallbackQuery += fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(argsFallback)-1, len(argsFallback))

		var errFallback error
		rows, errFallback = r.db.QueryContext(ctx, fallbackQuery, argsFallback...)
		if errFallback != nil {
			return nil, 0, err
		}
	}
	defer func() { _ = rows.Close() }()

	var decisions []RoutingDecisionLog
	for rows.Next() {
		var d RoutingDecisionLog
		var promptPreview, taskType, complexity, policyName, selectedModel, selectedProvider, budgetStatus, downgradeReason, errorMsg sql.NullString
		var scoresBreakdown sql.NullString
		var statusCode, latencyMs, ttftMs sql.NullInt64
		var retryCount sql.NullInt64
		if hasPromptCol {
			if err := rows.Scan(&d.ID, &d.RequestID, &d.UserID, &d.OrgID, &promptPreview, &taskType, &complexity,
				&policyName, &d.Candidates, &selectedModel, &selectedProvider,
				&budgetStatus, &d.EstimatedCost, &d.ActualCost, &downgradeReason, &scoresBreakdown,
				&statusCode, &latencyMs, &ttftMs, &retryCount, &errorMsg, &d.CreatedAt); err != nil {
				return nil, 0, err
			}
			d.PromptPreview = promptPreview.String
			if scoresBreakdown.Valid && scoresBreakdown.String != "" {
				d.ScoresBreakdown = json.RawMessage(scoresBreakdown.String)
			}
			if statusCode.Valid {
				v := int(statusCode.Int64)
				d.StatusCode = &v
			}
			if latencyMs.Valid {
				v := int(latencyMs.Int64)
				d.LatencyMs = &v
			}
			if ttftMs.Valid {
				v := int(ttftMs.Int64)
				d.TTFTMs = &v
			}
			if retryCount.Valid {
				d.RetryCount = int(retryCount.Int64)
			}
			d.ErrorMessage = errorMsg.String
		} else {
			if err := rows.Scan(&d.ID, &d.RequestID, &d.UserID, &d.OrgID, &taskType, &complexity,
				&policyName, &d.Candidates, &selectedModel, &selectedProvider,
				&budgetStatus, &d.EstimatedCost, &d.ActualCost, &downgradeReason, &d.CreatedAt); err != nil {
				return nil, 0, err
			}
		}
		d.TaskType = taskType.String
		d.Complexity = complexity.String
		d.PolicyName = policyName.String
		d.SelectedModel = selectedModel.String
		d.SelectedProvider = selectedProvider.String
		d.BudgetStatus = budgetStatus.String
		d.DowngradeReason = downgradeReason.String
		decisions = append(decisions, d)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	if decisions == nil {
		decisions = []RoutingDecisionLog{}
	}
	return decisions, total, nil
}

func (r *RoutingDecisionRepository) ListByUserID(ctx context.Context, userID string, limit, offset int) ([]RoutingDecisionLog, error) {
	decisions, _, err := r.ListWithFilter(ctx, userID, limit, offset)
	return decisions, err
}
