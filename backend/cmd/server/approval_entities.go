// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package main

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// approvalEntityOwnership proves that the entity an approval request is about
// belongs to the caller's tenant (#1520). Both lookups are the tenant-scoped
// repository finders, which answer (nil, nil) for an absent or foreign id.
type approvalEntityOwnership struct {
	risks    domain.RiskRepository
	controls domain.ComplianceRepository
}

func (o approvalEntityOwnership) RiskOwned(ctx context.Context, tenantID, id uuid.UUID) (bool, error) {
	r, err := o.risks.GetByID(ctx, id, tenantID)
	if err != nil {
		return false, err
	}
	return r != nil, nil
}

func (o approvalEntityOwnership) ControlOwned(ctx context.Context, tenantID, id uuid.UUID) (bool, error) {
	c, err := o.controls.GetControlByID(ctx, id, tenantID, time.Now())
	if err != nil {
		return false, err
	}
	return c != nil, nil
}
