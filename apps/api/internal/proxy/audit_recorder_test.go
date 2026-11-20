package proxy

import (
	"strings"
	"testing"

	"github.com/lib/pq"
	"github.com/roozylabs/prism/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashPayload(t *testing.T) {
	prompt := "Explain Quantum Computing"
	h1 := HashPayload(prompt)
	h2 := HashPayload(prompt)

	assert.NotEmpty(t, h1)
	assert.Equal(t, h1, h2)
	assert.Len(t, h1, 64) // SHA-256 hex string is 64 characters
	assert.Equal(t, "", HashPayload(""))
}

func TestComputeSignatureHash(t *testing.T) {
	reqID := "req-12345"
	userID := "user-admin"
	pHash := HashPayload("Hello World")
	rHash := HashPayload("AI Response")
	model := "gpt-4o"

	sig1 := ComputeSignatureHash(reqID, userID, pHash, rHash, model)
	sig2 := ComputeSignatureHash(reqID, userID, pHash, rHash, model)

	assert.NotEmpty(t, sig1)
	assert.Equal(t, sig1, sig2)
	assert.Len(t, sig1, 64)

	// Tamper test
	sigTampered := ComputeSignatureHash(reqID, userID, pHash, HashPayload("Tampered Response"), model)
	assert.NotEqual(t, sig1, sigTampered)
}

func TestDeriveAuditHMACKey(t *testing.T) {
	explicit := DeriveAuditHMACKey("my-dedicated-hmac-secret", "fallback-encryption-key")
	assert.Equal(t, []byte("my-dedicated-hmac-secret"), explicit)

	derived1 := DeriveAuditHMACKey("", "encryption-secret-0123456789abcdef")
	derived2 := DeriveAuditHMACKey("  ", "encryption-secret-0123456789abcdef")
	derivedOther := DeriveAuditHMACKey("", "different-secret-0123456789abcdef")

	require.Len(t, derived1, 32)
	assert.Equal(t, derived1, derived2)
	assert.NotEqual(t, derived1, derivedOther)
	assert.Nil(t, DeriveAuditHMACKey("", ""))
}

func TestAuditRecorder_V2HMACSigningAndTamperDetection(t *testing.T) {
	key := DeriveAuditHMACKey("audit-signing-secret-v2", "")
	rec := NewAuditRecorderWithKey(nil, key)

	gwKeyID := "gk-123"
	agentID := "agent-sec"
	trail := &models.AIAuditTrail{
		ID:               "audit-1",
		RequestID:        "req-v2-1",
		UserID:           "user-alice",
		OrgID:            "org-acme",
		GatewayKeyID:     &gwKeyID,
		AgentID:          &agentID,
		UserRole:         "developer",
		ModelSlug:        "claude-3-7-sonnet",
		FailoverChain:    pq.StringArray{"anthropic:cred-1", "openai:cred-2"},
		ToolsInvoked:     pq.StringArray{"linear__create_issue"},
		MCPServersCalled: pq.StringArray{"linear"},
		PromptTokens:     120,
		CompletionTokens: 45,
		TotalTokens:      165,
		TotalCostUSD:     0.012345,
		StatusCode:       200,
		LatencyMS:        420,
		TTFTMS:           95,
		PromptHash:       HashPayload(`[{"role":"user","content":"Audit this"}]`),
		ResponseHash:     HashPayload("All checks passed"),
		ComplianceStatus: "compliant",
	}

	rec.Sign(trail)
	require.True(t, strings.HasPrefix(trail.SignatureHash, AuditSignatureV2Prefix))

	res := rec.VerifyTrail(trail)
	require.True(t, res.Valid)

	t.Run("rejects wrong verification key", func(t *testing.T) {
		wrong := NewAuditRecorderWithKey(nil, []byte("attacker-key"))
		assert.False(t, wrong.VerifyTrail(trail).Valid)
	})

	t.Run("rejects missing verification key on v2 record", func(t *testing.T) {
		unkeyed := NewAuditRecorder(nil)
		assert.False(t, unkeyed.VerifyTrail(trail).Valid)
	})

	tamperCases := []struct {
		name   string
		mutate func(clone *models.AIAuditTrail)
	}{
		{"OrgID", func(c *models.AIAuditTrail) { c.OrgID = "org-other" }},
		{"AgentID", func(c *models.AIAuditTrail) { other := "agent-spoof"; c.AgentID = &other }},
		{"TotalCostUSD", func(c *models.AIAuditTrail) { c.TotalCostUSD = 0.0001 }},
		{"ComplianceStatus", func(c *models.AIAuditTrail) { c.ComplianceStatus = "denied" }},
		{"ToolsInvoked", func(c *models.AIAuditTrail) { c.ToolsInvoked = pq.StringArray{"bash"} }},
		{"FailoverChain", func(c *models.AIAuditTrail) { c.FailoverChain = pq.StringArray{"tampered"} }},
	}

	for _, tc := range tamperCases {
		t.Run("detects tampering of "+tc.name, func(t *testing.T) {
			cloned := *trail
			cloned.FailoverChain = append(pq.StringArray{}, trail.FailoverChain...)
			cloned.ToolsInvoked = append(pq.StringArray{}, trail.ToolsInvoked...)
			cloned.MCPServersCalled = append(pq.StringArray{}, trail.MCPServersCalled...)
			tc.mutate(&cloned)
			out := rec.VerifyTrail(&cloned)
			assert.False(t, out.Valid)
		})
	}
}

func TestAuditRecorder_LegacySignatureCompatibility(t *testing.T) {
	rec := NewAuditRecorderWithKey(nil, []byte("hmac-key"))
	trail := &models.AIAuditTrail{
		ID:           "legacy-1",
		RequestID:    "req-legacy",
		UserID:       "user-1",
		PromptHash:   HashPayload("prompt"),
		ResponseHash: HashPayload("resp"),
		ModelSlug:    "gpt-4o",
	}
	trail.SignatureHash = ComputeSignatureHash(trail.RequestID, trail.UserID, trail.PromptHash, trail.ResponseHash, trail.ModelSlug)

	assert.True(t, rec.VerifyTrail(trail).Valid)

	trail.ResponseHash = HashPayload("forged")
	assert.False(t, rec.VerifyTrail(trail).Valid)
}
