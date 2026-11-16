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

// DeleteUseCase soft-deletes a control the tenant created.
type DeleteUseCase struct {
	repo Repository
	now  Clock
}

// NewDeleteUseCase builds the use case.
func NewDeleteUseCase(repo Repository, now Clock) *DeleteUseCase {
	return &DeleteUseCase{repo: repo, now: now}
}

// IsSplitControl tells a control the legacy split made (actor {system,
// control_split}). Its lifecycle follows its legacy requirement (D-108 Q8), so
// this API does not delete it.
func IsSplitControl(c canonical.Control) bool {
	return c.Provenance.Actor.Kind == canonical.ActorSystem && c.Provenance.Actor.ID == SplitActorID
}

// SplitActorID is provenance.actor.id of everything the D7 split writes. It is
// the value infrastructure/controlchain stamps; split_test there pins it.
const SplitActorID = "control_split"

// Execute soft-deletes the control (deleted_at, tenant-filtered). Its edges are
// kept and every reader filters on the liveness of both ends (A10.3). A split
// control is ErrConflict; an absent, deleted or foreign id is ErrNotFound.
func (u *DeleteUseCase) Execute(ctx context.Context, c Caller, id uuid.UUID) error {
	if err := c.authorize(PermissionDelete); err != nil {
		return err
	}
	now := u.now().UTC()
	err := u.repo.DeleteControl(ctx, c.TenantID, id, func(cur canonical.Control) (*domain.AuditEvent, error) {
		if IsSplitControl(cur) {
			return nil, domain.InternalControlError(domain.ErrConflict, domain.IntCtlSplitControl)
		}
		before := snapshot(cur)
		return auditEvent(c, domain.AuditActionDelete, "control deleted", before, nil, changedFields(before, nil), id, now), nil
	})
	if err != nil {
		return fmt.Errorf("application.DeleteInternalControl: %w", err)
	}
	return nil
}
