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

// CategoryStore is the narrow port that proves a category belongs to the tenant.
// Satisfied structurally by repository.GormRiskCategoryRepository, whose GetByID
// filters on id AND tenant_id and answers nil for a row that is not the tenant's.
type CategoryStore interface {
	GetByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.RiskCategory, error)
}

// ownsCategory refuses a category that is not the tenant's (#1319). Before it,
// a risk could be pointed at another organisation's category by id, and reading
// the risk back returned that category's name and tenant. A foreign id and an
// unknown id answer the same not-found, so the answer does not confirm that the
// category exists elsewhere. A nil id (unclassified) always passes.
func ownsCategory(ctx context.Context, store CategoryStore, tenantID uuid.UUID, id *uuid.UUID) error {
	if id == nil || *id == uuid.Nil || store == nil {
		return nil
	}
	c, err := store.GetByID(ctx, *id, tenantID)
	if err != nil {
		return err
	}
	if c == nil {
		return domain.NewNotFoundError("category", id.String())
	}
	return nil
}
