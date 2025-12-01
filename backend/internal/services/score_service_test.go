package services

import (
	"testing"

	"github.com/opendefender/openrisk/internal/core/domain"
)

func TestComputeRiskScore_NoAssets(t *testing.T) {
	s := ComputeRiskScore(3, 4, nil)
	if s != 12.0 {
		t.Fatalf("expected 12.0 got %v", s)
	}
}

func TestComputeRiskScore_WithAssets(t *testing.T) {
	assets := []*domain.Asset{
		{Criticality: domain.CriticalityLow},
		{Criticality: domain.CriticalityHigh},
	}
	s := ComputeRiskScore(2, 5, assets) // base=10, factors=(0.8+1.25)/2=1.025 => 10.25
	if s != 10.25 {
		t.Fatalf("expected 10.25 got %v", s)
	}
}
