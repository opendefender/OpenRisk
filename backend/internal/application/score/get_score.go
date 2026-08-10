// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package score assembles the inputs the canonical scoring model needs and hands
// them to internal/domain/scoring, which does the arithmetic.
//
// The split matters: this package knows about repositories, tenants and
// permissions; it knows NOTHING about weights, bands or bounds. Every number the
// product displays comes out of the domain package, so there is exactly one
// implementation of "what is the score" and exactly one of "what is it called".
package score

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/domain/scoring"
)

// ---------------------------------------------------------------------------
// Ports — narrow, OPTIONAL and nil-safe, satisfied structurally by the existing
// repositories so no shared interface has to change (mocks stay intact).
//
// A missing or erroring source degrades ITS OWN factor, never the whole score:
// the factor is flagged unavailable and its weight is redistributed. That is the
// opposite of scoring it zero, which would read as "excellent" — the single most
// dangerous failure mode a security score can have.
// ---------------------------------------------------------------------------

type (
	// RiskCounter counts the register by criticality (concrete method on
	// GormRiskRepository, deliberately off the domain port).
	RiskCounter interface {
		CountRisksByCriticality(ctx context.Context, tenantID uuid.UUID) (map[string]int, error)
	}
	// RiskReader loads one risk, tenant-scoped.
	RiskReader interface {
		GetByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.Risk, error)
	}
	// RiskLister lists the tenant's risks (used for the asset scope's linked
	// exposure, and for the tenant scope's mitigation credit).
	RiskLister interface {
		ListRisksForFinancial(ctx context.Context, tenantID uuid.UUID) ([]domain.Risk, error)
	}
	// AssetReader loads one asset, tenant-scoped.
	AssetReader interface {
		GetByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.Asset, error)
	}
	// ComplianceCounter reports coverage: total applicable controls and gaps.
	ComplianceCounter interface {
		ControlTotals(ctx context.Context, tenantID uuid.UUID) (total, gaps int, err error)
	}
	// VulnStatsReader reports the tenant's vulnerability posture.
	VulnStatsReader interface {
		Stats(ctx context.Context, tenantID uuid.UUID) (*domain.VulnStats, error)
	}
	// VulnListReader lists a tenant's vulnerabilities (asset scope).
	VulnListReader interface {
		List(ctx context.Context, tenantID uuid.UUID, q domain.VulnerabilityQuery) (*domain.PaginatedResult[domain.Vulnerability], error)
	}
	// IncidentPressureReader reports open/critical incident counts.
	IncidentPressureReader interface {
		OpenIncidentCounts(ctx context.Context, tenantID uuid.UUID) (open, criticalOpen int, err error)
	}
	// MitigationReader lists the mitigation plans of one risk, for the residual
	// score. Signature mirrors GormMitigationRepository exactly (string tenant,
	// no context) — this port exists to consume that repository, not to redesign it.
	MitigationReader interface {
		ListByRiskID(tenantID string, riskID uuid.UUID) ([]domain.Mitigation, error)
	}
)

// UseCase computes the canonical score for any scope.
type UseCase struct {
	riskCounts RiskCounter
	risk       RiskReader
	risks      RiskLister
	assets     AssetReader
	compliance ComplianceCounter
	vulnStats  VulnStatsReader
	vulns      VulnListReader
	incidents  IncidentPressureReader
	mitigation MitigationReader
	now        func() time.Time
}

// New builds a use case with no sources attached.
func New() *UseCase {
	return &UseCase{now: func() time.Time { return time.Now().UTC() }}
}

func (uc *UseCase) WithRiskCounts(s RiskCounter) *UseCase     { uc.riskCounts = s; return uc }
func (uc *UseCase) WithRisk(s RiskReader) *UseCase            { uc.risk = s; return uc }
func (uc *UseCase) WithRisks(s RiskLister) *UseCase           { uc.risks = s; return uc }
func (uc *UseCase) WithAssets(s AssetReader) *UseCase         { uc.assets = s; return uc }
func (uc *UseCase) WithCompliance(s ComplianceCounter) *UseCase { uc.compliance = s; return uc }
func (uc *UseCase) WithVulnStats(s VulnStatsReader) *UseCase  { uc.vulnStats = s; return uc }
func (uc *UseCase) WithVulns(s VulnListReader) *UseCase       { uc.vulns = s; return uc }
func (uc *UseCase) WithIncidents(s IncidentPressureReader) *UseCase { uc.incidents = s; return uc }
func (uc *UseCase) WithMitigations(s MitigationReader) *UseCase { uc.mitigation = s; return uc }

// Execute computes the score for one scope.
//
// id is required for the risk and asset scopes and ignored for tenant.
func (uc *UseCase) Execute(ctx context.Context, tenantID uuid.UUID, scope scoring.Scope, id uuid.UUID) (*scoring.Result, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewValidationError("tenant is required")
	}

	switch scope {
	case scoring.ScopeTenant:
		return uc.tenantScore(ctx, tenantID)
	case scoring.ScopeRisk:
		if id == uuid.Nil {
			return nil, domain.NewValidationError("id is required for scope=risk")
		}
		return uc.riskScore(ctx, tenantID, id)
	case scoring.ScopeAsset:
		if id == uuid.Nil {
			return nil, domain.NewValidationError("id is required for scope=asset")
		}
		return uc.assetScore(ctx, tenantID, id)
	}
	return nil, domain.NewValidationError("unknown scope: " + string(scope))
}

// ---------------------------------------------------------------------------
// Tenant
// ---------------------------------------------------------------------------

func (uc *UseCase) tenantScore(ctx context.Context, tenantID uuid.UUID) (*scoring.Result, error) {
	in := scoring.TenantInput{}

	if uc.riskCounts != nil {
		if counts, err := uc.riskCounts.CountRisksByCriticality(ctx, tenantID); err == nil {
			in.CriticalRisks = counts["critical"]
			in.HighRisks = counts["high"]
			in.TotalRisks = counts["critical"] + counts["high"] + counts["medium"] + counts["low"]
			in.HasRiskData = true
		}
	}

	if uc.compliance != nil {
		if total, gaps, err := uc.compliance.ControlTotals(ctx, tenantID); err == nil && total > 0 {
			in.ApplicableControls = total
			in.ImplementedControls = total - gaps
			in.HasComplianceData = true
		}
	}

	if uc.vulnStats != nil {
		if stats, err := uc.vulnStats.Stats(ctx, tenantID); err == nil && stats != nil {
			in.KEVVulnerabilities = int(stats.KEVCount)
			in.CriticalVulnerabilities = int(stats.BySeverity["critical"])
			in.HasVulnData = true
		}
	}

	if uc.incidents != nil {
		if open, criticalOpen, err := uc.incidents.OpenIncidentCounts(ctx, tenantID); err == nil {
			in.OpenIncidents = open
			in.CriticalOpenIncidents = criticalOpen
			in.HasIncidentData = true
		}
	}

	// Tenant-level mitigation credit: the average declared effectiveness across
	// the register. Averaging (rather than taking the best) is deliberate — one
	// well-treated risk does not mitigate the organisation.
	in.MitigationEffectiveness = uc.averageEffectiveness(ctx, tenantID)

	result := scoring.ComputeTenant(in, uc.now())
	return &result, nil
}

// averageEffectiveness returns the mean declared mitigation effectiveness across
// the tenant's risks, or 0 when unknown.
func (uc *UseCase) averageEffectiveness(ctx context.Context, tenantID uuid.UUID) float64 {
	if uc.risks == nil {
		return 0
	}
	risks, err := uc.risks.ListRisksForFinancial(ctx, tenantID)
	if err != nil || len(risks) == 0 {
		return 0
	}
	var sum float64
	for i := range risks {
		if e := risks[i].MitigationEffectiveness; e != nil {
			sum += *e
		}
	}
	return sum / float64(len(risks))
}

// ---------------------------------------------------------------------------
// Risk
// ---------------------------------------------------------------------------

func (uc *UseCase) riskScore(ctx context.Context, tenantID, riskID uuid.UUID) (*scoring.Result, error) {
	if uc.risk == nil {
		return nil, domain.NewNotFoundError("risk", riskID)
	}
	r, err := uc.risk.GetByID(ctx, riskID, tenantID)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, domain.NewNotFoundError("risk", riskID)
	}

	in := scoring.RiskInput{
		Probability: r.Probability,
		Impact:      r.Impact,
	}

	// Asset criticality: the highest among the risk's linked assets. GetByID
	// preloads the many2many, so no extra query.
	for _, a := range r.Assets {
		if a == nil {
			continue
		}
		if f := a.Criticality.ScoreFactor(); f > in.AssetCriticality {
			in.AssetCriticality = f
			in.HasAsset = true
		}
	}

	in.MitigationEffectiveness = uc.riskEffectiveness(ctx, tenantID, r)

	result := scoring.ComputeRisk(in, uc.now())
	return &result, nil
}

// riskEffectiveness resolves the mitigation credit for one risk.
//
// Two sources, in order of authority:
//  1. the explicitly declared MitigationEffectiveness (the CRQ field an analyst
//     sets deliberately) — a stated figure beats an inferred one;
//  2. otherwise, the mean completion of the risk's mitigation plans, so a risk
//     whose treatment is half done gets half the credit.
func (uc *UseCase) riskEffectiveness(ctx context.Context, tenantID uuid.UUID, r *domain.Risk) float64 {
	if r.MitigationEffectiveness != nil {
		return *r.MitigationEffectiveness
	}
	if uc.mitigation == nil {
		return 0
	}
	plans, err := uc.mitigation.ListByRiskID(tenantID.String(), r.ID)
	if err != nil || len(plans) == 0 {
		return 0
	}

	var sum float64
	var counted int
	for _, p := range plans {
		if p.Status == domain.MitigationCancelled {
			// A cancelled plan mitigates nothing; counting it would let someone
			// lower a residual score by planning and abandoning work.
			continue
		}
		sum += float64(p.Progress) / 100
		counted++
	}
	if counted == 0 {
		return 0
	}
	// Capped: a treatment plan reduces exposure, it does not eliminate the risk.
	// 0.9 leaves the residual score honestly non-zero however complete the plan.
	const maxCredit = 0.9
	avg := sum / float64(counted)
	if avg > maxCredit {
		avg = maxCredit
	}
	return avg
}

// ---------------------------------------------------------------------------
// Asset
// ---------------------------------------------------------------------------

func (uc *UseCase) assetScore(ctx context.Context, tenantID, assetID uuid.UUID) (*scoring.Result, error) {
	if uc.assets == nil {
		return nil, domain.NewNotFoundError("asset", assetID)
	}
	a, err := uc.assets.GetByID(ctx, assetID, tenantID)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, domain.NewNotFoundError("asset", assetID)
	}

	in := scoring.AssetInput{Criticality: a.Criticality.ScoreFactor()}

	// Linked-risk exposure: the worst INHERENT risk score among the risks that
	// touch this asset, on the canonical scale — not a raw P×I×AC, which would
	// mix two scales in one breakdown.
	if uc.risks != nil {
		if risks, err := uc.risks.ListRisksForFinancial(ctx, tenantID); err == nil {
			for i := range risks {
				if !riskTouchesAsset(&risks[i], assetID) {
					continue
				}
				linked := scoring.ComputeRisk(scoring.RiskInput{
					Probability:      risks[i].Probability,
					Impact:           risks[i].Impact,
					AssetCriticality: in.Criticality,
					HasAsset:         true,
				}, uc.now()).Inherent
				if linked > in.MaxLinkedRiskScore {
					in.MaxLinkedRiskScore = linked
				}
				in.HasLinkedRisks = true
			}
		}
	}

	if uc.vulns != nil {
		page, err := uc.vulns.List(ctx, tenantID, domain.VulnerabilityQuery{
			AssetID:  &assetID,
			PageSize: 200,
		})
		if err == nil && page != nil {
			in.HasVulnData = true
			for _, v := range page.Data {
				if v.CVSSScore > in.MaxCVSS {
					in.MaxCVSS = v.CVSSScore
				}
				in.OpenVulnerabilities++
			}
		}
	}

	// Internet exposure is NOT wired: domain.Asset carries no reachability signal
	// today (no tags, and Type alone does not settle it — a "Server" may be
	// air-gapped or public). HasExposureData stays false, so the factor is
	// reported as "not measured" and its weight is redistributed. Guessing here
	// would put a number on the screen that nothing in the database supports.
	result := scoring.ComputeAsset(in, uc.now())
	return &result, nil
}

func riskTouchesAsset(r *domain.Risk, assetID uuid.UUID) bool {
	if r.AssetID != nil && *r.AssetID == assetID {
		return true
	}
	for _, a := range r.Assets {
		if a != nil && a.ID == assetID {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Preview — the live figure a form shows while you drag a slider
// ---------------------------------------------------------------------------

// PreviewInput is the body of POST /score/preview. It carries the values the user
// is currently editing, so the form can show the real score BEFORE saving.
//
// It is deliberately the same computation, in the same package, as the persisted
// score: a preview that used a different formula would be a lie told at exactly
// the moment the user is deciding.
type PreviewInput struct {
	Scope                   string   `json:"scope"`
	Probability             *float64 `json:"probability"`
	Impact                  *float64 `json:"impact"`
	AssetCriticality        *float64 `json:"asset_criticality"`
	MitigationEffectiveness *float64 `json:"mitigation_effectiveness"`
}

// Preview computes a score from raw form values, persisting nothing.
func (uc *UseCase) Preview(in PreviewInput) (*scoring.Result, error) {
	scope, ok := scoring.ParseScope(in.Scope)
	if !ok {
		scope = scoring.ScopeRisk // the only scope a form edits today
	}
	if scope != scoring.ScopeRisk {
		return nil, domain.NewValidationError("preview supports scope=risk only")
	}

	risk := scoring.RiskInput{}
	if in.Probability != nil {
		risk.Probability = *in.Probability
	}
	if in.Impact != nil {
		risk.Impact = *in.Impact
	}
	if in.AssetCriticality != nil {
		risk.AssetCriticality = *in.AssetCriticality
		risk.HasAsset = true
	}
	if in.MitigationEffectiveness != nil {
		risk.MitigationEffectiveness = *in.MitigationEffectiveness
	}

	result := scoring.ComputeRisk(risk, uc.now())
	return &result, nil
}
