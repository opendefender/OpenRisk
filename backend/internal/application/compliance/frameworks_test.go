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
	repo := &MockComplianceRepository{}
	uc := NewCreateFrameworkUseCase(repo)

	fw, err := uc.Execute(context.Background(), CreateFrameworkInput{
		Name: "ISO 27001", Version: "2022", Description: "Information security",
	})

	require.NoError(t, err)
	require.NotNil(t, fw)
	assert.Equal(t, "ISO 27001", fw.Name)
	assert.NotEqual(t, uuid.Nil, fw.ID)
}

func TestCreateFrameworkUseCase_MissingName_Validation(t *testing.T) {
	repo := &MockComplianceRepository{}
	uc := NewCreateFrameworkUseCase(repo)

	_, err := uc.Execute(context.Background(), CreateFrameworkInput{Version: "2022"})

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

	_, err := uc.Execute(context.Background(), CreateFrameworkInput{Name: "ISO 27001", Version: "2022"})

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrConflict)
}

func TestGetFrameworkUseCase_Success(t *testing.T) {
	fwID := uuid.New()
	repo := &MockComplianceRepository{
		getFrameworkByIDFunc: func(ctx context.Context, id uuid.UUID) (*domain.ComplianceFramework, error) {
			if id == fwID {
				return &domain.ComplianceFramework{ID: fwID, Name: "SOC 2"}, nil
			}
			return nil, nil
		},
	}
	uc := NewGetFrameworkUseCase(repo)

	fw, err := uc.Execute(context.Background(), fwID)

	require.NoError(t, err)
	assert.Equal(t, "SOC 2", fw.Name)
}

func TestGetFrameworkUseCase_NotFound(t *testing.T) {
	repo := &MockComplianceRepository{}
	uc := NewGetFrameworkUseCase(repo)

	_, err := uc.Execute(context.Background(), uuid.New())

	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrNotFound))
}

func TestListFrameworksUseCase_Success(t *testing.T) {
	repo := &MockComplianceRepository{
		listFrameworksFunc: func(ctx context.Context) ([]domain.ComplianceFramework, error) {
			return []domain.ComplianceFramework{{Name: "ISO 27001"}, {Name: "SOC 2"}}, nil
		},
	}
	uc := NewListFrameworksUseCase(repo)

	frameworks, err := uc.Execute(context.Background())

	require.NoError(t, err)
	assert.Len(t, frameworks, 2)
}
