// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package asset

import (
	"context"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// validateAttributes runs a raw attribute bag through the tenant's schema and
// returns the coerced values plus the schema that governed them.
//
// The validator is an optional dependency, and this is where "optional" is given
// its exact meaning: a MISSING validator accepts an EMPTY bag (an asset with no
// typed attributes is a legitimate asset) and refuses a NON-EMPTY one. It never
// stores unvalidated values. Everywhere else in the codebase an absent optional
// port degrades its slice of the result; here degrading would mean writing
// attributes nobody checked, which is the failure this feature removes.
func validateAttributes(
	ctx context.Context,
	v AttributeValidator,
	tenantID uuid.UUID,
	cat domain.AssetCategory,
	in map[string]any,
) (domain.AssetAttributes, []domain.AttributeDef, error) {
	if v == nil {
		if len(in) == 0 {
			return nil, nil, nil
		}
		return nil, nil, domain.NewInternalError("asset attribute schemas are not available on this server")
	}
	return v.ValidateFor(ctx, tenantID, cat, in)
}

func (uc *CreateAssetUseCase) validateAttributes(ctx context.Context, tenantID uuid.UUID, cat domain.AssetCategory, in map[string]any) (domain.AssetAttributes, []domain.AttributeDef, error) {
	return validateAttributes(ctx, uc.attrs, tenantID, cat, in)
}

func (uc *UpdateAssetUseCase) validateAttributes(ctx context.Context, tenantID uuid.UUID, cat domain.AssetCategory, in map[string]any) (domain.AssetAttributes, []domain.AttributeDef, error) {
	return validateAttributes(ctx, uc.attrs, tenantID, cat, in)
}
