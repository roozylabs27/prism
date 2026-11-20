package proxy

import (
	"math"
	"testing"

	"github.com/roozylabs/prism/internal/models"
)

func TestNormalizePolicyWeights(t *testing.T) {
	raw := map[string]float64{
		"task_match": 35,
		"quality":    35,
		"cost":       15,
		"speed":      15,
	}
	norm := NormalizePolicyWeights(raw)
	if raw["task_match"] != 35 {
		t.Fatalf("expected input map not to be mutated, got %v", raw)
	}

	sum := 0.0
	for _, v := range norm {
		sum += v
	}
	if math.Abs(sum-1.0) > 1e-9 {
		t.Fatalf("expected normalized weights to sum to 1.0, got %f (%v)", sum, norm)
	}
	if math.Abs(norm["task_match"]-0.35) > 1e-9 || math.Abs(norm["cost"]-0.15) > 1e-9 {
		t.Fatalf("unexpected normalized values: %v", norm)
	}

	fallback := NormalizePolicyWeights(map[string]float64{"quality": 0, "cost": 0})
	if math.Abs(fallback["task_match"]-0.35) > 1e-9 || math.Abs(fallback["quality"]-0.35) > 1e-9 {
		t.Fatalf("expected fallback to DefaultRoutingPolicy weights, got %v", fallback)
	}
}

func TestScoreCandidatesWithTelemetry_ScaleInvariantWeights(t *testing.T) {
	candidates := []*models.Model{
		{
			ID:               "m-fast",
			Name:             "fast-mini",
			Slug:             "fast-mini",
			QualityScore:     75,
			SpeedScore:       95,
			InputPricePer1M:  0.15,
			OutputPricePer1M: 0.60,
			ContextWindow:    128000,
		},
		{
			ID:               "m-smart",
			Name:             "smart-pro",
			Slug:             "smart-pro",
			QualityScore:     96,
			SpeedScore:       60,
			InputPricePer1M:  3.00,
			OutputPricePer1M: 15.00,
			ContextWindow:    200000,
		},
	}
	chars := RequestCharacteristics{
		Task:          TaskCoding,
		Complexity:    ComplexityMedium,
		ContextTokens: 2000,
	}

	decimalPolicy := &RoutingPolicy{
		Name:        "decimal",
		Weights:     map[string]float64{"task_match": 0.35, "quality": 0.35, "cost": 0.15, "speed": 0.15},
		Constraints: map[string]float64{"max_cost_per_request": 1.0},
	}
	percentPolicy := &RoutingPolicy{
		Name:        "percent",
		Weights:     map[string]float64{"task_match": 35, "quality": 35, "cost": 15, "speed": 15},
		Constraints: map[string]float64{"max_cost_per_request": 1.0},
	}

	sDec := ScoreCandidatesWithTelemetry(candidates, chars, decimalPolicy, nil)
	sPct := ScoreCandidatesWithTelemetry(candidates, chars, percentPolicy, nil)
	if len(sDec) != len(sPct) || len(sDec) != 2 {
		t.Fatalf("expected 2 scored candidates, got dec=%d pct=%d", len(sDec), len(sPct))
	}
	for i := range sDec {
		if math.Abs(sDec[i].Score-sPct[i].Score) > 1e-6 {
			t.Fatalf("score mismatch for %s: decimal=%f percent=%f", sDec[i].Model.Slug, sDec[i].Score, sPct[i].Score)
		}
	}
}
