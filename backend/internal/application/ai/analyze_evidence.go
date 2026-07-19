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
	evidence, err := uc.compliance.GetEvidenceByID(ctx, evidenceID, tenantID)
	if err != nil {
		return nil, err
	}
	if evidence == nil {
		return nil, domain.NewNotFoundError("evidence", evidenceID)
	}

	ec := llm.EvidenceContext{
		Locale:              llm.Locale(locale),
		EvidenceFilename:    evidence.Filename,
		EvidenceDescription: evidence.Description,
	}

	// Resolve the control the evidence proves, and its framework name.
	if control, err := uc.compliance.GetControlByID(ctx, evidence.ControlID, tenantID); err == nil && control != nil {
		ec.ControlCode = control.ReferenceCode
		ec.ControlName = control.Name
		ec.ControlDescription = control.Description
		if fw, err := uc.compliance.GetFrameworkByID(ctx, control.FrameworkID, tenantID); err == nil && fw != nil {
			ec.FrameworkName = fw.Name
		}
	}

	assessment, generatedBy := invoke(uc.assistant, func(a llm.Assistant) (llm.EvidenceAssessment, error) {
		return a.AnalyzeEvidence(ctx, ec)
	})
	return &EvidenceAssessmentResult{Assessment: assessment, GeneratedBy: generatedBy}, nil
}
