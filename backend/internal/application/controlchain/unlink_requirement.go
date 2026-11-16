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

// UnlinkUseCase removes the link of a control to a requirement.
type UnlinkUseCase struct {
	repo Repository
	now  Clock
}

// NewUnlinkUseCase builds the use case.
func NewUnlinkUseCase(repo Repository, now Clock) *UnlinkUseCase {
	return &UnlinkUseCase{repo: repo, now: now}
}

// Execute deletes the edge. The one link a split control has to its own source
// requirement is refused with ErrConflict: the boot reconciler owns that pair and
// would insert it again, so the unlink would silently revert. Every other link is
// freely removed. An absent control or edge is ErrNotFound.
func (u *UnlinkUseCase) Execute(ctx context.Context, c Caller, controlID, requirementID uuid.UUID) error {
	if err := c.authorize(PermissionUpdate); err != nil {
		return err
	}
	if controlID == canonical.ControlIDFor(requirementID) {
		return domain.InternalControlError(domain.ErrConflict, domain.IntCtlSplitLink)
	}
	now := u.now().UTC()
	before := domain.JSONMap{"control_id": controlID.String(), "requirement_id": requirementID.String()}
	ev := auditEvent(c, domain.AuditActionUpdate, "control unlinked from a requirement", before, nil,
		domain.StringList{"requirements"}, controlID, now)
	if err := u.repo.UnlinkRequirement(ctx, c.TenantID, controlID, requirementID, ev); err != nil {
		return fmt.Errorf("application.UnlinkInternalControl: %w", err)
	}
	return nil
}
