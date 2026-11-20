package models

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

const (
	OutboxEventRequestLog      = "request_log"
	OutboxEventAIAuditTrail    = "ai_audit_trail"
	OutboxEventGatewayKeyUsage = "gateway_key_usage"

	OutboxStatusPending    = "pending"
	OutboxStatusDelivered  = "delivered"
	OutboxStatusDeadLetter = "dead_letter"
)

// DeliveryOutboxItem represents a durable outbox record queued when an async or
// fallback write to request_logs, ai_audit_trails, or gateway_api_keys fails.
type DeliveryOutboxItem struct {
	ID            string          `json:"id" db:"id"`
	EventType     string          `json:"eventType" db:"event_type"`
	OrgID         string          `json:"orgId,omitempty" db:"org_id"`
	UserID        string          `json:"userId,omitempty" db:"user_id"`
	Payload       json.RawMessage `json:"payload" db:"payload"`
	Attempts      int             `json:"attempts" db:"attempts"`
	MaxAttempts   int             `json:"maxAttempts" db:"max_attempts"`
	Status        string          `json:"status" db:"status"`
	LastError     string          `json:"lastError,omitempty" db:"last_error"`
	NextAttemptAt time.Time       `json:"nextAttemptAt" db:"next_attempt_at"`
	CreatedAt     time.Time       `json:"createdAt" db:"created_at"`
	UpdatedAt     time.Time       `json:"updatedAt" db:"updated_at"`
}

// OutboxGatewayKeyUsagePayload stores the identifier needed to replay a failed
// gateway key usage increment with its original tenant attribution.
type OutboxGatewayKeyUsagePayload struct {
	GatewayKeyID string `json:"gatewayKeyId"`
	OrgID        string `json:"orgId,omitempty"`
	UserID       string `json:"userId,omitempty"`
}

// outboxRequestLogDTO preserves sql.NullString ErrorMessage and raw JSONB
// Attempts across JSON round-trips because RequestLog.MarshalJSON serializes
// ErrorMessage as a plain *string rather than {String, Valid}.
type outboxRequestLogDTO struct {
	ID              string          `json:"id"`
	RequestID       string          `json:"requestId"`
	OrgID           string          `json:"orgId"`
	WorkspaceID     string          `json:"workspaceId"`
	ProjectID       string          `json:"projectId"`
	AgentID         *string         `json:"agentId,omitempty"`
	GatewayAPIKeyID *string         `json:"gatewayApiKeyId,omitempty"`
	ProviderID      *string         `json:"providerId,omitempty"`
	ProviderType    string          `json:"providerType,omitempty"`
	CredentialID    *string         `json:"credentialId,omitempty"`
	CredentialName  string          `json:"credentialName,omitempty"`
	Model           string          `json:"model"`
	StatusCode      int             `json:"statusCode"`
	LatencyMs       int             `json:"latencyMs"`
	InputTokens     int             `json:"inputTokens"`
	OutputTokens    int             `json:"outputTokens"`
	TotalTokens     int             `json:"totalTokens"`
	CostUSD         float64         `json:"costUsd"`
	EstimatedCost   float64         `json:"estimatedCost"`
	ErrorMessage    *string         `json:"errorMessage,omitempty"`
	RetryCount      int             `json:"retryCount"`
	ClientIP        string          `json:"clientIp,omitempty"`
	UserAgent       string          `json:"userAgent,omitempty"`
	ClientApp       string          `json:"clientApp,omitempty"`
	IsStream        bool            `json:"isStream"`
	TTFTMs          int             `json:"ttftMs"`
	ResponseHash    string          `json:"responseHash"`
	ResponseBytes   int             `json:"responseBytes"`
	Attempts        json.RawMessage `json:"attempts,omitempty"`
	CreatedAt       time.Time       `json:"createdAt"`
}

// EncodeOutboxRequestLog serializes a RequestLog into a lossless JSON payload
// suitable for durable storage in delivery_outbox.
func EncodeOutboxRequestLog(logEntry *RequestLog) (json.RawMessage, error) {
	if logEntry == nil {
		return nil, fmt.Errorf("encode outbox request log: log is nil")
	}

	var errMsg *string
	if logEntry.ErrorMessage.Valid {
		s := logEntry.ErrorMessage.String
		errMsg = &s
	}

	var attempts json.RawMessage
	if len(logEntry.Attempts) > 0 && json.Valid(logEntry.Attempts) {
		attempts = json.RawMessage(logEntry.Attempts)
	}

	dto := outboxRequestLogDTO{
		ID:              logEntry.ID,
		RequestID:       logEntry.RequestID,
		OrgID:           logEntry.OrgID,
		WorkspaceID:     logEntry.WorkspaceID,
		ProjectID:       logEntry.ProjectID,
		AgentID:         logEntry.AgentID,
		GatewayAPIKeyID: logEntry.GatewayAPIKeyID,
		ProviderID:      logEntry.ProviderID,
		ProviderType:    logEntry.ProviderType,
		CredentialID:    logEntry.CredentialID,
		CredentialName:  logEntry.CredentialName,
		Model:           logEntry.Model,
		StatusCode:      logEntry.StatusCode,
		LatencyMs:       logEntry.LatencyMs,
		InputTokens:     logEntry.InputTokens,
		OutputTokens:    logEntry.OutputTokens,
		TotalTokens:     logEntry.TotalTokens,
		CostUSD:         logEntry.CostUSD,
		EstimatedCost:   logEntry.EstimatedCost,
		ErrorMessage:    errMsg,
		RetryCount:      logEntry.RetryCount,
		ClientIP:        logEntry.ClientIP,
		UserAgent:       logEntry.UserAgent,
		ClientApp:       logEntry.ClientApp,
		IsStream:        logEntry.IsStream,
		TTFTMs:          logEntry.TTFTMs,
		ResponseHash:    logEntry.ResponseHash,
		ResponseBytes:   logEntry.ResponseBytes,
		Attempts:        attempts,
		CreatedAt:       logEntry.CreatedAt,
	}

	b, err := json.Marshal(dto)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}

// DecodeOutboxRequestLog reconstructs a RequestLog from a delivery_outbox
// JSON payload, restoring sql.NullString and raw Attempts bytes.
func DecodeOutboxRequestLog(raw json.RawMessage) (*RequestLog, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("decode outbox request log: empty payload")
	}

	var dto outboxRequestLogDTO
	if err := json.Unmarshal(raw, &dto); err != nil {
		return nil, err
	}

	var errMsg sql.NullString
	if dto.ErrorMessage != nil {
		errMsg = sql.NullString{String: *dto.ErrorMessage, Valid: true}
	}

	var attempts []byte
	if len(dto.Attempts) > 0 && string(dto.Attempts) != "null" {
		attempts = []byte(dto.Attempts)
	}

	return &RequestLog{
		ID:              dto.ID,
		RequestID:       dto.RequestID,
		OrgID:           dto.OrgID,
		WorkspaceID:     dto.WorkspaceID,
		ProjectID:       dto.ProjectID,
		AgentID:         dto.AgentID,
		GatewayAPIKeyID: dto.GatewayAPIKeyID,
		ProviderID:      dto.ProviderID,
		ProviderType:    dto.ProviderType,
		CredentialID:    dto.CredentialID,
		CredentialName:  dto.CredentialName,
		Model:           dto.Model,
		StatusCode:      dto.StatusCode,
		LatencyMs:       dto.LatencyMs,
		InputTokens:     dto.InputTokens,
		OutputTokens:    dto.OutputTokens,
		TotalTokens:     dto.TotalTokens,
		CostUSD:         dto.CostUSD,
		EstimatedCost:   dto.EstimatedCost,
		ErrorMessage:    errMsg,
		RetryCount:      dto.RetryCount,
		ClientIP:        dto.ClientIP,
		UserAgent:       dto.UserAgent,
		ClientApp:       dto.ClientApp,
		IsStream:        dto.IsStream,
		TTFTMs:          dto.TTFTMs,
		ResponseHash:    dto.ResponseHash,
		ResponseBytes:   dto.ResponseBytes,
		Attempts:        attempts,
		CreatedAt:       dto.CreatedAt,
	}, nil
}
