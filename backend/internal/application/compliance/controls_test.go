// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package compliance

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateControlUseCase_Success(t *testing.T) {
	tenantID := uuid.New()
	fwID := uuid.New()
	repo := &MockComplianceRepository{
		getFrameworkByIDFunc: func(ctx context.Context, id, tenantID uuid.UUID) (*domain.ComplianceFramework, error) {
			return &domain.ComplianceFramework{ID: fwID, Name: "ISO 27001"}, nil
		},
	}
	uc := NewCreateControlUseCase(repo)

	control, err := uc.Execute(context.Background(), tenantID, CreateControlInput{
		FrameworkID: fwID, ReferenceCode: "A.5.1.1", Name: "Policies for information security",
	})

	require.NoError(t, err)
	require.NotNil(t, control)
	assert.Equal(t, tenantID, control.TenantID)
	assert.Equal(t, domain.ControlStatusNotImplemented, control.Status, "new controls must always start not_implemented")
}

func TestCreateControlUseCase_MissingName_Validation(t *testing.T) {
	repo := &MockComplianceRepository{}
	uc := NewCreateControlUseCase(repo)

	_, err := uc.Execute(context.Background(), uuid.New(), CreateControlInput{FrameworkID: uuid.New()})

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrValidation)
}

func TestCreateControlUseCase_FrameworkNotFound(t *testing.T) {
	repo := &MockComplianceRepository{
		getFrameworkByIDFunc: func(ctx context.Context, id, tenantID uuid.UUID) (*domain.ComplianceFramework, error) {
			return nil, nil
		},
	}
	uc := NewCreateControlUseCase(repo)

	_, err := uc.Execute(context.Background(), uuid.New(), CreateControlInput{FrameworkID: uuid.New(), Name: "x"})

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestCreateControlUseCase_DuplicateReferenceCode_Conflict(t *testing.T) {
	tenantID := uuid.New()
	fwID := uuid.New()
	repo := &MockComplianceRepository{
		getFrameworkByIDFunc: func(ctx context.Context, id, tenantID uuid.UUID) (*domain.ComplianceFramework, error) {
			return &domain.ComplianceFramework{ID: fwID}, nil
		},
		listControlsByFrameworkFunc: func(ctx context.Context, tid, fid uuid.UUID) ([]domain.ComplianceControl, error) {
			return []domain.ComplianceControl{{ReferenceCode: "A.5.1.1"}}, nil
		},
	}
	uc := NewCreateControlUseCase(repo)

	_, err := uc.Execute(context.Background(), tenantID, CreateControlInput{
		FrameworkID: fwID, ReferenceCode: "A.5.1.1", Name: "duplicate",
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrConflict)
}

func TestGetControlUseCase_CrossTenant_NotFound(t *testing.T) {
	tenantA := uuid.New()
	tenantB := uuid.New()
	controlID := uuid.New()
	repo := &MockComplianceRepository{
		getControlByIDFunc: func(ctx context.Context, id, tid uuid.UUID) (*domain.ComplianceControl, error) {
			if tid == tenantA {
				return &domain.ComplianceControl{ID: controlID, TenantID: tenantA}, nil
			}
			return nil, nil // repository already returns nil for cross-tenant
		},
	}
	uc := NewGetControlUseCase(repo)

	_, err := uc.Execute(context.Background(), tenantB, controlID)

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound, "cross-tenant access must surface as 404, not leak the resource")
}

func TestUpdateControlUseCase_Success(t *testing.T) {
	tenantID := uuid.New()
	controlID := uuid.New()
	var saved *domain.ComplianceControl
	repo := &MockComplianceRepository{
		getControlByIDFunc: func(ctx context.Context, id, tid uuid.UUID) (*domain.ComplianceControl, error) {
			return &domain.ComplianceControl{ID: controlID, TenantID: tenantID, Status: domain.ControlStatusNotImplemented}, nil
		},
		updateControlFunc: func(ctx context.Context, c *domain.ComplianceControl) error {
			saved = c
			return nil
		},
	}
	uc := NewUpdateControlUseCase(repo)

	implemented := domain.ControlStatusImplemented
	control, err := uc.Execute(context.Background(), tenantID, controlID, UpdateControlInput{Status: &implemented})

	require.NoError(t, err)
	assert.Equal(t, domain.ControlStatusImplemented, control.Status)
	require.NotNil(t, saved)
	assert.Equal(t, tenantID, saved.TenantID)
}

func TestUpdateControlUseCase_InvalidStatus_Validation(t *testing.T) {
	tenantID := uuid.New()
	controlID := uuid.New()
	repo := &MockComplianceRepository{
		getControlByIDFunc: func(ctx context.Context, id, tid uuid.UUID) (*domain.ComplianceControl, error) {
			return &domain.ComplianceControl{ID: controlID, TenantID: tenantID}, nil
		},
	}
	uc := NewUpdateControlUseCase(repo)

	bogus := domain.ControlStatus("on_fire")
	_, err := uc.Execute(context.Background(), tenantID, controlID, UpdateControlInput{Status: &bogus})

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrValidation)
}

func TestUpdateControlUseCase_CrossTenant_NotFound(t *testing.T) {
	repo := &MockComplianceRepository{
		getControlByIDFunc: func(ctx context.Context, id, tid uuid.UUID) (*domain.ComplianceControl, error) {
			return nil, nil
		},
	}
	uc := NewUpdateControlUseCase(repo)

	_, err := uc.Execute(context.Background(), uuid.New(), uuid.New(), UpdateControlInput{})

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestDeleteControlUseCase_Success(t *testing.T) {
	tenantID := uuid.New()
	controlID := uuid.New()
	deleted := false
	repo := &MockComplianceRepository{
		getControlByIDFunc: func(ctx context.Context, id, tid uuid.UUID) (*domain.ComplianceControl, error) {
			return &domain.ComplianceControl{ID: controlID, TenantID: tenantID}, nil
		},
		deleteControlFunc: func(ctx context.Context, id, tid uuid.UUID) error {
			deleted = true
			return nil
		},
	}
	uc := NewDeleteControlUseCase(repo)

	err := uc.Execute(context.Background(), tenantID, controlID)

	require.NoError(t, err)
	assert.True(t, deleted)
}

func TestDeleteControlUseCase_CrossTenant_NotFound(t *testing.T) {
	repo := &MockComplianceRepository{
		getControlByIDFunc: func(ctx context.Context, id, tid uuid.UUID) (*domain.ComplianceControl, error) {
			return nil, nil
		},
	}
	uc := NewDeleteControlUseCase(repo)

	err := uc.Execute(context.Background(), uuid.New(), uuid.New())

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}
