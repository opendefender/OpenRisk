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
			// One evidence present → the strict "implemented needs a proof" rule is satisfied.
			return &domain.ComplianceControl{
				ID: controlID, TenantID: tenantID, Status: domain.ControlStatusNotImplemented,
				Evidences: []domain.ControlEvidence{{ID: uuid.New(), TenantID: tenantID, ControlID: controlID}},
			}, nil
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

// A control with no evidence cannot transition to "implemented" — the strict
// compliance rule mirrored client-side in FrameworkDetail.tsx.
func TestUpdateControlUseCase_ImplementedWithoutEvidence_Blocked(t *testing.T) {
	tenantID := uuid.New()
	controlID := uuid.New()
	updateCalled := false
	repo := &MockComplianceRepository{
		getControlByIDFunc: func(ctx context.Context, id, tid uuid.UUID) (*domain.ComplianceControl, error) {
			return &domain.ComplianceControl{ID: controlID, TenantID: tenantID, Status: domain.ControlStatusInProgress}, nil
		},
		updateControlFunc: func(ctx context.Context, c *domain.ComplianceControl) error {
			updateCalled = true
			return nil
		},
	}
	uc := NewUpdateControlUseCase(repo)

	implemented := domain.ControlStatusImplemented
	_, err := uc.Execute(context.Background(), tenantID, controlID, UpdateControlInput{Status: &implemented})

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrValidation)
	assert.False(t, updateCalled, "the control must not be persisted when the evidence rule fails")
}

// Statuses other than "implemented" never require evidence.
func TestUpdateControlUseCase_InProgressWithoutEvidence_Allowed(t *testing.T) {
	tenantID := uuid.New()
	controlID := uuid.New()
	repo := &MockComplianceRepository{
		getControlByIDFunc: func(ctx context.Context, id, tid uuid.UUID) (*domain.ComplianceControl, error) {
			return &domain.ComplianceControl{ID: controlID, TenantID: tenantID, Status: domain.ControlStatusNotImplemented}, nil
		},
	}
	uc := NewUpdateControlUseCase(repo)

	inProgress := domain.ControlStatusInProgress
	control, err := uc.Execute(context.Background(), tenantID, controlID, UpdateControlInput{Status: &inProgress})

	require.NoError(t, err)
	assert.Equal(t, domain.ControlStatusInProgress, control.Status)
}

// Editing a control that is ALREADY implemented (e.g. renaming it) must not be
// blocked even if it has no evidence — the rule only guards the transition in.
func TestUpdateControlUseCase_AlreadyImplemented_NotReblocked(t *testing.T) {
	tenantID := uuid.New()
	controlID := uuid.New()
	repo := &MockComplianceRepository{
		getControlByIDFunc: func(ctx context.Context, id, tid uuid.UUID) (*domain.ComplianceControl, error) {
			return &domain.ComplianceControl{ID: controlID, TenantID: tenantID, Status: domain.ControlStatusImplemented}, nil
		},
	}
	uc := NewUpdateControlUseCase(repo)

	newName := "renamed"
	implemented := domain.ControlStatusImplemented
	_, err := uc.Execute(context.Background(), tenantID, controlID, UpdateControlInput{Name: &newName, Status: &implemented})

	require.NoError(t, err)
}

// ListControls attaches each control's evidence count from a single grouped query.
func TestListControlsUseCase_AttachesEvidenceCount(t *testing.T) {
	tenantID := uuid.New()
	fwID := uuid.New()
	c1 := uuid.New()
	c2 := uuid.New()
	repo := &MockComplianceRepository{
		listControlsByFrameworkFunc: func(ctx context.Context, tid, fid uuid.UUID) ([]domain.ComplianceControl, error) {
			return []domain.ComplianceControl{
				{ID: c1, TenantID: tenantID, FrameworkID: fwID},
				{ID: c2, TenantID: tenantID, FrameworkID: fwID},
			}, nil
		},
		countEvidencesByFwFunc: func(ctx context.Context, tid, fid uuid.UUID) (map[uuid.UUID]int, error) {
			return map[uuid.UUID]int{c1: 3}, nil // c2 absent → 0
		},
	}
	uc := NewListControlsUseCase(repo)

	controls, err := uc.Execute(context.Background(), tenantID, fwID)

	require.NoError(t, err)
	require.Len(t, controls, 2)
	assert.Equal(t, 3, controls[0].EvidenceCount)
	assert.Equal(t, 0, controls[1].EvidenceCount, "controls with no evidence report a zero count")
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
