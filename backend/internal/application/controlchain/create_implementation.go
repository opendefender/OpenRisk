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
	"github.com/opendefender/openrisk/internal/domain/canonical"
)

// CreateImplementationInput is the content of a new implementation. There is no
// field for the id, the tenant, the source, the confidence or the provenance:
// they are set here and a request that carries them is decoded into a DTO that
// drops them.
type CreateImplementationInput struct {
	Description  string
	Status       string // empty: planned
	InPlaceSince *string
	OwnerID      *uuid.UUID
	AssigneeID   *uuid.UUID
	ReviewerID   *uuid.UUID
	AppliesTo    []TargetRef
}

// CreateImplementationUseCase adds an implementation to a control of the tenant.
type CreateImplementationUseCase struct {
	repo   ImplementationRepository
	owners OwnerValidator
	now    Clock
}

// NewCreateImplementationUseCase builds the use case. owners must not be nil.
func NewCreateImplementationUseCase(repo ImplementationRepository, owners OwnerValidator, now Clock) *CreateImplementationUseCase {
	return &CreateImplementationUseCase{repo: repo, owners: owners, now: now}
}

// Execute builds the implementation with a server-generated id and the
// user_entered envelope, validates it, and stores it with its applies_to edges
// and its audit entry in one transaction. The control and every asset must
// resolve in the caller's tenant (ErrNotFound otherwise, never 403). No target is
// ErrValidation.
func (u *CreateImplementationUseCase) Execute(ctx context.Context, c Caller, controlID uuid.UUID, in CreateImplementationInput) (*ImplementationView, error) {
	if err := c.authorize(PermissionCreate); err != nil {
		return nil, err
	}
	if err := checkLongText("description", &in.Description); err != nil {
		return nil, err
	}
	targets, err := cleanTargets(in.AppliesTo)
	if err != nil {
		return nil, err
	}
	conf, err := userConfidence()
	if err != nil {
		return nil, validation(err)
	}
	now := u.now().UTC()
	id := uuid.New()
	var since *canonical.Date
	if s := blankToNil(in.InPlaceSince); s != nil {
		d := canonical.Date(*s)
		since = &d
	}
	impl := canonical.ControlImplementation{
		Envelope:     newEnvelope(c, id, conf, now),
		ControlID:    controlID,
		Description:  in.Description,
		Status:       canonical.ImplementationStatus(defaultTo(in.Status, string(canonical.ImplementationPlanned))),
		InPlaceSince: since,
		Ownership:    canonical.Ownership{OwnerID: in.OwnerID, AssigneeID: in.AssigneeID, ReviewerID: in.ReviewerID},
	}
	if err := validation(impl.Validate()); err != nil {
		return nil, err
	}
	if err := validateOwners(ctx, u.owners, c.TenantID, ownershipPatchOf(impl.Ownership)); err != nil {
		return nil, err
	}
	edges := make([]canonical.Edge, 0, len(targets))
	for _, assetID := range targets {
		e, err := appliesToEdge(c, id, assetID, conf, now)
		if err != nil {
			return nil, err
		}
		edges = append(edges, e)
	}

	after := implementationSnapshot(impl, targets)
	ev := auditEventOf(auditImplementationType, c, domain.AuditActionCreate, "control implementation created", nil, after,
		changedFields(nil, after), id, now)
	if err := u.repo.CreateImplementation(ctx, c.TenantID, impl, edges, ev); err != nil {
		return nil, fmt.Errorf("application.CreateControlImplementation: %w", err)
	}
	v := ImplementationViewOf(ImplementationRecord{Implementation: impl, AppliesTo: targets})
	return &v, nil
}
