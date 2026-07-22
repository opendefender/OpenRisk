// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
// This file is part of the OpenRisk Enterprise Edition and is NOT covered by the
// AGPL; it is licensed under the OpenRisk Commercial License (see LICENSE.commercial).

package ai

import (
	"context"
	"strings"
	"testing"
)

func TestTemplateAssistant_SuggestTreatmentPlan(t *testing.T) {
	a := NewTemplateAssistant()

	// Critical risk → mitigate strategy with a high-priority first action.
	plan, err := a.SuggestTreatmentPlan(context.Background(), RiskContext{
		Locale:      LocaleFR,
		Name:        "RDP exposé",
		Criticality: "critical",
		Probability: 0.6,
		Impact:      9,
		Score:       16.2,
		AssetName:   "srv-paie-01",
		ALEXAF:      50_000_000,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.RecommendedStrategy != "mitigate" {
		t.Fatalf("expected mitigate, got %q", plan.RecommendedStrategy)
	}
	if len(plan.Actions) == 0 || plan.Actions[0].Priority != "high" {
		t.Fatalf("expected a high-priority first action, got %+v", plan.Actions)
	}
	if !strings.Contains(plan.Summary, "srv-paie-01") {
		t.Fatalf("summary should mention the asset: %q", plan.Summary)
	}
	if !strings.Contains(plan.Summary, "FCFA") {
		t.Fatalf("summary should mention the FCFA exposure: %q", plan.Summary)
	}

	// Low risk → accept strategy.
	low, _ := a.SuggestTreatmentPlan(context.Background(), RiskContext{Locale: LocaleEN, Name: "x", Criticality: "low"})
	if low.RecommendedStrategy != "accept" {
		t.Fatalf("expected accept for low risk, got %q", low.RecommendedStrategy)
	}
}

func TestTemplateAssistant_DetectEmergingRisks(t *testing.T) {
	a := NewTemplateAssistant()
	res, err := a.DetectEmergingRisks(context.Background(), IntelInput{
		Locale: LocaleFR,
		Source: "threat-intel",
		Text:   "A new ransomware strain is spreading via phishing emails exploiting CVE-2024-1234.",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// ransomware + phishing + cve- → at least 3 candidate risks.
	if len(res.Risks) < 3 {
		t.Fatalf("expected >=3 emerging risks, got %d: %+v", len(res.Risks), res.Risks)
	}
	for _, r := range res.Risks {
		if r.SuggestedProbability < 0 || r.SuggestedProbability > 1 {
			t.Fatalf("probability out of range: %v", r.SuggestedProbability)
		}
		if r.SuggestedImpact < 0 || r.SuggestedImpact > 10 {
			t.Fatalf("impact out of range: %v", r.SuggestedImpact)
		}
	}

	// Known-risk dedupe.
	deduped, _ := a.DetectEmergingRisks(context.Background(), IntelInput{
		Locale:     LocaleEN,
		Text:       "ransomware outbreak",
		KnownRisks: []string{"Ransomware exposure"},
	})
	for _, r := range deduped.Risks {
		if strings.EqualFold(r.Title, "Ransomware exposure") {
			t.Fatalf("known risk should have been deduped")
		}
	}
}

func TestTemplateAssistant_Answer(t *testing.T) {
	a := NewTemplateAssistant()

	// No context → honest "not found", never a fabricated answer.
	empty, _ := a.Answer(context.Background(), AssistantQuery{Locale: LocaleFR, Question: "?"})
	if len(empty.Sources) != 0 {
		t.Fatalf("expected no sources when no context")
	}

	// With snippets → grounded answer that cites the refs.
	ans, err := a.Answer(context.Background(), AssistantQuery{
		Locale:   LocaleEN,
		Question: "What are our AWS risks?",
		Snippets: []KnowledgeSnippet{
			{Kind: "risk", Ref: "RSK-1", Title: "Exposed S3 bucket", Detail: "public bucket on AWS"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ans.Sources) != 1 || ans.Sources[0] != "RSK-1" {
		t.Fatalf("expected the snippet ref as source, got %+v", ans.Sources)
	}
}

func TestTemplateAssistant_SummarizeAudit(t *testing.T) {
	a := NewTemplateAssistant()
	nar, err := a.SummarizeAudit(context.Background(), AuditContext{
		Locale:          LocaleFR,
		Title:           "Audit ISO 27001",
		Type:            "internal",
		FrameworkName:   "ISO 27001",
		TotalControls:   93,
		Implemented:     70,
		Gaps:            23,
		PercentComplete: 75.2,
		TopGaps:         []AuditGapItem{{Code: "A.8.8", Name: "Vuln mgmt", Status: "not_implemented"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(nar.ExecutiveSummary, "ISO 27001") {
		t.Fatalf("summary should mention the framework: %q", nar.ExecutiveSummary)
	}
	if len(nar.Recommendations) == 0 {
		t.Fatalf("expected recommendations")
	}
	if !strings.Contains(nar.Findings, "A.8.8") {
		t.Fatalf("findings should list the top gap: %q", nar.Findings)
	}
}

func TestTemplateAssistant_AnalyzeEvidence(t *testing.T) {
	a := NewTemplateAssistant()

	// Strong overlap + extracted content → satisfies with decent confidence.
	ok, err := a.AnalyzeEvidence(context.Background(), EvidenceContext{
		Locale:             LocaleEN,
		ControlCode:        "A.5.15",
		ControlName:        "Access control policy",
		ControlDescription: "policy access control review approval",
		EvidenceFilename:   "access-control-policy.pdf",
		EvidenceExcerpt:    "This access control policy defines review and approval of access.",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok.Verdict != "satisfies" {
		t.Fatalf("expected satisfies, got %q (conf %v)", ok.Verdict, ok.Confidence)
	}

	// No content, no overlap → insufficient, never a false satisfies.
	bad, _ := a.AnalyzeEvidence(context.Background(), EvidenceContext{
		Locale:           LocaleEN,
		ControlCode:      "A.5.15",
		ControlName:      "Access control policy",
		EvidenceFilename: "vacation-photos.zip",
	})
	if bad.Verdict == "satisfies" {
		t.Fatalf("must never say satisfies for an unrelated file")
	}
	if bad.Confidence > 0.5 {
		t.Fatalf("confidence should be low without content, got %v", bad.Confidence)
	}
}

// TemplateAssistant must satisfy the Assistant interface.
var _ Assistant = (*TemplateAssistant)(nil)
