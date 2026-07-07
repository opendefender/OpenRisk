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

// CreateControlInput is the input for instantiating a control under a
// framework, for a given tenant. Status always starts at not_implemented —
// changing it is a distinct, later step (see UpdateControlUseCase).
type CreateControlInput struct {
	FrameworkID   uuid.UUID
	ReferenceCode string
	Name          string
	Description   string
}

// CreateControlUseCase handles instantiating a compliance control for a
// tenant under an existing global framework.
type CreateControlUseCase struct {
	repo domain.ComplianceRepository
}

func NewCreateControlUseCase(repo domain.ComplianceRepository) *CreateControlUseCase {
	return &CreateControlUseCase{repo: repo}
}

func (uc *CreateControlUseCase) Execute(ctx context.Context, tenantID uuid.UUID, input CreateControlInput) (*domain.ComplianceControl, error) {
	if input.Name == "" {
		return nil, domain.NewValidationError("name is required")
	}

	fw, err := uc.repo.GetFrameworkByID(ctx, input.FrameworkID)
	if err != nil {
		return nil, err
	}
	if fw == nil {
		return nil, domain.NewNotFoundError("framework", input.FrameworkID)
	}

	// Fast, friendly pre-check for the common (non-concurrent) case. The
	// repository also enforces this via a DB unique index and translates a
	// real race into the same domain.ErrConflict — see
	// GormComplianceRepository.CreateControl's doc comment.
	if input.ReferenceCode != "" {
		existing, err := uc.repo.ListControlsByFramework(ctx, tenantID, input.FrameworkID)
		if err != nil {
			return nil, err
		}
		for _, c := range existing {
			if c.ReferenceCode == input.ReferenceCode {
				return nil, domain.NewConflictError("control", "reference_code")
			}
		}
	}

	control := &domain.ComplianceControl{
		ID:            uuid.New(),
		TenantID:      tenantID,
		FrameworkID:   input.FrameworkID,
		ReferenceCode: input.ReferenceCode,
		Name:          input.Name,
		Description:   input.Description,
		Status:        domain.ControlStatusNotImplemented,
	}

	if err := uc.repo.CreateControl(ctx, control); err != nil {
		return nil, err
	}
	return control, nil
}
