// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package profile

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// GetMyProfile returns the caller's own profile with effective preferences.
func (s *Service) GetMyProfile(ctx context.Context, tenantID, userID uuid.UUID) (*View, error) {
	u, err := s.loadSelf(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("profile.GetMyProfile: %w", err)
	}
	return s.view(ctx, tenantID, u), nil
}
