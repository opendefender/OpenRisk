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

// TreatmentPlanResult wraps the assistant output with the provider that produced it.
type TreatmentPlanResult struct {
	Plan        llm.TreatmentPlan `json:"plan"`
	GeneratedBy string            `json:"generated_by"`
}

// SuggestTreatmentPlanUseCase synthesises a risk and proposes a remediation plan
// (spec §12.1). The asset port is optional: when set, the plan is enriched with the
// linked asset's context.
type SuggestTreatmentPlanUseCase struct {
	assistant llm.Assistant
	risks     RiskReader
	assets    AssetReader // optional
}

func NewSuggestTreatmentPlanUseCase(assistant llm.Assistant, risks RiskReader) *SuggestTreatmentPlanUseCase {
	return &SuggestTreatmentPlanUseCase{assistant: assistant, risks: risks}
}

// WithAssetReader enriches the plan with the linked asset's context.
func (uc *SuggestTreatmentPlanUseCase) WithAssetReader(a AssetReader) *SuggestTreatmentPlanUseCase {
	uc.assets = a
	return uc
}

// Execute builds the risk context and asks the assistant for a treatment plan.
func (uc *SuggestTreatmentPlanUseCase) Execute(ctx context.Context, tenantID, riskID uuid.UUID, locale string) (*TreatmentPlanResult, error) {
	risk, err := uc.risks.GetByID(ctx, riskID, tenantID)
	if err != nil {
		return nil, err
	}
	if risk == nil {
		return nil, domain.NewNotFoundError("risk", riskID)
	}

	rc := llm.RiskContext{
		Locale:      llm.Locale(locale),
		Name:        firstNonEmpty(risk.Name, risk.Title),
		Description: risk.Description,
		Criticality: string(risk.Criticality),
		Probability: risk.Probability,
		Impact:      risk.Impact,
		Score:       risk.Score,
		Tags:        []string(risk.Tags),
		Frameworks:  []string(risk.Frameworks),
	}
	// Rough annual loss expectancy when both CRQ inputs are present (SLE × ARO).
	if risk.SLEXAF != nil && risk.ARO != nil && *risk.SLEXAF > 0 && *risk.ARO > 0 {
		rc.ALEXAF = int64(*risk.SLEXAF * *risk.ARO)
	}
	// Enrich with the linked asset when available.
	if uc.assets != nil && risk.AssetID != nil {
		if asset, err := uc.assets.GetByID(ctx, *risk.AssetID, tenantID); err == nil && asset != nil {
			rc.AssetName = asset.Name
			rc.AssetType = asset.Type
			rc.AssetCriticality = string(asset.Criticality)
		}
	}

	plan, generatedBy := invoke(uc.assistant, func(a llm.Assistant) (llm.TreatmentPlan, error) {
		return a.SuggestTreatmentPlan(ctx, rc)
	})
	return &TreatmentPlanResult{Plan: plan, GeneratedBy: generatedBy}, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
