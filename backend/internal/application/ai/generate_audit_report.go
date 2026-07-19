// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package ai

import (
	"context"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	llm "github.com/opendefender/openrisk/pkg/ai"
)

// AuditReportResult wraps the executive audit narrative with its provider.
type AuditReportResult struct {
	Report      llm.AuditNarrative `json:"report"`
	GeneratedBy string             `json:"generated_by"`
}

// GenerateAuditReportUseCase synthesises an executive report from a completed (or
// in-progress) audit campaign (spec §12.4). It composes the tenant-scoped audit,
// the gap analysis for its framework (program-wide when the audit has none), and
// the open remediation count — all optional/nil-safe.
type GenerateAuditReportUseCase struct {
	assistant llm.Assistant
	audits    AuditReader
	gap       GapAnalyzer // optional
}

func NewGenerateAuditReportUseCase(assistant llm.Assistant, audits AuditReader) *GenerateAuditReportUseCase {
	return &GenerateAuditReportUseCase{assistant: assistant, audits: audits}
}

// WithGapAnalyzer enriches the report with a live gap analysis.
func (uc *GenerateAuditReportUseCase) WithGapAnalyzer(g GapAnalyzer) *GenerateAuditReportUseCase {
	uc.gap = g
	return uc
}

// Execute builds the audit context and asks the assistant to write the report.
func (uc *GenerateAuditReportUseCase) Execute(ctx context.Context, tenantID, auditID uuid.UUID, locale string) (*AuditReportResult, error) {
	audit, err := uc.audits.GetAuditByID(ctx, auditID, tenantID)
	if err != nil {
		return nil, err
	}
	if audit == nil {
		return nil, domain.NewNotFoundError("audit", auditID)
	}

	auditCtx := llm.AuditContext{
		Locale:  llm.Locale(locale),
		Title:   audit.Title,
		Type:    string(audit.Type),
		Status:  string(audit.Status),
		Auditor: audit.Auditor,
		Scope:   audit.Scope,
	}

	// Gap analysis (framework-scoped when the audit targets one, else program-wide).
	if uc.gap != nil {
		frameworkID := uuid.Nil
		if audit.FrameworkID != nil {
			frameworkID = *audit.FrameworkID
		}
		if ga, err := uc.gap.Execute(ctx, tenantID, frameworkID); err == nil && ga != nil {
			auditCtx.TotalControls = ga.TotalControls
			auditCtx.Gaps = ga.TotalGaps
			auditCtx.Implemented = ga.TotalControls - ga.TotalGaps
			if ga.TotalControls > 0 {
				auditCtx.PercentComplete = float64(auditCtx.Implemented) / float64(ga.TotalControls) * 100
			}
			if len(ga.Frameworks) == 1 {
				auditCtx.FrameworkName = ga.Frameworks[0].FrameworkName
			}
			// Top open gaps (bounded) for the findings section.
			for i, g := range ga.Gaps {
				if i >= 8 {
					break
				}
				auditCtx.TopGaps = append(auditCtx.TopGaps, llm.AuditGapItem{
					Code:   g.ReferenceCode,
					Name:   g.Name,
					Status: string(g.Status),
				})
			}
		}
	}

	// Open remediation plans tied to this audit.
	if remediations, err := uc.audits.ListRemediations(ctx, tenantID, domain.RemediationFilter{AuditID: &auditID}); err == nil {
		for _, r := range remediations {
			if r.Status != domain.RemediationStatusCompleted && r.Status != domain.RemediationStatusCancelled {
				auditCtx.OpenRemediations++
			}
		}
	}

	report, generatedBy := invoke(uc.assistant, func(a llm.Assistant) (llm.AuditNarrative, error) {
		return a.SummarizeAudit(ctx, auditCtx)
	})
	return &AuditReportResult{Report: report, GeneratedBy: generatedBy}, nil
}
