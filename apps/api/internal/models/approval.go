package models

import (
	"encoding/json"
	"time"
)

const (
	ApprovalStatusPending  = "pending"
	ApprovalStatusApproved = "approved"
	ApprovalStatusDenied   = "denied"
	ApprovalStatusExpired  = "expired"
	ApprovalStatusConsumed = "consumed"
)

// ApprovalRequest represents a persisted human-in-the-loop hold created when a tool
// or MCP invocation matches a "require_approval" governance policy or a tool with
// require_approval=true.
type ApprovalRequest struct {
	ID            string          `json:"id" db:"id"`
	OrgID         string          `json:"orgId,omitempty" db:"org_id"`
	UserID        string          `json:"userId" db:"user_id"`
	AgentID       string          `json:"agentId,omitempty" db:"agent_id"`
	AgentName     string          `json:"agentName,omitempty" db:"agent_name"`
	ToolName      string          `json:"toolName" db:"tool_name"`
	MCPServerName string          `json:"mcpServerName,omitempty" db:"mcp_server_name"`
	ArgumentsJSON json.RawMessage `json:"argumentsJson" db:"arguments_json"`
	ArgumentsHash string          `json:"argumentsHash" db:"arguments_hash"`
	PolicyID      string          `json:"policyId,omitempty" db:"policy_id"`
	PolicyName    string          `json:"policyName,omitempty" db:"policy_name"`
	Reason        string          `json:"reason,omitempty" db:"reason"`
	Status        string          `json:"status" db:"status"`
	DecidedBy     string          `json:"decidedBy,omitempty" db:"decided_by"`
	DecisionNote  string          `json:"decisionNote,omitempty" db:"decision_note"`
	DecidedAt     *time.Time      `json:"decidedAt,omitempty" db:"decided_at"`
	ExpiresAt     time.Time       `json:"expiresAt" db:"expires_at"`
	ConsumedAt    *time.Time      `json:"consumedAt,omitempty" db:"consumed_at"`
	CreatedAt     time.Time       `json:"createdAt" db:"created_at"`
	UpdatedAt     time.Time       `json:"updatedAt" db:"updated_at"`
}
