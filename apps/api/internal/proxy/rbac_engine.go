package proxy

import (
	"context"
	"fmt"
	"strings"

	"github.com/roozylabs/prism/internal/models"
)

type PolicyFinder interface {
	ListByUserID(ctx context.Context, userID string) ([]models.GovernancePolicy, error)
}

type RBACEngine struct {
	policies PolicyFinder
}

func NewRBACEngine(policies PolicyFinder) *RBACEngine {
	return &RBACEngine{policies: policies}
}

func matchPattern(pattern, target string) bool {
	if pattern == "" || pattern == "*" {
		return true
	}
	if target == "" {
		return false
	}
	return globMatch([]rune(strings.ToLower(pattern)), []rune(strings.ToLower(target)))
}

// globMatch evaluates target against an OS-independent wildcard pattern where
// '*' matches zero or more runes (including slashes) and '?' matches any single
// rune. Substring matching is intentionally avoided so "gpt-4" never matches "gpt-4o-mini".
func globMatch(pattern, target []rune) bool {
	pIdx, tIdx := 0, 0
	starIdx, matchIdx := -1, 0

	for tIdx < len(target) {
		if pIdx < len(pattern) && (pattern[pIdx] == '?' || pattern[pIdx] == target[tIdx]) {
			pIdx++
			tIdx++
		} else if pIdx < len(pattern) && pattern[pIdx] == '*' {
			starIdx = pIdx
			matchIdx = tIdx
			pIdx++
		} else if starIdx != -1 {
			pIdx = starIdx + 1
			matchIdx++
			tIdx = matchIdx
		} else {
			return false
		}
	}

	for pIdx < len(pattern) && pattern[pIdx] == '*' {
		pIdx++
	}
	return pIdx == len(pattern)
}

func (e *RBACEngine) Evaluate(ctx context.Context, userID string, req models.RBACEvaluationRequest) (*models.RBACEvaluationResult, error) {
	policies, err := e.policies.ListByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list governance policies: %w", err)
	}

	if len(policies) == 0 {
		return &models.RBACEvaluationResult{
			Allowed:        true,
			Reason:         "no explicit governance policies defined; default allow",
			EvaluatedCount: 0,
		}, nil
	}

	var matchedAllow *models.GovernancePolicy
	evaluated := 0

	for i := range policies {
		p := &policies[i]
		if !p.Enabled {
			continue
		}
		evaluated++

		// Match role if specified and not wildcard/admin
		if p.Role != "" && p.Role != "*" && req.Role != "" && !strings.EqualFold(p.Role, req.Role) {
			continue
		}

		// Match Agent Pattern
		if !matchPattern(p.AgentPattern, req.AgentName) {
			continue
		}

		// Match Model Pattern
		if req.ModelSlug != "" && !matchPattern(p.ModelPattern, req.ModelSlug) {
			continue
		}

		// Match Tool Pattern (if policy targets specific tool, request tool or prompt payload must match)
		if p.ToolPattern != "" && p.ToolPattern != "*" {
			matched := matchPattern(p.ToolPattern, req.ToolName)
			if !matched && req.UserPrompt != "" {
				matched = matchPattern(p.ToolPattern, req.UserPrompt)
			}
			if !matched {
				continue
			}
		}

		// Match Resource Pattern (if policy targets specific resource, request resource or prompt payload must match)
		if p.ResourcePattern != "" && p.ResourcePattern != "*" {
			matched := matchPattern(p.ResourcePattern, req.ResourceName)
			if !matched && req.UserPrompt != "" {
				matched = matchPattern(p.ResourcePattern, req.UserPrompt)
			}
			if !matched {
				continue
			}
		}

		// If policy matches:
		if strings.EqualFold(p.Effect, "deny") {
			return &models.RBACEvaluationResult{
				Allowed:        false,
				MatchedPolicy:  p,
				Reason:         fmt.Sprintf("access denied by policy %q (%s)", p.Name, p.ID),
				EvaluatedCount: evaluated,
			}, nil
		}

		if matchedAllow == nil && strings.EqualFold(p.Effect, "allow") {
			matchedAllow = p
		}
	}

	if matchedAllow != nil {
		return &models.RBACEvaluationResult{
			Allowed:        true,
			MatchedPolicy:  matchedAllow,
			Reason:         fmt.Sprintf("access granted by policy %q (%s)", matchedAllow.Name, matchedAllow.ID),
			EvaluatedCount: evaluated,
		}, nil
	}

	return &models.RBACEvaluationResult{
		Allowed:        true,
		Reason:         "no matching deny policy found; default allow",
		EvaluatedCount: evaluated,
	}, nil
}
