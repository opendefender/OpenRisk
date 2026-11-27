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

// memAuditRepo records what CreateAudit was asked to store.
type memAuditRepo struct {
	domain.ComplianceAuditRepository
	created []*domain.ComplianceAudit
}

func (m *memAuditRepo) CreateAudit(_ context.Context, a *domain.ComplianceAudit) error {
	m.created = append(m.created, a)
	return nil
}

// tenantFrameworks mirrors the tenant-scoped finder: (nil, nil) for a framework
// that is absent or belongs to another tenant.
type tenantFrameworks map[uuid.UUID]uuid.UUID // framework id -> owning tenant

func (f tenantFrameworks) GetFrameworkByID(_ context.Context, id, tenantID uuid.UUID) (*domain.ComplianceFramework, error) {
	if owner, ok := f[id]; ok && owner == tenantID {
		return &domain.ComplianceFramework{ID: id, TenantID: tenantID}, nil
	}
	return nil, nil
}

func TestCreateAudit_Success(t *testing.T) {
	tenant, own := uuid.New(), uuid.New()
	repo := &memAuditRepo{}
	uc := NewCreateAuditUseCase(repo, tenantFrameworks{own: tenant})

	withFramework, err := uc.Execute(context.Background(), tenant, uuid.New(), CreateAuditInput{Title: "ISO", Type: "internal", FrameworkID: &own})
	require.NoError(t, err)
	assert.Equal(t, &own, withFramework.FrameworkID)

	// framework_id is optional: absent stays accepted.
	withoutFramework, err := uc.Execute(context.Background(), tenant, uuid.New(), CreateAuditInput{Title: "ISO", Type: "internal"})
	require.NoError(t, err)
	assert.Nil(t, withoutFramework.FrameworkID)
	assert.Len(t, repo.created, 2)
}

// #1520: a foreign and an unknown framework answer alike, and nothing is stored.
func TestCreateAudit_NotFound_ForeignAndUnknownAlike(t *testing.T) {
	tenant, other := uuid.New(), uuid.New()
	foreign, unknown := uuid.New(), uuid.New()
	repo := &memAuditRepo{}
	uc := NewCreateAuditUseCase(repo, tenantFrameworks{foreign: other})

	_, errForeign := uc.Execute(context.Background(), tenant, uuid.New(), CreateAuditInput{Title: "x", Type: "internal", FrameworkID: &foreign})
	_, errUnknown := uc.Execute(context.Background(), tenant, uuid.New(), CreateAuditInput{Title: "x", Type: "internal", FrameworkID: &unknown})

	require.ErrorIs(t, errForeign, domain.ErrNotFound)
	require.ErrorIs(t, errUnknown, domain.ErrNotFound)
	assert.Equal(t, domain.MessageFromError(errForeign), domain.MessageFromError(errUnknown))
	assert.Equal(t, domain.HTTPStatusFromError(errForeign), domain.HTTPStatusFromError(errUnknown))
	assert.Empty(t, repo.created, "nothing is written on a 404")
}

func TestCreateAudit_Unauthorized(t *testing.T) {
	repo := &memAuditRepo{}
	uc := NewCreateAuditUseCase(repo, tenantFrameworks{})

	_, err := uc.Execute(context.Background(), uuid.Nil, uuid.New(), CreateAuditInput{Title: "x", Type: "internal"})

	require.ErrorIs(t, err, domain.ErrUnauthorized)
	assert.Empty(t, repo.created)
}
