package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/utils"
)

type RequestLogRepository struct {
	db *sql.DB
}

func NewRequestLogRepository(db *sql.DB) *RequestLogRepository {
	return &RequestLogRepository{db: db}
}

func (r *RequestLogRepository) Create(ctx context.Context, log *models.RequestLog) error {
	if log.ID == "" {
		log.ID = uuid.New().String()
	}
	if log.RequestID == "" {
		log.RequestID = fmt.Sprintf("req_%d", time.Now().UnixNano())
	}
	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now()
	}
	attemptsJSON := string(log.Attempts)
	if attemptsJSON == "" || attemptsJSON == "null" {
		attemptsJSON = "[]"
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO request_logs (id, request_id, org_id, workspace_id, project_id, agent_id, gateway_api_key_id, provider_id, credential_id, model,
		                          status_code, latency_ms, input_tokens, output_tokens, total_tokens,
		                          cost_usd, error_message, retry_count, client_ip, user_agent, client_app,
		                          is_stream, ttft_ms, response_hash, response_bytes, attempts, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26::jsonb, $27)`,
		log.ID, log.RequestID, log.OrgID, log.WorkspaceID, log.ProjectID, log.AgentID, log.GatewayAPIKeyID, log.ProviderID, log.CredentialID, log.Model,
		log.StatusCode, log.LatencyMs, log.InputTokens, log.OutputTokens, log.TotalTokens,
		log.CostUSD, log.ErrorMessage, log.RetryCount, log.ClientIP, log.UserAgent, log.ClientApp,
		log.IsStream, log.TTFTMs, log.ResponseHash, log.ResponseBytes, attemptsJSON, log.CreatedAt,
	)
	return err
}

func requestLogOwnerClause(ctx context.Context, userID string, args []any) (string, []any) {
	orgID := tenantOrgID(ctx)
	if orgID != "" {
		if userID == "" {
			userID = tenantUserID(ctx)
		}
		if userID != "" {
			args = append(args, orgID, userID)
			oIdx, uIdx := len(args)-1, len(args)
			return fmt.Sprintf("(rl.org_id = $%d OR ((rl.org_id IS NULL OR rl.org_id = '') AND (gak.org_id = $%d OR ((gak.org_id IS NULL OR gak.org_id = '') AND gak.user_id = $%d))))", oIdx, oIdx, uIdx), args
		}
		args = append(args, orgID)
		oIdx := len(args)
		return fmt.Sprintf("(rl.org_id = $%d OR ((rl.org_id IS NULL OR rl.org_id = '') AND gak.org_id = $%d))", oIdx, oIdx), args
	}
	if userID != "" {
		args = append(args, userID)
		return fmt.Sprintf("gak.user_id = $%d", len(args)), args
	}
	return "", args
}

func (r *RequestLogRepository) ListByUserID(ctx context.Context, userID string, limit, offset int) ([]models.RequestLog, error) {
	whereClause, args := requestLogOwnerClause(ctx, userID, nil)
	if whereClause == "" {
		whereClause = "1=1"
	}
	args = append(args, limit, offset)
	limitIdx, offsetIdx := len(args)-1, len(args)
	rows, err := r.db.QueryContext(ctx,
		`SELECT rl.id, rl.request_id, rl.gateway_api_key_id, rl.provider_id, rl.credential_id, rl.model,
		        rl.status_code, rl.latency_ms, rl.input_tokens, rl.output_tokens, rl.total_tokens,
		        COALESCE(rl.cost_usd, 0), rl.error_message, rl.retry_count, COALESCE(rl.client_ip, ''), COALESCE(rl.user_agent, ''),
		        COALESCE(rl.client_app, ''), COALESCE(rl.is_stream, false), COALESCE(rl.ttft_ms, 0), rl.created_at
		 FROM request_logs rl
		 LEFT JOIN gateway_api_keys gak ON rl.gateway_api_key_id = gak.id
		 WHERE `+whereClause+fmt.Sprintf(`
		 ORDER BY rl.created_at DESC
		 LIMIT $%d OFFSET $%d`, limitIdx, offsetIdx), args...,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var logs []models.RequestLog
	for rows.Next() {
		var l models.RequestLog
		if err := rows.Scan(&l.ID, &l.RequestID, &l.GatewayAPIKeyID, &l.ProviderID, &l.CredentialID,
			&l.Model, &l.StatusCode, &l.LatencyMs, &l.InputTokens, &l.OutputTokens,
			&l.TotalTokens, &l.CostUSD, &l.ErrorMessage, &l.RetryCount, &l.ClientIP, &l.UserAgent,
			&l.ClientApp, &l.IsStream, &l.TTFTMs, &l.CreatedAt); err != nil {
			return nil, err
		}
		l.EstimatedCost = l.CostUSD
		logs = append(logs, l)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return logs, nil
}

type LogFilter struct {
	UserID   string
	Provider string
	Model    string
	Status   int
	Search   string
	AgentID  string
	Limit    int
	Offset   int
}

func (r *RequestLogRepository) ListWithFilter(ctx context.Context, f LogFilter) ([]models.RequestLog, int, error) {
	where := []string{}
	args := []interface{}{}
	argIdx := 1

	if f.UserID != "" || tenantOrgID(ctx) != "" {
		clause, nextArgs := requestLogOwnerClause(ctx, f.UserID, args)
		if clause != "" {
			where = append(where, clause)
			args = nextArgs
			argIdx = len(args) + 1
		}
	}
	if f.Provider != "" {
		where = append(where, fmt.Sprintf("p.slug = $%d", argIdx))
		args = append(args, f.Provider)
		argIdx++
	}
	if f.Model != "" {
		where = append(where, fmt.Sprintf("rl.model = $%d", argIdx))
		args = append(args, f.Model)
		argIdx++
	}
	if f.Status > 0 {
		where = append(where, fmt.Sprintf("rl.status_code = $%d", argIdx))
		args = append(args, f.Status)
		argIdx++
	}
	if f.AgentID != "" {
		where = append(where, fmt.Sprintf("rl.agent_id = $%d", argIdx))
		args = append(args, f.AgentID)
		argIdx++
	}
	if f.Search != "" {
		where = append(where, fmt.Sprintf("(rl.model ILIKE $%d OR rl.error_message ILIKE $%d)", argIdx, argIdx))
		args = append(args, "%"+f.Search+"%")
		argIdx++
	}

	whereClause := "1=1"
	if len(where) > 0 {
		whereClause = strings.Join(where, " AND ")
	}

	countQuery := fmt.Sprintf(
		`SELECT COUNT(*) FROM request_logs rl
		 LEFT JOIN gateway_api_keys gak ON rl.gateway_api_key_id = gak.id
		 LEFT JOIN providers p ON rl.provider_id = p.id
		 WHERE %s`, whereClause)

	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := f.Offset

	query := fmt.Sprintf(
		`SELECT rl.id, rl.request_id, rl.gateway_api_key_id, rl.provider_id, rl.credential_id,
		        COALESCE(c.name, '') as credential_name, rl.model,
		        rl.status_code, rl.latency_ms, rl.input_tokens, rl.output_tokens, rl.total_tokens,
		        COALESCE(rl.cost_usd, 0), rl.error_message, rl.retry_count, COALESCE(rl.client_ip, ''), COALESCE(rl.user_agent, ''),
		        COALESCE(rl.client_app, ''), COALESCE(rl.is_stream, false), COALESCE(rl.ttft_ms, 0), rl.created_at
		 FROM request_logs rl
		 LEFT JOIN gateway_api_keys gak ON rl.gateway_api_key_id = gak.id
		 LEFT JOIN providers p ON rl.provider_id = p.id
		 LEFT JOIN credentials c ON rl.credential_id = c.id
		 WHERE %s
		 ORDER BY rl.created_at DESC
		 LIMIT $%d OFFSET $%d`, whereClause, argIdx, argIdx+1)

	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	var logs []models.RequestLog
	for rows.Next() {
		var l models.RequestLog
		var rawCredName string
		if err := rows.Scan(&l.ID, &l.RequestID, &l.GatewayAPIKeyID, &l.ProviderID, &l.CredentialID,
			&rawCredName, &l.Model, &l.StatusCode, &l.LatencyMs, &l.InputTokens, &l.OutputTokens,
			&l.TotalTokens, &l.CostUSD, &l.ErrorMessage, &l.RetryCount, &l.ClientIP, &l.UserAgent,
			&l.ClientApp, &l.IsStream, &l.TTFTMs, &l.CreatedAt); err != nil {
			return nil, 0, err
		}
		if rawCredName != "" {
			l.CredentialName = utils.MaskEmailName(rawCredName)
		}
		l.EstimatedCost = l.CostUSD
		logs = append(logs, l)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}

type DashboardStats struct {
	TotalRequests        int64   `json:"totalRequests"`
	TotalTokens          int64   `json:"totalTokens"`
	InputTokens          int64   `json:"inputTokens"`
	OutputTokens         int64   `json:"outputTokens"`
	TotalEstimatedCost   float64 `json:"totalEstimatedCost"`
	AvgLatency           float64 `json:"avgLatency"`
	ErrorRate            float64 `json:"errorRate"`
	FailoverRecoveryRate float64 `json:"failoverRecoveryRate"`
	ActiveProviders      int     `json:"activeProviders"`
	ActiveCredentials    int     `json:"activeCredentials"`
	ActiveKeys           int     `json:"activeKeys"`
}

type UsagePoint struct {
	Date          string  `json:"date"`
	Model         string  `json:"model"`
	Requests      int64   `json:"requests"`
	Tokens        int64   `json:"tokens"`
	EstimatedCost float64 `json:"estimatedCost"`
}

func (r *RequestLogRepository) GetStats(ctx context.Context, userID string, days int, startDate, endDate string) (*DashboardStats, error) {
	s := &DashboardStats{}

	var query string
	ownerClause, args := requestLogOwnerClause(ctx, userID, nil)
	if ownerClause == "" {
		ownerClause = "1=1"
	}

	if startDate != "" && endDate != "" {
		args = append(args, startDate, endDate)
		startIdx, endIdx := len(args)-1, len(args)
		query = fmt.Sprintf(`SELECT COALESCE(COUNT(*), 0),
		        COALESCE(SUM(total_tokens), 0),
		        COALESCE(SUM(input_tokens), 0),
		        COALESCE(SUM(output_tokens), 0),
		        COALESCE(SUM(cost_usd), 0),
		        COALESCE(AVG(latency_ms), 0),
		        COALESCE(CASE WHEN COUNT(*) > 0 THEN (COUNT(*) FILTER (WHERE status_code >= 400))::float / COUNT(*) * 100 ELSE 0 END, 0),
		        COALESCE(
		          CASE WHEN COUNT(*) FILTER (WHERE retry_count > 0) > 0
		            THEN (COUNT(*) FILTER (WHERE retry_count > 0 AND status_code < 400))::float /
		                 COUNT(*) FILTER (WHERE retry_count > 0) * 100
		            ELSE 100
		          END, 100
		        )
		 FROM request_logs rl
		 LEFT JOIN gateway_api_keys gak ON rl.gateway_api_key_id = gak.id
		 WHERE %s
		   AND rl.created_at >= $%d::DATE
		   AND rl.created_at < ($%d::DATE + INTERVAL '1 day')`, ownerClause, startIdx, endIdx)
	} else if days > 0 {
		args = append(args, days)
		daysIdx := len(args)
		query = fmt.Sprintf(`SELECT COALESCE(COUNT(*), 0),
		        COALESCE(SUM(total_tokens), 0),
		        COALESCE(SUM(input_tokens), 0),
		        COALESCE(SUM(output_tokens), 0),
		        COALESCE(SUM(cost_usd), 0),
		        COALESCE(AVG(latency_ms), 0),
		        COALESCE(CASE WHEN COUNT(*) > 0 THEN (COUNT(*) FILTER (WHERE status_code >= 400))::float / COUNT(*) * 100 ELSE 0 END, 0),
		        COALESCE(
		          CASE WHEN COUNT(*) FILTER (WHERE retry_count > 0) > 0
		            THEN (COUNT(*) FILTER (WHERE retry_count > 0 AND status_code < 400))::float /
		                 COUNT(*) FILTER (WHERE retry_count > 0) * 100
		            ELSE 100
		          END, 100
		        )
		 FROM request_logs rl
		 LEFT JOIN gateway_api_keys gak ON rl.gateway_api_key_id = gak.id
		 WHERE %s
		   AND rl.created_at >= NOW() - ($%d || ' days')::INTERVAL`, ownerClause, daysIdx)
	} else {
		query = fmt.Sprintf(`SELECT COALESCE(COUNT(*), 0),
		        COALESCE(SUM(total_tokens), 0),
		        COALESCE(SUM(input_tokens), 0),
		        COALESCE(SUM(output_tokens), 0),
		        COALESCE(SUM(cost_usd), 0),
		        COALESCE(AVG(latency_ms), 0),
		        COALESCE(CASE WHEN COUNT(*) > 0 THEN (COUNT(*) FILTER (WHERE status_code >= 400))::float / COUNT(*) * 100 ELSE 0 END, 0),
		        COALESCE(
		          CASE WHEN COUNT(*) FILTER (WHERE retry_count > 0) > 0
		            THEN (COUNT(*) FILTER (WHERE retry_count > 0 AND status_code < 400))::float /
		                 COUNT(*) FILTER (WHERE retry_count > 0) * 100
		            ELSE 100
		          END, 100
		        )
		 FROM request_logs rl
		 LEFT JOIN gateway_api_keys gak ON rl.gateway_api_key_id = gak.id
		 WHERE %s`, ownerClause)
	}

	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&s.TotalRequests, &s.TotalTokens, &s.InputTokens, &s.OutputTokens,
		&s.TotalEstimatedCost, &s.AvgLatency, &s.ErrorRate, &s.FailoverRecoveryRate,
	)
	if err != nil {
		return nil, err
	}

	provClause, provArgs := tenantOwnerClause(ctx, "", userID, nil)
	err = r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM providers WHERE (`+provClause+`) AND enabled = true`, provArgs...,
	).Scan(&s.ActiveProviders)
	if err != nil {
		return nil, err
	}

	credClause, credArgs := tenantOwnerClause(ctx, "c.", userID, nil)
	err = r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM credentials c
		 WHERE (`+credClause+`) AND c.enabled = true AND c.status = 'active'`, credArgs...,
	).Scan(&s.ActiveCredentials)
	if err != nil {
		return nil, err
	}

	keyClause, keyArgs := tenantOwnerClause(ctx, "", userID, nil)
	err = r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM gateway_api_keys WHERE (`+keyClause+`) AND enabled = true`, keyArgs...,
	).Scan(&s.ActiveKeys)
	if err != nil {
		return nil, err
	}

	return s, nil
}

func (r *RequestLogRepository) GetUsageChart(ctx context.Context, userID string, days int, startDate, endDate string) ([]UsagePoint, error) {
	var query string
	ownerClause, args := requestLogOwnerClause(ctx, userID, nil)
	if ownerClause == "" {
		ownerClause = "1=1"
	}

	bucketFmt := "YYYY-MM-DD"
	if days == 1 || (startDate != "" && startDate == endDate) {
		bucketFmt = "YYYY-MM-DD HH24:00"
	}

	if startDate != "" && endDate != "" {
		args = append(args, startDate, endDate)
		startIdx, endIdx := len(args)-1, len(args)
		query = fmt.Sprintf(`SELECT TO_CHAR(rl.created_at, '%s') as date,
		        COALESCE(NULLIF(rl.model, ''), 'default') as model,
		        COUNT(*) as requests,
		        COALESCE(SUM(total_tokens), 0) as tokens,
		        COALESCE(SUM(cost_usd), 0) as cost
		 FROM request_logs rl
		 LEFT JOIN gateway_api_keys gak ON rl.gateway_api_key_id = gak.id
		 WHERE %s
		   AND rl.created_at >= $%d::DATE
		   AND rl.created_at < ($%d::DATE + INTERVAL '1 day')
		 GROUP BY TO_CHAR(rl.created_at, '%s'), COALESCE(NULLIF(rl.model, ''), 'default')
		 ORDER BY date ASC, model ASC`, bucketFmt, ownerClause, startIdx, endIdx, bucketFmt)
	} else if days > 0 {
		if days > 365 {
			days = 365
		}
		args = append(args, days)
		daysIdx := len(args)
		query = fmt.Sprintf(`SELECT TO_CHAR(rl.created_at, '%s') as date,
		        COALESCE(NULLIF(rl.model, ''), 'default') as model,
		        COUNT(*) as requests,
		        COALESCE(SUM(total_tokens), 0) as tokens,
		        COALESCE(SUM(cost_usd), 0) as cost
		 FROM request_logs rl
		 LEFT JOIN gateway_api_keys gak ON rl.gateway_api_key_id = gak.id
		 WHERE %s
		   AND rl.created_at >= NOW() - ($%d || ' days')::INTERVAL
		 GROUP BY TO_CHAR(rl.created_at, '%s'), COALESCE(NULLIF(rl.model, ''), 'default')
		 ORDER BY date ASC, model ASC`, bucketFmt, ownerClause, daysIdx, bucketFmt)
	} else {
		query = fmt.Sprintf(`SELECT TO_CHAR(rl.created_at, '%s') as date,
		        COALESCE(NULLIF(rl.model, ''), 'default') as model,
		        COUNT(*) as requests,
		        COALESCE(SUM(total_tokens), 0) as tokens,
		        COALESCE(SUM(cost_usd), 0) as cost
		 FROM request_logs rl
		 LEFT JOIN gateway_api_keys gak ON rl.gateway_api_key_id = gak.id
		 WHERE %s
		 GROUP BY TO_CHAR(rl.created_at, '%s'), COALESCE(NULLIF(rl.model, ''), 'default')
		 ORDER BY date ASC, model ASC`, bucketFmt, ownerClause, bucketFmt)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var points []UsagePoint
	for rows.Next() {
		var p UsagePoint
		if err := rows.Scan(&p.Date, &p.Model, &p.Requests, &p.Tokens, &p.EstimatedCost); err != nil {
			return nil, err
		}
		points = append(points, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return points, nil
}

type ProviderHealth struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	Status      string  `json:"status"`
	CredCount   int     `json:"credCount"`
	HealthScore float64 `json:"health_score"`
}

func (r *RequestLogRepository) GetProviderHealth(ctx context.Context, userID string) ([]ProviderHealth, error) {
	var query string
	var args []interface{}

	if userID != "" || tenantOrgID(ctx) != "" {
		credClause, nextArgs := tenantOwnerClause(ctx, "c.", userID, nil)
		provClause, allArgs := tenantOwnerClause(ctx, "p.", userID, nextArgs)
		query = fmt.Sprintf(`SELECT p.id, p.name, p.type, p.enabled,
		        (SELECT COUNT(*) FROM credentials c WHERE c.provider_id = p.id AND c.enabled = true AND c.status = 'active' AND (%s)) as cred_count
		 FROM providers p
		 WHERE (%s) OR (p.user_id IS NULL AND (p.org_id IS NULL OR p.org_id = ''))
		 ORDER BY p.name`, credClause, provClause)
		args = allArgs
	} else {
		query = `SELECT p.id, p.name, p.type, p.enabled,
		        (SELECT COUNT(*) FROM credentials c WHERE c.provider_id = p.id AND c.enabled = true AND c.status = 'active') as cred_count
		 FROM providers p
		 WHERE p.user_id IS NULL AND (p.org_id IS NULL OR p.org_id = '')
		 ORDER BY p.name`
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var providers []ProviderHealth
	for rows.Next() {
		var ph ProviderHealth
		var enabled bool
		if err := rows.Scan(&ph.ID, &ph.Name, &ph.Type, &enabled, &ph.CredCount); err != nil {
			return nil, err
		}
		if !enabled {
			ph.Status = "down"
		} else if ph.CredCount == 0 {
			ph.Status = "degraded"
		} else {
			ph.Status = "healthy"
		}
		providers = append(providers, ph)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return providers, nil
}

type ClientAppStat struct {
	ClientApp string  `json:"clientApp"`
	Requests  int64   `json:"requests"`
	Tokens    int64   `json:"tokens"`
	CostUSD   float64 `json:"costUsd"`
}

type ModelStat struct {
	Model        string  `json:"model"`
	Requests     int64   `json:"requests"`
	Tokens       int64   `json:"tokens"`
	CostUSD      float64 `json:"costUsd"`
	AvgTTFTMs    float64 `json:"avgTtftMs"`
	AvgLatencyMs float64 `json:"avgLatencyMs"`
}

type LogAnalytics struct {
	TotalSpendUSD       float64         `json:"totalSpendUsd"`
	EstimatedSavingsUSD float64         `json:"estimatedSavingsUsd"`
	AvgTTFTMs           float64         `json:"avgTtftMs"`
	AvgLatencyMs        float64         `json:"avgLatencyMs"`
	ClientApps          []ClientAppStat `json:"clientApps"`
	Models              []ModelStat     `json:"models"`
}

func (r *RequestLogRepository) GetLogAnalytics(ctx context.Context, userID string, days int) (*LogAnalytics, error) {
	if days < 0 || days > 365 {
		days = 30
	}

	analytics := &LogAnalytics{
		ClientApps: []ClientAppStat{},
		Models:     []ModelStat{},
	}

	ownerClause, args := requestLogOwnerClause(ctx, userID, nil)
	if ownerClause == "" {
		ownerClause = "1=1"
	}
	dateClause := ""
	if days > 0 {
		args = append(args, days)
		daysIdx := len(args)
		dateClause = fmt.Sprintf(" AND rl.created_at >= NOW() - ($%d || ' days')::INTERVAL", daysIdx)
	}

	// 1. Overall Summary & Estimated Savings
	// Baseline flagship cost: $3.00/1M input ($0.000003), $15.00/1M output ($0.000015)
	err := r.db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT COALESCE(SUM(cost_usd), 0),
		        COALESCE(AVG(latency_ms), 0),
		        COALESCE(AVG(CASE WHEN ttft_ms > 0 THEN ttft_ms END), 0),
		        COALESCE(SUM(GREATEST(0, (input_tokens * 0.000003 + output_tokens * 0.000015) - cost_usd)), 0)
		 FROM request_logs rl
		 LEFT JOIN gateway_api_keys gak ON rl.gateway_api_key_id = gak.id
		 WHERE %s%s`, ownerClause, dateClause), args...,
	).Scan(&analytics.TotalSpendUSD, &analytics.AvgLatencyMs, &analytics.AvgTTFTMs, &analytics.EstimatedSavingsUSD)
	if err != nil {
		return nil, err
	}

	// 2. Client Apps Breakdown
	rowsApps, err := r.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT COALESCE(NULLIF(rl.client_app, ''), 'Unknown Client') as client_app,
		        COUNT(*) as requests,
		        COALESCE(SUM(total_tokens), 0) as tokens,
		        COALESCE(SUM(cost_usd), 0) as cost_usd
		 FROM request_logs rl
		 LEFT JOIN gateway_api_keys gak ON rl.gateway_api_key_id = gak.id
		 WHERE %s%s
		 GROUP BY COALESCE(NULLIF(rl.client_app, ''), 'Unknown Client')
		 ORDER BY cost_usd DESC, requests DESC`, ownerClause, dateClause), args...,
	)
	if err == nil {
		defer func() { _ = rowsApps.Close() }()
		for rowsApps.Next() {
			var app ClientAppStat
			if err := rowsApps.Scan(&app.ClientApp, &app.Requests, &app.Tokens, &app.CostUSD); err == nil {
				analytics.ClientApps = append(analytics.ClientApps, app)
			}
		}
		_ = rowsApps.Err()
	}

	// 3. Models Breakdown
	rowsModels, err := r.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT COALESCE(NULLIF(rl.model, ''), 'default') as model,
		        COUNT(*) as requests,
		        COALESCE(SUM(total_tokens), 0) as tokens,
		        COALESCE(SUM(cost_usd), 0) as cost_usd,
		        COALESCE(AVG(CASE WHEN ttft_ms > 0 THEN ttft_ms END), 0) as avg_ttft,
		        COALESCE(AVG(latency_ms), 0) as avg_latency
		 FROM request_logs rl
		 LEFT JOIN gateway_api_keys gak ON rl.gateway_api_key_id = gak.id
		 WHERE %s%s
		 GROUP BY COALESCE(NULLIF(rl.model, ''), 'default')
		 ORDER BY cost_usd DESC, requests DESC`, ownerClause, dateClause), args...,
	)
	if err == nil {
		defer func() { _ = rowsModels.Close() }()
		for rowsModels.Next() {
			var ms ModelStat
			if err := rowsModels.Scan(&ms.Model, &ms.Requests, &ms.Tokens, &ms.CostUSD, &ms.AvgTTFTMs, &ms.AvgLatencyMs); err == nil {
				analytics.Models = append(analytics.Models, ms)
			}
		}
		_ = rowsModels.Err()
	}

	return analytics, nil
}

func (r *RequestLogRepository) GetAgentStats(ctx context.Context, userID, agentID string, days int) (*models.AgentStats, error) {
	if days <= 0 || days > 90 {
		days = 30
	}

	stats := &models.AgentStats{
		SuccessRate: 1.0,
	}

	query := `
		SELECT COUNT(*),
		       COALESCE(SUM(rl.total_tokens), 0),
		       COALESCE(SUM(rl.cost_usd), 0),
		       ROUND(COALESCE(AVG(rl.latency_ms), 0))::int,
		       COALESCE(SUM(CASE WHEN rl.status_code >= 200 AND rl.status_code < 300 THEN 1 ELSE 0 END), 0)
		FROM request_logs rl
		WHERE rl.agent_id = $1
		  AND rl.created_at >= NOW() - ($2 || ' days')::INTERVAL
	`
	agentArgs := []any{agentID, days}
	tenantWhere := ""
	if orgID := tenantOrgID(ctx); orgID != "" {
		agentArgs = append(agentArgs, orgID)
		idx := len(agentArgs)
		tenantWhere = fmt.Sprintf(" AND (rl.org_id = $%d OR ((rl.org_id IS NULL OR rl.org_id = '') AND EXISTS (SELECT 1 FROM gateway_api_keys gak WHERE gak.id = rl.gateway_api_key_id AND gak.org_id = $%d)))", idx, idx)
	} else if userID != "" {
		agentArgs = append(agentArgs, userID)
		idx := len(agentArgs)
		tenantWhere = fmt.Sprintf(" AND EXISTS (SELECT 1 FROM gateway_api_keys gak WHERE gak.id = rl.gateway_api_key_id AND gak.user_id = $%d)", idx)
	}
	if wsClause, wsArgs := appendOptionalWorkspaceClause(ctx, "rl.", agentArgs); wsClause != "" {
		tenantWhere += " AND " + wsClause
		agentArgs = wsArgs
	}
	query += tenantWhere
	var successCount int
	err := r.db.QueryRowContext(ctx, query, agentArgs...).Scan(
		&stats.TotalRequests,
		&stats.TotalTokens,
		&stats.TotalCostUSD,
		&stats.AvgLatencyMs,
		&successCount,
	)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	if stats.TotalRequests > 0 {
		stats.SuccessRate = float64(successCount) / float64(stats.TotalRequests)
	}

	// Tool invocations count if any
	toolQuery := `SELECT COUNT(*) FROM tool_invocations ti
		 INNER JOIN request_logs rl ON ti.request_id = rl.request_id
		 WHERE rl.agent_id = $1 AND rl.created_at >= NOW() - ($2 || ' days')::INTERVAL` + tenantWhere
	_ = r.db.QueryRowContext(ctx, toolQuery, agentArgs...).Scan(&stats.ToolCallsCount)

	return stats, nil
}

// GetAgentTotalSpendUSD returns the agent's month-to-date spend in USD from request_logs.
func (r *RequestLogRepository) GetAgentTotalSpendUSD(ctx context.Context, agentID string) (float64, error) {
	if agentID == "" {
		return 0, nil
	}
	var spent sql.NullFloat64
	query := `SELECT COALESCE(SUM(cost_usd), 0)
		 FROM request_logs
		 WHERE agent_id = $1
		   AND created_at >= date_trunc('month', NOW())`
	args := []any{agentID}
	if orgID := tenantOrgID(ctx); orgID != "" {
		args = append(args, orgID)
		idx := len(args)
		query += fmt.Sprintf(" AND (org_id = $%d OR ((org_id IS NULL OR org_id = '') AND gateway_api_key_id IN (SELECT id FROM gateway_api_keys WHERE org_id = $%d)))", idx, idx)
	}
	if wsClause, wsArgs := appendOptionalWorkspaceClause(ctx, "", args); wsClause != "" {
		query += " AND " + wsClause
		args = wsArgs
	}
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&spent)
	if err != nil {
		return 0, err
	}
	return spent.Float64, nil
}

