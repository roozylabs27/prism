package proxy

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/lib/pq"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/repository"
)

const AuditSignatureV2Prefix = "v2:"

type AuditRecorder struct {
	repo    *repository.AuditTrailRepository
	hmacKey []byte
}

func NewAuditRecorder(repo *repository.AuditTrailRepository) *AuditRecorder {
	return &AuditRecorder{repo: repo}
}

// NewAuditRecorderWithKey creates an AuditRecorder configured with a keyed
// HMAC-SHA256 secret for tamper-evident v2 signatures.
func NewAuditRecorderWithKey(repo *repository.AuditTrailRepository, hmacKey []byte) *AuditRecorder {
	r := &AuditRecorder{repo: repo}
	r.SetHMACKey(hmacKey)
	return r
}

// SetHMACKey configures the HMAC key used for signing and verifying v2 audit
// records, cloning the input slice to isolate caller mutations.
func (r *AuditRecorder) SetHMACKey(hmacKey []byte) {
	if r == nil {
		return
	}
	if len(hmacKey) == 0 {
		r.hmacKey = nil
		return
	}
	r.hmacKey = append([]byte(nil), hmacKey...)
}

// DeriveAuditHMACKey returns explicitKey bytes when configured, or derives a
// domain-separated 32-byte key from encryptionKey when non-empty.
func DeriveAuditHMACKey(explicitKey, encryptionKey string) []byte {
	if trimmed := strings.TrimSpace(explicitKey); trimmed != "" {
		return []byte(trimmed)
	}
	if trimmedEnc := strings.TrimSpace(encryptionKey); trimmedEnc != "" {
		mac := hmac.New(sha256.New, []byte(trimmedEnc))
		_, _ = mac.Write([]byte("prism-audit-hmac-v2"))
		return mac.Sum(nil)
	}
	return nil
}

func HashPayload(payload string) string {
	if payload == "" {
		return ""
	}
	hash := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(hash[:])
}

// ComputeSignatureHash preserves the legacy unkeyed 5-field SHA-256 signature
// for pre-v2 audit trail verification compatibility.
func ComputeSignatureHash(reqID, userID, promptHash, responseHash, modelSlug string) string {
	raw := fmt.Sprintf("%s:%s:%s:%s:%s", reqID, userID, promptHash, responseHash, modelSlug)
	hash := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(hash[:])
}

type canonicalAuditEnvelope struct {
	RequestID         string   `json:"requestId"`
	OrgID             string   `json:"orgId"`
	UserID            string   `json:"userId"`
	GatewayKeyID      string   `json:"gatewayKeyId"`
	AgentID           string   `json:"agentId"`
	UserRole          string   `json:"userRole"`
	ModelSlug         string   `json:"modelSlug"`
	PromptHash        string   `json:"promptHash"`
	ResponseHash      string   `json:"responseHash"`
	PromptTokens      int      `json:"promptTokens"`
	CompletionTokens  int      `json:"completionTokens"`
	TotalTokens       int      `json:"totalTokens"`
	TotalCostUSD      string   `json:"totalCostUsd"`
	StatusCode        int      `json:"statusCode"`
	LatencyMS         int      `json:"latencyMs"`
	TTFTMS            int      `json:"ttftMs"`
	ComplianceStatus  string   `json:"complianceStatus"`
	FailoverChain     []string `json:"failoverChain"`
	ToolsInvoked      []string `json:"toolsInvoked"`
	ResourcesAccessed []string `json:"resourcesAccessed"`
	MCPServersCalled  []string `json:"mcpServersCalled"`
}

func derefOrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func nonNilSlice(s pq.StringArray) []string {
	if len(s) == 0 {
		return []string{}
	}
	out := make([]string, len(s))
	copy(out, s)
	return out
}

func normalizeTrailDefaults(trail *models.AIAuditTrail) {
	if trail == nil {
		return
	}
	if trail.UserRole == "" {
		trail.UserRole = "developer"
	}
	if trail.ComplianceStatus == "" {
		trail.ComplianceStatus = "compliant"
	}
	if trail.FailoverChain == nil {
		trail.FailoverChain = pq.StringArray{}
	}
	if trail.ToolsInvoked == nil {
		trail.ToolsInvoked = pq.StringArray{}
	}
	if trail.ResourcesAccessed == nil {
		trail.ResourcesAccessed = pq.StringArray{}
	}
	if trail.MCPServersCalled == nil {
		trail.MCPServersCalled = pq.StringArray{}
	}
}

// ComputeTrailHMAC computes a keyed HMAC-SHA256 signature prefixed with "v2:"
// over all canonical financial, tenant, and security fields of an AIAuditTrail.
func ComputeTrailHMAC(key []byte, trail *models.AIAuditTrail) string {
	if trail == nil || len(key) == 0 {
		return ""
	}
	env := canonicalAuditEnvelope{
		RequestID:         trail.RequestID,
		OrgID:             trail.OrgID,
		UserID:            trail.UserID,
		GatewayKeyID:      derefOrEmpty(trail.GatewayKeyID),
		AgentID:           derefOrEmpty(trail.AgentID),
		UserRole:          trail.UserRole,
		ModelSlug:         trail.ModelSlug,
		PromptHash:        trail.PromptHash,
		ResponseHash:      trail.ResponseHash,
		PromptTokens:      trail.PromptTokens,
		CompletionTokens:  trail.CompletionTokens,
		TotalTokens:       trail.TotalTokens,
		TotalCostUSD:      strconv.FormatFloat(trail.TotalCostUSD, 'f', 8, 64),
		StatusCode:        trail.StatusCode,
		LatencyMS:         trail.LatencyMS,
		TTFTMS:            trail.TTFTMS,
		ComplianceStatus:  trail.ComplianceStatus,
		FailoverChain:     nonNilSlice(trail.FailoverChain),
		ToolsInvoked:      nonNilSlice(trail.ToolsInvoked),
		ResourcesAccessed: nonNilSlice(trail.ResourcesAccessed),
		MCPServersCalled:  nonNilSlice(trail.MCPServersCalled),
	}
	raw, _ := json.Marshal(env)
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(raw)
	return AuditSignatureV2Prefix + hex.EncodeToString(mac.Sum(nil))
}

// Sign applies canonical defaults and populates SignatureHash on trail using
// keyed HMAC-SHA256 ("v2:") when configured, or legacy SHA-256 otherwise.
func (r *AuditRecorder) Sign(trail *models.AIAuditTrail) {
	if trail == nil {
		return
	}
	normalizeTrailDefaults(trail)
	if r != nil && len(r.hmacKey) > 0 {
		trail.SignatureHash = ComputeTrailHMAC(r.hmacKey, trail)
		return
	}
	trail.SignatureHash = ComputeSignatureHash(
		trail.RequestID,
		trail.UserID,
		trail.PromptHash,
		trail.ResponseHash,
		trail.ModelSlug,
	)
}

func (r *AuditRecorder) Record(ctx context.Context, trail *models.AIAuditTrail) error {
	if trail == nil {
		return fmt.Errorf("audit record: trail is nil")
	}
	if trail.SignatureHash == "" {
		r.Sign(trail)
	}
	return r.repo.Create(ctx, trail)
}

// VerifyTrail checks an in-memory AIAuditTrail against the recorder's signing
// key (for "v2:" signatures) or legacy unkeyed signature scheme.
func (r *AuditRecorder) VerifyTrail(trail *models.AIAuditTrail) *models.AuditVerificationResult {
	if trail == nil {
		return &models.AuditVerificationResult{
			Valid:   false,
			Message: "TAMPER WARNING: Audit trail record is nil!",
		}
	}

	var expectedHash string
	var isValid bool
	msg := "Audit record cryptographic signature verified; tamper-free."

	if strings.HasPrefix(trail.SignatureHash, AuditSignatureV2Prefix) {
		if r == nil || len(r.hmacKey) == 0 {
			return &models.AuditVerificationResult{
				AuditID:       trail.ID,
				RequestID:     trail.RequestID,
				Valid:         false,
				SignatureHash: trail.SignatureHash,
				ExpectedHash:  "",
				Message:       "TAMPER WARNING: Audit HMAC key unavailable for v2 signature verification!",
			}
		}
		expectedHash = ComputeTrailHMAC(r.hmacKey, trail)
		isValid = hmac.Equal([]byte(trail.SignatureHash), []byte(expectedHash))
	} else {
		expectedHash = ComputeSignatureHash(
			trail.RequestID,
			trail.UserID,
			trail.PromptHash,
			trail.ResponseHash,
			trail.ModelSlug,
		)
		isValid = hmac.Equal([]byte(trail.SignatureHash), []byte(expectedHash))
	}

	if !isValid {
		msg = "TAMPER WARNING: Cryptographic signature mismatch detected!"
	}

	return &models.AuditVerificationResult{
		AuditID:       trail.ID,
		RequestID:     trail.RequestID,
		Valid:         isValid,
		SignatureHash: trail.SignatureHash,
		ExpectedHash:  expectedHash,
		Message:       msg,
	}
}

func (r *AuditRecorder) VerifyIntegrity(ctx context.Context, id, userID string) (*models.AuditVerificationResult, error) {
	if r == nil || r.repo == nil {
		return nil, fmt.Errorf("audit trail repository not configured")
	}
	trail, err := r.repo.FindByID(ctx, id, userID)
	if err != nil {
		return nil, fmt.Errorf("audit trail not found: %w", err)
	}
	return r.VerifyTrail(trail), nil
}
