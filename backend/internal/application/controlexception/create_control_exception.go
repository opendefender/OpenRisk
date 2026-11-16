// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package controlexception

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// CreateInput is the request body of a creation, plus the control from the route.
type CreateInput struct {
	ControlID uuid.UUID
	OwnerID   uuid.UUID
	Reason    string
	StartsAt  *time.Time
	ExpiresAt *time.Time
}

// CreateUseCase records an exception on a control.
type CreateUseCase struct {
	repo   Repository
	owners OwnerValidator
	now    Clock
}

// NewCreateUseCase builds the use case. owners must not be nil: an owner that
// was never checked is a silent store of another tenant's user.
func NewCreateUseCase(repo Repository, owners OwnerValidator, now Clock) *CreateUseCase {
	return &CreateUseCase{repo: repo, owners: owners, now: now}
}

// Execute validates the input at the caller's instant, gates the control and the
// owner through the caller's tenant, and stores the exception with its audit
// entry in one transaction.
func (u *CreateUseCase) Execute(ctx context.Context, c Caller, in CreateInput) (*View, error) {
	if err := c.authorize(PermissionUpdate); err != nil {
		return nil, err
	}
	now := u.now()
	ex, err := domain.NewControlException(domain.NewControlExceptionInput{
		TenantID: c.TenantID, ControlID: in.ControlID, OwnerID: in.OwnerID, Reason: in.Reason,
		ApprovedBy: c.UserID, StartsAt: in.StartsAt, ExpiresAt: in.ExpiresAt,
	}, now)
	if err != nil {
		return nil, err
	}

	ok, err := u.repo.ControlExists(ctx, c.TenantID, in.ControlID)
	if err != nil {
		return nil, fmt.Errorf("application.CreateControlException: %w", err)
	}
	if !ok {
		return nil, domain.ControlExceptionError(domain.ErrNotFound, domain.CtlExcControlNotFound)
	}

	if u.owners == nil {
		return nil, domain.ControlExceptionError(domain.ErrInternal, domain.CtlExcUnavailable)
	}
	if err := u.owners.Validate(ctx, c.TenantID, domain.OwnershipPatch{Owner: domain.Assign(in.OwnerID)}); err != nil {
		if isValidation(err) {
			// Another tenant's user and a deactivated one read the same: the
			// message names no user.
			return nil, domain.ControlExceptionError(domain.ErrValidation, domain.CtlExcOwnerInvalid)
		}
		return nil, fmt.Errorf("application.CreateControlException: %w", err)
	}

	after := snapshot(*ex, now)
	ev := auditEvent(c, domain.AuditActionCreate, "control exception created", nil, after,
		domain.StringList{"owner_id", "reason", "starts_at", "expires_at"}, ex.ID, now)
	if err := u.repo.Create(ctx, ex, ev); err != nil {
		return nil, fmt.Errorf("application.CreateControlException: %w", err)
	}
	v := ViewOf(*ex, now)
	return &v, nil
}
