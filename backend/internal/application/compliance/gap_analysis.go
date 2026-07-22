// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package compliance

import (
	"context"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// GapControl is one unsatisfied control surfaced by the gap analysis — a control
// that is neither implemented nor marked not-applicable, i.e. an open compliance
// gap the tenant must remediate.
type GapControl struct {
	ControlID       uuid.UUID            `json:"control_id"`
	FrameworkID     uuid.UUID            `json:"framework_id"`
	FrameworkName   string               `json:"framework_name"`
	ReferenceCode   string               `json:"reference_code"`
	Name            string               `json:"name"`
	Description     string               `json:"description"`
	Status          domain.ControlStatus `json:"status"`
	SourceReference string               `json:"source_reference"`
	EvidenceCount   int                  `json:"evidence_count"`
}

// FrameworkGapSummary rolls up gap counts per framework so the UI can show, at a
// glance, which framework carries the most risk.
type FrameworkGapSummary struct {
	FrameworkID     uuid.UUID `json:"framework_id"`
	FrameworkName   string    `json:"framework_name"`
	Version         string    `json:"version"`
	Total           int       `json:"total"`
	Implemented     int       `json:"implemented"`
	InProgress      int       `json:"in_progress"`
	NotImplemented  int       `json:"not_implemented"`
	NotApplicable   int       `json:"not_applicable"`
	Gaps            int       `json:"gaps"`
	PercentComplete float64   `json:"percent_complete"`
}

// GapAnalysis is the "analyse d'écarts" DTO: every open gap across a tenant's
// frameworks, plus per-framework and overall roll-ups. Computed, not persisted.
type GapAnalysis struct {
	TotalControls int                   `json:"total_controls"`
	TotalGaps     int                   `json:"total_gaps"`
	Frameworks    []FrameworkGapSummary `json:"frameworks"`
	Gaps          []GapControl          `json:"gaps"`
}

// GetGapAnalysisUseCase identifies unsatisfied controls across all of a tenant's
// frameworks (or a single framework when a frameworkID is supplied). It reuses the
// existing repository ports — no new persistence method needed — and stays fully
// tenant-scoped.
type GetGapAnalysisUseCase struct {
	repo domain.ComplianceRepository
}

func NewGetGapAnalysisUseCase(repo domain.ComplianceRepository) *GetGapAnalysisUseCase {
	return &GetGapAnalysisUseCase{repo: repo}
}

// isGap reports whether a control counts as an open compliance gap: everything
// that is not fully implemented and not explicitly out of scope.
func isGap(s domain.ControlStatus) bool {
	return s != domain.ControlStatusImplemented && s != domain.ControlStatusNotApplicable
}

// Execute runs the gap analysis. If frameworkID is uuid.Nil, it spans every
// framework the tenant owns; otherwise it scopes to that single framework.
func (uc *GetGapAnalysisUseCase) Execute(ctx context.Context, tenantID uuid.UUID, frameworkID uuid.UUID) (*GapAnalysis, error) {
	var frameworks []domain.ComplianceFramework

	if frameworkID != uuid.Nil {
		fw, err := uc.repo.GetFrameworkByID(ctx, frameworkID, tenantID)
		if err != nil {
			return nil, err
		}
		if fw == nil {
			return nil, domain.NewNotFoundError("framework", frameworkID)
		}
		frameworks = []domain.ComplianceFramework{*fw}
	} else {
		all, err := uc.repo.ListFrameworks(ctx, tenantID)
		if err != nil {
			return nil, err
		}
		frameworks = all
	}

	result := &GapAnalysis{
		Frameworks: make([]FrameworkGapSummary, 0, len(frameworks)),
		Gaps:       make([]GapControl, 0),
	}

	for _, fw := range frameworks {
		controls, err := uc.repo.ListControlsByFramework(ctx, tenantID, fw.ID)
		if err != nil {
			return nil, err
		}

		// Evidence counts in one grouped query per framework (no N+1).
		evCounts, err := uc.repo.CountEvidencesByFramework(ctx, tenantID, fw.ID)
		if err != nil {
			return nil, err
		}

		summary := FrameworkGapSummary{
			FrameworkID:   fw.ID,
			FrameworkName: fw.Name,
			Version:       fw.Version,
			Total:         len(controls),
		}

		for _, c := range controls {
			switch c.Status {
			case domain.ControlStatusImplemented:
				summary.Implemented++
			case domain.ControlStatusInProgress:
				summary.InProgress++
			case domain.ControlStatusNotApplicable:
				summary.NotApplicable++
			default:
				summary.NotImplemented++
			}

			if isGap(c.Status) {
				summary.Gaps++
				result.Gaps = append(result.Gaps, GapControl{
					ControlID:       c.ID,
					FrameworkID:     fw.ID,
					FrameworkName:   fw.Name,
					ReferenceCode:   c.ReferenceCode,
					Name:            c.Name,
					Description:     c.Description,
					Status:          c.Status,
					SourceReference: c.SourceReference,
					EvidenceCount:   evCounts[c.ID],
				})
			}
		}

		applicable := summary.Total - summary.NotApplicable
		if applicable > 0 {
			summary.PercentComplete = float64(summary.Implemented) / float64(applicable) * 100
		}

		result.TotalControls += summary.Total
		result.TotalGaps += summary.Gaps
		result.Frameworks = append(result.Frameworks, summary)
	}

	return result, nil
}
