// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package complianceaudit

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
)

// updAuditRepo serves one stored audit and records what UpdateAudit stores.
type updAuditRepo struct {
	domain.ComplianceAuditRepository
	audit   domain.ComplianceAudit
	updated int
}

func (r *updAuditRepo) GetAuditByID(_ context.Context, id, tenantID uuid.UUID) (*domain.ComplianceAudit, error) {
	if id != r.audit.ID || tenantID != r.audit.TenantID {
		return nil, nil
	}
	cp := r.audit
	return &cp, nil
}

func (r *updAuditRepo) UpdateAudit(_ context.Context, a *domain.ComplianceAudit) error {
	r.updated++
	r.audit = *a
	return nil
}

func newUpdRepo(tenant uuid.UUID) *updAuditRepo {
	return &updAuditRepo{audit: domain.ComplianceAudit{ID: uuid.New(), TenantID: tenant, Title: "t", Type: domain.AuditTypeInternal}}
}

func TestUpdateAudit_Success(t *testing.T) {
	tenant, own := uuid.New(), uuid.New()
	repo := newUpdRepo(tenant)
	uc := NewUpdateAuditUseCase(repo, tenantFrameworks{own: tenant})

	got, err := uc.Execute(context.Background(), tenant, repo.audit.ID, UpdateAuditInput{FrameworkID: &own})
	require.NoError(t, err)
	assert.Equal(t, &own, got.FrameworkID)
	assert.Equal(t, 1, repo.updated)

	// No framework_id in the body stays accepted, and clearing needs no lookup.
	title := "new"
	_, err = NewUpdateAuditUseCase(repo, tenantFrameworks{}).Execute(context.Background(), tenant, repo.audit.ID, UpdateAuditInput{Title: &title})
	require.NoError(t, err)
	got, err = NewUpdateAuditUseCase(repo, tenantFrameworks{}).Execute(context.Background(), tenant, repo.audit.ID, UpdateAuditInput{ClearFramework: true})
	require.NoError(t, err)
	assert.Nil(t, got.FrameworkID)
}

// #1520: a foreign and an unknown framework answer alike, nothing is stored.
func TestUpdateAudit_NotFound(t *testing.T) {
	tenant, other := uuid.New(), uuid.New()
	foreign, unknown := uuid.New(), uuid.New()
	repo := newUpdRepo(tenant)
	uc := NewUpdateAuditUseCase(repo, tenantFrameworks{foreign: other})

	_, errForeign := uc.Execute(context.Background(), tenant, repo.audit.ID, UpdateAuditInput{FrameworkID: &foreign})
	_, errUnknown := uc.Execute(context.Background(), tenant, repo.audit.ID, UpdateAuditInput{FrameworkID: &unknown})

	require.ErrorIs(t, errForeign, domain.ErrNotFound)
	require.ErrorIs(t, errUnknown, domain.ErrNotFound)
	assert.Equal(t, domain.MessageFromError(errForeign), domain.MessageFromError(errUnknown))
	assert.Equal(t, domain.HTTPStatusFromError(errForeign), domain.HTTPStatusFromError(errUnknown))
	assert.Zero(t, repo.updated, "nothing is written on a 404")
	assert.Nil(t, repo.audit.FrameworkID)

	// A foreign or unknown audit id is the same not-found too.
	_, errAudit := uc.Execute(context.Background(), tenant, uuid.New(), UpdateAuditInput{})
	require.ErrorIs(t, errAudit, domain.ErrNotFound)
}

func TestUpdateAudit_Unauthorized(t *testing.T) {
	tenant, own := uuid.New(), uuid.New()
	repo := newUpdRepo(tenant)
	uc := NewUpdateAuditUseCase(repo, tenantFrameworks{own: tenant})

	_, err := uc.Execute(context.Background(), uuid.Nil, repo.audit.ID, UpdateAuditInput{FrameworkID: &own})

	require.ErrorIs(t, err, domain.ErrUnauthorized)
	assert.Zero(t, repo.updated)
}
