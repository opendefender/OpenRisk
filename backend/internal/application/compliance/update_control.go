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

// UpdateControlInput is a partial update — nil fields are left unchanged,
// mirroring internal/application/risk's UpdateRiskInput convention.
type UpdateControlInput struct {
	ReferenceCode *string
	Name          *string
	Description   *string
	Status        *domain.ControlStatus
}

var validControlStatuses = map[domain.ControlStatus]bool{
	domain.ControlStatusNotImplemented: true,
	domain.ControlStatusInProgress:     true,
	domain.ControlStatusImplemented:    true,
	domain.ControlStatusNotApplicable:  true,
}

// UpdateControlUseCase handles updating a control's fields, most notably
// its implementation status — this is the step of the compliance
// lifecycle a tenant walks through as they work a framework.
type UpdateControlUseCase struct {
	repo domain.ComplianceRepository
}

func NewUpdateControlUseCase(repo domain.ComplianceRepository) *UpdateControlUseCase {
	return &UpdateControlUseCase{repo: repo}
}

func (uc *UpdateControlUseCase) Execute(ctx context.Context, tenantID, controlID uuid.UUID, input UpdateControlInput) (*domain.ComplianceControl, error) {
	control, err := uc.repo.GetControlByID(ctx, controlID, tenantID)
	if err != nil {
		return nil, err
	}
	if control == nil {
		return nil, domain.NewNotFoundError("control", controlID)
	}

	if input.Status != nil {
		if !validControlStatuses[*input.Status] {
			return nil, domain.NewValidationError("invalid status: " + string(*input.Status))
		}
		control.Status = *input.Status
	}
	if input.ReferenceCode != nil {
		control.ReferenceCode = *input.ReferenceCode
	}
	if input.Name != nil {
		if *input.Name == "" {
			return nil, domain.NewValidationError("name cannot be empty")
		}
		control.Name = *input.Name
	}
	if input.Description != nil {
		control.Description = *input.Description
	}

	control.TenantID = tenantID // defense in depth, never let a caller move a control to another tenant
	if err := uc.repo.UpdateControl(ctx, control); err != nil {
		return nil, err
	}
	return control, nil
}
