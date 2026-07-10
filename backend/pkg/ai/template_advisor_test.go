// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package ai

import (
	"context"
	"strings"
	"testing"
)

func samplePosture(loc Locale) BoardPosture {
	return BoardPosture{
		Locale:                loc,
		OrganizationName:      "Banque Atlantique",
		PeriodLabel:           "Juillet 2026",
		RisksCritical:         2,
		RisksHigh:             5,
		RisksMedium:           8,
		RisksLow:              3,
		RisksTotal:            18,
		FinancialExposureFCFA: 175_000_000,
		OverallCompliancePercent: 62,
		Frameworks: []FrameworkPosture{
			{Name: "ISO 27001", Version: "2022", Total: 93, Applicable: 93, Implemented: 60, PercentComplete: 64},
			{Name: "BCEAO", Version: "", Total: 35, Applicable: 35, Implemented: 14, PercentComplete: 40},
		},
	}
}

func TestFormatFCFA(t *testing.T) {
	cases := map[int64]string{
		0:           "0 FCFA",
		500:         "500 FCFA",
		1500:        "1 500 FCFA",
		1000000:     "1 000 000 FCFA",
		175000000:   "175 000 000 FCFA",
	}
	for in, want := range cases {
		if got := FormatFCFA(in); got != want {
			t.Errorf("FormatFCFA(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestTemplateAdvisor_Deterministic(t *testing.T) {
	adv := NewTemplateAdvisor()
	p := samplePosture(LocaleFR)

	a, err := adv.GenerateBoardNarrative(context.Background(), p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, _ := adv.GenerateBoardNarrative(context.Background(), p)
	if a.ExecutiveSummary != b.ExecutiveSummary || len(a.Recommendations) != len(b.Recommendations) {
		t.Errorf("advisor is not deterministic")
	}
}

func TestTemplateAdvisor_FR_MentionsKeyFigures(t *testing.T) {
	adv := NewTemplateAdvisor()
	n, err := adv.GenerateBoardNarrative(context.Background(), samplePosture(LocaleFR))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(n.ExecutiveSummary, "175 000 000 FCFA") {
		t.Errorf("executive summary should mention the FCFA exposure, got: %q", n.ExecutiveSummary)
	}
	if !strings.Contains(n.ExecutiveSummary, "Banque Atlantique") {
		t.Errorf("executive summary should mention the organization")
	}
	if len(n.Recommendations) == 0 {
		t.Errorf("expected at least one recommendation")
	}
	// 2 critical risks + 62%% compliance => at least the critical-risk and the
	// 80%%-baseline recommendations should appear.
	if len(n.Recommendations) < 2 {
		t.Errorf("expected several recommendations for a weak posture, got %d", len(n.Recommendations))
	}
}

func TestTemplateAdvisor_EN(t *testing.T) {
	adv := NewTemplateAdvisor()
	n, err := adv.GenerateBoardNarrative(context.Background(), samplePosture(LocaleEN))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(n.ExecutiveSummary, "board of directors") {
		t.Errorf("EN summary should be in English, got: %q", n.ExecutiveSummary)
	}
}

func TestTemplateAdvisor_NoCriticalNoFrameworks(t *testing.T) {
	adv := NewTemplateAdvisor()
	p := BoardPosture{Locale: LocaleFR, PeriodLabel: "Août 2026", OverallCompliancePercent: 90}
	n, err := adv.GenerateBoardNarrative(context.Background(), p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(n.RiskCommentary, "Aucun risque de niveau critique") {
		t.Errorf("expected the no-critical-risk sentence, got: %q", n.RiskCommentary)
	}
	if !strings.Contains(n.ComplianceCommentary, "Aucun référentiel") {
		t.Errorf("expected the no-framework sentence, got: %q", n.ComplianceCommentary)
	}
}

func TestNewAdvisor_FallsBackToTemplateWithoutKey(t *testing.T) {
	if got := NewAdvisor("", ""); got.Name() != "template" {
		t.Errorf("without an API key NewAdvisor should return the template advisor, got %q", got.Name())
	}
	if got := NewAdvisor("sk-test", ""); got.Name() == "template" {
		t.Errorf("with an API key NewAdvisor should return the Claude advisor")
	}
}
