// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package profile

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/imageupload"
)

// UploadMyAvatar stores a new avatar for the caller and deletes the previous
// one. The picture is validated by its bytes (PNG, JPEG, WebP, ≤ 1 MB).
func (s *Service) UploadMyAvatar(ctx context.Context, tenantID, userID uuid.UUID, content io.Reader) (*View, error) {
	if s.blobs == nil {
		return nil, domain.NewInternalError("file storage unavailable")
	}
	if tenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("no organization in context")
	}
	u, err := s.loadSelf(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("profile.UploadMyAvatar: %w", err)
	}
	img, err := imageupload.Read(content)
	if err != nil {
		if errors.Is(err, imageupload.ErrTooLarge) || errors.Is(err, imageupload.ErrUnsupported) || errors.Is(err, imageupload.ErrEmpty) {
			return nil, domain.NewValidationError("avatar: " + err.Error())
		}
		return nil, fmt.Errorf("profile.UploadMyAvatar: %w", err)
	}
	key, err := s.blobs.Save(ctx, tenantID, "avatar"+img.Extension, img.Reader())
	if err != nil {
		return nil, fmt.Errorf("profile.UploadMyAvatar: %w", err)
	}
	if err := s.users.UpdateUserColumns(ctx, u.ID, map[string]interface{}{
		"avatar_key": key, "avatar_url": AvatarPath(u.ID),
	}); err != nil {
		_ = s.blobs.Delete(ctx, key)
		return nil, fmt.Errorf("profile.UploadMyAvatar: %w", err)
	}
	if u.AvatarKey != "" && u.AvatarKey != key {
		_ = s.blobs.Delete(ctx, u.AvatarKey) // best-effort: an orphan file is not a failed upload
	}
	s.record(ctx, tenantID, u.ID, "user.avatar_updated", nil)

	u.AvatarKey, u.AvatarURL = key, AvatarPath(u.ID)
	return s.view(ctx, tenantID, u), nil
}

// DeleteMyAvatar removes the caller's avatar. Removing none is not an error.
func (s *Service) DeleteMyAvatar(ctx context.Context, tenantID, userID uuid.UUID) (*View, error) {
	u, err := s.loadSelf(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("profile.DeleteMyAvatar: %w", err)
	}
	if u.AvatarKey == "" {
		return s.view(ctx, tenantID, u), nil
	}
	if err := s.users.UpdateUserColumns(ctx, u.ID, map[string]interface{}{"avatar_key": "", "avatar_url": ""}); err != nil {
		return nil, fmt.Errorf("profile.DeleteMyAvatar: %w", err)
	}
	if s.blobs != nil {
		_ = s.blobs.Delete(ctx, u.AvatarKey)
	}
	s.record(ctx, tenantID, u.ID, "user.avatar_removed", nil)
	u.AvatarKey, u.AvatarURL = "", ""
	return s.view(ctx, tenantID, u), nil
}
