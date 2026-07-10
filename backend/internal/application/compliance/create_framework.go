// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package compliance

import (
	"context"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// CreateFrameworkInput is the input for creating a tenant-scoped compliance
// framework (e.g. "ISO 27001", "COBAC").
type CreateFrameworkInput struct {
	Name        string
	Version     string
	Description string
}

// CreateFrameworkUseCase handles creation of a new tenant-scoped compliance
// framework.
type CreateFrameworkUseCase struct {
	repo domain.ComplianceRepository
}

func NewCreateFrameworkUseCase(repo domain.ComplianceRepository) *CreateFrameworkUseCase {
	return &CreateFrameworkUseCase{repo: repo}
}

func (uc *CreateFrameworkUseCase) Execute(ctx context.Context, tenantID uuid.UUID, input CreateFrameworkInput) (*domain.ComplianceFramework, error) {
	if input.Name == "" {
		return nil, domain.NewValidationError("name is required")
	}
	if len(input.Name) > 255 {
		return nil, domain.NewValidationError("name must be 255 characters or less")
	}

	fw := &domain.ComplianceFramework{
		ID:          uuid.New(),
		TenantID:    tenantID,
		Name:        input.Name,
		Version:     input.Version,
		Description: input.Description,
	}

	if err := uc.repo.CreateFramework(ctx, fw); err != nil {
		return nil, err
	}
	return fw, nil
}
