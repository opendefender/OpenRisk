package services

import (
	"testing"

	"github.com/opendefender/openrisk/internal/core/domain"
)

func TestComputeRiskScore_NoAssets(t testing.T) {
	s := ComputeRiskScore(, , nil)
	if s != . {
		t.Fatalf("expected . got %v", s)
	}
}

func TestComputeRiskScore_WithAssets(t testing.T) {
	assets := []domain.Asset{
		{Criticality: domain.CriticalityLow},
		{Criticality: domain.CriticalityHigh},
	}
	s := ComputeRiskScore(, , assets) // base=, factors=(.+.)/=. => .
	if s != . {
		t.Fatalf("expected . got %v", s)
	}
}
