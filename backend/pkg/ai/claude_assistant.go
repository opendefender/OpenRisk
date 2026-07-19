// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

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

// ClaudeAssistant implements the unified Assistant interface against the Claude
// API (claude-opus-4-8 by default). It is created only when an API key is present;
// the application layer treats every call as best-effort and falls back to the
// TemplateAssistant on any error (missing key, network, refusal, malformed JSON),
// so an AI feature is never a hard dependency.
//
// Each capability builds a system+user prompt, calls the Messages API with
// adaptive thinking, and decodes a strict JSON object. The JSON contract keeps the
// reply machine-parseable; the deterministic fallback guarantees a result even
// when the model is unavailable.
type ClaudeAssistant struct {
	client anthropic.Client
	model  string
}

// NewClaudeAssistant builds a Claude-backed assistant. model may be empty to use
// the project default (claude-opus-4-8, see claudeModel in claude_advisor.go).
func NewClaudeAssistant(apiKey, model string) *ClaudeAssistant {
	if model == "" {
		model = claudeModel
	}
	return &ClaudeAssistant{
		client: anthropic.NewClient(option.WithAPIKey(apiKey)),
		model:  model,
	}
}

func (a *ClaudeAssistant) Name() string { return a.model }

// complete runs one Messages API call with adaptive thinking and returns the raw
// text of the reply. Callers pass a system prompt that mandates a strict JSON
// object; they then decode with parseInto.
func (a *ClaudeAssistant) complete(ctx context.Context, system, user string) (string, error) {
	// A generous timeout: these features are interactive but not latency-critical,
	// and adaptive thinking can take a while.
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	msg, err := a.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(a.model),
		MaxTokens: 4096,
		System:    []anthropic.TextBlockParam{{Text: system}},
		// Adaptive thinking is the only supported thinking mode on Opus 4.8; a bit
		// of reasoning improves GRC analysis quality.
		Thinking: anthropic.ThinkingConfigParamUnion{
			OfAdaptive: &anthropic.ThinkingConfigAdaptiveParam{},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(user)),
		},
	})
	if err != nil {
		return "", fmt.Errorf("claude assistant: messages.new: %w", err)
	}
	if msg.StopReason == anthropic.StopReasonRefusal {
		return "", fmt.Errorf("claude assistant: request refused by safety classifier")
	}
	raw := collectText(msg)
	if strings.TrimSpace(raw) == "" {
		return "", fmt.Errorf("claude assistant: empty response")
	}
	return raw, nil
}

// parseInto extracts the JSON object from the model's reply (tolerating a fence or
// surrounding prose) and decodes it into dst.
func parseInto(raw string, dst any) error {
	jsonStr := extractJSONObject(raw)
	if jsonStr == "" {
		return fmt.Errorf("no JSON object found in response")
	}
	if err := json.Unmarshal([]byte(jsonStr), dst); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}
	return nil
}

// langOf maps a locale to a prompt-friendly language name.
func langOf(l Locale) string {
	if l.Normalize() == LocaleEN {
		return "English"
	}
	return "français"
}

// -----------------------------------------------------------------------------
// 1. Treatment plan
// -----------------------------------------------------------------------------

func (a *ClaudeAssistant) SuggestTreatmentPlan(ctx context.Context, in RiskContext) (TreatmentPlan, error) {
	system := "Tu es un expert GRC (gouvernance, risque, conformité) qui aide une équipe sécurité à traiter un risque. " +
		"À partir du contexte du risque, produis (a) une synthèse claire et non alarmiste, (b) une stratégie de traitement recommandée " +
		"parmi exactement: mitigate, accept, transfer, avoid, et (c) un plan d'actions concret, ordonné et actionnable. " +
		"Réponds STRICTEMENT par un objet JSON valide, sans texte autour ni Markdown, avec exactement ces clés: " +
		"summary (string), recommended_strategy (string: mitigate|accept|transfer|avoid), " +
		"actions (tableau de 2 à 6 objets {title, description, priority(high|medium|low)}), rationale (string). " +
		"Rédige toute la prose en " + langOf(in.Locale) + "."

	payload := struct {
		Name             string   `json:"name"`
		Description      string   `json:"description"`
		Criticality      string   `json:"criticality"`
		Probability      float64  `json:"probability_0_1"`
		Impact           float64  `json:"impact_0_10"`
		Score            float64  `json:"score"`
		Tags             []string `json:"tags"`
		Frameworks       []string `json:"frameworks"`
		AssetName        string   `json:"asset_name,omitempty"`
		AssetType        string   `json:"asset_type,omitempty"`
		AssetCriticality string   `json:"asset_criticality,omitempty"`
		ALEFCFA          int64    `json:"annual_loss_expectancy_fcfa,omitempty"`
	}{
		in.Name, in.Description, in.Criticality, in.Probability, in.Impact, in.Score,
		in.Tags, in.Frameworks, in.AssetName, in.AssetType, in.AssetCriticality, in.ALEXAF,
	}
	data, _ := json.MarshalIndent(payload, "", "  ")
	user := "Contexte du risque:\n\n" + string(data) + "\n\nProduis la synthèse et le plan de traitement au format JSON demandé."

	raw, err := a.complete(ctx, system, user)
	if err != nil {
		return TreatmentPlan{}, err
	}
	var out TreatmentPlan
	if err := parseInto(raw, &out); err != nil {
		return TreatmentPlan{}, fmt.Errorf("claude assistant: %w", err)
	}
	if strings.TrimSpace(out.Summary) == "" {
		return TreatmentPlan{}, fmt.Errorf("claude assistant: response missing summary")
	}
	return out, nil
}

// -----------------------------------------------------------------------------
// 2. Emerging risk detection
// -----------------------------------------------------------------------------

func (a *ClaudeAssistant) DetectEmergingRisks(ctx context.Context, in IntelInput) (EmergingRisksResult, error) {
	system := "Tu es un analyste cyber-menaces. On te fournit un texte brut (rapport de threat intelligence, actualité, logs). " +
		"Identifie les risques émergents pertinents pour une organisation et propose de nouveaux risques à ajouter au registre. " +
		"N'invente rien qui ne soit pas étayé par le texte. Ignore les risques déjà connus fournis. " +
		"Réponds STRICTEMENT par un objet JSON valide, sans texte autour ni Markdown, avec exactement ces clés: " +
		"summary (string), risks (tableau de 0 à 8 objets {title, description, category, severity(critical|high|medium|low), " +
		"rationale, suggested_probability(0.0-1.0), suggested_impact(0.0-10.0)}). " +
		"Rédige toute la prose en " + langOf(in.Locale) + "."

	payload := struct {
		Source     string   `json:"source"`
		Context    string   `json:"context,omitempty"`
		KnownRisks []string `json:"known_risks,omitempty"`
		Text       string   `json:"text"`
	}{in.Source, in.Context, in.KnownRisks, in.Text}
	data, _ := json.MarshalIndent(payload, "", "  ")
	user := "Analyse ce contenu et propose les risques émergents au format JSON demandé:\n\n" + string(data)

	raw, err := a.complete(ctx, system, user)
	if err != nil {
		return EmergingRisksResult{}, err
	}
	var out EmergingRisksResult
	if err := parseInto(raw, &out); err != nil {
		return EmergingRisksResult{}, fmt.Errorf("claude assistant: %w", err)
	}
	return out, nil
}

// -----------------------------------------------------------------------------
// 3. Natural-language assistant (RAG Q&A)
// -----------------------------------------------------------------------------

func (a *ClaudeAssistant) Answer(ctx context.Context, in AssistantQuery) (AssistantAnswer, error) {
	system := "Tu es l'assistant GRC d'OpenRisk. Tu réponds aux questions de l'équipe sécurité sur SA base de connaissances " +
		"(risques, contrôles de conformité, vulnérabilités) fournie dans le contexte. " +
		"Base-toi UNIQUEMENT sur le contexte fourni; si l'information manque, dis-le clairement plutôt que d'inventer. " +
		"Sois concret, cite les références utilisées (codes de contrôle, CVE, noms de risque). " +
		"Réponds STRICTEMENT par un objet JSON valide, sans texte autour ni Markdown, avec exactement ces clés: " +
		"answer (string), sources (tableau de chaînes, les références du contexte que tu as utilisées). " +
		"Rédige la réponse en " + langOf(in.Locale) + "."

	var b strings.Builder
	if in.OrgName != "" {
		b.WriteString("Organisation: " + in.OrgName + "\n\n")
	}
	b.WriteString("=== Contexte GRC (base de connaissances du tenant) ===\n")
	if len(in.Snippets) == 0 {
		b.WriteString("(aucun élément pertinent trouvé)\n")
	}
	for _, s := range in.Snippets {
		b.WriteString(fmt.Sprintf("- [%s] %s — %s: %s\n", s.Kind, s.Ref, s.Title, s.Detail))
	}
	if len(in.History) > 0 {
		b.WriteString("\n=== Historique de conversation ===\n")
		for _, t := range in.History {
			b.WriteString(t.Role + ": " + t.Text + "\n")
		}
	}
	b.WriteString("\n=== Question ===\n" + in.Question + "\n\nRéponds au format JSON demandé.")

	raw, err := a.complete(ctx, system, b.String())
	if err != nil {
		return AssistantAnswer{}, err
	}
	var out AssistantAnswer
	if err := parseInto(raw, &out); err != nil {
		return AssistantAnswer{}, fmt.Errorf("claude assistant: %w", err)
	}
	if strings.TrimSpace(out.Answer) == "" {
		return AssistantAnswer{}, fmt.Errorf("claude assistant: empty answer")
	}
	return out, nil
}

// -----------------------------------------------------------------------------
// 4. Audit report generation
// -----------------------------------------------------------------------------

func (a *ClaudeAssistant) SummarizeAudit(ctx context.Context, in AuditContext) (AuditNarrative, error) {
	system := "Tu es un auditeur senior en conformité qui rédige le rapport exécutif d'une campagne d'audit. " +
		"À partir des résultats fournis, rédige un rapport clair, factuel et orienté décision, destiné à la direction. " +
		"Réponds STRICTEMENT par un objet JSON valide, sans texte autour ni Markdown, avec exactement ces clés: " +
		"executive_summary (string, 3 à 5 phrases), findings (string), recommendations (tableau de 3 à 6 chaînes actionnables), " +
		"conclusion (string). Rédige toute la prose en " + langOf(in.Locale) + "."

	payload := struct {
		Title            string         `json:"title"`
		Type             string         `json:"type"`
		Status           string         `json:"status"`
		Auditor          string         `json:"auditor,omitempty"`
		Scope            string         `json:"scope,omitempty"`
		Framework        string         `json:"framework,omitempty"`
		TotalControls    int            `json:"total_controls"`
		Implemented      int            `json:"implemented"`
		Gaps             int            `json:"gaps"`
		PercentComplete  float64        `json:"percent_complete"`
		OpenRemediations int            `json:"open_remediations"`
		TopGaps          []AuditGapItem `json:"top_gaps,omitempty"`
	}{
		in.Title, in.Type, in.Status, in.Auditor, in.Scope, in.FrameworkName,
		in.TotalControls, in.Implemented, in.Gaps, in.PercentComplete, in.OpenRemediations, in.TopGaps,
	}
	data, _ := json.MarshalIndent(payload, "", "  ")
	user := "Résultats de l'audit (chiffres à ne pas modifier):\n\n" + string(data) + "\n\nRédige le rapport exécutif au format JSON demandé."

	raw, err := a.complete(ctx, system, user)
	if err != nil {
		return AuditNarrative{}, err
	}
	var out AuditNarrative
	if err := parseInto(raw, &out); err != nil {
		return AuditNarrative{}, fmt.Errorf("claude assistant: %w", err)
	}
	if strings.TrimSpace(out.ExecutiveSummary) == "" {
		return AuditNarrative{}, fmt.Errorf("claude assistant: response missing executive_summary")
	}
	return out, nil
}

// -----------------------------------------------------------------------------
// 5. Evidence document analysis
// -----------------------------------------------------------------------------

func (a *ClaudeAssistant) AnalyzeEvidence(ctx context.Context, in EvidenceContext) (EvidenceAssessment, error) {
	system := "Tu es un auditeur qui vérifie si une preuve documentaire répond bien à l'exigence d'un contrôle de conformité. " +
		"On te donne l'exigence du contrôle et les informations sur la preuve téléversée (nom de fichier, description, et extrait de contenu si disponible). " +
		"Évalue si la preuve satisfait le contrôle. Sois prudent: si le contenu n'est pas disponible, base-toi sur les métadonnées et abaisse ta confiance. " +
		"Réponds STRICTEMENT par un objet JSON valide, sans texte autour ni Markdown, avec exactement ces clés: " +
		"verdict (string: satisfies|partial|insufficient|unrelated), confidence (0.0-1.0), rationale (string), " +
		"gaps (tableau de chaînes), suggestions (tableau de chaînes). " +
		"Rédige toute la prose en " + langOf(in.Locale) + "."

	payload := struct {
		Framework           string `json:"framework,omitempty"`
		ControlCode         string `json:"control_code"`
		ControlName         string `json:"control_name"`
		ControlDescription  string `json:"control_description"`
		EvidenceFilename    string `json:"evidence_filename"`
		EvidenceDescription string `json:"evidence_description,omitempty"`
		EvidenceExcerpt     string `json:"evidence_excerpt,omitempty"`
	}{
		in.FrameworkName, in.ControlCode, in.ControlName, in.ControlDescription,
		in.EvidenceFilename, in.EvidenceDescription, in.EvidenceExcerpt,
	}
	data, _ := json.MarshalIndent(payload, "", "  ")
	user := "Contrôle et preuve à évaluer:\n\n" + string(data) + "\n\nRends ton verdict au format JSON demandé."

	raw, err := a.complete(ctx, system, user)
	if err != nil {
		return EvidenceAssessment{}, err
	}
	var out EvidenceAssessment
	if err := parseInto(raw, &out); err != nil {
		return EvidenceAssessment{}, fmt.Errorf("claude assistant: %w", err)
	}
	if strings.TrimSpace(out.Verdict) == "" {
		return EvidenceAssessment{}, fmt.Errorf("claude assistant: response missing verdict")
	}
	return out, nil
}
