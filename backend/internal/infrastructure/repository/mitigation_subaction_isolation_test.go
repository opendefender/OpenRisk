// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

// #706 — mitigation_subactions had no table, so the repository methods that
// filtered on the sub-action id alone were unreachable. Creating the table made
// them live. mitigation_subactions has no tenant_id: isolation goes through the
// parent mitigation, and these tests pin that for every path that used to skip
// it — Create, CheckDependency, CanComplete, GetDependencies, HasCycle.

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type subActionFixture struct {
	repo    *GormMitigationSubActionRepository
	db      *gorm.DB
	tenantA uuid.UUID
	tenantB uuid.UUID
	planA   *domain.Mitigation
	planA2  *domain.Mitigation
	planB   *domain.Mitigation
}

func newSubActionFixture(t *testing.T) *subActionFixture {
	t.Helper()
	_, db := setupMitigationTxRepo(t, false)
	f := &subActionFixture{
		repo:    &GormMitigationSubActionRepository{db: db},
		db:      db,
		tenantA: uuid.New(),
		tenantB: uuid.New(),
	}
	f.planA = newTxPlan(f.tenantA)
	f.planA2 = newTxPlan(f.tenantA)
	f.planB = newTxPlan(f.tenantB)
	for _, p := range []*domain.Mitigation{f.planA, f.planA2, f.planB} {
		require.NoError(t, db.Create(p).Error)
	}
	return f
}

// seed writes a sub-action directly, bypassing Create's checks — the shape a
// row written before those checks existed could have.
func (f *subActionFixture) seed(t *testing.T, plan *domain.Mitigation, title string, completed bool, dependsOn *uuid.UUID) *domain.MitigationSubAction {
	t.Helper()
	s := &domain.MitigationSubAction{ID: uuid.New(), MitigationID: plan.ID, Title: title, Completed: completed, DependsOn: dependsOn}
	require.NoError(t, f.db.Create(s).Error)
	return s
}

func isValidation(err error) bool { return errors.Is(err, domain.ErrValidation) }

// --- Create ------------------------------------------------------------------

func TestSubActionCreate_Success(t *testing.T) {
	f := newSubActionFixture(t)
	first := f.seed(t, f.planA, "Inventory exposed hosts", false, nil)

	next := &domain.MitigationSubAction{ID: uuid.New(), MitigationID: f.planA.ID, Title: "Patch them", DependsOn: &first.ID}
	require.NoError(t, f.repo.Create(f.tenantA.String(), next))

	list, err := f.repo.List(f.tenantA.String(), f.planA.ID)
	require.NoError(t, err)
	assert.Len(t, list, 2)
}

func TestSubActionCreate_NotFound(t *testing.T) {
	f := newSubActionFixture(t)
	err := f.repo.Create(f.tenantA.String(), &domain.MitigationSubAction{ID: uuid.New(), MitigationID: uuid.New(), Title: "Orphan"})
	assert.ErrorIs(t, err, domain.ErrForbidden)
}

func TestSubActionCreate_Unauthorized(t *testing.T) {
	f := newSubActionFixture(t)
	// Tenant A writing into tenant B's plan.
	err := f.repo.Create(f.tenantA.String(), &domain.MitigationSubAction{ID: uuid.New(), MitigationID: f.planB.ID, Title: "Intruder"})
	assert.ErrorIs(t, err, domain.ErrForbidden)

	var count int64
	// Deliberately unscoped: the assertion reads tenant B's plan from outside any tenant to prove tenant A wrote nothing into it.
	require.NoError(t, f.db.Model(&domain.MitigationSubAction{}).Where("mitigation_id = ?", f.planB.ID).Count(&count).Error)
	assert.Zero(t, count, "nothing may be written into another tenant's plan")
}

// --- CheckDependency ---------------------------------------------------------

func TestSubActionCheckDependency_Success(t *testing.T) {
	f := newSubActionFixture(t)
	dep := f.seed(t, f.planA, "First", false, nil)
	self := f.seed(t, f.planA, "Second", false, nil)
	assert.NoError(t, f.repo.CheckDependency(f.tenantA.String(), f.planA.ID, &self.ID, dep.ID))
}

func TestSubActionCheckDependency_NotFound(t *testing.T) {
	f := newSubActionFixture(t)
	assert.True(t, isValidation(f.repo.CheckDependency(f.tenantA.String(), f.planA.ID, nil, uuid.New())))
}

func TestSubActionCheckDependency_Unauthorized(t *testing.T) {
	f := newSubActionFixture(t)
	foreign := f.seed(t, f.planB, "Tenant B's secret step", false, nil)
	otherPlan := f.seed(t, f.planA2, "Same tenant, other plan", false, nil)
	self := f.seed(t, f.planA, "Mine", false, nil)

	err := f.repo.CheckDependency(f.tenantA.String(), f.planA.ID, &self.ID, foreign.ID)
	assert.True(t, isValidation(err), "another tenant's sub-action is refused")
	assert.NotContains(t, err.Error(), foreign.Title)

	assert.True(t, isValidation(f.repo.CheckDependency(f.tenantA.String(), f.planA.ID, &self.ID, otherPlan.ID)),
		"a sub-action of another plan is refused")
	assert.True(t, isValidation(f.repo.CheckDependency(f.tenantA.String(), f.planA.ID, &self.ID, self.ID)),
		"a sub-action cannot depend on itself")

	createErr := f.repo.Create(f.tenantA.String(), &domain.MitigationSubAction{ID: uuid.New(), MitigationID: f.planA.ID, Title: "x", DependsOn: &foreign.ID})
	assert.True(t, isValidation(createErr), "Create applies the same rule")
}

// --- CanComplete -------------------------------------------------------------

func TestSubActionCanComplete_Success(t *testing.T) {
	f := newSubActionFixture(t)
	done := f.seed(t, f.planA, "Done step", true, nil)
	next := f.seed(t, f.planA, "Next step", false, &done.ID)
	ok, err := f.repo.CanComplete(f.tenantA.String(), next.ID)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestSubActionCanComplete_NotFound(t *testing.T) {
	f := newSubActionFixture(t)
	_, err := f.repo.CanComplete(f.tenantA.String(), uuid.New())
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestSubActionCanComplete_Unauthorized(t *testing.T) {
	f := newSubActionFixture(t)
	foreignStep := f.seed(t, f.planB, "Tenant B step", false, nil)
	_, err := f.repo.CanComplete(f.tenantA.String(), foreignStep.ID)
	assert.ErrorIs(t, err, domain.ErrNotFound, "another tenant's sub-action does not exist for tenant A")

	// A dependency pointing across tenants, written before the check existed:
	// it must read as missing, never disclose the foreign row's title.
	secret := f.seed(t, f.planB, "Confidential: unpatched core banking host", false, nil)
	legacy := f.seed(t, f.planA, "Legacy row", false, &secret.ID)
	ok, err := f.repo.CanComplete(f.tenantA.String(), legacy.ID)
	assert.False(t, ok)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), secret.Title)
}

// --- GetDependencies / HasCycle ------------------------------------------------

func TestSubActionGetDependencies_Unauthorized(t *testing.T) {
	f := newSubActionFixture(t)
	mine := f.seed(t, f.planA, "Mine", false, nil)
	f.seed(t, f.planA, "Depends on mine", false, &mine.ID)
	// A tenant B row naming tenant A's id (only possible via a legacy write).
	f.seed(t, f.planB, "Foreign dependent", false, &mine.ID)

	deps, err := f.repo.GetDependencies(f.tenantA.String(), mine.ID)
	require.NoError(t, err)
	require.Len(t, deps, 1)
	assert.Equal(t, "Depends on mine", deps[0].Title)
}

func TestSubActionHasCycle_Success(t *testing.T) {
	f := newSubActionFixture(t)
	a := f.seed(t, f.planA, "A", false, nil)
	b := f.seed(t, f.planA, "B", false, &a.ID)
	cycle, err := f.repo.HasCycle(f.tenantA.String(), a.ID, b.ID)
	require.NoError(t, err)
	assert.True(t, cycle, "A depending on B, which depends on A, is a cycle")
}

func TestSubActionHasCycle_Unauthorized(t *testing.T) {
	f := newSubActionFixture(t)
	mine := f.seed(t, f.planA, "Mine", false, nil)
	// Tenant B chain that loops back to tenant A's id: invisible to tenant A.
	foreign := f.seed(t, f.planB, "Foreign", false, &mine.ID)
	cycle, err := f.repo.HasCycle(f.tenantA.String(), mine.ID, foreign.ID)
	require.NoError(t, err)
	assert.False(t, cycle, "the walk never follows another tenant's rows")
}
