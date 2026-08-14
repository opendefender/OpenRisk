// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
// This file is part of the OpenRisk Enterprise Edition and is NOT covered by the
// AGPL; it is licensed under the OpenRisk Commercial License (see LICENSE.commercial).

package ai

import (
	"context"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	llm "github.com/opendefender/openrisk/pkg/ai"
)

// EvidenceAssessmentResult wraps the documentary-compliance verdict with its provider.
type EvidenceAssessmentResult struct {
	Assessment  llm.EvidenceAssessment `json:"assessment"`
	GeneratedBy string                 `json:"generated_by"`
}

// AnalyzeEvidenceUseCase checks whether an uploaded evidence meets the intent of
// the control it is attached to (spec §12.5). It resolves the evidence, its
// control, and the control's framework — all tenant-scoped — and hands the pair to
// the assistant.
//
// Note: this pass reasons from the evidence's filename + description (metadata).
// Extracting text from the stored file (PDF/image OCR) to feed a real excerpt is a
// documented next step; without it the assistant lowers its confidence rather than
// asserting a false "satisfies".
type AnalyzeEvidenceUseCase struct {
	assistant  llm.Assistant
	compliance ComplianceReader
}

func NewAnalyzeEvidenceUseCase(assistant llm.Assistant, compliance ComplianceReader) *AnalyzeEvidenceUseCase {
	return &AnalyzeEvidenceUseCase{assistant: assistant, compliance: compliance}
}

// Execute loads the evidence + control context and asks the assistant for a verdict.
func (uc *AnalyzeEvidenceUseCase) Execute(ctx context.Context, tenantID, evidenceID uuid.UUID, locale string) (*EvidenceAssessmentResult, error) {
	evidence, err := uc.compliance.GetEvidenceByID(ctx, tenantID, evidenceID)
	if err != nil {
		return nil, err
	}
	if evidence == nil {
		return nil, domain.NewNotFoundError("evidence", evidenceID)
	}

	filename := evidence.Filename
	if filename == "" {
		filename = evidence.Title
	}
	ec := llm.EvidenceContext{
		Locale:              llm.Locale(locale),
		EvidenceFilename:    filename,
		EvidenceDescription: evidence.Description,
	}

	// Resolve a control the artifact answers, and its framework name.
	//
	// An artifact in the library can answer several controls; the assistant is
	// asked about ONE of them (the first link), because "does this satisfy the
	// control?" has no meaning averaged across a dozen different requirements.
	// Analysing against a specific control is a per-control call, which is how the
	// UI invokes it from the control drawer.
	if links, err := uc.compliance.ListLinks(ctx, tenantID, []uuid.UUID{evidence.ID}); err == nil && len(links) > 0 {
		if control, err := uc.compliance.GetControlByID(ctx, links[0].ControlID, tenantID); err == nil && control != nil {
			ec.ControlCode = control.ReferenceCode
			ec.ControlName = control.Name
			ec.ControlDescription = control.Description
			if fw, err := uc.compliance.GetFrameworkByID(ctx, control.FrameworkID, tenantID); err == nil && fw != nil {
				ec.FrameworkName = fw.Name
			}
		}
	}

	assessment, generatedBy := invoke(uc.assistant, func(a llm.Assistant) (llm.EvidenceAssessment, error) {
		return a.AnalyzeEvidence(ctx, ec)
	})
	return &EvidenceAssessmentResult{Assessment: assessment, GeneratedBy: generatedBy}, nil
}
