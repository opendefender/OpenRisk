// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package ai

import "context"

// Assistant is the unified AI surface for the GRC assistant features (spec §12).
// It is deliberately shaped like the board-report Advisor: two implementations
// satisfy it — a ClaudeAssistant that calls the Claude API when ANTHROPIC_API_KEY
// is set, and a deterministic TemplateAssistant that always works with no key and
// is what the application layer falls back to on any LLM error. The five methods
// map one-to-one onto the five spec capabilities:
//
//  1. SuggestTreatmentPlan  — risk synthesis + smart remediation plan
//  2. DetectEmergingRisks   — scan free text (threat-intel, news, logs) for risks
//  3. Answer                — natural-language Q&A over the tenant's GRC knowledge
//  4. SummarizeAudit        — executive audit report from a campaign's results
//  5. AnalyzeEvidence       — check an uploaded proof against a control's intent
//
// Every input DTO is pure (no domain/ or GORM types) so this package stays free of
// internal/ imports and is trivially unit-testable. Context assembly (the RAG
// retrieval and tenant-scoped data access) lives in internal/application/ai; this
// package only turns already-gathered context into prose.
type Assistant interface {
	// SuggestTreatmentPlan synthesises a risk and proposes a remediation plan.
	SuggestTreatmentPlan(ctx context.Context, in RiskContext) (TreatmentPlan, error)
	// DetectEmergingRisks reads free text and proposes candidate new risks.
	DetectEmergingRisks(ctx context.Context, in IntelInput) (EmergingRisksResult, error)
	// Answer responds to a natural-language question using retrieved GRC context.
	Answer(ctx context.Context, in AssistantQuery) (AssistantAnswer, error)
	// SummarizeAudit writes an executive report from an audit campaign's results.
	SummarizeAudit(ctx context.Context, in AuditContext) (AuditNarrative, error)
	// AnalyzeEvidence checks whether an uploaded evidence meets a control's intent.
	AnalyzeEvidence(ctx context.Context, in EvidenceContext) (EvidenceAssessment, error)
	// Name identifies the assistant for provenance (e.g. "claude-opus-4-8" or "template").
	Name() string
}

// -----------------------------------------------------------------------------
// 1. Treatment plan
// -----------------------------------------------------------------------------

// RiskContext is the already-assembled context for a single risk. AssetName and
// the financial fields may be empty/zero when unknown — the assistant degrades
// gracefully rather than inventing values.
type RiskContext struct {
	Locale           Locale
	Name             string
	Description      string
	Criticality      string   // low|medium|high|critical
	Probability      float64  // 0..1
	Impact           float64  // 0..10
	Score            float64  // Score Engine P×I×AC
	Tags             []string
	Frameworks       []string
	AssetName        string
	AssetType        string
	AssetCriticality string
	ALEXAF           int64 // annual loss expectancy in FCFA, 0 if not quantified
}

// TreatmentPlanAction is one concrete, ordered remediation step.
type TreatmentPlanAction struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    string `json:"priority"` // high|medium|low
}

// TreatmentPlan is the synthesised risk summary plus a suggested remediation plan.
type TreatmentPlan struct {
	Summary             string                `json:"summary"`
	RecommendedStrategy string                `json:"recommended_strategy"` // mitigate|accept|transfer|avoid
	Actions             []TreatmentPlanAction `json:"actions"`
	Rationale           string                `json:"rationale"`
}

// -----------------------------------------------------------------------------
// 2. Emerging risk detection
// -----------------------------------------------------------------------------

// IntelInput is a block of free text to scan for emerging risks, plus optional
// hints. KnownRisks lets the assistant avoid proposing duplicates of what the
// tenant already tracks.
type IntelInput struct {
	Locale     Locale
	Source     string   // e.g. "threat-intel", "news", "logs"
	Text       string   // the raw text to analyse
	KnownRisks []string // existing risk titles (dedupe hint)
	Context    string   // optional sector/geography hint
}

// EmergingRisk is one candidate risk the assistant suggests creating. The
// suggested P/I are on the Score Engine scale so the UI can pre-fill a draft.
type EmergingRisk struct {
	Title                string  `json:"title"`
	Description          string  `json:"description"`
	Category             string  `json:"category"`
	Severity             string  `json:"severity"` // critical|high|medium|low
	Rationale            string  `json:"rationale"`
	SuggestedProbability float64 `json:"suggested_probability"` // 0..1
	SuggestedImpact      float64 `json:"suggested_impact"`      // 0..10
}

// EmergingRisksResult carries the detected risks plus a one-line summary.
type EmergingRisksResult struct {
	Summary string         `json:"summary"`
	Risks   []EmergingRisk `json:"risks"`
}

// -----------------------------------------------------------------------------
// 3. Natural-language assistant (RAG Q&A)
// -----------------------------------------------------------------------------

// KnowledgeSnippet is one retrieved piece of tenant GRC context. The application
// layer performs a hybrid keyword retrieval over the tenant's risks, controls and
// vulnerabilities and passes the top matches here — this is the "RAG context".
type KnowledgeSnippet struct {
	Kind   string // "risk"|"control"|"vulnerability"|"framework"
	Ref    string // reference code / CVE / name
	Title  string
	Detail string
}

// ChatTurn is one prior turn of the conversation, for multi-turn continuity.
type ChatTurn struct {
	Role string // "user"|"assistant"
	Text string
}

// AssistantQuery is a user question plus the retrieved context and history.
type AssistantQuery struct {
	Locale   Locale
	Question string
	History  []ChatTurn
	Snippets []KnowledgeSnippet
	OrgName  string
}

// AssistantAnswer is the assistant's reply. Sources lists the human-readable refs
// it grounded the answer on (e.g. "RSK — Log4Shell", "ISO A.8.8").
type AssistantAnswer struct {
	Answer  string   `json:"answer"`
	Sources []string `json:"sources"`
}

// -----------------------------------------------------------------------------
// 4. Audit report generation
// -----------------------------------------------------------------------------

// AuditGapItem is one notable open gap surfaced in an audit.
type AuditGapItem struct {
	Code   string
	Name   string
	Status string
}

// AuditContext is the assembled result of an audit campaign.
type AuditContext struct {
	Locale           Locale
	Title            string
	Type             string
	Status           string
	Auditor          string
	Scope            string
	FrameworkName    string
	TotalControls    int
	Implemented      int
	Gaps             int
	PercentComplete  float64
	TopGaps          []AuditGapItem
	OpenRemediations int
}

// AuditNarrative is the executive audit report, ready to export.
type AuditNarrative struct {
	ExecutiveSummary string   `json:"executive_summary"`
	Findings         string   `json:"findings"`
	Recommendations  []string `json:"recommendations"`
	Conclusion       string   `json:"conclusion"`
}

// -----------------------------------------------------------------------------
// 5. Evidence document analysis
// -----------------------------------------------------------------------------

// EvidenceContext pairs an uploaded evidence with the control it is meant to
// satisfy. EvidenceExcerpt is any extracted text (may be empty for binary files);
// the assistant reasons from filename/description when no text is available.
type EvidenceContext struct {
	Locale              Locale
	ControlCode         string
	ControlName         string
	ControlDescription  string
	FrameworkName       string
	EvidenceFilename    string
	EvidenceDescription string
	EvidenceExcerpt     string
}

// EvidenceAssessment is the documentary-compliance verdict.
type EvidenceAssessment struct {
	Verdict     string   `json:"verdict"`    // satisfies|partial|insufficient|unrelated
	Confidence  float64  `json:"confidence"` // 0..1
	Rationale   string   `json:"rationale"`
	Gaps        []string `json:"gaps"`
	Suggestions []string `json:"suggestions"`
}
