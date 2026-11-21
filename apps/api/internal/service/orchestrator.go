package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/database"
	"github.com/roozylabs/prism/internal/middleware"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/proxy"
	goredis "github.com/roozylabs/prism/internal/redis"
	"github.com/roozylabs/prism/internal/repository"
	"github.com/roozylabs/prism/internal/telemetry"
	"github.com/roozylabs/prism/internal/utils"
)

type OrchestrationResult struct {
	Denied       bool                   `json:"denied"`
	HTTPStatus   int                    `json:"httpStatus"`
	ErrorCode    string                  `json:"errorCode"`
	ErrorMessage string                  `json:"errorMessage"`
	PolicyID     string                  `json:"policyId"`
	PolicyName   string                  `json:"policyName"`
	RequestID    string                  `json:"requestId"`
	Response     *proxy.ProviderResponse `json:"response,omitempty"`
	RequestLog   *models.RequestLog      `json:"requestLog,omitempty"`
}

type ExecutionOrchestrator struct {
	engine         *proxy.Engine
	admission      *proxy.AdmissionController
	budgetManager  *proxy.BudgetManager
	gatewayKeys    *repository.GatewayKeyRepository
	requestLogs    *repository.RequestLogRepository
	eventPublisher *goredis.EventPublisher
	pricingRepo    *repository.ModelPricingRepository
	auditRecorder  *proxy.AuditRecorder
	postProcessor  *AsyncPostProcessor
	outbox         OutboxEnqueuer
}

func NewExecutionOrchestrator(
	engine *proxy.Engine,
	admission *proxy.AdmissionController,
	gatewayKeys *repository.GatewayKeyRepository,
	requestLogs *repository.RequestLogRepository,
	eventPublisher *goredis.EventPublisher,
	pricingRepo *repository.ModelPricingRepository,
	auditRecorder *proxy.AuditRecorder,
) *ExecutionOrchestrator {
	var bm *proxy.BudgetManager
	if admission != nil {
		bm = admission.BudgetManager()
	}
	return &ExecutionOrchestrator{
		engine:         engine,
		admission:      admission,
		budgetManager:  bm,
		gatewayKeys:    gatewayKeys,
		requestLogs:    requestLogs,
		eventPublisher: eventPublisher,
		pricingRepo:    pricingRepo,
		auditRecorder:  auditRecorder,
	}
}

func (o *ExecutionOrchestrator) SetPostProcessor(p *AsyncPostProcessor) {
	o.postProcessor = p
}

// SetBudgetManager configures the BudgetManager used for atomic in-flight spend
// reservation around inference requests.
func (o *ExecutionOrchestrator) SetBudgetManager(bm *proxy.BudgetManager) {
	if o != nil {
		o.budgetManager = bm
	}
}

// SetOutbox attaches a durable delivery outbox for synchronous fallback when
// AsyncPostProcessor is nil or when recording denied audit trails fails.
func (o *ExecutionOrchestrator) SetOutbox(outbox OutboxEnqueuer) {
	if o != nil && !isNilInterface(outbox) {
		o.outbox = outbox
	}
}

func hashRequestPrompt(req *proxy.ProxyRequest, requestID string) string {
	if req != nil && len(req.Messages) > 0 {
		if raw, err := json.Marshal(req.Messages); err == nil {
			return proxy.HashPayload(string(raw))
		}
	}
	model := ""
	if req != nil {
		model = req.Model
	}
	return proxy.HashPayload(model + ":" + requestID)
}

func (o *ExecutionOrchestrator) ExecuteChatCompletions(
	c *gin.Context,
	ctx context.Context,
	req *proxy.ProxyRequest,
	gatewayKey *models.GatewayAPIKey,
	tenantCtx models.TenantContext,
	agentID string,
	agentName string,
	userRole string,
	requestID string,
	clientIP string,
	userAgent string,
	clientApp string,
) (*OrchestrationResult, error) {
	if userRole == "" {
		userRole = "developer"
	}
	if agentName == "" && agentID != "" {
		agentName = agentID
	}

	var promptBuilder strings.Builder
	for _, m := range req.Messages {
		if content, ok := m["content"].(string); ok {
			promptBuilder.WriteString(content)
			promptBuilder.WriteString(" ")
		}
	}

	promptHash := hashRequestPrompt(req, requestID)

	userID := ""
	if gatewayKey != nil {
		userID = gatewayKey.UserID
	}
	ctx = database.WithTenantContext(ctx, database.TenantDBContext{
		UserID:      userID,
		OrgID:       tenantCtx.OrgID,
		WorkspaceID: tenantCtx.WorkspaceID,
		ProjectID:   tenantCtx.ProjectID,
	})

	ctx, rootSpan := telemetry.StartSpan(ctx, "prism.gateway.chat_completions")
	telemetry.SetSpanAttributes(rootSpan, map[string]interface{}{
		"prism.request_id":   requestID,
		"prism.org_id":       tenantCtx.OrgID,
		"prism.workspace_id": tenantCtx.WorkspaceID,
		"prism.project_id":   tenantCtx.ProjectID,
		"prism.agent_id":     agentID,
		"prism.model":        req.Model,
		"prism.stream":       req.Stream,
	})
	defer rootSpan.End()

	// 1. Admission Control Gate
	if c != nil && req.Agent == nil {
		req.Agent = resolvedAgent(c)
	}
	admReq := proxy.AdmissionRequest{
		UserID:        userID,
		Role:          userRole,
		AgentID:       agentID,
		AgentName:     agentName,
		Agent:         req.Agent,
		ModelSlug:     req.Model,
		PromptPayload: promptBuilder.String(),
		TenantCtx:     tenantCtx,
	}

	admCtx, admSpan := telemetry.StartSpan(ctx, "prism.admission.evaluate")
	admRes, err := o.admission.Evaluate(admCtx, admReq)
	admSpan.End()
	if err != nil {
		return nil, fmt.Errorf("admission control evaluate: %w", err)
	}

	if admRes.Decision == proxy.AdmissionDeny {
		// Record audit event for denied request
		o.recordDeniedAuditTrail(ctx, requestID, gatewayKey, tenantCtx, agentID, agentName, userRole, req.Model, promptHash, admRes.Reason)

		return &OrchestrationResult{
			Denied:       true,
			HTTPStatus:   admRes.HTTPStatus,
			ErrorCode:    admRes.ErrorCode,
			ErrorMessage: admRes.Reason,
			PolicyID:     admRes.PolicyID,
			PolicyName:   admRes.PolicyName,
			RequestID:    requestID,
		}, nil
	}

	// 1b. Atomic in-flight budget spend reservation (prevents concurrent hard-limit races)
	bm := o.budgetManager
	if bm == nil && o.admission != nil {
		bm = o.admission.BudgetManager()
	}
	var budgetReservation *proxy.BudgetReservation
	actualCostUSD := 0.0
	if bm != nil && userID != "" {
		estCost := estimateRequestCostUSD(req)
		res, bStatus, rErr := bm.Reserve(ctx, userID, estCost)
		if rErr != nil {
			return nil, fmt.Errorf("budget reserve: %w", rErr)
		}
		if res == nil && bStatus != nil && bStatus.Status == "exceeded" && bStatus.Budget != nil && bStatus.Budget.HardLimit {
			reason := fmt.Sprintf("%s budget limit of %q exceeded", bStatus.LimitPeriod, bStatus.Budget.Name)
			o.recordDeniedAuditTrail(ctx, requestID, gatewayKey, tenantCtx, agentID, agentName, userRole, req.Model, promptHash, reason)
			return &OrchestrationResult{
				Denied:       true,
				HTTPStatus:   429,
				ErrorCode:    "budget_exceeded_error",
				ErrorMessage: reason,
				RequestID:    requestID,
			}, nil
		}
		budgetReservation = res
		defer func() {
			bm.Release(budgetReservation, actualCostUSD)
		}()
	}

	// Attach budget status to proxy response header if downgraded or warned
	if admRes.BudgetStatus != "" && c != nil {
		c.Header("X-Prism-Budget-Status", admRes.BudgetStatus)
	}

	// 1c. Concurrent stream limit enforcement across org/workspace/project quota scopes
	if req.Stream && o.admission != nil {
		releaseStreams, streamDeny, sErr := o.admission.AcquireStreamSlots(ctx, requestID, tenantCtx)
		if sErr != nil {
			return nil, fmt.Errorf("stream concurrency limit: %w", sErr)
		}
		if streamDeny != nil {
			o.recordDeniedAuditTrail(ctx, requestID, gatewayKey, tenantCtx, agentID, agentName, userRole, req.Model, promptHash, streamDeny.Reason)
			return &OrchestrationResult{
				Denied:       true,
				HTTPStatus:   streamDeny.HTTPStatus,
				ErrorCode:    streamDeny.ErrorCode,
				ErrorMessage: streamDeny.Reason,
				RequestID:    requestID,
			}, nil
		}
		if releaseStreams != nil {
			defer releaseStreams()
		}
	}

	// 2. Stream vs Non-Stream Proxy Execution
	var providerResp *proxy.ProviderResponse
	var log *models.RequestLog

	if req.Stream {
		log, err = o.engine.ProxyStream(c, req, gatewayKey)
		if err != nil {
			return nil, err
		}
	} else {
		providerResp, log, err = o.engine.Proxy(c, req, gatewayKey)
		if err != nil {
			return nil, err
		}
	}

	if log != nil {
		// 3. Populate Canonical Tenant & Metadata Attribution
		log.RequestID = requestID
		log.OrgID = tenantCtx.OrgID
		log.WorkspaceID = tenantCtx.WorkspaceID
		log.ProjectID = tenantCtx.ProjectID
		if agentID != "" {
			log.AgentID = &agentID
		}
		log.ClientIP = clientIP
		log.UserAgent = userAgent
		log.ClientApp = clientApp
		log.IsStream = req.Stream

		// The route's model row may carry no price; the pricing table knows
		// free models from unpriced ones.
		if log.CostUSD == 0 && log.TotalTokens > 0 && o.pricingRepo != nil {
			log.CostUSD = o.pricingRepo.CalculateCost(log.Model, log.ProviderType, log.InputTokens, log.OutputTokens)
		}
		actualCostUSD = log.CostUSD

		if c != nil {
			c.Header("X-Request-Id", requestID)
			c.Header("X-Prism-Model", log.Model)
			c.Header("X-Prism-Provider", log.ProviderType)
			// X-Prism-Routing-Policy is set by the engine with the policy that
			// routed a prism-auto request; it used to echo the model name here.
			c.Header("X-Prism-Failovers", fmt.Sprintf("%d", log.RetryCount))
		}

		if o.postProcessor != nil {
			o.postProcessor.Enqueue(&PostProcessTask{
				Ctx:              ctx,
				RequestID:        requestID,
				Log:              log,
				GatewayKey:       gatewayKey,
				AgentID:          agentID,
				AgentName:        agentName,
				UserRole:         userRole,
				ClientIP:         clientIP,
				UserAgent:        userAgent,
				ClientApp:        clientApp,
				PromptHash:       promptHash,
				ToolsInvoked:     log.ToolsInvoked,
				MCPServersCalled: log.MCPServersCalled,
			})
		} else {
			// Save request log to database
			if o.requestLogs != nil {
				if err := o.requestLogs.Create(ctx, log); err != nil {
					enqueueOutboxFallback(ctx, o.outbox, models.OutboxEventRequestLog, log.OrgID, userID, log, err)
				}
			}
			if o.gatewayKeys != nil && gatewayKey != nil && gatewayKey.ID != "" {
				if err := o.gatewayKeys.IncrementUsage(ctx, gatewayKey.ID); err != nil {
					enqueueOutboxFallback(ctx, o.outbox, models.OutboxEventGatewayKeyUsage, log.OrgID, userID, models.OutboxGatewayKeyUsagePayload{
						GatewayKeyID: gatewayKey.ID,
						OrgID:        log.OrgID,
						UserID:       userID,
					}, err)
				}
			}
			o.publishEvents(c, ctx, requestID, log, gatewayKey)
			o.recordAuditTrail(ctx, log, gatewayKey, agentID, agentName, userRole, promptHash)
		}
	}

	return &OrchestrationResult{
		Denied:     false,
		HTTPStatus: 200,
		RequestID:  requestID,
		Response:   providerResp,
		RequestLog: log,
	}, nil
}

func (o *ExecutionOrchestrator) publishEvents(c *gin.Context, ctx context.Context, requestID string, log *models.RequestLog, gatewayKey *models.GatewayAPIKey) {
	if o.eventPublisher == nil || log == nil {
		return
	}

	keyPrefix := ""
	gatewayKeyName := ""
	ownerID := ""
	if c != nil {
		ownerID = c.GetString("userId")
	}
	if gatewayKey != nil {
		keyPrefix = gatewayKey.KeyPrefix
		gatewayKeyName = gatewayKey.Name
		if gatewayKey.UserID != "" {
			ownerID = gatewayKey.UserID
		}
	}

	eventData := map[string]interface{}{
		"requestId":      requestID,
		"model":          log.Model,
		"provider":       log.ProviderType,
		"statusCode":     log.StatusCode,
		"latencyMs":      log.LatencyMs,
		"inputTokens":    log.InputTokens,
		"outputTokens":   log.OutputTokens,
		"totalTokens":    log.TotalTokens,
		"costUsd":        log.CostUSD,
		"clientApp":      log.ClientApp,
		"orgId":          log.OrgID,
		"workspaceId":    log.WorkspaceID,
		"projectId":      log.ProjectID,
		"keyPrefix":      keyPrefix,
		"gatewayKeyName": gatewayKeyName,
		"timestamp":      log.CreatedAt,
	}

	_ = o.eventPublisher.Publish(ctx, ownerID, "request_log_created", eventData)
}

func (o *ExecutionOrchestrator) recordDeniedAuditTrail(
	ctx context.Context,
	requestID string,
	gatewayKey *models.GatewayAPIKey,
	tenantCtx models.TenantContext,
	agentID string,
	agentName string,
	userRole string,
	modelSlug string,
	promptHash string,
	reason string,
) {
	if o.auditRecorder == nil {
		return
	}

	if promptHash == "" {
		promptHash = proxy.HashPayload(modelSlug + ":" + requestID)
	}

	userID := ""
	if gatewayKey != nil {
		userID = gatewayKey.UserID
	}
	ctx = database.WithTenantContext(ctx, database.TenantDBContext{
		UserID:      userID,
		OrgID:       tenantCtx.OrgID,
		WorkspaceID: tenantCtx.WorkspaceID,
		ProjectID:   tenantCtx.ProjectID,
	})

	trail := &models.AIAuditTrail{
		RequestID:         requestID,
		UserID:            userID,
		OrgID:             tenantCtx.OrgID,
		UserRole:          userRole,
		ModelSlug:         modelSlug,
		FailoverChain:     []string{},
		ToolsInvoked:      []string{},
		ResourcesAccessed: []string{},
		MCPServersCalled:  []string{},
		PromptTokens:      0,
		CompletionTokens:  0,
		TotalTokens:       0,
		TotalCostUSD:      0,
		StatusCode:        403,
		LatencyMS:         0,
		TTFTMS:            0,
		PromptHash:        promptHash,
		ResponseHash:      utils.HashSHA256("denied:" + reason),
		ComplianceStatus:  "denied",
	}

	if gatewayKey != nil && gatewayKey.ID != "" {
		trail.GatewayKeyID = &gatewayKey.ID
	}
	if agentID != "" {
		trail.AgentID = &agentID
		trail.AgentName = &agentName
	}

	if err := o.auditRecorder.Record(ctx, trail); err != nil {
		enqueueOutboxFallback(ctx, o.outbox, models.OutboxEventAIAuditTrail, trail.OrgID, trail.UserID, trail, err)
	}
}

func (o *ExecutionOrchestrator) recordAuditTrail(
	ctx context.Context,
	log *models.RequestLog,
	gatewayKey *models.GatewayAPIKey,
	agentID string,
	agentName string,
	userRole string,
	promptHash string,
) {
	if o.auditRecorder == nil || log == nil {
		return
	}

	userID := ""
	if gatewayKey != nil {
		userID = gatewayKey.UserID
	}
	ctx = database.WithTenantContext(ctx, database.TenantDBContext{
		UserID:      userID,
		OrgID:       log.OrgID,
		WorkspaceID: log.WorkspaceID,
		ProjectID:   log.ProjectID,
	})

	trail := buildSuccessAuditTrail(log, gatewayKey, agentID, agentName, userRole, promptHash, log.ToolsInvoked, log.MCPServersCalled)
	if trail == nil {
		return
	}
	if err := o.auditRecorder.Record(ctx, trail); err != nil {
		enqueueOutboxFallback(ctx, o.outbox, models.OutboxEventAIAuditTrail, trail.OrgID, trail.UserID, trail, err)
	}
}

func extractFailoverChain(attemptsBytes []byte) []string {
	if len(attemptsBytes) == 0 {
		return []string{}
	}

	var attempts []map[string]interface{}
	if err := json.Unmarshal(attemptsBytes, &attempts); err != nil {
		return []string{}
	}

	var chain []string
	for _, att := range attempts {
		credID, _ := att["credential_id"].(string)
		if credID == "" {
			continue
		}
		label, _ := att["provider_name"].(string)
		if label == "" {
			label, _ = att["provider_id"].(string)
		}
		if label == "" {
			label, _ = att["model"].(string)
		}
		if label != "" {
			chain = append(chain, fmt.Sprintf("%s:%s", label, credID))
		} else {
			chain = append(chain, credID)
		}
	}
	if chain == nil {
		return []string{}
	}
	return chain
}

// resolvedAgent is the agent the gateway middleware resolved for a live
// request. Async jobs have no request context; admission resolves their agent
// from the stored id instead.
func resolvedAgent(c *gin.Context) *models.Agent {
	if c == nil {
		return nil
	}
	return middleware.AgentFromContext(c)
}

// estimateRequestCostUSD returns a conservative pre-request spend reservation
// floor so concurrent requests cannot race past a hard budget limit.
func estimateRequestCostUSD(req *proxy.ProxyRequest) float64 {
	if req == nil {
		return 0.001
	}
	chars := 0
	for _, m := range req.Messages {
		if s, ok := m["content"].(string); ok {
			chars += len(s)
		}
	}
	estInputTokens := float64(chars) / 4.0
	estOutputTokens := float64(req.MaxTokens)
	if estOutputTokens <= 0 {
		estOutputTokens = 256
	}
	// Conservative blended rate ($0.50/1M input, $1.50/1M output) with $0.001 floor.
	est := (estInputTokens/1_000_000.0)*0.50 + (estOutputTokens/1_000_000.0)*1.50
	if est < 0.001 {
		return 0.001
	}
	return est
}
