package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/roozylabs/prism/internal/middleware"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/proxy"
	"github.com/roozylabs/prism/internal/service"
	"github.com/roozylabs/prism/internal/utils"
)

const (
	mcpProtocolVersion   = "2025-03-26"
	delegateAITaskTool   = "delegate_ai_task"
	maxDelegatedPrompt   = 100_000
	defaultMCPCallTimout = 60 * time.Second
)

type mcpServerCatalog interface {
	ListByUserID(ctx context.Context, userID string) ([]models.MCPServer, error)
}

type mcpToolCatalog interface {
	ListByServerID(ctx context.Context, serverID string) ([]models.MCPTool, error)
}

type mcpToolExecutor interface {
	ExecuteTool(ctx context.Context, userID, serverName, toolName string, args map[string]interface{}, encKey string) (*models.MCPToolExecutionResult, error)
}

type delegatedTaskOrchestrator interface {
	ExecuteChatCompletions(
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
	) (*service.OrchestrationResult, error)
}

// MCPServerHandler exposes Prism's inbound governed MCP Gateway over JSON-RPC
// 2.0 Streamable HTTP (POST /v1/mcp and POST /api/v1/mcp).
type MCPServerHandler struct {
	servers         mcpServerCatalog
	tools           mcpToolCatalog
	gateway         mcpToolExecutor
	orchestrator    delegatedTaskOrchestrator
	admission       *proxy.AdmissionController
	auditRecorder   *proxy.AuditRecorder
	roleResolver    MemberRoleResolver
	routingPolicies routingPolicyFinder
	encKey          string
}

func NewMCPServerHandler(
	servers mcpServerCatalog,
	tools mcpToolCatalog,
	gateway mcpToolExecutor,
	orchestrator delegatedTaskOrchestrator,
	admission *proxy.AdmissionController,
	auditRecorder *proxy.AuditRecorder,
	roleResolver MemberRoleResolver,
	routingPolicies routingPolicyFinder,
	encKey string,
) *MCPServerHandler {
	return &MCPServerHandler{
		servers:         servers,
		tools:           tools,
		gateway:         gateway,
		orchestrator:    orchestrator,
		admission:       admission,
		auditRecorder:   auditRecorder,
		roleResolver:    roleResolver,
		routingPolicies: routingPolicies,
		encKey:          encKey,
	}
}

type mcpJSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type mcpJSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      interface{}   `json:"id,omitempty"`
	Result  interface{}   `json:"result,omitempty"`
	Error   *mcpRPCError  `json:"error,omitempty"`
}

type mcpRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type mcpToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

type mcpTextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type mcpCallToolResult struct {
	Content []mcpTextContent `json:"content"`
	IsError bool             `json:"isError,omitempty"`
}

// Handle processes inbound MCP JSON-RPC 2.0 requests (initialize, ping,
// tools/list, tools/call).
func (h *MCPServerHandler) Handle(c *gin.Context) {
	gkVal, exists := c.Get("gatewayKey")
	gatewayKey, _ := gkVal.(*models.GatewayAPIKey)
	if !exists || gatewayKey == nil {
		c.JSON(http.StatusUnauthorized, mcpJSONRPCResponse{
			JSONRPC: "2.0",
			Error:   &mcpRPCError{Code: -32001, Message: "Gateway API key required"},
		})
		return
	}

	var rpcReq mcpJSONRPCRequest
	if err := c.ShouldBindJSON(&rpcReq); err != nil {
		c.JSON(http.StatusBadRequest, mcpJSONRPCResponse{
			JSONRPC: "2.0",
			Error:   &mcpRPCError{Code: -32700, Message: "Invalid JSON-RPC payload"},
		})
		return
	}
	if rpcReq.JSONRPC == "" {
		rpcReq.JSONRPC = "2.0"
	}
	if rpcReq.JSONRPC != "2.0" || rpcReq.Method == "" {
		c.JSON(http.StatusBadRequest, mcpJSONRPCResponse{
			JSONRPC: "2.0",
			ID:      rpcReq.ID,
			Error:   &mcpRPCError{Code: -32600, Message: "Invalid JSON-RPC 2.0 request"},
		})
		return
	}

	switch rpcReq.Method {
	case "initialize":
		c.JSON(http.StatusOK, mcpJSONRPCResponse{
			JSONRPC: "2.0",
			ID:      rpcReq.ID,
			Result: map[string]interface{}{
				"protocolVersion": mcpProtocolVersion,
				"capabilities": map[string]interface{}{
					"tools": map[string]interface{}{
						"listChanged": false,
					},
				},
				"serverInfo": map[string]interface{}{
					"name":    "prism-mcp-gateway",
					"version": "1.0.0",
				},
			},
		})
	case "notifications/initialized":
		if rpcReq.ID == nil {
			c.Status(http.StatusAccepted)
			return
		}
		c.JSON(http.StatusOK, mcpJSONRPCResponse{JSONRPC: "2.0", ID: rpcReq.ID, Result: map[string]interface{}{}})
	case "ping":
		c.JSON(http.StatusOK, mcpJSONRPCResponse{JSONRPC: "2.0", ID: rpcReq.ID, Result: map[string]interface{}{}})
	case "tools/list":
		h.handleToolsList(c, gatewayKey, rpcReq)
	case "tools/call":
		h.handleToolsCall(c, gatewayKey, rpcReq)
	default:
		c.JSON(http.StatusOK, mcpJSONRPCResponse{
			JSONRPC: "2.0",
			ID:      rpcReq.ID,
			Error:   &mcpRPCError{Code: -32601, Message: fmt.Sprintf("Method %q not found", rpcReq.Method)},
		})
	}
}

func (h *MCPServerHandler) handleToolsList(c *gin.Context, gatewayKey *models.GatewayAPIKey, rpcReq mcpJSONRPCRequest) {
	if _, err := middleware.ResolveCanonicalTenantContext(c, gatewayKey); err != nil {
		c.JSON(http.StatusForbidden, mcpJSONRPCResponse{
			JSONRPC: "2.0",
			ID:      rpcReq.ID,
			Error:   &mcpRPCError{Code: -32001, Message: err.Error()},
		})
		return
	}

	agent := middleware.AgentFromContext(c)
	toolsList := make([]mcpToolDefinition, 0)

	if isMCPToolPermittedForAgent(agent, "", delegateAITaskTool, delegateAITaskTool) {
		toolsList = append(toolsList, delegateAITaskDefinition())
	}

	if h.servers != nil && h.tools != nil {
		srvs, err := h.servers.ListByUserID(middleware.ContextWithTenant(c), gatewayKey.UserID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, mcpJSONRPCResponse{
				JSONRPC: "2.0",
				ID:      rpcReq.ID,
				Error:   &mcpRPCError{Code: -32603, Message: "Failed to list upstream MCP servers"},
			})
			return
		}

		for _, srv := range srvs {
			if !srv.Enabled {
				continue
			}
			if agent != nil && !proxy.CheckAgentMCPServerAccess(agent, srv.Name, srv.DisplayName, srv.ID).ToolAllowed {
				continue
			}
			srvTools, err := h.tools.ListByServerID(c.Request.Context(), srv.ID)
			if err != nil {
				continue
			}
			for _, t := range srvTools {
				if !t.Enabled {
					continue
				}
				fullName := fmt.Sprintf("%s__%s", srv.Name, t.Name)
				if !isMCPToolPermittedForAgent(agent, srv.Name, t.Name, fullName) {
					continue
				}
				schema := map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				}
				if len(t.InputSchema) > 0 {
					var parsed map[string]interface{}
					if json.Unmarshal(t.InputSchema, &parsed) == nil && len(parsed) > 0 {
						schema = parsed
					}
				}
				desc := t.Description
				if desc == "" {
					desc = fmt.Sprintf("Upstream tool %s from MCP server %s", t.Name, srv.Name)
				}
				toolsList = append(toolsList, mcpToolDefinition{
					Name:        fullName,
					Description: desc,
					InputSchema: schema,
				})
			}
		}
	}

	c.JSON(http.StatusOK, mcpJSONRPCResponse{
		JSONRPC: "2.0",
		ID:      rpcReq.ID,
		Result: map[string]interface{}{
			"tools": toolsList,
		},
	})
}

func (h *MCPServerHandler) handleToolsCall(c *gin.Context, gatewayKey *models.GatewayAPIKey, rpcReq mcpJSONRPCRequest) {
	var params struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}
	if err := json.Unmarshal(rpcReq.Params, &params); err != nil || strings.TrimSpace(params.Name) == "" {
		c.JSON(http.StatusOK, mcpJSONRPCResponse{
			JSONRPC: "2.0",
			ID:      rpcReq.ID,
			Error:   &mcpRPCError{Code: -32602, Message: "tools/call requires non-empty tool name"},
		})
		return
	}
	if params.Arguments == nil {
		params.Arguments = map[string]interface{}{}
	}

	tenantCtx, err := middleware.ResolveCanonicalTenantContext(c, gatewayKey)
	if err != nil {
		c.JSON(http.StatusForbidden, mcpJSONRPCResponse{
			JSONRPC: "2.0",
			ID:      rpcReq.ID,
			Error:   &mcpRPCError{Code: -32001, Message: err.Error()},
		})
		return
	}

	userRole := "developer"
	if h.roleResolver != nil && gatewayKey.UserID != "" && tenantCtx.OrgID != "" {
		resolvedRole, rErr := h.roleResolver.GetOrganizationMemberRole(c.Request.Context(), tenantCtx.OrgID, gatewayKey.UserID)
		if rErr != nil || resolvedRole == "" {
			c.JSON(http.StatusForbidden, mcpJSONRPCResponse{
				JSONRPC: "2.0",
				ID:      rpcReq.ID,
				Error:   &mcpRPCError{Code: -32001, Message: ErrMemberRoleUnresolved.Error()},
			})
			return
		}
		userRole = resolvedRole
	}

	agent := middleware.AgentFromContext(c)
	var agentID, agentName string
	if agent != nil {
		agentID, agentName = agent.ID, agent.Name
	}

	requestID := middleware.GetRequestID(c)
	if _, err := uuid.Parse(requestID); err != nil {
		requestID = uuid.New().String()
	}

	ctx, cancel := context.WithTimeout(middleware.ContextWithTenant(c), defaultMCPCallTimout)
	defer cancel()

	if params.Name == delegateAITaskTool {
		h.handleDelegateAITask(c, ctx, gatewayKey, tenantCtx, agent, agentID, agentName, userRole, requestID, rpcReq.ID, params.Arguments)
		return
	}

	parts := strings.SplitN(params.Name, "__", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		c.JSON(http.StatusOK, mcpJSONRPCResponse{
			JSONRPC: "2.0",
			ID:      rpcReq.ID,
			Error:   &mcpRPCError{Code: -32602, Message: "Unknown tool or missing <server>__<tool> namespace"},
		})
		return
	}
	serverName, toolName := parts[0], parts[1]

	// Enforce qualified + short-name agent permissions cleanly.
	if !isMCPToolPermittedForAgent(agent, serverName, toolName, params.Name) {
		reason := fmt.Sprintf("tool %q on MCP server %q is not permitted for agent", params.Name, serverName)
		h.recordMCPAudit(ctx, requestID, gatewayKey, agentID, agentName, userRole, params.Name, serverName, http.StatusForbidden, 0, "denied", reason)
		c.JSON(http.StatusOK, mcpJSONRPCResponse{
			JSONRPC: "2.0",
			ID:      rpcReq.ID,
			Error:   &mcpRPCError{Code: -32001, Message: reason},
		})
		return
	}

	// Enforce RBAC, Quotas, Budgets & MCP server allowance through AdmissionController.
	if h.admission != nil {
		admRes, admErr := h.admission.Evaluate(ctx, proxy.AdmissionRequest{
			UserID:        gatewayKey.UserID,
			Role:          userRole,
			AgentID:       agentID,
			AgentName:     agentName,
			Agent:         agent,
			ToolName:      params.Name,
			MCPServerName: serverName,
			TenantCtx:     tenantCtx,
		})
		if admErr != nil {
			c.JSON(http.StatusOK, mcpJSONRPCResponse{
				JSONRPC: "2.0",
				ID:      rpcReq.ID,
				Error:   &mcpRPCError{Code: -32603, Message: "Governance evaluation failed"},
			})
			return
		}
		if admRes.Decision == proxy.AdmissionDeny {
			h.recordMCPAudit(ctx, requestID, gatewayKey, agentID, agentName, userRole, params.Name, serverName, admRes.HTTPStatus, 0, "denied", admRes.Reason)
			rpcCode := -32001
			if admRes.HTTPStatus == http.StatusTooManyRequests {
				rpcCode = -32002
			}
			c.JSON(http.StatusOK, mcpJSONRPCResponse{
				JSONRPC: "2.0",
				ID:      rpcReq.ID,
				Error:   &mcpRPCError{Code: rpcCode, Message: admRes.Reason, Data: map[string]string{"code": admRes.ErrorCode}},
			})
			return
		}
	}

	if h.gateway == nil {
		c.JSON(http.StatusOK, mcpJSONRPCResponse{
			JSONRPC: "2.0",
			ID:      rpcReq.ID,
			Error:   &mcpRPCError{Code: -32603, Message: "MCP execution gateway unavailable"},
		})
		return
	}

	res, err := h.gateway.ExecuteTool(ctx, gatewayKey.UserID, serverName, toolName, params.Arguments, h.encKey)
	if err != nil {
		h.recordMCPAudit(ctx, requestID, gatewayKey, agentID, agentName, userRole, params.Name, serverName, http.StatusBadGateway, 0, "error", err.Error())
		c.JSON(http.StatusOK, mcpJSONRPCResponse{
			JSONRPC: "2.0",
			ID:      rpcReq.ID,
			Result: mcpCallToolResult{
				IsError: true,
				Content: []mcpTextContent{{Type: "text", Text: err.Error()}},
			},
		})
		return
	}

	argsJSON, _ := json.Marshal(params.Arguments)
	h.recordMCPAudit(ctx, requestID, gatewayKey, agentID, agentName, userRole, params.Name, serverName, res.StatusCode, res.LatencyMs, "compliant", string(argsJSON))

	outBytes, _ := json.Marshal(map[string]interface{}{
		"server":     res.Server,
		"tool":       res.Tool,
		"request_id": requestID,
		"latency_ms": res.LatencyMs,
		"result":     res.Result,
	})
	c.Header("X-Request-ID", requestID)
	c.JSON(http.StatusOK, mcpJSONRPCResponse{
		JSONRPC: "2.0",
		ID:      rpcReq.ID,
		Result: mcpCallToolResult{
			Content: []mcpTextContent{{Type: "text", Text: string(outBytes)}},
		},
	})
}

var validDelegatedTaskTypes = map[string]string{
	"classification":        "Classify the input accurately and return the category with brief rationale.",
	"extraction":            "Extract the requested structured facts strictly from the provided input.",
	"summarization":         "Summarize the provided input concisely while preserving key facts.",
	"structured_generation": "Generate valid, schema-clean structured output matching the request.",
	"analysis":              "Perform concise analytical evaluation of the provided input.",
	"coding":                "Complete the requested software engineering task cleanly and accurately.",
	"general":               "Complete the delegated AI subtask accurately and concisely.",
}

func (h *MCPServerHandler) handleDelegateAITask(
	c *gin.Context,
	ctx context.Context,
	gatewayKey *models.GatewayAPIKey,
	tenantCtx models.TenantContext,
	agent *models.Agent,
	agentID, agentName, userRole, requestID string,
	rpcID interface{},
	args map[string]interface{},
) {
	if !isMCPToolPermittedForAgent(agent, "", delegateAITaskTool, delegateAITaskTool) {
		reason := "agent is not permitted to invoke delegate_ai_task"
		h.recordMCPAudit(ctx, requestID, gatewayKey, agentID, agentName, userRole, delegateAITaskTool, "", http.StatusForbidden, 0, "denied", reason)
		c.JSON(http.StatusOK, mcpJSONRPCResponse{
			JSONRPC: "2.0",
			ID:      rpcID,
			Error:   &mcpRPCError{Code: -32001, Message: reason},
		})
		return
	}

	taskType, _ := args["task_type"].(string)
	taskType = strings.ToLower(strings.TrimSpace(taskType))
	taskInstruction, ok := validDelegatedTaskTypes[taskType]
	if !ok {
		c.JSON(http.StatusOK, mcpJSONRPCResponse{
			JSONRPC: "2.0",
			ID:      rpcID,
			Error:   &mcpRPCError{Code: -32602, Message: "task_type must be one of: classification, extraction, summarization, structured_generation, analysis, coding, general"},
		})
		return
	}

	prompt, _ := args["prompt"].(string)
	prompt = strings.TrimSpace(prompt)
	if prompt == "" || len(prompt) > maxDelegatedPrompt {
		c.JSON(http.StatusOK, mcpJSONRPCResponse{
			JSONRPC: "2.0",
			ID:      rpcID,
			Error:   &mcpRPCError{Code: -32602, Message: fmt.Sprintf("prompt must be non-empty and at most %d characters", maxDelegatedPrompt)},
		})
		return
	}

	if h.orchestrator == nil {
		c.JSON(http.StatusOK, mcpJSONRPCResponse{
			JSONRPC: "2.0",
			ID:      rpcID,
			Error:   &mcpRPCError{Code: -32603, Message: "AI delegation orchestrator unavailable"},
		})
		return
	}

	modelHint, _ := args["model_hint"].(string)
	modelHint = strings.TrimSpace(modelHint)
	if modelHint == "" {
		modelHint = models.SmartRouterModel
	}

	routingPolicyID := ""
	policyName := ""
	if policyRef, _ := args["routing_policy"].(string); strings.TrimSpace(policyRef) != "" {
		ref := strings.TrimSpace(policyRef)
		if h.routingPolicies == nil {
			c.JSON(http.StatusOK, mcpJSONRPCResponse{
				JSONRPC: "2.0",
				ID:      rpcID,
				Error:   &mcpRPCError{Code: -32602, Message: "Routing policy lookup unavailable"},
			})
			return
		}
		p, pErr := h.routingPolicies.FindByID(ctx, ref, gatewayKey.UserID)
		if pErr != nil || p == nil {
			p, pErr = h.routingPolicies.FindByName(ctx, ref, gatewayKey.UserID)
		}
		if pErr != nil || p == nil || !p.Enabled {
			c.JSON(http.StatusOK, mcpJSONRPCResponse{
				JSONRPC: "2.0",
				ID:      rpcID,
				Error:   &mcpRPCError{Code: -32602, Message: fmt.Sprintf("Routing policy %q not found or disabled", ref)},
			})
			return
		}
		routingPolicyID = p.ID
		policyName = p.Name
	}

	// Enforce tool-level admission first so RBAC policies denying delegate_ai_task
	// are enforced before inference execution runs.
	if h.admission != nil {
		toolAdm, admErr := h.admission.Evaluate(ctx, proxy.AdmissionRequest{
			UserID:        gatewayKey.UserID,
			Role:          userRole,
			AgentID:       agentID,
			AgentName:     agentName,
			Agent:         agent,
			ModelSlug:     modelHint,
			ToolName:      delegateAITaskTool,
			PromptPayload: prompt,
			TenantCtx:     tenantCtx,
		})
		if admErr != nil {
			c.JSON(http.StatusOK, mcpJSONRPCResponse{
				JSONRPC: "2.0",
				ID:      rpcID,
				Error:   &mcpRPCError{Code: -32603, Message: "Delegated task admission check failed"},
			})
			return
		}
		if toolAdm.Decision == proxy.AdmissionDeny {
			h.recordMCPAudit(ctx, requestID, gatewayKey, agentID, agentName, userRole, delegateAITaskTool, "", toolAdm.HTTPStatus, 0, "denied", toolAdm.Reason)
			rpcCode := -32001
			if toolAdm.HTTPStatus == http.StatusTooManyRequests {
				rpcCode = -32002
			}
			c.JSON(http.StatusOK, mcpJSONRPCResponse{
				JSONRPC: "2.0",
				ID:      rpcID,
				Error:   &mcpRPCError{Code: rpcCode, Message: toolAdm.Reason, Data: map[string]string{"code": toolAdm.ErrorCode}},
			})
			return
		}
	}

	systemPrompt, _ := args["system_prompt"].(string)
	systemPrompt = strings.TrimSpace(systemPrompt)
	if systemPrompt == "" {
		systemPrompt = taskInstruction
	} else {
		systemPrompt = taskInstruction + "\n\n" + systemPrompt
	}
	if outFmt, _ := args["output_format"].(string); strings.EqualFold(strings.TrimSpace(outFmt), "json") {
		systemPrompt += "\nRespond strictly with valid JSON."
	}

	proxyReq := proxy.ProxyRequest{
		Model:           modelHint,
		RoutingPolicyID: routingPolicyID,
		Agent:           agent,
		Messages: []map[string]interface{}{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": prompt},
		},
	}
	if maxTok, ok := asInt(args["max_tokens"]); ok && maxTok > 0 && maxTok <= 16384 {
		proxyReq.MaxTokens = maxTok
	}
	if temp, ok := asFloat(args["temperature"]); ok && temp >= 0 && temp <= 2 {
		proxyReq.Temperature = temp
	}

	res, err := h.orchestrator.ExecuteChatCompletions(
		nil,
		ctx,
		&proxyReq,
		gatewayKey,
		tenantCtx,
		agentID,
		agentName,
		userRole,
		requestID,
		c.ClientIP(),
		c.GetHeader("User-Agent"),
		"MCP Delegated Task",
	)
	if err != nil {
		h.recordMCPAudit(ctx, requestID, gatewayKey, agentID, agentName, userRole, delegateAITaskTool, "", http.StatusBadGateway, 0, "error", err.Error())
		c.JSON(http.StatusOK, mcpJSONRPCResponse{
			JSONRPC: "2.0",
			ID:      rpcID,
			Error:   &mcpRPCError{Code: -32603, Message: "Delegated task execution failed: " + err.Error()},
		})
		return
	}
	if res.Denied {
		h.recordMCPAudit(ctx, requestID, gatewayKey, agentID, agentName, userRole, delegateAITaskTool, "", res.HTTPStatus, 0, "denied", res.ErrorMessage)
		rpcCode := -32001
		if res.HTTPStatus == http.StatusTooManyRequests {
			rpcCode = -32002
		}
		c.JSON(http.StatusOK, mcpJSONRPCResponse{
			JSONRPC: "2.0",
			ID:      rpcID,
			Error:   &mcpRPCError{Code: rpcCode, Message: res.ErrorMessage, Data: map[string]string{"code": res.ErrorCode}},
		})
		return
	}

	outputText := extractAssistantText(res.Response)
	executedModel := modelHint
	executedProvider := ""
	var promptTokens, completionTokens, totalTokens, latencyMs int
	var costUSD float64
	if res.RequestLog != nil {
		if res.RequestLog.Model != "" {
			executedModel = res.RequestLog.Model
		}
		executedProvider = res.RequestLog.ProviderType
		promptTokens = res.RequestLog.InputTokens
		completionTokens = res.RequestLog.OutputTokens
		totalTokens = res.RequestLog.TotalTokens
		costUSD = res.RequestLog.CostUSD
		latencyMs = res.RequestLog.LatencyMs
	} else if res.Response != nil {
		if res.Response.Model != "" {
			executedModel = res.Response.Model
		}
		promptTokens = res.Response.Usage.PromptTokens
		completionTokens = res.Response.Usage.CompletionTokens
		totalTokens = res.Response.Usage.TotalTokens
	}
	if policyName == "" {
		policyName = "default"
	}

	h.recordMCPAudit(ctx, requestID, gatewayKey, agentID, agentName, userRole, delegateAITaskTool, "", http.StatusOK, latencyMs, "compliant", prompt)

	payloadBytes, _ := json.Marshal(map[string]interface{}{
		"output": outputText,
		"disclosure": map[string]interface{}{
			"executed_model":    executedModel,
			"executed_provider": executedProvider,
			"routing_policy":    policyName,
			"task_type":         taskType,
			"request_id":        requestID,
			"latency_ms":        latencyMs,
			"usage": map[string]interface{}{
				"prompt_tokens":     promptTokens,
				"completion_tokens": completionTokens,
				"total_tokens":      totalTokens,
				"cost_usd":          costUSD,
			},
		},
	})
	c.Header("X-Request-ID", requestID)
	c.Header("X-Prism-Model", executedModel)
	if executedProvider != "" {
		c.Header("X-Prism-Provider", executedProvider)
	}
	c.JSON(http.StatusOK, mcpJSONRPCResponse{
		JSONRPC: "2.0",
		ID:      rpcID,
		Result: mcpCallToolResult{
			Content: []mcpTextContent{{Type: "text", Text: string(payloadBytes)}},
		},
	})
}

func (h *MCPServerHandler) recordMCPAudit(ctx context.Context, reqID string, key *models.GatewayAPIKey, agentID, agentName, userRole, toolName, serverName string, statusCode, latencyMs int, compliance, payload string) {
	if h.auditRecorder == nil || key == nil {
		return
	}
	servers := []string{}
	if serverName != "" {
		servers = []string{serverName}
	}
	trail := &models.AIAuditTrail{
		RequestID:         reqID,
		UserID:            key.UserID,
		GatewayKeyID:      &key.ID,
		UserRole:          userRole,
		ModelSlug:         "mcp:" + toolName,
		FailoverChain:     []string{},
		ToolsInvoked:      []string{toolName},
		ResourcesAccessed: []string{},
		MCPServersCalled:  servers,
		StatusCode:        statusCode,
		LatencyMS:         latencyMs,
		PromptHash:        utils.HashSHA256(toolName + ":" + payload),
		ResponseHash:      utils.HashSHA256(compliance + ":" + reqID),
		ComplianceStatus:  compliance,
	}
	if agentID != "" {
		trail.AgentID = &agentID
		trail.AgentName = &agentName
	}
	_ = h.auditRecorder.Record(ctx, trail)
}

// isMCPToolPermittedForAgent evaluates explicit deny rules across both the
// qualified (<server>__<tool>) and short (<tool>) names, then checks allow rules.
func isMCPToolPermittedForAgent(ag *models.Agent, serverName, toolName, fullName string) bool {
	if ag == nil {
		return true
	}
	if !ag.Enabled {
		return false
	}
	if serverName != "" && !proxy.CheckAgentMCPServerAccess(ag, serverName, "", "").ToolAllowed {
		return false
	}
	fullRes := proxy.CheckAgentToolAccess(ag, fullName)
	shortRes := fullRes
	if toolName != "" && toolName != fullName {
		shortRes = proxy.CheckAgentToolAccess(ag, toolName)
	}
	// Explicit deny on either the qualified name or short tool name always blocks.
	if strings.Contains(fullRes.Reason, "explicitly denied") || strings.Contains(shortRes.Reason, "explicitly denied") {
		return false
	}
	return fullRes.ToolAllowed || shortRes.ToolAllowed
}

func extractAssistantText(resp *proxy.ProviderResponse) string {
	if resp == nil || len(resp.Choices) == 0 || resp.Choices[0].Message == nil {
		return ""
	}
	if s, ok := resp.Choices[0].Message["content"].(string); ok {
		return s
	}
	return ""
}

func asInt(v interface{}) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case json.Number:
		parsed, err := n.Int64()
		return int(parsed), err == nil
	default:
		return 0, false
	}
}

func asFloat(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case json.Number:
		parsed, err := n.Float64()
		return parsed, err == nil
	default:
		return 0, false
	}
}

func delegateAITaskDefinition() mcpToolDefinition {
	return mcpToolDefinition{
		Name:        delegateAITaskTool,
		Description: "Delegate a focused AI subtask (classification, extraction, summarization, structured generation, analysis, or coding) through Prism Smart Router under tenant budget, policy, and audit governance. Discloses the exact executed model and cost.",
		InputSchema: map[string]interface{}{
			"type":     "object",
			"required": []string{"task_type", "prompt"},
			"properties": map[string]interface{}{
				"task_type": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"classification", "extraction", "summarization", "structured_generation", "analysis", "coding", "general"},
					"description": "Type of subtask for routing classification and system framing.",
				},
				"prompt": map[string]interface{}{
					"type":        "string",
					"description": "The subtask input prompt.",
				},
				"system_prompt": map[string]interface{}{
					"type":        "string",
					"description": "Optional additional system instruction.",
				},
				"routing_policy": map[string]interface{}{
					"type":        "string",
					"description": "Optional tenant-owned routing policy ID or exact name.",
				},
				"model_hint": map[string]interface{}{
					"type":        "string",
					"description": "Optional model slug when explicit selection is desired; defaults to prism-auto.",
				},
				"max_tokens": map[string]interface{}{
					"type":        "integer",
					"description": "Optional maximum completion tokens (1-16384).",
				},
				"temperature": map[string]interface{}{
					"type":        "number",
					"description": "Optional sampling temperature (0.0-2.0).",
				},
				"output_format": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"text", "json"},
					"description": "Optional output format preference.",
				},
			},
		},
	}
}
