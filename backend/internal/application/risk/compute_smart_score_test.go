// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package risk

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/scoring"
	"gorm.io/datatypes"
)

// --- Minimal test doubles for the smart-score ports. ---

type mockWeightsRepo struct {
	get    *domain.RiskScoringWeights
	getErr error
	saved  *domain.RiskScoringWeights
}

func (m *mockWeightsRepo) GetByTenant(ctx context.Context, tenantID uuid.UUID) (*domain.RiskScoringWeights, error) {
	return m.get, m.getErr
}
func (m *mockWeightsRepo) Upsert(ctx context.Context, w *domain.RiskScoringWeights) error {
	m.saved = w
	return nil
}

// mockAssetRepo implements domain.AssetRepository; only GetByID is exercised.
type mockAssetRepo struct {
	asset *domain.Asset
}

func (m *mockAssetRepo) Create(ctx context.Context, a *domain.Asset) error { return nil }
func (m *mockAssetRepo) GetByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.Asset, error) {
	if m.asset != nil && m.asset.ID == id {
		return m.asset, nil
	}
	return nil, nil
}
func (m *mockAssetRepo) List(ctx context.Context, tenantID uuid.UUID) ([]domain.Asset, error) {
	return nil, nil
}
func (m *mockAssetRepo) Update(ctx context.Context, a *domain.Asset) error              { return nil }
func (m *mockAssetRepo) Delete(ctx context.Context, id, tenantID uuid.UUID) error        { return nil }
func (m *mockAssetRepo) CreateSnapshot(ctx context.Context, s *domain.AssetSnapshot) error { return nil }
func (m *mockAssetRepo) ListSnapshots(ctx context.Context, assetID, tenantID uuid.UUID) ([]domain.AssetSnapshot, error) {
	return nil, nil
}

type mockVulnLister struct {
	vulns []domain.Vulnerability
}

func (m *mockVulnLister) List(ctx context.Context, tenantID uuid.UUID, q domain.VulnerabilityQuery) (*domain.PaginatedResult[domain.Vulnerability], error) {
	return &domain.PaginatedResult[domain.Vulnerability]{Data: m.vulns, Total: int64(len(m.vulns))}, nil
}

type mockIncidentCounter struct{ n int }

func (m *mockIncidentCounter) CountForAsset(ctx context.Context, tenantID, assetID uuid.UUID, name string) (int, error) {
	return m.n, nil
}

type mockPersister struct {
	calledScore float64
	calledLevel string
	called      bool
}

func (m *mockPersister) UpdateSmartScore(ctx context.Context, riskID, tenantID uuid.UUID, score float64, level string, factors datatypes.JSON, computedAt time.Time) error {
	m.called = true
	m.calledScore = score
	m.calledLevel = level
	return nil
}

func TestComputeSmartScore_Success(t *testing.T) {
	tenantID := uuid.New()
	riskID := uuid.New()
	assetID := uuid.New()

	risk := &domain.Risk{ID: riskID, TenantID: tenantID, AssetID: &assetID, Impact: 8, Tags: []string{"internet-facing"}}
	asset := &domain.Asset{ID: assetID, TenantID: tenantID, Name: "web-01", Type: "Server", Criticality: domain.CriticalityCritical}

	riskRepo := &MockRiskRepository{
		getByIDFunc: func(ctx context.Context, id, tid uuid.UUID) (*domain.Risk, error) {
			if id == riskID && tid == tenantID {
				return risk, nil
			}
			return nil, nil
		},
	}
	persister := &mockPersister{}

	uc := NewComputeSmartScoreUseCase(riskRepo, &mockWeightsRepo{}).
		WithAssetRepo(&mockAssetRepo{asset: asset}).
		WithVulnLister(&mockVulnLister{vulns: []domain.Vulnerability{
			{CVSSScore: 9.8, KEV: true, ExploitAvailable: true, ExploitMaturity: "high", EPSS: 0.9},
			{CVSSScore: 7.2},
		}}).
		WithIncidents(&mockIncidentCounter{n: 3}).
		WithPersister(persister)

	res, err := uc.Execute(context.Background(), tenantID, riskID, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Score < 60 {
		t.Fatalf("expected a high score for a critical, exposed, KEV-exploited asset, got %.2f", res.Score)
	}
	if len(res.Factors) != len(scoring.FactorKeys) {
		t.Fatalf("expected %d factors, got %d", len(scoring.FactorKeys), len(res.Factors))
	}
	// The exploitability factor should be near maxed by the KEV vuln.
	var exp float64
	for _, f := range res.Factors {
		if f.Key == scoring.FactorExploitability {
			exp = f.Value
		}
	}
	if exp < 0.9 {
		t.Fatalf("KEV vuln should max exploitability, got %.2f", exp)
	}
	if !persister.called {
		t.Fatal("expected the score to be persisted")
	}
	if persister.calledScore != res.Score {
		t.Fatalf("persisted score %.2f != returned score %.2f", persister.calledScore, res.Score)
	}
}

func TestComputeSmartScore_NotFound(t *testing.T) {
	riskRepo := &MockRiskRepository{
		getByIDFunc: func(ctx context.Context, id, tid uuid.UUID) (*domain.Risk, error) { return nil, nil },
	}
	uc := NewComputeSmartScoreUseCase(riskRepo, &mockWeightsRepo{})
	_, err := uc.Execute(context.Background(), uuid.New(), uuid.New(), false)
	if err == nil {
		t.Fatal("expected a not-found error")
	}
}

func TestComputeSmartScore_DegradesWithoutOptionalDeps(t *testing.T) {
	tenantID := uuid.New()
	riskID := uuid.New()
	risk := &domain.Risk{ID: riskID, TenantID: tenantID, Impact: 2}

	riskRepo := &MockRiskRepository{
		getByIDFunc: func(ctx context.Context, id, tid uuid.UUID) (*domain.Risk, error) {
			return risk, nil
		},
	}
	// No asset/vuln/compliance/incident/quantifier wired: must not crash, low score.
	uc := NewComputeSmartScoreUseCase(riskRepo, &mockWeightsRepo{})
	res, err := uc.Execute(context.Background(), tenantID, riskID, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Criticality != scoring.CriticalityLow {
		t.Fatalf("a bare risk should be Low, got %s (%.2f)", res.Criticality, res.Score)
	}
}

func TestComputeSmartScore_PreviewUsesSuppliedWeights(t *testing.T) {
	tenantID := uuid.New()
	riskID := uuid.New()
	assetID := uuid.New()
	risk := &domain.Risk{ID: riskID, TenantID: tenantID, AssetID: &assetID}
	asset := &domain.Asset{ID: assetID, TenantID: tenantID, Criticality: domain.CriticalityCritical}

	riskRepo := &MockRiskRepository{
		getByIDFunc: func(ctx context.Context, id, tid uuid.UUID) (*domain.Risk, error) { return risk, nil },
	}
	uc := NewComputeSmartScoreUseCase(riskRepo, &mockWeightsRepo{}).WithAssetRepo(&mockAssetRepo{asset: asset})

	// All weight on business criticality (others explicitly 0) → score ≈ 100 ×
	// (3.0/3.0) = 100. A full 8-key map is what the config UI always sends; a
	// partial map would blend with the engine defaults (safety net).
	weights := scoring.FactorWeights{
		scoring.FactorBusinessCriticality: 1.0,
		scoring.FactorInternetExposure:    0,
		scoring.FactorVulnerabilities:     0,
		scoring.FactorControlMaturity:     0,
		scoring.FactorIncidentHistory:     0,
		scoring.FactorExploitability:      0,
		scoring.FactorFinancialValue:      0,
		scoring.FactorThreatIntel:         0,
	}
	res, err := uc.Preview(context.Background(), tenantID, riskID, weights)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Score < 99 {
		t.Fatalf("all-business-weight on a critical asset should be ~100, got %.2f", res.Score)
	}
}

func TestGetRiskWeights_DefaultWhenUnset(t *testing.T) {
	uc := NewGetRiskWeightsUseCase(&mockWeightsRepo{get: nil})
	w, err := uc.Execute(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Defaults from the engine: vulnerabilities is the heaviest at 0.20.
	if w.Vulnerabilities != 0.20 {
		t.Fatalf("expected default vuln weight 0.20, got %.4f", w.Vulnerabilities)
	}
}

func TestUpdateRiskWeights_Success(t *testing.T) {
	repo := &mockWeightsRepo{}
	uc := NewUpdateRiskWeightsUseCase(repo)
	_, err := uc.Execute(context.Background(), uuid.New(), uuid.New(), UpdateRiskWeightsInput{
		BusinessCriticality: 0.2, InternetExposure: 0.1, Vulnerabilities: 0.3,
		ControlMaturity: 0.1, IncidentHistory: 0.05, Exploitability: 0.15,
		FinancialValue: 0.05, ThreatIntel: 0.05,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.saved == nil || repo.saved.Vulnerabilities != 0.3 {
		t.Fatal("expected the weights to be upserted with the new vuln weight")
	}
}

func TestUpdateRiskWeights_RejectsAllZero(t *testing.T) {
	uc := NewUpdateRiskWeightsUseCase(&mockWeightsRepo{})
	_, err := uc.Execute(context.Background(), uuid.New(), uuid.New(), UpdateRiskWeightsInput{})
	if err == nil {
		t.Fatal("expected validation error when all weights are zero")
	}
}

func TestUpdateRiskWeights_RejectsOutOfRange(t *testing.T) {
	uc := NewUpdateRiskWeightsUseCase(&mockWeightsRepo{})
	_, err := uc.Execute(context.Background(), uuid.New(), uuid.New(), UpdateRiskWeightsInput{Vulnerabilities: 1.5})
	if err == nil {
		t.Fatal("expected validation error when a weight exceeds 1.0")
	}
}
