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

// LinkUseCase links a control to a requirement it addresses (addressed_by).
type LinkUseCase struct {
	repo Repository
	now  Clock
}

// NewLinkUseCase builds the use case.
func NewLinkUseCase(repo Repository, now Clock) *LinkUseCase {
	return &LinkUseCase{repo: repo, now: now}
}

// Execute builds the edge with the user_entered envelope and inserts it after
// both ends resolved in the caller's tenant (the S2 resolver): either end absent,
// deleted or foreign is ErrNotFound. It reports whether the edge was new;
// linking twice is the same edge.
func (u *LinkUseCase) Execute(ctx context.Context, c Caller, controlID, requirementID uuid.UUID) (created bool, err error) {
	if err := c.authorize(PermissionUpdate); err != nil {
		return false, err
	}
	now := u.now().UTC()
	conf, err := userConfidence()
	if err != nil {
		return false, validation(err)
	}
	edgeID := uuid.New()
	edge := canonical.Edge{
		ID: edgeID, TenantID: c.TenantID,
		From:       canonical.RecordRef{Type: canonical.TypeRequirement, ID: requirementID},
		Verb:       canonical.VerbAddressedBy,
		To:         canonical.EdgeTarget{Type: canonical.TypeControl, ID: controlID, TenantID: c.TenantID},
		Source:     canonical.SourceOpenRiskUser,
		SourceID:   edgeID.String(),
		Confidence: conf,
		Provenance: userProvenance(c.UserID),
		CreatedAt:  now, UpdatedAt: now,
	}
	if err := validation(edge.Validate()); err != nil {
		return false, err
	}
	after := domain.JSONMap{"control_id": controlID.String(), "requirement_id": requirementID.String()}
	ev := auditEvent(c, domain.AuditActionUpdate, "control linked to a requirement", nil, after,
		domain.StringList{"requirements"}, controlID, now)
	created, err = u.repo.LinkRequirement(ctx, c.TenantID, edge, ev)
	if err != nil {
		return false, fmt.Errorf("application.LinkInternalControl: %w", err)
	}
	return created, nil
}
