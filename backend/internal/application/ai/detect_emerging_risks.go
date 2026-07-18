// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package ai

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	llm "github.com/opendefender/openrisk/pkg/ai"
)

// EmergingRisksResult wraps the assistant output with its provider.
type EmergingRisksResult struct {
	Result      llm.EmergingRisksResult `json:"result"`
	GeneratedBy string                  `json:"generated_by"`
}

// DetectEmergingRisksUseCase scans free text (threat intel, news, logs) for
// emerging risks (spec §12.2). The risk lister is optional: when set, the tenant's
// existing risk titles are passed as a dedupe hint so the assistant avoids
// re-proposing what is already tracked.
type DetectEmergingRisksUseCase struct {
	assistant llm.Assistant
	risks     RiskLister // optional
}

func NewDetectEmergingRisksUseCase(assistant llm.Assistant) *DetectEmergingRisksUseCase {
	return &DetectEmergingRisksUseCase{assistant: assistant}
}

// WithRiskLister supplies existing risk titles for de-duplication.
func (uc *DetectEmergingRisksUseCase) WithRiskLister(r RiskLister) *DetectEmergingRisksUseCase {
	uc.risks = r
	return uc
}

// DetectInput carries the caller's text and hints.
type DetectInput struct {
	Source  string
	Text    string
	Context string
	Locale  string
}

// Execute analyses the text and returns candidate emerging risks. The text is
// validated (non-empty) so we never call the LLM on an empty prompt.
func (uc *DetectEmergingRisksUseCase) Execute(ctx context.Context, tenantID uuid.UUID, in DetectInput) (*EmergingRisksResult, error) {
	if strings.TrimSpace(in.Text) == "" {
		return nil, domain.NewValidationError("text is required")
	}

	input := llm.IntelInput{
		Locale:  llm.Locale(in.Locale),
		Source:  firstNonEmpty(in.Source, "text"),
		Text:    in.Text,
		Context: in.Context,
	}
	// Dedupe hint: the tenant's existing risk titles (bounded).
	if uc.risks != nil {
		if page, err := uc.risks.List(ctx, tenantID, domain.RiskQuery{Page: 1, Limit: 100}); err == nil && page != nil {
			for _, r := range page.Data {
				title := firstNonEmpty(r.Name, r.Title)
				if title != "" {
					input.KnownRisks = append(input.KnownRisks, title)
				}
			}
		}
	}

	res, generatedBy := invoke(uc.assistant, func(a llm.Assistant) (llm.EmergingRisksResult, error) {
		return a.DetectEmergingRisks(ctx, input)
	})
	return &EmergingRisksResult{Result: res, GeneratedBy: generatedBy}, nil
}
