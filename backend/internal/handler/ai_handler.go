// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package handler

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	appai "github.com/opendefender/openrisk/internal/application/ai"
	"github.com/opendefender/openrisk/internal/domain"
	llm "github.com/opendefender/openrisk/pkg/ai"
)

// AIHandler wires the GRC AI assistant use cases (spec §12) to HTTP. Every AI call
// is best-effort: the use cases fall back to a deterministic template assistant, so
// these endpoints always return 200 with a result even without an API key. The
// response records generated_by so the UI can show whether Claude or the template
// produced it.
type AIHandler struct {
	assistant     llm.Assistant
	treatmentUC   *appai.SuggestTreatmentPlanUseCase
	emergingUC    *appai.DetectEmergingRisksUseCase
	queryUC       *appai.AssistantQueryUseCase
	auditReportUC *appai.GenerateAuditReportUseCase
	evidenceUC    *appai.AnalyzeEvidenceUseCase
}

func NewAIHandler(
	assistant llm.Assistant,
	treatment *appai.SuggestTreatmentPlanUseCase,
	emerging *appai.DetectEmergingRisksUseCase,
	query *appai.AssistantQueryUseCase,
	auditReport *appai.GenerateAuditReportUseCase,
	evidence *appai.AnalyzeEvidenceUseCase,
) *AIHandler {
	return &AIHandler{
		assistant:     assistant,
		treatmentUC:   treatment,
		emergingUC:    emerging,
		queryUC:       query,
		auditReportUC: auditReport,
		evidenceUC:    evidence,
	}
}

// localeOf resolves the caller's locale from a request field or the ?locale query,
// defaulting to French (the primary market).
func localeOf(c *fiber.Ctx, field string) string {
	l := strings.ToLower(strings.TrimSpace(field))
	if l == "" {
		l = strings.ToLower(strings.TrimSpace(c.Query("locale")))
	}
	if l == "en" {
		return "en"
	}
	return "fr"
}

// Status reports whether a real LLM is configured and which model is active.
// GET /ai/status
func (h *AIHandler) Status(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"llm_enabled": llm.IsLLMBacked(h.assistant),
		"model":       h.assistant.Name(),
	})
}

// --- 1. Treatment plan ------------------------------------------------------

type treatmentPlanInput struct {
	Locale string `json:"locale"`
}

// SuggestTreatmentPlan synthesises a risk and proposes a remediation plan.
// POST /ai/risks/:id/treatment-plan
func (h *AIHandler) SuggestTreatmentPlan(c *fiber.Ctx) error {
	riskID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return writeAppError(c, domain.NewValidationError("invalid risk id"))
	}
	in := new(treatmentPlanInput)
	_ = c.BodyParser(in)

	res, err := h.treatmentUC.Execute(c.UserContext(), tenantID(c), riskID, localeOf(c, in.Locale))
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(res)
}

// --- 2. Emerging risks ------------------------------------------------------

type emergingRisksInput struct {
	Source  string `json:"source"`
	Text    string `json:"text"`
	Context string `json:"context"`
	Locale  string `json:"locale"`
}

// DetectEmergingRisks scans free text for candidate new risks.
// POST /ai/emerging-risks
func (h *AIHandler) DetectEmergingRisks(c *fiber.Ctx) error {
	in := new(emergingRisksInput)
	if err := c.BodyParser(in); err != nil {
		return writeAppError(c, domain.NewValidationError("invalid request body"))
	}
	res, err := h.emergingUC.Execute(c.UserContext(), tenantID(c), appai.DetectInput{
		Source:  in.Source,
		Text:    in.Text,
		Context: in.Context,
		Locale:  localeOf(c, in.Locale),
	})
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(res)
}

// --- 3. Natural-language assistant (Q&A) ------------------------------------

type assistantQueryInput struct {
	Question string `json:"question"`
	History  []struct {
		Role string `json:"role"`
		Text string `json:"text"`
	} `json:"history"`
	Locale string `json:"locale"`
}

// AssistantQuery answers a natural-language GRC question grounded in the tenant's
// knowledge base.
// POST /ai/assistant/query
func (h *AIHandler) AssistantQuery(c *fiber.Ctx) error {
	in := new(assistantQueryInput)
	if err := c.BodyParser(in); err != nil {
		return writeAppError(c, domain.NewValidationError("invalid request body"))
	}
	history := make([]llm.ChatTurn, 0, len(in.History))
	for _, t := range in.History {
		role := "user"
		if t.Role == "assistant" || t.Role == "ai" {
			role = "assistant"
		}
		history = append(history, llm.ChatTurn{Role: role, Text: t.Text})
	}

	res, err := h.queryUC.Execute(c.UserContext(), tenantID(c), appai.QueryInput{
		Question: in.Question,
		History:  history,
		Locale:   localeOf(c, in.Locale),
	})
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(res)
}

// --- 4. Audit report --------------------------------------------------------

type auditReportInput struct {
	Locale string `json:"locale"`
}

// GenerateAuditReport writes an executive report from an audit campaign.
// POST /ai/audits/:id/report
func (h *AIHandler) GenerateAuditReport(c *fiber.Ctx) error {
	auditID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return writeAppError(c, domain.NewValidationError("invalid audit id"))
	}
	in := new(auditReportInput)
	_ = c.BodyParser(in)

	res, err := h.auditReportUC.Execute(c.UserContext(), tenantID(c), auditID, localeOf(c, in.Locale))
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(res)
}

// --- 5. Evidence analysis ---------------------------------------------------

type evidenceAnalysisInput struct {
	Locale string `json:"locale"`
}

// AnalyzeEvidence checks whether an uploaded evidence meets its control's intent.
// POST /ai/evidence/:id/analyze
func (h *AIHandler) AnalyzeEvidence(c *fiber.Ctx) error {
	evidenceID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return writeAppError(c, domain.NewValidationError("invalid evidence id"))
	}
	in := new(evidenceAnalysisInput)
	_ = c.BodyParser(in)

	res, err := h.evidenceUC.Execute(c.UserContext(), tenantID(c), evidenceID, localeOf(c, in.Locale))
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(res)
}
