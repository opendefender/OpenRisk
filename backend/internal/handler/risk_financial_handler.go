// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/application/risk"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/middleware"
	"github.com/opendefender/openrisk/pkg/crq"
	"github.com/opendefender/openrisk/pkg/validation"
)

// FinancialAnalyticsHandler serves the tenant-wide financial dashboard
// (GET /analytics/financial) that backs the CISO/CFO screen.
type FinancialAnalyticsHandler struct {
	summaryUC *risk.FinancialSummaryUseCase
}

// NewFinancialAnalyticsHandler builds the handler.
func NewFinancialAnalyticsHandler(summaryUC *risk.FinancialSummaryUseCase) *FinancialAnalyticsHandler {
	return &FinancialAnalyticsHandler{summaryUC: summaryUC}
}

// GetFinancialSummary GET /analytics/financial — aggregated financial posture
// (portfolio ALE, worst-case, residual, remediation budget, ROSI, breakdown by
// criticality, top exposures) for the caller's tenant.
func (h *FinancialAnalyticsHandler) GetFinancialSummary(c *fiber.Ctx) error {
	orgID := uuid.Nil
	if mwCtx := middleware.GetContext(c); mwCtx != nil {
		orgID = mwCtx.OrganizationID
	}
	summary, err := h.summaryUC.Execute(c.UserContext(), orgID)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(summary)
}

// financialInputsFromRisk maps a risk's stored monetary drivers onto the CRQ
// engine's input struct.
func financialInputsFromRisk(r *domain.Risk) crq.FinancialInputs {
	return crq.FinancialInputs{
		SLEXAF:                  r.SLEXAF,
		ARO:                     r.ARO,
		DowntimeHours:           r.DowntimeHours,
		HourlyDowntimeCostXAF:   r.HourlyDowntimeCostXAF,
		DataLossCostXAF:         r.DataLossCostXAF,
		FinesXAF:                r.FinesXAF,
		OtherDirectCostXAF:      r.OtherDirectCostXAF,
		RemediationCostXAF:      r.RemediationCostXAF,
		MitigationEffectiveness: r.MitigationEffectiveness,
	}
}

// GetRiskFinancial GET /risks/:id/financial — returns the full financial
// assessment (SLE, downtime cost, worst/average loss, ALE, remediation, ROSI) for
// one risk from its stored drivers. Tenant-scoped via the use case.
func (h *RiskHandler) GetRiskFinancial(c *fiber.Ctx) error {
	riskID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid UUID"})
	}
	orgID := uuid.Nil
	if mwCtx := middleware.GetContext(c); mwCtx != nil {
		orgID = mwCtx.OrganizationID
	}

	r, err := h.getRiskUseCase.Execute(c.UserContext(), orgID, riskID)
	if err != nil {
		return writeAppError(c, err)
	}
	if r == nil {
		return c.Status(404).JSON(fiber.Map{"error": "risk not found"})
	}
	if h.crq == nil {
		return c.Status(500).JSON(fiber.Map{"error": "quantifier not configured"})
	}

	assessment := h.crq.Assess(financialInputsFromRisk(r), string(r.Criticality))
	return c.JSON(assessment)
}

// SimulateFinancialInput carries per-field overrides for a what-if investment
// scenario. Every field is optional: nil = use the risk's stored value. This
// powers the CISO/CFO "investment scenario" simulator without persisting.
type SimulateFinancialInput struct {
	SLEXAF                  *float64 `json:"sle_xaf" validate:"omitempty,min=0"`
	ARO                     *float64 `json:"aro" validate:"omitempty,min=0"`
	DowntimeHours           *float64 `json:"downtime_hours" validate:"omitempty,min=0"`
	HourlyDowntimeCostXAF   *float64 `json:"hourly_downtime_cost_xaf" validate:"omitempty,min=0"`
	DataLossCostXAF         *float64 `json:"data_loss_cost_xaf" validate:"omitempty,min=0"`
	FinesXAF                *float64 `json:"fines_xaf" validate:"omitempty,min=0"`
	OtherDirectCostXAF      *float64 `json:"other_direct_cost_xaf" validate:"omitempty,min=0"`
	RemediationCostXAF      *float64 `json:"remediation_cost_xaf" validate:"omitempty,min=0"`
	MitigationEffectiveness *float64 `json:"mitigation_effectiveness" validate:"omitempty,min=0,max=1"`
}

// SimulateRiskFinancial POST /risks/:id/simulate — recomputes the financial
// assessment with the supplied overrides layered on the risk's stored drivers,
// WITHOUT persisting anything. Guarded by risks:read (read-only simulation).
func (h *RiskHandler) SimulateRiskFinancial(c *fiber.Ctx) error {
	riskID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid UUID"})
	}

	input := new(SimulateFinancialInput)
	if err := c.BodyParser(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid input"})
	}
	if err := validation.GetValidator().Struct(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "validation_failed", "details": err.Error()})
	}

	orgID := uuid.Nil
	if mwCtx := middleware.GetContext(c); mwCtx != nil {
		orgID = mwCtx.OrganizationID
	}

	r, err := h.getRiskUseCase.Execute(c.UserContext(), orgID, riskID)
	if err != nil {
		return writeAppError(c, err)
	}
	if r == nil {
		return c.Status(404).JSON(fiber.Map{"error": "risk not found"})
	}
	if h.crq == nil {
		return c.Status(500).JSON(fiber.Map{"error": "quantifier not configured"})
	}

	in := financialInputsFromRisk(r)
	// Layer overrides (only fields the caller supplied).
	if input.SLEXAF != nil {
		in.SLEXAF = input.SLEXAF
	}
	if input.ARO != nil {
		in.ARO = input.ARO
	}
	if input.DowntimeHours != nil {
		in.DowntimeHours = input.DowntimeHours
	}
	if input.HourlyDowntimeCostXAF != nil {
		in.HourlyDowntimeCostXAF = input.HourlyDowntimeCostXAF
	}
	if input.DataLossCostXAF != nil {
		in.DataLossCostXAF = input.DataLossCostXAF
	}
	if input.FinesXAF != nil {
		in.FinesXAF = input.FinesXAF
	}
	if input.OtherDirectCostXAF != nil {
		in.OtherDirectCostXAF = input.OtherDirectCostXAF
	}
	if input.RemediationCostXAF != nil {
		in.RemediationCostXAF = input.RemediationCostXAF
	}
	if input.MitigationEffectiveness != nil {
		in.MitigationEffectiveness = input.MitigationEffectiveness
	}

	assessment := h.crq.Assess(in, string(r.Criticality))
	return c.JSON(assessment)
}
