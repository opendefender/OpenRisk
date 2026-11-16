// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package controlchain

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

func invalidPaging() error {
	return domain.InternalControlError(domain.ErrValidation, domain.IntCtlInvalidPaging)
}

// ListRequirementsUseCase lists the requirements a control addresses.
type ListRequirementsUseCase struct{ repo Repository }

// NewListRequirementsUseCase builds the use case.
func NewListRequirementsUseCase(repo Repository) *ListRequirementsUseCase {
	return &ListRequirementsUseCase{repo: repo}
}

// Execute returns the live requirements linked to the control. An absent or
// foreign control is ErrNotFound; a requirement deleted since the link is not listed.
func (u *ListRequirementsUseCase) Execute(ctx context.Context, c Caller, controlID uuid.UUID) ([]domain.InternalControlRequirement, error) {
	if err := c.authorize(PermissionRead); err != nil {
		return nil, err
	}
	rs, err := u.repo.ListRequirementsOfControl(ctx, c.TenantID, controlID)
	if err != nil {
		return nil, fmt.Errorf("application.ListControlRequirements: %w", err)
	}
	if rs == nil {
		rs = []domain.InternalControlRequirement{}
	}
	return rs, nil
}

// ListForRequirementUseCase lists the controls that address a requirement.
type ListForRequirementUseCase struct{ repo Repository }

// NewListForRequirementUseCase builds the use case.
func NewListForRequirementUseCase(repo Repository) *ListForRequirementUseCase {
	return &ListForRequirementUseCase{repo: repo}
}

// Execute returns the live controls linked to the requirement, the split control
// next to the ones a user wrote. requirementID is the legacy compliance_controls
// id. An absent or foreign requirement is ErrNotFound.
func (u *ListForRequirementUseCase) Execute(ctx context.Context, c Caller, requirementID uuid.UUID) ([]ControlView, error) {
	if err := c.authorize(PermissionRead); err != nil {
		return nil, err
	}
	cs, err := u.repo.ListControlsOfRequirement(ctx, c.TenantID, requirementID)
	if err != nil {
		return nil, fmt.Errorf("application.ListRequirementControls: %w", err)
	}
	return viewsOf(cs), nil
}
