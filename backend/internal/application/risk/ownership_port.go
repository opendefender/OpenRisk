// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package risk

import (
	"context"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// OwnershipManager validates an ownership patch against the tenant's members,
// applies it, and tells the newly assigned people about it.
//
// Narrow port, satisfied structurally by application/ownership.Service — this
// package does not import it. Optional and nil-safe everywhere: with no manager
// wired the patch is still applied (see applyOwnership), it is simply neither
// membership-checked nor announced.
type OwnershipManager interface {
	Apply(ctx context.Context, tenantID uuid.UUID, block *domain.Ownership, patch domain.OwnershipPatch, actor uuid.UUID) ([]domain.OwnershipChange, error)
	Notify(ctx context.Context, tenantID uuid.UUID, changes []domain.OwnershipChange, subject domain.OwnershipSubject)
}

// applyOwnership routes a patch through the manager when there is one, and
// applies it directly otherwise. Returns the changes worth notifying about.
func applyOwnership(ctx context.Context, mgr OwnershipManager, tenantID uuid.UUID, block *domain.Ownership, patch domain.OwnershipPatch, actor uuid.UUID) ([]domain.OwnershipChange, error) {
	if block == nil || patch.IsEmpty() {
		return nil, nil
	}
	if mgr != nil {
		return mgr.Apply(ctx, tenantID, block, patch, actor)
	}
	before := *block
	patch.Apply(block)
	return domain.DiffOwnership(before, *block, actor), nil
}
