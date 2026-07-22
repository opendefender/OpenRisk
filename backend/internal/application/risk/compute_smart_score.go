// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package risk

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/crq"
	"github.com/opendefender/openrisk/pkg/scoring"
	"gorm.io/datatypes"
)

// --- Narrow ports the smart-score assembler reads from. Each is OPTIONAL (nil-safe
// via the With* setters): a missing source simply makes its factor(s) degrade
// gracefully, never fails the computation. Concrete repositories satisfy them. ---

// VulnLister lists a tenant's vulnerabilities (factors 3, 6, 8). Satisfied by
// domain.VulnerabilityRepository.
type VulnLister interface {
	List(ctx context.Context, tenantID uuid.UUID, q domain.VulnerabilityQuery) (*domain.PaginatedResult[domain.Vulnerability], error)
}

// ComplianceCoverageSource computes control maturity (factor 4). Satisfied by
// domain.ComplianceRepository.
type ComplianceCoverageSource interface {
	ListFrameworks(ctx context.Context, tenantID uuid.UUID) ([]domain.ComplianceFramework, error)
	ListControlsByFramework(ctx context.Context, tenantID uuid.UUID, frameworkID uuid.UUID) ([]domain.ComplianceControl, error)
}

// IncidentCounter counts past incidents on an asset (factor 5).
type IncidentCounter interface {
	CountForAsset(ctx context.Context, tenantID uuid.UUID, assetID uuid.UUID, assetName string) (int, error)
}

// SmartScorePersister writes the computed score back onto the risk. Satisfied by
// GormRiskRepository via its concrete UpdateSmartScore method (kept off the domain
// port so RiskRepository mocks stay valid).
type SmartScorePersister interface {
	UpdateSmartScore(ctx context.Context, riskID, tenantID uuid.UUID, score float64, level string, factors datatypes.JSON, computedAt time.Time) error
}

// ComputeSmartScoreUseCase assembles the eight-factor SmartInput for a risk from
// the risk itself, its asset, the vulnerability register, compliance posture,
// incident history and the CRQ model, then runs the pure engine
// (pkg/scoring.ComputeSmart) with the tenant's configured weights. Tenant-scoped.
type ComputeSmartScoreUseCase struct {
	riskRepo    domain.RiskRepository
	weightsRepo domain.RiskScoringWeightsRepository

	// Optional signal sources — nil-safe.
	assetRepo  domain.AssetRepository
	vulnLister VulnLister
	compliance ComplianceCoverageSource
	incidents  IncidentCounter
	quantifier *crq.Quantifier
	persister  SmartScorePersister
}

// NewComputeSmartScoreUseCase builds the use case with its two required ports.
func NewComputeSmartScoreUseCase(riskRepo domain.RiskRepository, weightsRepo domain.RiskScoringWeightsRepository) *ComputeSmartScoreUseCase {
	return &ComputeSmartScoreUseCase{riskRepo: riskRepo, weightsRepo: weightsRepo}
}

// WithAssetRepo wires the asset lookup (business criticality + exposure).
func (uc *ComputeSmartScoreUseCase) WithAssetRepo(r domain.AssetRepository) *ComputeSmartScoreUseCase {
	uc.assetRepo = r
	return uc
}

// WithVulnLister wires the vulnerability register (vulns, exploitability, threat).
func (uc *ComputeSmartScoreUseCase) WithVulnLister(v VulnLister) *ComputeSmartScoreUseCase {
	uc.vulnLister = v
	return uc
}

// WithCompliance wires compliance posture (control maturity).
func (uc *ComputeSmartScoreUseCase) WithCompliance(c ComplianceCoverageSource) *ComputeSmartScoreUseCase {
	uc.compliance = c
	return uc
}

// WithIncidents wires the incident counter (incident history).
func (uc *ComputeSmartScoreUseCase) WithIncidents(i IncidentCounter) *ComputeSmartScoreUseCase {
	uc.incidents = i
	return uc
}

// WithQuantifier wires the CRQ engine (financial value).
func (uc *ComputeSmartScoreUseCase) WithQuantifier(q *crq.Quantifier) *ComputeSmartScoreUseCase {
	uc.quantifier = q
	return uc
}

// WithPersister wires the write-back of the computed score onto the risk.
func (uc *ComputeSmartScoreUseCase) WithPersister(p SmartScorePersister) *ComputeSmartScoreUseCase {
	uc.persister = p
	return uc
}

// Execute computes the smart score for a risk using the tenant's stored weights.
// When persist is true and a persister is wired, the score (and its frozen
// breakdown) is written back onto the risk so the register can sort/badge on it.
func (uc *ComputeSmartScoreUseCase) Execute(ctx context.Context, tenantID, riskID uuid.UUID, persist bool) (*scoring.SmartResult, error) {
	weights, err := uc.effectiveWeights(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	return uc.computeWith(ctx, tenantID, riskID, weights, persist)
}

// Preview computes the smart score with caller-supplied weights WITHOUT persisting
// — this powers the "adjust the weighting live" simulator in the config UI.
func (uc *ComputeSmartScoreUseCase) Preview(ctx context.Context, tenantID, riskID uuid.UUID, weights scoring.FactorWeights) (*scoring.SmartResult, error) {
	return uc.computeWith(ctx, tenantID, riskID, weights, false)
}

func (uc *ComputeSmartScoreUseCase) effectiveWeights(ctx context.Context, tenantID uuid.UUID) (scoring.FactorWeights, error) {
	w, err := uc.weightsRepo.GetByTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if w == nil {
		return scoring.DefaultFactorWeights(), nil
	}
	return w.ToFactorWeights(), nil
}

func (uc *ComputeSmartScoreUseCase) computeWith(ctx context.Context, tenantID, riskID uuid.UUID, weights scoring.FactorWeights, persist bool) (*scoring.SmartResult, error) {
	r, err := uc.riskRepo.GetByID(ctx, riskID, tenantID)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, domain.NewNotFoundError("risk", riskID)
	}

	in := uc.assembleInput(ctx, tenantID, r)
	result := scoring.ComputeSmart(in, weights)

	if persist && uc.persister != nil {
		factorsJSON, mErr := json.Marshal(result.Factors)
		if mErr == nil {
			// Best-effort persistence: a write failure must not fail the read.
			_ = uc.persister.UpdateSmartScore(ctx, riskID, tenantID, result.Score, string(result.Criticality), datatypes.JSON(factorsJSON), time.Now())
		}
	}
	return &result, nil
}

// assembleInput gathers every factor signal for a risk. Each source is best-effort:
// an error or a nil dependency leaves that factor at its graceful-degradation value.
func (uc *ComputeSmartScoreUseCase) assembleInput(ctx context.Context, tenantID uuid.UUID, r *domain.Risk) scoring.SmartInput {
	in := scoring.SmartInput{}

	// Resolve the linked asset once (feeds factors 1, 2, 3, 5, 6, 8).
	var asset *domain.Asset
	if r.AssetID != nil && uc.assetRepo != nil {
		if a, err := uc.assetRepo.GetByID(ctx, *r.AssetID, tenantID); err == nil {
			asset = a
		}
	}
	// Fall back to the first many2many-linked asset: risks created via `asset_ids`
	// populate the risk_assets join (preloaded by GetByID) rather than the single
	// AssetID pointer, so without this the asset-derived factors would never fire.
	if asset == nil && len(r.Assets) > 0 {
		asset = r.Assets[0]
	}

	// 1. Business criticality — asset criticality factor, else the risk's own impact.
	if asset != nil {
		in.BusinessCriticalityFactor = asset.Criticality.ScoreFactor()
	} else if r.Impact > 0 {
		// Map the risk impact (0–10) onto the asset-criticality range (0.1–3.0).
		f := r.Impact * 0.3
		if f < 0.1 {
			f = 0.1
		}
		in.BusinessCriticalityFactor = f
	}

	// 2. Internet exposure — risk tags first (explicit), else the asset type.
	in.InternetExposure = deriveExposure(r, asset)

	// 3/6/8. Vulnerabilities, exploitability, active threats — from the register.
	if uc.vulnLister != nil && asset != nil {
		q := domain.NewVulnerabilityQuery()
		q.Limit = 200
		q.AssetID = &asset.ID
		q.Statuses = []string{
			string(domain.VulnStatusOpen), string(domain.VulnStatusTriaged),
			string(domain.VulnStatusInRemediation),
		}
		if page, err := uc.vulnLister.List(ctx, tenantID, q); err == nil && page != nil {
			applyVulnSignals(&in, page.Data)
		}
	}
	// A CTI-sourced risk correlates with live threat intel even without a matched
	// vulnerability row (factor 8 floor).
	if r.Source == domain.SourceCTIAuto || (r.SourceCVEID != nil && *r.SourceCVEID != "") {
		if in.ActiveThreatSignal < 0.7 {
			in.ActiveThreatSignal = 0.7
		}
	}

	// 4. Control maturity — implemented-control coverage for the risk's frameworks
	//    (or tenant-wide when the risk links none).
	if uc.compliance != nil {
		if maturity, assessed := uc.controlMaturity(ctx, tenantID, r); assessed {
			in.ControlsAssessed = true
			in.ControlMaturity = maturity
		}
	}

	// 5. Incident history — past incidents recorded against the asset.
	if uc.incidents != nil && asset != nil {
		if n, err := uc.incidents.CountForAsset(ctx, tenantID, asset.ID, asset.Name); err == nil {
			in.IncidentCount = n
		}
	}

	// 7. Financial value — annual loss expectancy via the CRQ engine.
	if uc.quantifier != nil {
		assessment := uc.quantifier.Assess(financialInputs(r), string(r.Criticality))
		in.ALEXAF = assessment.ALE.XAF
	}

	return in
}

// applyVulnSignals folds the asset's open vulnerabilities into the vuln/exploit/
// threat factors: count + worst CVSS, worst-case exploit signals, KEV → threat.
func applyVulnSignals(in *scoring.SmartInput, vulns []domain.Vulnerability) {
	in.VulnerabilityCount = len(vulns)
	for _, v := range vulns {
		if v.CVSSScore > in.MaxCVSS {
			in.MaxCVSS = v.CVSSScore
		}
		if v.EPSS > in.EPSS {
			in.EPSS = v.EPSS
		}
		if v.KEV {
			in.KEV = true
			in.ActiveThreatSignal = 1.0 // CISA-KEV = actively exploited in the wild
		}
		if v.ExploitAvailable {
			in.ExploitAvailable = true
		}
		if maturityRank(v.ExploitMaturity) > maturityRank(in.ExploitMaturity) {
			in.ExploitMaturity = v.ExploitMaturity
		}
	}
}

// maturityRank orders exploit-maturity labels so we keep the worst one.
func maturityRank(m string) int {
	switch strings.ToLower(m) {
	case "high":
		return 3
	case "functional":
		return 2
	case "poc":
		return 1
	default:
		return 0
	}
}

// controlMaturity returns the implemented-control coverage [0,1] for the risk's
// frameworks (or tenant-wide) and whether any control existed to assess.
func (uc *ComputeSmartScoreUseCase) controlMaturity(ctx context.Context, tenantID uuid.UUID, r *domain.Risk) (float64, bool) {
	frameworks, err := uc.compliance.ListFrameworks(ctx, tenantID)
	if err != nil || len(frameworks) == 0 {
		return 0, false
	}

	// Restrict to the risk's linked frameworks when it names any (by name match).
	wanted := map[string]bool{}
	for _, f := range r.Frameworks {
		wanted[strings.ToLower(strings.TrimSpace(f))] = true
	}

	var total, implemented int
	for _, fw := range frameworks {
		if len(wanted) > 0 && !wanted[strings.ToLower(fw.Name)] {
			continue
		}
		controls, cErr := uc.compliance.ListControlsByFramework(ctx, tenantID, fw.ID)
		if cErr != nil {
			continue
		}
		for _, ctrl := range controls {
			if ctrl.Status == domain.ControlStatusNotApplicable {
				continue
			}
			total++
			if ctrl.Status == domain.ControlStatusImplemented {
				implemented++
			}
		}
	}
	if total == 0 {
		return 0, false
	}
	return float64(implemented) / float64(total), true
}

// deriveExposure maps a risk+asset to a 0–1 internet-exposure signal. Explicit
// risk tags win; otherwise the asset type gives a coarse hint; unknown → 0.5.
func deriveExposure(r *domain.Risk, asset *domain.Asset) float64 {
	for _, t := range r.Tags {
		switch strings.ToLower(strings.TrimSpace(t)) {
		case "internet-facing", "internet_facing", "public", "external", "public-facing", "exposed":
			return 1.0
		case "internal", "private", "isolated", "air-gapped":
			return 0.1
		}
	}
	if asset != nil {
		switch strings.ToLower(asset.Type) {
		case "cloud", "saas", "application", "website", "api":
			return ExposureCloud
		case "database", "storage", "server":
			return 0.4
		case "laptop", "workstation", "user":
			return 0.2
		}
	}
	return 0.5
}

// ExposureCloud is the default exposure for internet-reachable service types.
const ExposureCloud = 0.6

// financialInputs (shared with financial_summary.go) maps a risk's stored monetary
// drivers onto the CRQ engine input.
