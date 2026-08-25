// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
)

// memPolicyStore is an in-memory MFAPolicyStore keyed by tenant, so cross-tenant
// leakage would show up as a wrong answer rather than as a passing test.
type memPolicyStore struct {
	rows map[uuid.UUID]domain.MFAPolicy
}

func newMemPolicyStore() *memPolicyStore {
	return &memPolicyStore{rows: map[uuid.UUID]domain.MFAPolicy{}}
}

func (m *memPolicyStore) GetMFAPolicy(_ context.Context, tenantID uuid.UUID) (*domain.MFAPolicy, error) {
	if row, ok := m.rows[tenantID]; ok {
		return &row, nil
	}
	return nil, nil
}

func (m *memPolicyStore) SaveMFAPolicy(_ context.Context, p *domain.MFAPolicy) error {
	if err := p.Validate(); err != nil {
		return err
	}
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	m.rows[p.TenantID] = *p
	return nil
}

// recordingInvalidator proves the cache is dropped on save.
type recordingInvalidator struct{ tenants []uuid.UUID }

func (r *recordingInvalidator) InvalidateTenant(t uuid.UUID) { r.tenants = append(r.tenants, t) }

func policyUseCases(store MFAPolicyStore) (*GetMFAPolicyUseCase, *UpdateMFAPolicyUseCase) {
	orgRoles, businessRoles := domain.DefaultMFAPrivilegeRoles()
	return NewGetMFAPolicyUseCase(store, orgRoles, businessRoles),
		NewUpdateMFAPolicyUseCase(store, orgRoles, businessRoles)
}

func TestGetMFAPolicy_UnsavedTenantReadsAsTheShippedDefault(t *testing.T) {
	get, _ := policyUseCases(newMemPolicyStore())

	view, err := get.Execute(context.Background(), uuid.New())
	require.NoError(t, err)

	assert.Equal(t, 7, view.GraceDays)
	assert.False(t, view.Configured,
		"the screen must be able to say 'using the default' rather than imply somebody chose it")
	assert.Equal(t, 0, view.MinDays)
	assert.Equal(t, 90, view.MaxDays)
	assert.Contains(t, view.PrivilegedOrgRoles, "admin")
	assert.Contains(t, view.PrivilegedBusinessRoles, "rssi")
}

func TestUpdateMFAPolicy_PersistsAndReadsBack(t *testing.T) {
	store := newMemPolicyStore()
	get, update := policyUseCases(store)
	tenant, actor := uuid.New(), uuid.New()

	saved, err := update.Execute(context.Background(), UpdateMFAPolicyInput{
		TenantID: tenant, ActorID: actor, GraceDays: 3,
	})
	require.NoError(t, err)
	assert.Equal(t, 3, saved.GraceDays)
	require.NotNil(t, saved.UpdatedByID)
	assert.Equal(t, actor, *saved.UpdatedByID)

	read, err := get.Execute(context.Background(), tenant)
	require.NoError(t, err)
	assert.Equal(t, 3, read.GraceDays)
	assert.True(t, read.Configured)
}

func TestUpdateMFAPolicy_AcceptsZeroAsRequireImmediately(t *testing.T) {
	store := newMemPolicyStore()
	_, update := policyUseCases(store)

	view, err := update.Execute(context.Background(), UpdateMFAPolicyInput{TenantID: uuid.New(), GraceDays: 0})
	require.NoError(t, err)
	assert.Equal(t, 0, view.GraceDays)
}

func TestUpdateMFAPolicy_RejectsOutOfRangeValues(t *testing.T) {
	// The ceiling is what stops "configure the policy" becoming "switch it off".
	_, update := policyUseCases(newMemPolicyStore())

	for _, days := range []int{-1, 91, 100000} {
		_, err := update.Execute(context.Background(), UpdateMFAPolicyInput{TenantID: uuid.New(), GraceDays: days})
		require.Error(t, err, "grace_days=%d must be refused", days)
		appErr, ok := err.(*domain.AppError)
		require.True(t, ok)
		assert.Equal(t, domain.ErrValidation, appErr.Err,
			"a bad value is a 400 the form can render, not a 500")
	}
}

func TestMFAPolicy_RequiresATenant(t *testing.T) {
	get, update := policyUseCases(newMemPolicyStore())

	_, err := get.Execute(context.Background(), uuid.Nil)
	require.Error(t, err)
	_, err = update.Execute(context.Background(), UpdateMFAPolicyInput{TenantID: uuid.Nil, GraceDays: 5})
	require.Error(t, err)
}

func TestMFAPolicy_IsTenantScoped(t *testing.T) {
	// Tenant A at 7 days, tenant B at 1 day. Neither may read or move the other.
	store := newMemPolicyStore()
	get, update := policyUseCases(store)
	a, b := uuid.New(), uuid.New()

	_, err := update.Execute(context.Background(), UpdateMFAPolicyInput{TenantID: a, GraceDays: 7})
	require.NoError(t, err)
	_, err = update.Execute(context.Background(), UpdateMFAPolicyInput{TenantID: b, GraceDays: 1})
	require.NoError(t, err)

	viewA, err := get.Execute(context.Background(), a)
	require.NoError(t, err)
	viewB, err := get.Execute(context.Background(), b)
	require.NoError(t, err)

	assert.Equal(t, 7, viewA.GraceDays)
	assert.Equal(t, 1, viewB.GraceDays, "one tenant's window must never decide another's")

	// Saving A again leaves B where it was.
	_, err = update.Execute(context.Background(), UpdateMFAPolicyInput{TenantID: a, GraceDays: 30})
	require.NoError(t, err)
	viewB, err = get.Execute(context.Background(), b)
	require.NoError(t, err)
	assert.Equal(t, 1, viewB.GraceDays)
}

func TestUpdateMFAPolicy_DropsTheResolverCacheForThatTenantOnly(t *testing.T) {
	// A shortened window has to bite now. An administrator who sets "0 days" and
	// watches their colleagues keep working concludes the setting is decorative.
	inv := &recordingInvalidator{}
	_, update := policyUseCases(newMemPolicyStore())
	update.WithCacheInvalidator(inv)
	tenant := uuid.New()

	_, err := update.Execute(context.Background(), UpdateMFAPolicyInput{TenantID: tenant, GraceDays: 0})
	require.NoError(t, err)

	require.Len(t, inv.tenants, 1)
	assert.Equal(t, tenant, inv.tenants[0])
}

func TestUpdateMFAPolicy_DoesNotInvalidateOnARefusedWrite(t *testing.T) {
	inv := &recordingInvalidator{}
	_, update := policyUseCases(newMemPolicyStore())
	update.WithCacheInvalidator(inv)

	_, err := update.Execute(context.Background(), UpdateMFAPolicyInput{TenantID: uuid.New(), GraceDays: 999})
	require.Error(t, err)
	assert.Empty(t, inv.tenants, "nothing changed, so nothing to invalidate")
}
