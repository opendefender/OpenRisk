// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
// This file is part of the OpenRisk Enterprise Edition and is NOT covered by the
// AGPL; it is licensed under the OpenRisk Commercial License (see LICENSE.commercial).

package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// claudeModel is the model used for board narratives. Opus 4.8 is the default
// per the project's AI guidance (see docs/MASTER_PROMPT_V4.md and the claude-api
// skill). Overridable via ANTHROPIC_MODEL for cost/quality tuning.
const claudeModel = "claude-opus-4-8"

// ClaudeAdvisor asks the Claude API to write the board narrative. It is created
// only when an API key is present; the board-report use case falls back to the
// TemplateAdvisor if this returns an error, so a missing key or a transient API
// failure never blocks report generation.
type ClaudeAdvisor struct {
	client anthropic.Client
	model  string
}

// NewClaudeAdvisor builds a Claude-backed advisor. model may be empty to use the
// default (claude-opus-4-8).
func NewClaudeAdvisor(apiKey, model string) *ClaudeAdvisor {
	if model == "" {
		model = claudeModel
	}
	return &ClaudeAdvisor{
		client: anthropic.NewClient(option.WithAPIKey(apiKey)),
		model:  model,
	}
}

func (a *ClaudeAdvisor) Name() string { return a.model }

// GenerateBoardNarrative calls the Messages API with adaptive thinking and asks
// for a strict JSON object it can decode into a BoardNarrative. Any error
// (network, refusal, malformed JSON) is returned so the caller can fall back.
func (a *ClaudeAdvisor) GenerateBoardNarrative(ctx context.Context, p BoardPosture) (BoardNarrative, error) {
	// A generous timeout: report generation is human-in-the-loop and not latency
	// critical, and adaptive thinking can take a while.
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	system, user := a.buildPrompt(p)

	msg, err := a.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(a.model),
		MaxTokens: 4096,
		System:    []anthropic.TextBlockParam{{Text: system}},
		// Adaptive thinking is the only supported thinking mode on Opus 4.8; a
		// board narrative from figures benefits from a little reasoning.
		Thinking: anthropic.ThinkingConfigParamUnion{
			OfAdaptive: &anthropic.ThinkingConfigAdaptiveParam{},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(user)),
		},
	})
	if err != nil {
		return BoardNarrative{}, fmt.Errorf("claude advisor: messages.new: %w", err)
	}
	if msg.StopReason == anthropic.StopReasonRefusal {
		return BoardNarrative{}, fmt.Errorf("claude advisor: request refused by safety classifier")
	}

	raw := collectText(msg)
	if strings.TrimSpace(raw) == "" {
		return BoardNarrative{}, fmt.Errorf("claude advisor: empty response")
	}

	narrative, err := parseNarrative(raw)
	if err != nil {
		return BoardNarrative{}, fmt.Errorf("claude advisor: %w", err)
	}
	return narrative, nil
}

// collectText concatenates the text blocks of a response (thinking blocks carry
// an empty Text under the default omitted display and are skipped by the type check).
func collectText(msg *anthropic.Message) string {
	var b strings.Builder
	for _, block := range msg.Content {
		if block.Type == "text" {
			b.WriteString(block.Text)
		}
	}
	return b.String()
}

// narrativeJSON is the shape we ask Claude to emit.
type narrativeJSON struct {
	ExecutiveSummary     string   `json:"executive_summary"`
	RiskCommentary       string   `json:"risk_commentary"`
	ComplianceCommentary string   `json:"compliance_commentary"`
	FinancialCommentary  string   `json:"financial_commentary"`
	Recommendations      []string `json:"recommendations"`
}

// parseNarrative extracts the JSON object from the model's reply (tolerating a
// ```json fence or surrounding prose) and decodes it.
func parseNarrative(raw string) (BoardNarrative, error) {
	jsonStr := extractJSONObject(raw)
	if jsonStr == "" {
		return BoardNarrative{}, fmt.Errorf("no JSON object found in response")
	}
	var n narrativeJSON
	if err := json.Unmarshal([]byte(jsonStr), &n); err != nil {
		return BoardNarrative{}, fmt.Errorf("decode JSON: %w", err)
	}
	if strings.TrimSpace(n.ExecutiveSummary) == "" {
		return BoardNarrative{}, fmt.Errorf("response missing executive_summary")
	}
	return BoardNarrative{
		ExecutiveSummary:     strings.TrimSpace(n.ExecutiveSummary),
		RiskCommentary:       strings.TrimSpace(n.RiskCommentary),
		ComplianceCommentary: strings.TrimSpace(n.ComplianceCommentary),
		FinancialCommentary:  strings.TrimSpace(n.FinancialCommentary),
		Recommendations:      n.Recommendations,
	}, nil
}

// extractJSONObject returns the substring from the first '{' to the last '}',
// which strips any ```json fence or lead-in text the model may add.
func extractJSONObject(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start < 0 || end < 0 || end < start {
		return ""
	}
	return s[start : end+1]
}

// buildPrompt returns the system and user prompts. The instruction is explicit
// about audience (board), tone (non-technical), currency (FCFA) and output shape
// (JSON), so the reply decodes reliably.
func (a *ClaudeAdvisor) buildPrompt(p BoardPosture) (system, user string) {
	lang := "français"
	if p.Locale.Normalize() == LocaleEN {
		lang = "English"
	}

	system = "Tu es un conseiller en gouvernance, risque et conformité (GRC) qui rédige des rapports pour un conseil d'administration. " +
		"Ton public n'est PAS technique : évite le jargon sécurité, les scores bruts et les acronymes non expliqués. " +
		"Sois factuel, concis, orienté décision. Les montants sont en FCFA. " +
		"Réponds STRICTEMENT par un objet JSON valide, sans texte autour, sans balises Markdown, avec exactement ces clés : " +
		"executive_summary (string, 3 à 5 phrases), risk_commentary (string), compliance_commentary (string), " +
		"financial_commentary (string), recommendations (tableau de 3 à 5 chaînes, chacune une action concrète). " +
		"Rédige toute la prose en " + lang + "."

	// Feed the model the aggregated figures as compact JSON so it never invents numbers.
	posture := struct {
		Organization             string             `json:"organization"`
		Period                   string             `json:"period"`
		RisksCritical            int                `json:"risks_critical"`
		RisksHigh                int                `json:"risks_high"`
		RisksMedium              int                `json:"risks_medium"`
		RisksLow                 int                `json:"risks_low"`
		RisksTotal               int                `json:"risks_total"`
		FinancialExposureFCFA    int64              `json:"financial_exposure_fcfa"`
		OverallCompliancePercent float64            `json:"overall_compliance_percent"`
		Frameworks               []FrameworkPosture `json:"frameworks"`
	}{
		Organization:             p.OrganizationName,
		Period:                   p.PeriodLabel,
		RisksCritical:            p.RisksCritical,
		RisksHigh:                p.RisksHigh,
		RisksMedium:              p.RisksMedium,
		RisksLow:                 p.RisksLow,
		RisksTotal:               p.RisksTotal,
		FinancialExposureFCFA:    p.FinancialExposureFCFA,
		OverallCompliancePercent: p.OverallCompliancePercent,
		Frameworks:               p.Frameworks,
	}
	data, _ := json.MarshalIndent(posture, "", "  ")

	user = "Voici la posture agrégée de risque et de conformité (chiffres à ne pas modifier, montant déjà en FCFA) :\n\n" +
		string(data) +
		"\n\nRédige le rapport pour le conseil au format JSON demandé."
	return system, user
}
