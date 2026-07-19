// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package ai

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	appcompliance "github.com/opendefender/openrisk/internal/application/compliance"
	"github.com/opendefender/openrisk/internal/domain"
	llm "github.com/opendefender/openrisk/pkg/ai"
)

// The tests pass a real TemplateAssistant as the "primary" so the LLM path is
// exercised deterministically (the fallback is the same implementation), letting us
// assert on context assembly and retrieval rather than on model prose.
var tmpl = llm.NewTemplateAssistant()

// ---- mocks -----------------------------------------------------------------

type mockRiskReader struct {
	risk *domain.Risk
	err  error
}

func (m *mockRiskReader) GetByID(_ context.Context, _, _ uuid.UUID) (*domain.Risk, error) {
	return m.risk, m.err
}

type mockRiskLister struct{ risks []domain.Risk }

func (m *mockRiskLister) List(_ context.Context, _ uuid.UUID, _ domain.RiskQuery) (*domain.PaginatedResult[domain.Risk], error) {
	return &domain.PaginatedResult[domain.Risk]{Data: m.risks, Total: int64(len(m.risks))}, nil
}

type mockAssetReader struct{ asset *domain.Asset }

func (m *mockAssetReader) GetByID(_ context.Context, _, _ uuid.UUID) (*domain.Asset, error) {
	return m.asset, nil
}

type mockCompliance struct {
	frameworks []domain.ComplianceFramework
	controls   []domain.ComplianceControl
	evidence   *domain.ControlEvidence
}

func (m *mockCompliance) ListFrameworks(_ context.Context, _ uuid.UUID) ([]domain.ComplianceFramework, error) {
	return m.frameworks, nil
}
func (m *mockCompliance) ListControlsByFramework(_ context.Context, _, _ uuid.UUID) ([]domain.ComplianceControl, error) {
	return m.controls, nil
}
func (m *mockCompliance) GetFrameworkByID(_ context.Context, id, _ uuid.UUID) (*domain.ComplianceFramework, error) {
	for i := range m.frameworks {
		if m.frameworks[i].ID == id {
			return &m.frameworks[i], nil
		}
	}
	return nil, nil
}
func (m *mockCompliance) GetControlByID(_ context.Context, id, _ uuid.UUID) (*domain.ComplianceControl, error) {
	for i := range m.controls {
		if m.controls[i].ID == id {
			return &m.controls[i], nil
		}
	}
	return nil, nil
}
func (m *mockCompliance) GetEvidenceByID(_ context.Context, _, _ uuid.UUID) (*domain.ControlEvidence, error) {
	return m.evidence, nil
}

type mockAuditReader struct {
	audit        *domain.ComplianceAudit
	remediations []domain.RemediationPlan
}

func (m *mockAuditReader) GetAuditByID(_ context.Context, _, _ uuid.UUID) (*domain.ComplianceAudit, error) {
	return m.audit, nil
}
func (m *mockAuditReader) ListRemediations(_ context.Context, _ uuid.UUID, _ domain.RemediationFilter) ([]domain.RemediationPlan, error) {
	return m.remediations, nil
}

type mockGap struct{ ga *appcompliance.GapAnalysis }

func (m *mockGap) Execute(_ context.Context, _, _ uuid.UUID) (*appcompliance.GapAnalysis, error) {
	return m.ga, nil
}

// ---- treatment plan --------------------------------------------------------

func TestSuggestTreatmentPlan_Success(t *testing.T) {
	assetID := uuid.New()
	risks := &mockRiskReader{risk: &domain.Risk{
		Name: "RDP exposé", Criticality: "critical", Probability: 0.6, Impact: 9, Score: 16.2, AssetID: &assetID,
	}}
	assets := &mockAssetReader{asset: &domain.Asset{Name: "srv-paie-01", Type: "Server", Criticality: "CRITICAL"}}
	uc := NewSuggestTreatmentPlanUseCase(tmpl, risks).WithAssetReader(assets)

	res, err := uc.Execute(context.Background(), uuid.New(), uuid.New(), "fr")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Plan.RecommendedStrategy != "mitigate" {
		t.Fatalf("expected mitigate, got %q", res.Plan.RecommendedStrategy)
	}
	if res.GeneratedBy != "template" {
		t.Fatalf("expected template provider, got %q", res.GeneratedBy)
	}
}

func TestSuggestTreatmentPlan_NotFound(t *testing.T) {
	uc := NewSuggestTreatmentPlanUseCase(tmpl, &mockRiskReader{risk: nil})
	_, err := uc.Execute(context.Background(), uuid.New(), uuid.New(), "fr")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

// ---- emerging risks --------------------------------------------------------

func TestDetectEmergingRisks_Success(t *testing.T) {
	uc := NewDetectEmergingRisksUseCase(tmpl)
	res, err := uc.Execute(context.Background(), uuid.New(), DetectInput{
		Source: "threat-intel",
		Text:   "ransomware spreading via phishing exploiting CVE-2024-1",
		Locale: "en",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Result.Risks) < 3 {
		t.Fatalf("expected >=3 emerging risks, got %d", len(res.Result.Risks))
	}
}

func TestDetectEmergingRisks_EmptyText(t *testing.T) {
	uc := NewDetectEmergingRisksUseCase(tmpl)
	_, err := uc.Execute(context.Background(), uuid.New(), DetectInput{Text: "   "})
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

// ---- assistant query (RAG) -------------------------------------------------

func TestAssistantQuery_RetrievesContext(t *testing.T) {
	fwID := uuid.New()
	risks := &mockRiskLister{risks: []domain.Risk{{Name: "Exposed S3 bucket on AWS", Criticality: "high", Score: 12}}}
	comp := &mockCompliance{
		frameworks: []domain.ComplianceFramework{{ID: fwID, Name: "ISO 27001"}},
		controls:   []domain.ComplianceControl{{ID: uuid.New(), FrameworkID: fwID, ReferenceCode: "A.5.23", Name: "Cloud services security", Status: domain.ControlStatusNotImplemented}},
	}
	uc := NewAssistantQueryUseCase(tmpl).WithRisks(risks).WithCompliance(comp)

	res, err := uc.Execute(context.Background(), uuid.New(), QueryInput{Question: "What are our cloud AWS risks?", Locale: "en"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Retrieved) == 0 {
		t.Fatalf("expected retrieved snippets")
	}
	// Both a risk and a matched control should be retrieved.
	var kinds = map[string]bool{}
	for _, s := range res.Retrieved {
		kinds[s.Kind] = true
	}
	if !kinds["risk"] {
		t.Fatalf("expected a risk snippet, got %+v", res.Retrieved)
	}
	if !kinds["control"] {
		t.Fatalf("expected a control snippet (keyword 'cloud'), got %+v", res.Retrieved)
	}
}

func TestAssistantQuery_EmptyQuestion(t *testing.T) {
	uc := NewAssistantQueryUseCase(tmpl)
	_, err := uc.Execute(context.Background(), uuid.New(), QueryInput{Question: " "})
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

// ---- audit report ----------------------------------------------------------

func TestGenerateAuditReport_Success(t *testing.T) {
	fwID := uuid.New()
	audits := &mockAuditReader{audit: &domain.ComplianceAudit{Title: "Audit ISO", Type: "internal", Status: "completed", FrameworkID: &fwID}}
	gap := &mockGap{ga: &appcompliance.GapAnalysis{
		TotalControls: 93, TotalGaps: 23,
		Frameworks: []appcompliance.FrameworkGapSummary{{FrameworkName: "ISO 27001"}},
		Gaps:       []appcompliance.GapControl{{ReferenceCode: "A.8.8", Name: "Vuln mgmt", Status: domain.ControlStatusNotImplemented}},
	}}
	uc := NewGenerateAuditReportUseCase(tmpl, audits).WithGapAnalyzer(gap)

	res, err := uc.Execute(context.Background(), uuid.New(), uuid.New(), "fr")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Report.ExecutiveSummary == "" {
		t.Fatalf("expected an executive summary")
	}
}

func TestGenerateAuditReport_NotFound(t *testing.T) {
	uc := NewGenerateAuditReportUseCase(tmpl, &mockAuditReader{audit: nil})
	_, err := uc.Execute(context.Background(), uuid.New(), uuid.New(), "fr")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected not-found, got %v", err)
	}
}

// ---- evidence analysis -----------------------------------------------------

func TestAnalyzeEvidence_Success(t *testing.T) {
	ctrlID := uuid.New()
	fwID := uuid.New()
	comp := &mockCompliance{
		frameworks: []domain.ComplianceFramework{{ID: fwID, Name: "ISO 27001"}},
		controls:   []domain.ComplianceControl{{ID: ctrlID, FrameworkID: fwID, ReferenceCode: "A.5.15", Name: "Access control policy", Description: "policy access control"}},
		evidence:   &domain.ControlEvidence{ControlID: ctrlID, Filename: "access-control-policy.pdf"},
	}
	uc := NewAnalyzeEvidenceUseCase(tmpl, comp)

	res, err := uc.Execute(context.Background(), uuid.New(), uuid.New(), "en")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Assessment.Verdict == "" {
		t.Fatalf("expected a verdict")
	}
}

func TestAnalyzeEvidence_NotFound(t *testing.T) {
	uc := NewAnalyzeEvidenceUseCase(tmpl, &mockCompliance{evidence: nil})
	_, err := uc.Execute(context.Background(), uuid.New(), uuid.New(), "en")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected not-found, got %v", err)
	}
}
