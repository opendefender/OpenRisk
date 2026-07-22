// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

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
