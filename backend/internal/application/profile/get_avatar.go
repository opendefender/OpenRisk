// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package profile

import (
	"context"
	"fmt"
	"io"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/imageupload"
)

// Avatar is a picture ready to serve.
type Avatar struct {
	ContentType string
	Data        []byte
}

// GetAvatar returns a user's avatar to a caller in the same organization.
//
// users carries no tenant_id, so the gate is the parent: the target must hold
// an access-granting membership in the caller's tenant (the caller's own
// avatar always qualifies). A user of another tenant, a user without an
// avatar and a user that does not exist all answer the same 404.
func (s *Service) GetAvatar(ctx context.Context, tenantID, callerID, targetID uuid.UUID) (*Avatar, error) {
	if tenantID == uuid.Nil || callerID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("no session")
	}
	notFound := domain.NewNotFoundError("avatar", targetID)
	if s.blobs == nil {
		return nil, notFound
	}
	if targetID != callerID {
		m, err := s.members.GetOrganizationMember(ctx, targetID, tenantID)
		if err != nil {
			return nil, fmt.Errorf("profile.GetAvatar: %w", err)
		}
		if m == nil || !m.EffectiveStatus().GrantsAccess() {
			return nil, notFound
		}
	}
	u, err := s.users.GetByID(ctx, targetID)
	if err != nil {
		return nil, fmt.Errorf("profile.GetAvatar: %w", err)
	}
	if u == nil || u.AvatarKey == "" {
		return nil, notFound
	}
	rc, err := s.blobs.Open(ctx, u.AvatarKey)
	if err != nil {
		return nil, notFound
	}
	defer rc.Close()
	data, err := io.ReadAll(io.LimitReader(rc, imageupload.MaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("profile.GetAvatar: %w", err)
	}
	ct, ok := imageupload.Sniff(data)
	if !ok {
		return nil, notFound
	}
	return &Avatar{ContentType: ct, Data: data}, nil
}
