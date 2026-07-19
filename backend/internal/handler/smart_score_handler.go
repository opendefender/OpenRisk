// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/application/risk"
	"github.com/opendefender/openrisk/internal/middleware"
	"github.com/opendefender/openrisk/pkg/scoring"
	"github.com/opendefender/openrisk/pkg/validation"
)

// SmartScoreHandler serves the Smart Risk Calculation endpoints (spec §8): the
// per-risk multifactor score with its radar-ready breakdown, a non-persisting
// simulator for live weight tuning, and the per-tenant factor-weight config.
type SmartScoreHandler struct {
	computeUC       *risk.ComputeSmartScoreUseCase
	getWeightsUC    *risk.GetRiskWeightsUseCase
	updateWeightsUC *risk.UpdateRiskWeightsUseCase
}

// NewSmartScoreHandler builds the handler.
func NewSmartScoreHandler(
	computeUC *risk.ComputeSmartScoreUseCase,
	getWeightsUC *risk.GetRiskWeightsUseCase,
	updateWeightsUC *risk.UpdateRiskWeightsUseCase,
) *SmartScoreHandler {
	return &SmartScoreHandler{computeUC: computeUC, getWeightsUC: getWeightsUC, updateWeightsUC: updateWeightsUC}
}

// GetRiskSmartScore GET /risks/:id/smart-score — computes (and caches on the risk)
// the multifactor smart score and its eight-factor breakdown. Tenant-scoped.
func (h *SmartScoreHandler) GetRiskSmartScore(c *fiber.Ctx) error {
	riskID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid UUID"})
	}
	orgID := uuid.Nil
	if mwCtx := middleware.GetContext(c); mwCtx != nil {
		orgID = mwCtx.OrganizationID
	}

	res, err := h.computeUC.Execute(c.UserContext(), orgID, riskID, true)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(res)
}

// FactorWeightsInput is the full set of eight factor weights (relative; the engine
// normalises). Used by both the simulator and the weights-update endpoint.
type FactorWeightsInput struct {
	BusinessCriticality float64 `json:"business_criticality" validate:"min=0,max=1"`
	InternetExposure    float64 `json:"internet_exposure" validate:"min=0,max=1"`
	Vulnerabilities     float64 `json:"vulnerabilities" validate:"min=0,max=1"`
	ControlMaturity     float64 `json:"control_maturity" validate:"min=0,max=1"`
	IncidentHistory     float64 `json:"incident_history" validate:"min=0,max=1"`
	Exploitability      float64 `json:"exploitability" validate:"min=0,max=1"`
	FinancialValue      float64 `json:"financial_value" validate:"min=0,max=1"`
	ThreatIntel         float64 `json:"threat_intel" validate:"min=0,max=1"`
}

// toFactorWeights maps the DTO onto the engine's weight map.
func (in FactorWeightsInput) toFactorWeights() scoring.FactorWeights {
	return scoring.FactorWeights{
		scoring.FactorBusinessCriticality: in.BusinessCriticality,
		scoring.FactorInternetExposure:    in.InternetExposure,
		scoring.FactorVulnerabilities:     in.Vulnerabilities,
		scoring.FactorControlMaturity:     in.ControlMaturity,
		scoring.FactorIncidentHistory:     in.IncidentHistory,
		scoring.FactorExploitability:      in.Exploitability,
		scoring.FactorFinancialValue:      in.FinancialValue,
		scoring.FactorThreatIntel:         in.ThreatIntel,
	}
}

// SimulateRiskSmartScore POST /risks/:id/smart-score/simulate — recomputes the
// smart score with the supplied weights WITHOUT persisting, so the config UI can
// preview "what if we weighted the factors like this" live. Guarded by risks:read.
func (h *SmartScoreHandler) SimulateRiskSmartScore(c *fiber.Ctx) error {
	riskID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid UUID"})
	}
	input := new(FactorWeightsInput)
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

	res, err := h.computeUC.Preview(c.UserContext(), orgID, riskID, input.toFactorWeights())
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(res)
}

// GetRiskWeights GET /risk-scoring/weights — the tenant's effective factor weights
// (custom or the built-in defaults).
func (h *SmartScoreHandler) GetRiskWeights(c *fiber.Ctx) error {
	orgID := uuid.Nil
	if mwCtx := middleware.GetContext(c); mwCtx != nil {
		orgID = mwCtx.OrganizationID
	}
	w, err := h.getWeightsUC.Execute(c.UserContext(), orgID)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(w)
}

// UpdateRiskWeights PUT /risk-scoring/weights — persists the tenant's custom factor
// weights. Admin-guarded at the route; validated (each [0,1], at least one > 0).
func (h *SmartScoreHandler) UpdateRiskWeights(c *fiber.Ctx) error {
	input := new(FactorWeightsInput)
	if err := c.BodyParser(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid input"})
	}
	if err := validation.GetValidator().Struct(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "validation_failed", "details": err.Error()})
	}

	orgID := uuid.Nil
	actorID := uuid.Nil
	if mwCtx := middleware.GetContext(c); mwCtx != nil {
		orgID = mwCtx.OrganizationID
		actorID = mwCtx.UserID
	}

	w, err := h.updateWeightsUC.Execute(c.UserContext(), orgID, actorID, risk.UpdateRiskWeightsInput{
		BusinessCriticality: input.BusinessCriticality,
		InternetExposure:    input.InternetExposure,
		Vulnerabilities:     input.Vulnerabilities,
		ControlMaturity:     input.ControlMaturity,
		IncidentHistory:     input.IncidentHistory,
		Exploitability:      input.Exploitability,
		FinancialValue:      input.FinancialValue,
		ThreatIntel:         input.ThreatIntel,
	})
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(w)
}
