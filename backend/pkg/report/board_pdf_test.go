// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package report

import (
	"testing"
	"time"
)

// TestRenderBoardPDF exercises the renderer with French accents, an em dash and
// the typographic characters that previously panicked fpdf.SplitText, plus a
// draft status and several frameworks, to guard against regressions.
func TestRenderBoardPDF(t *testing.T) {
	data := BoardReportData{
		Locale:                   LocaleFR,
		OrganizationName:         "Banque Atlantique — Côte d'Ivoire",
		Title:                    "Rapport du conseil — Juillet 2026",
		PeriodLabel:              "Juillet 2026",
		GeneratedAt:              time.Now(),
		GeneratedBy:              "Alexandre Dembélé",
		GeneratedByModel:         "claude-opus-4-8",
		Status:                   "draft",
		RisksCritical:            2,
		RisksHigh:                5,
		RisksMedium:              8,
		RisksLow:                 3,
		RisksTotal:               18,
		FinancialExposureLabel:   "175 000 000 FCFA",
		OverallCompliancePercent: 62,
		Frameworks: []BoardFrameworkRow{
			{Name: "ISO/IEC 27001", Version: "2022", PercentComplete: 64, Implemented: 60, Applicable: 93},
			{Name: "BCEAO", PercentComplete: 40, Implemented: 14, Applicable: 35},
			{Name: "COBAC R-2016/04", PercentComplete: 22, Implemented: 10, Applicable: 45},
		},
		ExecutiveSummary:     "La posture d'ensemble est globalement satisfaisante mais perfectible — les risques critiques concentrent l'essentiel de l'exposition.",
		RiskCommentary:       "Le registre comprend 18 risques actifs, dont 2 critiques appelant un traitement immédiat.",
		ComplianceCommentary: "La conformité consolidée atteint 62 % ; le référentiel « COBAC » reste le moins avancé.",
		FinancialCommentary:  "L'exposition annuelle estimée s'élève à 175 000 000 FCFA (estimation d'ordre de grandeur).",
		Recommendations: []string{
			"Traiter en priorité les 2 risques critiques sous 30 jours.",
			"Renforcer la mise en œuvre des contrôles pour atteindre 80 %.",
		},
	}

	pdf, err := RenderBoardPDF(data)
	if err != nil {
		t.Fatalf("RenderBoardPDF returned error: %v", err)
	}
	if len(pdf) < 1000 {
		t.Fatalf("PDF suspiciously small: %d bytes", len(pdf))
	}
	if string(pdf[:4]) != "%PDF" {
		t.Fatalf("output is not a PDF (missing %%PDF header)")
	}
}

func TestRenderBoardPDF_EN_Approved_NoFrameworks(t *testing.T) {
	approvedAt := time.Now()
	data := BoardReportData{
		Locale:                   LocaleEN,
		OrganizationName:         "Acme Corp",
		PeriodLabel:              "July 2026",
		GeneratedAt:              time.Now(),
		Status:                   "approved",
		ApprovedBy:               "Jane Doe",
		ApprovedAt:               &approvedAt,
		FinancialExposureLabel:   "0 FCFA",
		OverallCompliancePercent: 0,
		ExecutiveSummary:         "This is the executive summary in English.",
	}
	pdf, err := RenderBoardPDF(data)
	if err != nil {
		t.Fatalf("RenderBoardPDF returned error: %v", err)
	}
	if string(pdf[:4]) != "%PDF" {
		t.Fatalf("output is not a PDF")
	}
}
