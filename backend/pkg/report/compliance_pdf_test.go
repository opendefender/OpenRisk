// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package report

import (
	"bytes"
	"testing"
	"time"
)

func sampleData() ComplianceReportData {
	rows := make([]ReportControlRow, 0, 60)
	// Enough rows to force at least one page break, plus a very long name/source
	// to exercise the wrapping + row-height logic.
	rows = append(rows, ReportControlRow{
		ReferenceCode:   "A.5.1",
		Name:            "Politiques de sécurité de l'information — un ensemble de politiques doit être défini, approuvé par la direction, publié et communiqué aux employés et aux parties externes concernées.",
		Status:          StatusImplemented,
		SourceReference: "ISO/IEC 27001:2022, Annexe A, A.5.1",
		EvidenceCount:   3,
	})
	statuses := []ControlStatus{StatusImplemented, StatusInProgress, StatusNotImplemented, StatusNotApplicable}
	for i := 0; i < 55; i++ {
		rows = append(rows, ReportControlRow{
			ReferenceCode:   "A.8." + string(rune('0'+i%10)),
			Name:            "Contrôle technique de démonstration numéro " + string(rune('A'+i%26)),
			Status:          statuses[i%len(statuses)],
			SourceReference: "ISO/IEC 27001:2022, Annexe A, A.8." + string(rune('0'+i%10)),
			EvidenceCount:   i % 3,
		})
	}
	return ComplianceReportData{
		Locale:           LocaleFR,
		OrganizationName: "Banque Atlantique Côte d'Ivoire",
		FrameworkName:    "ISO/IEC 27001",
		FrameworkVersion: "2022",
		GeneratedAt:      time.Date(2026, 7, 9, 10, 30, 0, 0, time.UTC),
		GeneratedBy:      "admin@opendefender.io",
		Total:            56,
		Applicable:       50,
		Implemented:      20,
		InProgress:       15,
		NotImplemented:   15,
		NotApplicable:    6,
		PercentComplete:  40.0,
		Controls:         rows,
	}
}

func TestRenderCompliancePDF_ProducesValidPDF(t *testing.T) {
	out, err := RenderCompliancePDF(sampleData())
	if err != nil {
		t.Fatalf("RenderCompliancePDF returned error: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("expected non-empty PDF output")
	}
	if !bytes.HasPrefix(out, []byte("%PDF-")) {
		t.Fatalf("output does not start with the PDF magic header, got %q", out[:min(8, len(out))])
	}
	if !bytes.Contains(out, []byte("%%EOF")) {
		t.Error("PDF output missing EOF trailer")
	}
}

func TestRenderCompliancePDF_EmptyControls(t *testing.T) {
	data := sampleData()
	data.Controls = nil
	data.Total, data.Applicable, data.Implemented, data.InProgress, data.NotImplemented, data.NotApplicable = 0, 0, 0, 0, 0, 0
	data.PercentComplete = 0

	out, err := RenderCompliancePDF(data)
	if err != nil {
		t.Fatalf("RenderCompliancePDF (empty) returned error: %v", err)
	}
	if !bytes.HasPrefix(out, []byte("%PDF-")) {
		t.Fatal("empty-controls report is not a valid PDF")
	}
}

func TestRenderCompliancePDF_EnglishLocale(t *testing.T) {
	data := sampleData()
	data.Locale = LocaleEN
	out, err := RenderCompliancePDF(data)
	if err != nil {
		t.Fatalf("RenderCompliancePDF (en) returned error: %v", err)
	}
	if !bytes.HasPrefix(out, []byte("%PDF-")) {
		t.Fatal("english report is not a valid PDF")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
