package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/proxy"
	"github.com/roozylabs/prism/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeApprovalStore struct {
	mu    sync.Mutex
	items map[string]*models.ApprovalRequest
}

func newFakeApprovalStore() *fakeApprovalStore {
	return &fakeApprovalStore{items: make(map[string]*models.ApprovalRequest)}
}

func (f *fakeApprovalStore) Create(_ context.Context, req *models.ApprovalRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if req.ID == "" {
		req.ID = uuid.New().String()
	}
	if req.Status == "" {
		req.Status = models.ApprovalStatusPending
	}
	now := time.Now()
	if req.CreatedAt.IsZero() {
		req.CreatedAt = now
	}
	req.UpdatedAt = now
	if req.ExpiresAt.IsZero() {
		req.ExpiresAt = now.Add(24 * time.Hour)
	}
	copyItem := *req
	f.items[req.ID] = &copyItem
	return nil
}

func (f *fakeApprovalStore) FindByID(_ context.Context, id, userID string) (*models.ApprovalRequest, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	it, ok := f.items[id]
	if !ok || (userID != "" && it.UserID != userID) {
		return nil, repository.ErrApprovalNotFound
	}
	cp := *it
	return &cp, nil
}

func (f *fakeApprovalStore) List(_ context.Context, userID, status string, _, _ int) ([]models.ApprovalRequest, int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []models.ApprovalRequest
	for _, it := range f.items {
		if userID != "" && it.UserID != userID {
			continue
		}
		if status != "" && status != "all" && it.Status != status {
			continue
		}
		out = append(out, *it)
	}
	return out, int64(len(out)), nil
}

func (f *fakeApprovalStore) Decide(_ context.Context, id, userID, actorID, decisionStatus, note string) (*models.ApprovalRequest, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	it, ok := f.items[id]
	if !ok || (userID != "" && it.UserID != userID) || it.Status != models.ApprovalStatusPending || !it.ExpiresAt.After(time.Now()) {
		return nil, repository.ErrApprovalNotDecidable
	}
	now := time.Now()
	it.Status = decisionStatus
	it.DecidedBy = actorID
	it.DecisionNote = note
	it.DecidedAt = &now
	it.UpdatedAt = now
	cp := *it
	return &cp, nil
}

func (f *fakeApprovalStore) VerifyAndConsume(_ context.Context, id, userID, toolName, mcpServerName, argumentsHash string) (*models.ApprovalRequest, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	it, ok := f.items[id]
	if !ok ||
		(userID != "" && it.UserID != userID) ||
		it.Status != models.ApprovalStatusApproved ||
		it.ToolName != toolName ||
		it.MCPServerName != mcpServerName ||
		it.ArgumentsHash != argumentsHash ||
		!it.ExpiresAt.After(time.Now()) {
		return nil, repository.ErrApprovalInvalidConsume
	}
	now := time.Now()
	it.Status = models.ApprovalStatusConsumed
	it.ConsumedAt = &now
	it.UpdatedAt = now
	cp := *it
	return &cp, nil
}

type fakeApprovalPolicyFinder struct {
	policies []models.GovernancePolicy
}

func (f *fakeApprovalPolicyFinder) ListByUserID(context.Context, string) ([]models.GovernancePolicy, error) {
	return f.policies, nil
}

type fakeToolDefFinder struct {
	tools map[string]*models.Tool
}

func (f *fakeToolDefFinder) FindByUserAndName(_ context.Context, _, name string) (*models.Tool, error) {
	return f.tools[name], nil
}

type fakeApprovalToolExecutor struct {
	calls int
}

func (f *fakeApprovalToolExecutor) Execute(_ context.Context, _ string, toolName string, _ map[string]interface{}, _ string) (*proxy.ToolExecutionResult, error) {
	f.calls++
	return &proxy.ToolExecutionResult{
		Backend:    "primary",
		StatusCode: http.StatusOK,
		LatencyMs:  12,
		Result:     map[string]interface{}{"executed": toolName},
	}, nil
}

type fakeApprovalMCPExecutor struct {
	calls int
}

func (f *fakeApprovalMCPExecutor) ExecuteTool(_ context.Context, _, serverName, toolName string, _ map[string]interface{}, _ string) (*models.MCPToolExecutionResult, error) {
	f.calls++
	return &models.MCPToolExecutionResult{
		Server:     serverName,
		Tool:       toolName,
		StatusCode: http.StatusOK,
		LatencyMs:  15,
		Result:     map[string]interface{}{"ok": true},
	}, nil
}

func TestApprovalWorkflow_ToolRequireApprovalLifecycleAndSingleUseReplay(t *testing.T) {
	gin.SetMode(gin.TestMode)

	approvals := newFakeApprovalStore()
	exec := &fakeApprovalToolExecutor{}
	rbac := proxy.NewRBACEngine(&fakeApprovalPolicyFinder{
		policies: []models.GovernancePolicy{{
			ID:           "pol-hitl",
			Name:         "Require Human Review on Production Deploy",
			Effect:       "require_approval",
			Role:         "*",
			AgentPattern: "*",
			ToolPattern:  "deploy_*",
			Priority:     10,
			Enabled:      true,
		}},
	})
	adm := proxy.NewAdmissionController(rbac, nil, nil, nil)

	toolHandler := &ToolGatewayHandler{
		gateway:   exec,
		admission: adm,
		approvals: approvals,
		tools:     &fakeToolDefFinder{},
	}
	appHandler := NewApprovalHandler(approvals, nil, nil)

	orgID := "org-1"
	gwKey := &models.GatewayAPIKey{ID: "key-1", UserID: "user-1", OrgID: &orgID}

	// 1. Initial call triggers 202 APPROVAL_REQUIRED without executing tool
	reqBody, _ := json.Marshal(ExecuteToolRequest{Args: map[string]interface{}{"env": "prod"}})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/tools/deploy_service/execute", bytes.NewReader(reqBody))
	c.Params = gin.Params{{Key: "toolName", Value: "deploy_service"}}
	c.Set("gatewayKey", gwKey)
	toolHandler.ExecuteTool(c)

	require.Equal(t, http.StatusAccepted, w.Code)
	assert.Equal(t, 0, exec.calls)
	var holdResp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &holdResp))
	approvalID := holdResp["approvalId"].(string)
	require.NotEmpty(t, approvalID)

	// 2. Replaying while still pending must fail with 403
	wPending := httptest.NewRecorder()
	cPending, _ := gin.CreateTestContext(wPending)
	cPending.Request = httptest.NewRequest(http.MethodPost, "/v1/tools/deploy_service/execute", bytes.NewReader(reqBody))
	cPending.Request.Header.Set("X-Prism-Approval-ID", approvalID)
	cPending.Params = gin.Params{{Key: "toolName", Value: "deploy_service"}}
	cPending.Set("gatewayKey", gwKey)
	toolHandler.ExecuteTool(cPending)
	assert.Equal(t, http.StatusForbidden, wPending.Code)
	assert.Equal(t, 0, exec.calls)

	// 3. Human approves request via ApprovalHandler
	wApprove := httptest.NewRecorder()
	cApprove, _ := gin.CreateTestContext(wApprove)
	cApprove.Request = httptest.NewRequest(http.MethodPost, "/api/approvals/"+approvalID+"/approve", bytes.NewBufferString(`{"note":"LGTM"}`))
	cApprove.Params = gin.Params{{Key: "id", Value: approvalID}}
	cApprove.Set("userId", "user-1")
	appHandler.Approve(cApprove)
	require.Equal(t, http.StatusOK, wApprove.Code)

	// 4. Gateway agent polls approval status
	wPoll := httptest.NewRecorder()
	cPoll, _ := gin.CreateTestContext(wPoll)
	cPoll.Request = httptest.NewRequest(http.MethodGet, "/v1/approvals/"+approvalID, nil)
	cPoll.Params = gin.Params{{Key: "approvalId", Value: approvalID}}
	cPoll.Set("gatewayKey", gwKey)
	appHandler.GetForGateway(cPoll)
	require.Equal(t, http.StatusOK, wPoll.Code)

	// 5. Swapped arguments with the approved ID must be rejected
	tamperedBody, _ := json.Marshal(ExecuteToolRequest{Args: map[string]interface{}{"env": "staging-hack"}})
	wTamper := httptest.NewRecorder()
	cTamper, _ := gin.CreateTestContext(wTamper)
	cTamper.Request = httptest.NewRequest(http.MethodPost, "/v1/tools/deploy_service/execute", bytes.NewReader(tamperedBody))
	cTamper.Request.Header.Set("X-Prism-Approval-ID", approvalID)
	cTamper.Params = gin.Params{{Key: "toolName", Value: "deploy_service"}}
	cTamper.Set("gatewayKey", gwKey)
	toolHandler.ExecuteTool(cTamper)
	assert.Equal(t, http.StatusForbidden, wTamper.Code)
	assert.Equal(t, 0, exec.calls)

	// 6. Exact replay executes tool and consumes the single-use approval
	wExec := httptest.NewRecorder()
	cExec, _ := gin.CreateTestContext(wExec)
	cExec.Request = httptest.NewRequest(http.MethodPost, "/v1/tools/deploy_service/execute", bytes.NewReader(reqBody))
	cExec.Request.Header.Set("X-Prism-Approval-ID", approvalID)
	cExec.Params = gin.Params{{Key: "toolName", Value: "deploy_service"}}
	cExec.Set("gatewayKey", gwKey)
	toolHandler.ExecuteTool(cExec)
	require.Equal(t, http.StatusOK, wExec.Code)
	assert.Equal(t, 1, exec.calls)

	// 7. Second replay with same consumed approval ID must fail
	wSecond := httptest.NewRecorder()
	cSecond, _ := gin.CreateTestContext(wSecond)
	cSecond.Request = httptest.NewRequest(http.MethodPost, "/v1/tools/deploy_service/execute", bytes.NewReader(reqBody))
	cSecond.Request.Header.Set("X-Prism-Approval-ID", approvalID)
	cSecond.Params = gin.Params{{Key: "toolName", Value: "deploy_service"}}
	cSecond.Set("gatewayKey", gwKey)
	toolHandler.ExecuteTool(cSecond)
	assert.Equal(t, http.StatusForbidden, wSecond.Code)
	assert.Equal(t, 1, exec.calls)
}

func TestApprovalWorkflow_ToolRequireApprovalFlagAndMCPHoldReplay(t *testing.T) {
	gin.SetMode(gin.TestMode)

	approvals := newFakeApprovalStore()
	exec := &fakeApprovalToolExecutor{}
	mcpExec := &fakeApprovalMCPExecutor{}

	// Even with no governance policy, tool.RequireApproval == true creates a hold.
	toolHandler := &ToolGatewayHandler{
		gateway:   exec,
		approvals: approvals,
		tools: &fakeToolDefFinder{
			tools: map[string]*models.Tool{
				"wipe_cache": {ID: "t1", Name: "wipe_cache", RequireApproval: true},
			},
		},
	}
	orgID := "org-1"
	gwKey := &models.GatewayAPIKey{ID: "key-1", UserID: "user-1", OrgID: &orgID}
	wTool := httptest.NewRecorder()
	cTool, _ := gin.CreateTestContext(wTool)
	cTool.Request = httptest.NewRequest(http.MethodPost, "/v1/tools/wipe_cache/execute", bytes.NewBufferString(`{"args":{"region":"us"}}`))
	cTool.Params = gin.Params{{Key: "toolName", Value: "wipe_cache"}}
	cTool.Set("gatewayKey", gwKey)
	toolHandler.ExecuteTool(cTool)
	require.Equal(t, http.StatusAccepted, wTool.Code)
	assert.Equal(t, 0, exec.calls)

	// MCP tools/call with require_approval RBAC policy pauses and replays cleanly with _approval_id
	rbac := proxy.NewRBACEngine(&fakeApprovalPolicyFinder{
		policies: []models.GovernancePolicy{{
			ID:           "pol-mcp",
			Name:         "MCP Drop Guard",
			Effect:       "require_approval",
			Role:         "*",
			AgentPattern: "*",
			ToolPattern:  "db__drop_*",
			Priority:     1,
			Enabled:      true,
		}},
	})
	adm := proxy.NewAdmissionController(rbac, nil, nil, nil)
	mcpHandler := NewMCPServerHandler(nil, nil, mcpExec, nil, adm, nil, nil, nil, "")
	mcpHandler.SetApprovals(approvals, nil)

	rpcPayload := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"db__drop_table","arguments":{"table":"temp_logs"}}}`
	wMCP := httptest.NewRecorder()
	cMCP, _ := gin.CreateTestContext(wMCP)
	cMCP.Request = httptest.NewRequest(http.MethodPost, "/v1/mcp", bytes.NewBufferString(rpcPayload))
	cMCP.Set("gatewayKey", gwKey)
	mcpHandler.Handle(cMCP)

	require.Equal(t, http.StatusAccepted, wMCP.Code)
	assert.Equal(t, 0, mcpExec.calls)

	var rpcResp mcpJSONRPCResponse
	require.NoError(t, json.Unmarshal(wMCP.Body.Bytes(), &rpcResp))
	require.NotNil(t, rpcResp.Error)
	assert.Equal(t, -32003, rpcResp.Error.Code)
	dataMap := rpcResp.Error.Data.(map[string]interface{})
	mcpApprovalID := dataMap["approvalId"].(string)

	// Approve then replay via _approval_id
	_, err := approvals.Decide(context.Background(), mcpApprovalID, "user-1", "admin-1", models.ApprovalStatusApproved, "approved")
	require.NoError(t, err)

	replayRPC := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"db__drop_table","arguments":{"table":"temp_logs","_approval_id":"` + mcpApprovalID + `"}}}`
	wReplay := httptest.NewRecorder()
	cReplay, _ := gin.CreateTestContext(wReplay)
	cReplay.Request = httptest.NewRequest(http.MethodPost, "/v1/mcp", bytes.NewBufferString(replayRPC))
	cReplay.Set("gatewayKey", gwKey)
	mcpHandler.Handle(cReplay)
	require.Equal(t, http.StatusOK, wReplay.Code)
	assert.Equal(t, 1, mcpExec.calls)
}
