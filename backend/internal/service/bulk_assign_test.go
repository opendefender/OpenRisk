// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package service

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
)

// #1213 — bulk assign_mitigation used to append a bare Mitigation through a
// GORM association: zero tenant_id, no creator, mitigation_id ignored. These
// tests pin the replacement: one plan per risk, written through
// CreateMitigationPlanUseCase in the job's tenant, by the job's author.

// fakeMitigationStore is a MitigationRepository that keeps plans in memory and
// answers GetByID with tenant isolation, like the GORM repository does.
type fakeMitigationStore struct {
	byID    map[uuid.UUID]*domain.Mitigation
	created []*domain.Mitigation
	tenants []string
}

func newFakeMitigationStore(seed ...*domain.Mitigation) *fakeMitigationStore {
	f := &fakeMitigationStore{byID: map[uuid.UUID]*domain.Mitigation{}}
	for _, m := range seed {
		f.byID[m.ID] = m
	}
	return f
}

func (f *fakeMitigationStore) GetByID(tenantID string, id uuid.UUID) (*domain.Mitigation, error) {
	m, ok := f.byID[id]
	if !ok || m.TenantID.String() != tenantID {
		return nil, domain.ErrNotFound
	}
	return m, nil
}

func (f *fakeMitigationStore) CreateWithSubActions(tenantID string, m *domain.Mitigation, _ []*domain.MitigationSubAction) error {
	f.tenants = append(f.tenants, tenantID)
	f.created = append(f.created, m)
	return nil
}

func (f *fakeMitigationStore) Create(string, *domain.Mitigation) error {
	return errors.New("bulk assign must not use the legacy single-row create")
}
func (f *fakeMitigationStore) GetByIDWithSubActions(string, uuid.UUID) (*domain.Mitigation, error) {
	return nil, domain.ErrNotFound
}
func (f *fakeMitigationStore) List(string, map[string]interface{}) ([]domain.Mitigation, error) {
	return nil, nil
}
func (f *fakeMitigationStore) Update(string, *domain.Mitigation) error { return nil }
func (f *fakeMitigationStore) Delete(string, uuid.UUID) error          { return nil }
func (f *fakeMitigationStore) ListByRiskID(tenantID string, riskID uuid.UUID) ([]domain.Mitigation, error) {
	var out []domain.Mitigation
	for _, m := range f.byID {
		if m.TenantID.String() == tenantID && m.RiskID == riskID {
			out = append(out, *m)
		}
	}
	for _, m := range f.created {
		if m.TenantID.String() == tenantID && m.RiskID == riskID {
			out = append(out, *m)
		}
	}
	return out, nil
}
func (f *fakeMitigationStore) RecalculateProgress(string, uuid.UUID) (int, error) { return 0, nil }

func setupBulkAssignDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := setupBulkTestDB(t)
	if err := db.Exec(`CREATE TABLE bulk_operation_logs (
		id TEXT PRIMARY KEY, bulk_operation_id TEXT, resource_id TEXT,
		resource_type TEXT, status TEXT, error_message TEXT, created_at DATETIME
	);`).Error; err != nil {
		t.Fatalf("create bulk_operation_logs: %v", err)
	}
	return db
}

func seedRiskID(t *testing.T, db *gorm.DB, tenant uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if err := db.Exec(`INSERT INTO risks (id, tenant_id, status, score) VALUES (?, ?, 'open', 5.0)`,
		id.String(), tenant.String()).Error; err != nil {
		t.Fatalf("seed risk: %v", err)
	}
	return id
}

func templatePlan(tenant uuid.UUID) *domain.Mitigation {
	return &domain.Mitigation{
		ID:          uuid.New(),
		TenantID:    tenant,
		RiskID:      uuid.New(),
		Title:       "Rotate exposed credentials",
		Description: "Rotate and revoke every key found in the leak",
		Priority:    domain.MitigationPriority("high"),
	}
}

func TestBulkAssign_Success(t *testing.T) {
	db := setupBulkAssignDB(t)
	tenantA, tenantB := uuid.New(), uuid.New()
	author := uuid.New()
	riskA1 := seedRiskID(t, db, tenantA)
	riskA2 := seedRiskID(t, db, tenantA)
	seedRiskID(t, db, tenantB) // must not receive a plan

	tpl := templatePlan(tenantA)
	store := newFakeMitigationStore(tpl)
	s := NewBulkOperationService()
	s.mitigations = store

	op := &domain.BulkOperation{
		ID:            insertOp(t, db, tenantA, author),
		OperationType: domain.BulkOperationTypeAssign,
		UpdateData:    map[string]interface{}{"mitigation_id": tpl.ID.String()},
		TenantID:      tenantA,
		CreatedBy:     author,
	}
	require.NoError(t, s.processBulkAssign(op))

	require.Len(t, store.created, 2, "one plan per risk of the tenant")
	assert.Equal(t, 2, op.ProcessedCount)
	assert.Equal(t, 0, op.ErrorCount)
	gotRisks := map[uuid.UUID]bool{}
	for i, plan := range store.created {
		assert.Equal(t, tenantA, plan.TenantID, "plan carries the job's tenant, never the zero UUID")
		assert.Equal(t, tenantA.String(), store.tenants[i], "written through the tenant-checked repository call")
		assert.Equal(t, author, plan.CreatedBy, "the job's author is the creator")
		require.NotNil(t, plan.OwnerID)
		assert.Equal(t, author, *plan.OwnerID)
		assert.Equal(t, tpl.Title, plan.Title)
		assert.Equal(t, tpl.Description, plan.Description)
		assert.Equal(t, tpl.Priority, plan.Priority)
		assert.Equal(t, domain.MitigationPlanned, plan.Status)
		assert.NotEqual(t, tpl.ID, plan.ID, "a copy, not the template itself")
		gotRisks[plan.RiskID] = true
	}
	assert.Equal(t, map[uuid.UUID]bool{riskA1: true, riskA2: true}, gotRisks)
}

// The request's mitigation_id is resolved when the job is created and carried
// to the worker; the top-level field and update_data are both accepted.
func TestBulkAssign_TemplateIDIsReadFromEitherField(t *testing.T) {
	id := uuid.New()
	got, err := assignTemplateID(&domain.CreateBulkOperationRequest{MitigationID: id})
	require.NoError(t, err)
	assert.Equal(t, id, got)

	got, err = assignTemplateID(&domain.CreateBulkOperationRequest{
		UpdateData: map[string]interface{}{"mitigation_id": id.String()},
	})
	require.NoError(t, err)
	assert.Equal(t, id, got)

	for _, data := range []map[string]interface{}{
		nil,
		{"mitigation_id": "not-a-uuid"},
		{"mitigation_id": uuid.Nil.String()},
		{"mitigation_id": 42},
	} {
		_, err := assignTemplateID(&domain.CreateBulkOperationRequest{UpdateData: data})
		assert.ErrorIs(t, err, domain.ErrValidation, "update_data %v", data)
	}
}

func TestBulkAssign_NotFound(t *testing.T) {
	db := setupBulkAssignDB(t)
	tenant, author := uuid.New(), uuid.New()
	seedRiskID(t, db, tenant)

	store := newFakeMitigationStore()
	s := NewBulkOperationService()
	s.mitigations = store

	_, err := s.CreateBulkOperation(author, tenant, &domain.CreateBulkOperationRequest{
		OperationType: domain.BulkOperationTypeAssign,
		MitigationID:  uuid.New(),
	})
	require.ErrorIs(t, err, domain.ErrNotFound)

	var jobs int64
	require.NoError(t, db.Table("bulk_operations").Count(&jobs).Error)
	assert.Zero(t, jobs, "no job is queued for a template that does not exist")

	// The worker re-checks: a template deleted after the job was queued
	// fails the job and writes nothing.
	op := &domain.BulkOperation{
		ID:            insertOp(t, db, tenant, author),
		OperationType: domain.BulkOperationTypeAssign,
		UpdateData:    map[string]interface{}{"mitigation_id": uuid.NewString()},
		TenantID:      tenant,
		CreatedBy:     author,
	}
	require.ErrorIs(t, s.processBulkAssign(op), domain.ErrNotFound)
	assert.Empty(t, store.created)
}

// Another tenant's plan cannot be used as a template: it is reported exactly
// like a missing one, and no plan is written in either tenant.
func TestBulkAssign_Unauthorized(t *testing.T) {
	db := setupBulkAssignDB(t)
	tenantA, tenantB := uuid.New(), uuid.New()
	author := uuid.New()
	seedRiskID(t, db, tenantA)

	foreign := templatePlan(tenantB)
	store := newFakeMitigationStore(foreign)
	s := NewBulkOperationService()
	s.mitigations = store

	_, err := s.CreateBulkOperation(author, tenantA, &domain.CreateBulkOperationRequest{
		OperationType: domain.BulkOperationTypeAssign,
		UpdateData:    map[string]interface{}{"mitigation_id": foreign.ID.String()},
	})
	require.ErrorIs(t, err, domain.ErrNotFound)

	op := &domain.BulkOperation{
		ID:            insertOp(t, db, tenantA, author),
		OperationType: domain.BulkOperationTypeAssign,
		UpdateData:    map[string]interface{}{"mitigation_id": foreign.ID.String()},
		TenantID:      tenantA,
		CreatedBy:     author,
	}
	require.ErrorIs(t, s.processBulkAssign(op), domain.ErrNotFound)
	assert.Empty(t, store.created, "no plan may be copied from another tenant")

	// A job with no mitigation_id at all is refused before it is queued.
	_, err = s.CreateBulkOperation(author, tenantA, &domain.CreateBulkOperationRequest{
		OperationType: domain.BulkOperationTypeAssign,
	})
	require.ErrorIs(t, err, domain.ErrValidation)
}

// Running the same job twice must not stack a second copy on every risk; a
// risk that already holds a plan with the template's title is skipped.
func TestBulkAssign_RerunIsIdempotent(t *testing.T) {
	db := setupBulkAssignDB(t)
	tenant, author := uuid.New(), uuid.New()
	riskWithTemplate := seedRiskID(t, db, tenant)
	seedRiskID(t, db, tenant)
	seedRiskID(t, db, tenant)

	tpl := templatePlan(tenant)
	tpl.RiskID = riskWithTemplate // the template already sits on one of the risks
	store := newFakeMitigationStore(tpl)
	s := NewBulkOperationService()
	s.mitigations = store

	run := func() *domain.BulkOperation {
		op := &domain.BulkOperation{
			ID:            insertOp(t, db, tenant, author),
			OperationType: domain.BulkOperationTypeAssign,
			UpdateData:    map[string]interface{}{"mitigation_id": tpl.ID.String()},
			TenantID:      tenant,
			CreatedBy:     author,
		}
		require.NoError(t, s.processBulkAssign(op))
		return op
	}

	first := run()
	assert.Len(t, store.created, 2, "the template's own risk is not given a copy of itself")
	assert.Equal(t, 3, first.ProcessedCount)

	second := run()
	assert.Len(t, store.created, 2, "a re-run writes nothing new")
	assert.Equal(t, 3, second.ProcessedCount)
	assert.Equal(t, 0, second.ErrorCount)

	var skipped int64
	require.NoError(t, db.Table("bulk_operation_logs").
		Where("bulk_operation_id = ? AND status = ?", second.ID.String(), "skipped").Count(&skipped).Error)
	assert.Equal(t, int64(3), skipped, "every risk of the re-run is logged as skipped")
}
