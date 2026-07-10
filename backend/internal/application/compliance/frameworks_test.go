// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package compliance

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateFrameworkUseCase_Success(t *testing.T) {
	tenantID := uuid.New()
	var created *domain.ComplianceFramework
	repo := &MockComplianceRepository{
		createFrameworkFunc: func(_ context.Context, fw *domain.ComplianceFramework) error {
			created = fw
			return nil
		},
	}
	uc := NewCreateFrameworkUseCase(repo)

	fw, err := uc.Execute(context.Background(), tenantID, CreateFrameworkInput{
		Name: "ISO 27001", Version: "2022", Description: "Information security",
	})

	require.NoError(t, err)
	require.NotNil(t, fw)
	assert.Equal(t, "ISO 27001", fw.Name)
	assert.NotEqual(t, uuid.Nil, fw.ID)
	assert.Equal(t, tenantID, fw.TenantID, "framework must be stamped with the caller's tenant")
	assert.Equal(t, tenantID, created.TenantID)
}

func TestCreateFrameworkUseCase_MissingName_Validation(t *testing.T) {
	repo := &MockComplianceRepository{}
	uc := NewCreateFrameworkUseCase(repo)

	_, err := uc.Execute(context.Background(), uuid.New(), CreateFrameworkInput{Version: "2022"})

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrValidation)
}

func TestCreateFrameworkUseCase_DuplicateFromRepo_Conflict(t *testing.T) {
	repo := &MockComplianceRepository{
		createFrameworkFunc: func(ctx context.Context, fw *domain.ComplianceFramework) error {
			return domain.NewConflictError("framework", "name+version")
		},
	}
	uc := NewCreateFrameworkUseCase(repo)

	_, err := uc.Execute(context.Background(), uuid.New(), CreateFrameworkInput{Name: "ISO 27001", Version: "2022"})

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrConflict)
}

func TestGetFrameworkUseCase_Success(t *testing.T) {
	fwID := uuid.New()
	tenantID := uuid.New()
	repo := &MockComplianceRepository{
		getFrameworkByIDFunc: func(_ context.Context, id, tid uuid.UUID) (*domain.ComplianceFramework, error) {
			assert.Equal(t, tenantID, tid, "framework lookup must be tenant-scoped")
			if id == fwID {
				return &domain.ComplianceFramework{ID: fwID, TenantID: tid, Name: "SOC 2"}, nil
			}
			return nil, nil
		},
	}
	uc := NewGetFrameworkUseCase(repo)

	fw, err := uc.Execute(context.Background(), tenantID, fwID)

	require.NoError(t, err)
	assert.Equal(t, "SOC 2", fw.Name)
}

func TestGetFrameworkUseCase_NotFound(t *testing.T) {
	repo := &MockComplianceRepository{}
	uc := NewGetFrameworkUseCase(repo)

	_, err := uc.Execute(context.Background(), uuid.New(), uuid.New())

	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrNotFound))
}

func TestListFrameworksUseCase_Success(t *testing.T) {
	tenantID := uuid.New()
	repo := &MockComplianceRepository{
		listFrameworksFunc: func(_ context.Context, tid uuid.UUID) ([]domain.ComplianceFramework, error) {
			assert.Equal(t, tenantID, tid, "listing must be tenant-scoped")
			return []domain.ComplianceFramework{{Name: "ISO 27001"}, {Name: "SOC 2"}}, nil
		},
	}
	uc := NewListFrameworksUseCase(repo)

	frameworks, err := uc.Execute(context.Background(), tenantID)

	require.NoError(t, err)
	assert.Len(t, frameworks, 2)
}
