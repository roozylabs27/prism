package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"time"

	"github.com/lib/pq"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/proxy"
	"github.com/roozylabs/prism/internal/telemetry"
)

// OutboxEnqueuer persists critical financial or security audit events when
// synchronous or async primary DB delivery encounters a transient failure.
type OutboxEnqueuer interface {
	Enqueue(ctx context.Context, item *models.DeliveryOutboxItem) error
}

func isNilInterface(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Ptr, reflect.Interface, reflect.Slice, reflect.Map, reflect.Func, reflect.Chan:
		return rv.IsNil()
	default:
		return false
	}
}

func enqueueOutboxFallback(
	ctx context.Context,
	outbox OutboxEnqueuer,
	eventType, orgID, userID string,
	payloadObj interface{},
	cause error,
) {
	if isNilInterface(outbox) {
		return
	}

	var raw json.RawMessage
	var err error
	if eventType == models.OutboxEventRequestLog {
		if rl, ok := payloadObj.(*models.RequestLog); ok && rl != nil {
			raw, err = models.EncodeOutboxRequestLog(rl)
		} else {
			err = fmt.Errorf("expected *models.RequestLog payload, got %T", payloadObj)
		}
	} else {
		raw, err = json.Marshal(payloadObj)
	}
	if err != nil {
		log.Printf("[Outbox] Error serializing fallback %s payload: %v", eventType, err)
		telemetry.RecordOutboxEvent(ctx, eventType, "enqueue_failed")
		return
	}

	lastErr := ""
	if cause != nil {
		lastErr = cause.Error()
	}

	outboxCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()

	item := &models.DeliveryOutboxItem{
		EventType: eventType,
		OrgID:     orgID,
		UserID:    userID,
		Payload:   raw,
		LastError: lastErr,
	}
	if err := outbox.Enqueue(outboxCtx, item); err != nil {
		log.Printf("[Outbox] CRITICAL: failed to enqueue %s (org=%s): %v (original: %s)", eventType, orgID, err, lastErr)
		telemetry.RecordOutboxEvent(outboxCtx, eventType, "enqueue_failed")
		return
	}
	telemetry.RecordOutboxEvent(outboxCtx, eventType, "enqueued")
}

func firstNonEmptyStrings(a, b []string) pq.StringArray {
	src := a
	if len(src) == 0 {
		src = b
	}
	if len(src) == 0 {
		return pq.StringArray{}
	}
	out := make(pq.StringArray, len(src))
	copy(out, src)
	return out
}

func buildSuccessAuditTrail(
	logEntry *models.RequestLog,
	gatewayKey *models.GatewayAPIKey,
	agentID, agentName, userRole, promptHash string,
	toolsInvoked, mcpServersCalled []string,
) *models.AIAuditTrail {
	if logEntry == nil {
		return nil
	}

	if promptHash == "" {
		promptHash = proxy.HashPayload(logEntry.Model + ":" + logEntry.RequestID)
	}
	responseHash := logEntry.ResponseHash
	if responseHash == "" {
		responseHash = proxy.HashPayload(fmt.Sprintf("%s:%s:%d", logEntry.Model, logEntry.ProviderType, logEntry.TotalTokens))
	}

	failoverChain := extractFailoverChain(logEntry.Attempts)
	if len(failoverChain) == 0 {
		failoverChain = pq.StringArray{}
	}

	userID := ""
	if gatewayKey != nil {
		userID = gatewayKey.UserID
	}
	role := userRole
	if role == "" {
		role = "developer"
	}

	trail := &models.AIAuditTrail{
		RequestID:         logEntry.RequestID,
		UserID:            userID,
		OrgID:             logEntry.OrgID,
		UserRole:          role,
		ModelSlug:         logEntry.Model,
		FailoverChain:     failoverChain,
		ToolsInvoked:      firstNonEmptyStrings(toolsInvoked, logEntry.ToolsInvoked),
		ResourcesAccessed: pq.StringArray{},
		MCPServersCalled:  firstNonEmptyStrings(mcpServersCalled, logEntry.MCPServersCalled),
		PromptTokens:      logEntry.InputTokens,
		CompletionTokens:  logEntry.OutputTokens,
		TotalTokens:       logEntry.TotalTokens,
		TotalCostUSD:      logEntry.CostUSD,
		StatusCode:        logEntry.StatusCode,
		LatencyMS:         logEntry.LatencyMs,
		TTFTMS:            logEntry.TTFTMs,
		PromptHash:        promptHash,
		ResponseHash:      responseHash,
		ComplianceStatus:  "compliant",
	}

	if gatewayKey != nil && gatewayKey.ID != "" {
		kID := gatewayKey.ID
		trail.GatewayKeyID = &kID
	}
	if agentID != "" {
		aID := agentID
		aName := agentName
		trail.AgentID = &aID
		trail.AgentName = &aName
	}
	return trail
}
