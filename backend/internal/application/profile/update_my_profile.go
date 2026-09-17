// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package profile

import (
	"context"
	"fmt"
	"sort"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// UpdateMyProfile edits the caller's own identity and preferences.
//
// The audit event names the fields that changed and carries no values: a
// phone number or a bio is personal data the governance trail does not need.
func (s *Service) UpdateMyProfile(ctx context.Context, tenantID, userID uuid.UUID, patch domain.UserProfilePatch) (*View, error) {
	if err := patch.Normalize(); err != nil {
		return nil, err
	}
	u, err := s.loadSelf(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("profile.UpdateMyProfile: %w", err)
	}

	current := map[string]string{
		"full_name": u.FullName, "department": u.Department, "phone": u.Phone, "bio": u.Bio,
		"timezone": u.Timezone, "locale": u.Locale, "date_format": u.DateFormat, "theme_mode": u.ThemeMode,
	}
	columns := map[string]interface{}{}
	var changed []string
	for col, next := range patch.Columns() {
		if current[col] == next {
			continue
		}
		columns[col] = next
		changed = append(changed, col)
	}
	if len(columns) == 0 {
		return s.view(ctx, tenantID, u), nil
	}
	if err := s.users.UpdateUserColumns(ctx, u.ID, columns); err != nil {
		return nil, fmt.Errorf("profile.UpdateMyProfile: %w", err)
	}
	sort.Strings(changed)
	s.record(ctx, tenantID, u.ID, "user.profile_updated", domain.JSONMap{"fields": changed})

	fresh, err := s.loadSelf(ctx, u.ID)
	if err != nil {
		return nil, fmt.Errorf("profile.UpdateMyProfile: %w", err)
	}
	return s.view(ctx, tenantID, fresh), nil
}
