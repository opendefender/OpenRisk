// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package complianceaudit

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- compact mocks (only the methods the use case touches carry behaviour) ---

type mockAuditRepo struct {
	audit *domain.ComplianceAudit
}

func (m *mockAuditRepo) CreateAudit(context.Context, *domain.ComplianceAudit) error { return nil }
func (m *mockAuditRepo) GetAuditByID(_ context.Context, _, _ uuid.UUID) (*domain.ComplianceAudit, error) {
	return m.audit, nil
}
func (m *mockAuditRepo) ListAudits(context.Context, uuid.UUID) ([]domain.ComplianceAudit, error) {
	return nil, nil
}
func (m *mockAuditRepo) UpdateAudit(context.Context, *domain.ComplianceAudit) error   { return nil }
func (m *mockAuditRepo) DeleteAudit(context.Context, uuid.UUID, uuid.UUID) error       { return nil }

type mockRemediationRepo struct {
	existing []domain.RemediationPlan
	created  []domain.RemediationPlan
}

func (m *mockRemediationRepo) CreateRemediation(_ context.Context, r *domain.RemediationPlan) error {
	m.created = append(m.created, *r)
	return nil
}
func (m *mockRemediationRepo) GetRemediationByID(context.Context, uuid.UUID, uuid.UUID) (*domain.RemediationPlan, error) {
	return nil, nil
}
func (m *mockRemediationRepo) ListRemediations(context.Context, uuid.UUID, domain.RemediationFilter) ([]domain.RemediationPlan, error) {
	return m.existing, nil
}
func (m *mockRemediationRepo) UpdateRemediation(context.Context, *domain.RemediationPlan) error { return nil }
func (m *mockRemediationRepo) DeleteRemediation(context.Context, uuid.UUID, uuid.UUID) error    { return nil }

// mockComplianceRepo stubs the whole port; only ListControlsByFramework carries data.
type mockComplianceRepo struct {
	controls []domain.ComplianceControl
}

func (m *mockComplianceRepo) CreateFramework(context.Context, *domain.ComplianceFramework) error { return nil }
func (m *mockComplianceRepo) GetFrameworkByID(context.Context, uuid.UUID, uuid.UUID) (*domain.ComplianceFramework, error) {
	return nil, nil
}
func (m *mockComplianceRepo) ListFrameworks(context.Context, uuid.UUID) ([]domain.ComplianceFramework, error) {
	return nil, nil
}
func (m *mockComplianceRepo) DeleteFramework(context.Context, uuid.UUID, uuid.UUID) error { return nil }
func (m *mockComplianceRepo) CreateControl(context.Context, *domain.ComplianceControl) error { return nil }
func (m *mockComplianceRepo) GetControlByID(context.Context, uuid.UUID, uuid.UUID) (*domain.ComplianceControl, error) {
	return nil, nil
}
func (m *mockComplianceRepo) ListControlsByFramework(context.Context, uuid.UUID, uuid.UUID) ([]domain.ComplianceControl, error) {
	return m.controls, nil
}
func (m *mockComplianceRepo) UpdateControl(context.Context, *domain.ComplianceControl) error { return nil }
func (m *mockComplianceRepo) DeleteControl(context.Context, uuid.UUID, uuid.UUID) error       { return nil }
func (m *mockComplianceRepo) DeleteControlsByFramework(context.Context, uuid.UUID, uuid.UUID) (int64, error) {
	return 0, nil
}
func (m *mockComplianceRepo) CreateEvidence(context.Context, *domain.ControlEvidence) error { return nil }
func (m *mockComplianceRepo) GetEvidenceByID(context.Context, uuid.UUID, uuid.UUID) (*domain.ControlEvidence, error) {
	return nil, nil
}
func (m *mockComplianceRepo) ListEvidencesByControl(context.Context, uuid.UUID, uuid.UUID) ([]domain.ControlEvidence, error) {
	return nil, nil
}
func (m *mockComplianceRepo) CountEvidencesByFramework(context.Context, uuid.UUID, uuid.UUID) (map[uuid.UUID]int, error) {
	return map[uuid.UUID]int{}, nil
}
func (m *mockComplianceRepo) DeleteEvidence(context.Context, uuid.UUID, uuid.UUID) error { return nil }

// --- tests ---

func TestGenerateRemediations_CreatesForGapsOnly(t *testing.T) {
	fwID := uuid.New()
	c1, c2, c3, c4 := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	auditRepo := &mockAuditRepo{audit: &domain.ComplianceAudit{ID: uuid.New(), FrameworkID: &fwID, Title: "Audit Q3"}}
	compRepo := &mockComplianceRepo{controls: []domain.ComplianceControl{
		{ID: c1, FrameworkID: fwID, ReferenceCode: "A.1", Status: domain.ControlStatusNotImplemented},
		{ID: c2, FrameworkID: fwID, ReferenceCode: "A.2", Status: domain.ControlStatusInProgress},
		{ID: c3, FrameworkID: fwID, ReferenceCode: "A.3", Status: domain.ControlStatusImplemented},   // not a gap
		{ID: c4, FrameworkID: fwID, ReferenceCode: "A.4", Status: domain.ControlStatusNotApplicable}, // not a gap
	}}
	remRepo := &mockRemediationRepo{}
	uc := NewGenerateRemediationsFromAuditUseCase(auditRepo, remRepo, compRepo)

	res, err := uc.Execute(context.Background(), uuid.New(), auditRepo.audit.ID, uuid.New())

	require.NoError(t, err)
	assert.Equal(t, 2, res.Created, "only the not_implemented + in_progress controls are gaps")
	assert.Equal(t, 0, res.Skipped)
	require.Len(t, remRepo.created, 2)
	// not_implemented → high, in_progress → medium
	got := map[string]domain.RemediationPriority{}
	for _, p := range remRepo.created {
		got[p.Title[:3]] = p.Priority
	}
	assert.Equal(t, domain.RemediationPriorityHigh, got["A.1"])
	assert.Equal(t, domain.RemediationPriorityMedium, got["A.2"])
}

func TestGenerateRemediations_SkipsControlsWithActivePlan(t *testing.T) {
	fwID := uuid.New()
	c1 := uuid.New()
	auditRepo := &mockAuditRepo{audit: &domain.ComplianceAudit{ID: uuid.New(), FrameworkID: &fwID}}
	compRepo := &mockComplianceRepo{controls: []domain.ComplianceControl{
		{ID: c1, FrameworkID: fwID, ReferenceCode: "A.1", Status: domain.ControlStatusNotImplemented},
	}}
	remRepo := &mockRemediationRepo{existing: []domain.RemediationPlan{
		{ID: uuid.New(), ControlID: &c1, Status: domain.RemediationStatusOpen},
	}}
	uc := NewGenerateRemediationsFromAuditUseCase(auditRepo, remRepo, compRepo)

	res, err := uc.Execute(context.Background(), uuid.New(), auditRepo.audit.ID, uuid.Nil)

	require.NoError(t, err)
	assert.Equal(t, 0, res.Created)
	assert.Equal(t, 1, res.Skipped, "control already has an active plan")
}

func TestGenerateRemediations_ProgramWideAuditRejected(t *testing.T) {
	auditRepo := &mockAuditRepo{audit: &domain.ComplianceAudit{ID: uuid.New(), FrameworkID: nil}}
	uc := NewGenerateRemediationsFromAuditUseCase(auditRepo, &mockRemediationRepo{}, &mockComplianceRepo{})
	_, err := uc.Execute(context.Background(), uuid.New(), auditRepo.audit.ID, uuid.Nil)
	assert.ErrorIs(t, err, domain.ErrValidation)
}

func TestGenerateRemediations_UnknownAudit404(t *testing.T) {
	uc := NewGenerateRemediationsFromAuditUseCase(&mockAuditRepo{audit: nil}, &mockRemediationRepo{}, &mockComplianceRepo{})
	_, err := uc.Execute(context.Background(), uuid.New(), uuid.New(), uuid.Nil)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}
