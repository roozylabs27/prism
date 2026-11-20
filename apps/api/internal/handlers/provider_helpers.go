package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"unicode"

	"github.com/roozylabs/prism/internal/models"
)

type createProviderRequest struct {
	Name            string `json:"name"`
	Slug            string `json:"slug"`
	BaseURL         string `json:"baseUrl"`
	Type            string `json:"type"`
	Enabled         *bool  `json:"enabled"`
	RoutingStrategy string `json:"routingStrategy"`
}

type updateProviderRequest struct {
	Name            *string `json:"name"`
	Slug            *string `json:"slug"`
	BaseURL         *string `json:"baseUrl"`
	Type            *string `json:"type"`
	Enabled         *bool   `json:"enabled"`
	RoutingStrategy *string `json:"routingStrategy"`
}

func buildProviderFromCreate(req createProviderRequest, userID string) models.Provider {
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	providerType := strings.TrimSpace(req.Type)
	if providerType == "" {
		providerType = "custom"
	}
	slug := slugifyProvider(req.Slug)
	if slug == "" {
		slug = uniqueProviderSlug(req.Name)
	}
	return models.Provider{
		UserID:          userID,
		Name:            strings.TrimSpace(req.Name),
		Slug:            slug,
		BaseURL:         strings.TrimSpace(req.BaseURL),
		Type:            providerType,
		Enabled:         enabled,
		RoutingStrategy: strings.TrimSpace(req.RoutingStrategy),
	}
}

func applyProviderUpdate(existing *models.Provider, req updateProviderRequest) {
	if req.Name != nil {
		existing.Name = strings.TrimSpace(*req.Name)
	}
	if req.Slug != nil {
		if slug := slugifyProvider(*req.Slug); slug != "" {
			existing.Slug = slug
		}
	}
	if req.BaseURL != nil {
		existing.BaseURL = strings.TrimSpace(*req.BaseURL)
	}
	if req.Type != nil {
		if t := strings.TrimSpace(*req.Type); t != "" {
			existing.Type = t
		}
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if req.RoutingStrategy != nil {
		if rs := strings.TrimSpace(*req.RoutingStrategy); rs != "" {
			existing.RoutingStrategy = rs
		}
	}
}

func slugifyProvider(raw string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(raw)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevDash = false
		} else if unicode.IsLetter(r) || unicode.IsDigit(r) {
			// Skip non-ASCII letters/digits rather than emitting invalid slug chars,
			// collapsing adjacent separators into a single dash.
			continue
		} else if !prevDash && b.Len() > 0 {
			b.WriteByte('-')
			prevDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func uniqueProviderSlug(name string) string {
	base := slugifyProvider(name)
	if base == "" {
		base = "provider"
	}
	var buf [3]byte
	_, _ = rand.Read(buf[:])
	return base + "-" + hex.EncodeToString(buf[:])
}

func trimString(s string) string {
	return strings.TrimSpace(s)
}
