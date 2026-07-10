// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package board

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/ai"
)

// GenerateBoardReportUseCase aggregates a tenant's risk and compliance posture,
// asks an ai.Advisor to write the board narrative, and persists the result as a
// DRAFT (human-in-the-loop: it must be reviewed and approved before diffusion).
//
// The Advisor is treated as best-effort: if the primary advisor (typically the
// Claude API) errors, the use case falls back to the deterministic
// TemplateAdvisor, so a board report is always producible — and records honestly,
// in GeneratedByModel, which one actually wrote the prose.
type GenerateBoardReportUseCase struct {
	reports  domain.BoardReportRepository
	risks    RiskPostureSource
	comp     domain.ComplianceRepository
	orgs     OrganizationLookup
	advisor  ai.Advisor
	exposure ExposureModel
	fallback ai.Advisor
}

func NewGenerateBoardReportUseCase(
	reports domain.BoardReportRepository,
	risks RiskPostureSource,
	comp domain.ComplianceRepository,
	orgs OrganizationLookup,
	advisor ai.Advisor,
	exposure ExposureModel,
) *GenerateBoardReportUseCase {
	return &GenerateBoardReportUseCase{
		reports:  reports,
		risks:    risks,
		comp:     comp,
		orgs:     orgs,
		advisor:  advisor,
		exposure: exposure,
		fallback: ai.NewTemplateAdvisor(),
	}
}

// GenerateBoardReportInput carries the caller's choices. Both fields are optional:
// an empty PeriodLabel defaults to the current month, an empty Locale to French.
type GenerateBoardReportInput struct {
	PeriodLabel string
	Locale      string
}

// Execute builds and persists a draft board report for (tenantID). requestedBy is
// recorded as the author.
func (uc *GenerateBoardReportUseCase) Execute(
	ctx context.Context,
	tenantID uuid.UUID,
	requestedBy uuid.UUID,
	input GenerateBoardReportInput,
) (*domain.BoardReport, error) {
	locale := ai.Locale(input.Locale).Normalize()
	period := input.PeriodLabel
	if period == "" {
		period = monthLabel(time.Now(), locale)
	}

	// --- Risk posture ---
	byCrit, err := uc.risks.CountRisksByCriticality(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("aggregate risks: %w", err)
	}
	critical := byCrit["critical"]
	high := byCrit["high"]
	medium := byCrit["medium"]
	low := byCrit["low"]
	total := critical + high + medium + low
	exposure := uc.exposure.Compute(critical, high, medium, low)

	// --- Compliance posture ---
	frameworks, overallPct, err := uc.aggregateCompliance(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("aggregate compliance: %w", err)
	}

	orgName := uc.resolveOrgName(ctx, tenantID)

	posture := ai.BoardPosture{
		Locale:                   locale,
		OrganizationName:         orgName,
		PeriodLabel:              period,
		RisksCritical:            critical,
		RisksHigh:                high,
		RisksMedium:              medium,
		RisksLow:                 low,
		RisksTotal:               total,
		FinancialExposureFCFA:    exposure,
		Frameworks:               toAIFrameworks(frameworks),
		OverallCompliancePercent: overallPct,
	}

	// --- Narrative (LLM best-effort, deterministic fallback) ---
	narrative, generatedBy := uc.narrate(ctx, posture)

	snapshot, _ := json.Marshal(frameworks)

	title := reportTitle(orgName, period, locale)

	report := &domain.BoardReport{
		TenantID:                 tenantID,
		Title:                    title,
		OrganizationName:         orgName,
		PeriodLabel:              period,
		Locale:                   string(locale),
		Status:                   domain.BoardReportDraft,
		RisksCritical:            critical,
		RisksHigh:                high,
		RisksMedium:              medium,
		RisksLow:                 low,
		RisksTotal:               total,
		FinancialExposureFCFA:    exposure,
		OverallCompliancePercent: overallPct,
		FrameworksSnapshot:       datatypes.JSON(snapshot),
		ExecutiveSummary:         narrative.ExecutiveSummary,
		RiskCommentary:           narrative.RiskCommentary,
		ComplianceCommentary:     narrative.ComplianceCommentary,
		FinancialCommentary:      narrative.FinancialCommentary,
		Recommendations:          narrative.Recommendations,
		GeneratedByModel:         generatedBy,
		CreatedBy:                requestedBy,
	}

	if err := uc.reports.Create(ctx, report); err != nil {
		return nil, err
	}
	return report, nil
}

// narrate calls the configured advisor and falls back to the template on error,
// returning the narrative and the name of whichever advisor actually produced it.
func (uc *GenerateBoardReportUseCase) narrate(ctx context.Context, posture ai.BoardPosture) (ai.BoardNarrative, string) {
	if uc.advisor != nil {
		if n, err := uc.advisor.GenerateBoardNarrative(ctx, posture); err == nil {
			return n, uc.advisor.Name()
		}
		// fall through to the deterministic fallback on any advisor error.
	}
	n, _ := uc.fallback.GenerateBoardNarrative(ctx, posture)
	return n, uc.fallback.Name()
}

// aggregateCompliance tallies every framework of the tenant and returns per-framework
// snapshots plus the overall implemented/applicable percentage across all frameworks.
func (uc *GenerateBoardReportUseCase) aggregateCompliance(ctx context.Context, tenantID uuid.UUID) ([]FrameworkSnapshot, float64, error) {
	fws, err := uc.comp.ListFrameworks(ctx, tenantID)
	if err != nil {
		return nil, 0, err
	}
	snapshots := make([]FrameworkSnapshot, 0, len(fws))
	var totalApplicable, totalImplemented int
	for _, fw := range fws {
		controls, err := uc.comp.ListControlsByFramework(ctx, tenantID, fw.ID)
		if err != nil {
			return nil, 0, err
		}
		var implemented, notApplicable int
		for _, c := range controls {
			switch c.Status {
			case domain.ControlStatusImplemented:
				implemented++
			case domain.ControlStatusNotApplicable:
				notApplicable++
			}
		}
		total := len(controls)
		applicable := total - notApplicable
		pct := 0.0
		if applicable > 0 {
			pct = float64(implemented) / float64(applicable) * 100
		}
		snapshots = append(snapshots, FrameworkSnapshot{
			Name:            fw.Name,
			Version:         fw.Version,
			Total:           total,
			Applicable:      applicable,
			Implemented:     implemented,
			PercentComplete: pct,
		})
		totalApplicable += applicable
		totalImplemented += implemented
	}
	overall := 0.0
	if totalApplicable > 0 {
		overall = float64(totalImplemented) / float64(totalApplicable) * 100
	}
	return snapshots, overall, nil
}

func (uc *GenerateBoardReportUseCase) resolveOrgName(ctx context.Context, tenantID uuid.UUID) string {
	if uc.orgs == nil {
		return ""
	}
	org, err := uc.orgs.GetByID(ctx, tenantID)
	if err != nil || org == nil {
		return ""
	}
	return org.Name
}

func toAIFrameworks(fws []FrameworkSnapshot) []ai.FrameworkPosture {
	out := make([]ai.FrameworkPosture, 0, len(fws))
	for _, f := range fws {
		out = append(out, ai.FrameworkPosture{
			Name:            f.Name,
			Version:         f.Version,
			Total:           f.Total,
			Applicable:      f.Applicable,
			Implemented:     f.Implemented,
			PercentComplete: f.PercentComplete,
		})
	}
	return out
}

// monthLabel formats a "Month YYYY" label in the given locale.
func monthLabel(t time.Time, locale ai.Locale) string {
	if locale == ai.LocaleEN {
		return t.Format("January 2006")
	}
	months := [...]string{
		"janvier", "février", "mars", "avril", "mai", "juin",
		"juillet", "août", "septembre", "octobre", "novembre", "décembre",
	}
	return fmt.Sprintf("%s %d", months[int(t.Month())-1], t.Year())
}

func reportTitle(org, period string, locale ai.Locale) string {
	if locale == ai.LocaleEN {
		if org != "" {
			return fmt.Sprintf("Board report — %s — %s", org, period)
		}
		return fmt.Sprintf("Board report — %s", period)
	}
	if org != "" {
		return fmt.Sprintf("Rapport du conseil — %s — %s", org, period)
	}
	return fmt.Sprintf("Rapport du conseil — %s", period)
}
