// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package membership

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/imageupload"
)

// LogoPath is the API path serving the caller's organization logo. There is no
// id in it: a member only ever reads their own organization's logo.
const LogoPath = "/api/v1/organization/logo"

// BlobStore persists logo bytes. Satisfied by pkg/storage.Storage.
type BlobStore interface {
	Save(ctx context.Context, tenantID uuid.UUID, filename string, content io.Reader) (string, error)
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}

// WithBlobStore enables the organization logo.
func (s *Service) WithBlobStore(b BlobStore) *Service { s.blobs = b; return s }

// Logo is a picture ready to serve.
type Logo struct {
	ContentType string
	Data        []byte
}

func (s *Service) editableOrg(ctx context.Context, tenantID uuid.UUID, canEdit bool) (*domain.Organization, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("no organization in context")
	}
	if !canEdit {
		return nil, domain.NewForbiddenError("organization:update is required")
	}
	if s.orgs == nil || s.orgWriter == nil || s.blobs == nil {
		return nil, domain.NewInternalError("organization branding unavailable")
	}
	org, err := s.orgs.GetByID(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if org == nil {
		return nil, domain.NewNotFoundError("organization", tenantID)
	}
	return org, nil
}

// UploadOrganizationLogo stores a new logo for the caller's organization and
// deletes the previous one (#718). PNG, JPEG or WebP up to 1 MB, by its bytes.
func (s *Service) UploadOrganizationLogo(ctx context.Context, tenantID, actorID uuid.UUID, canEdit bool, content io.Reader) (*OrganizationView, error) {
	org, err := s.editableOrg(ctx, tenantID, canEdit)
	if err != nil {
		return nil, fmt.Errorf("membership.UploadOrganizationLogo: %w", err)
	}
	img, err := imageupload.Read(content)
	if err != nil {
		if errors.Is(err, imageupload.ErrTooLarge) || errors.Is(err, imageupload.ErrUnsupported) || errors.Is(err, imageupload.ErrEmpty) {
			return nil, domain.NewValidationError("logo: " + err.Error())
		}
		return nil, fmt.Errorf("membership.UploadOrganizationLogo: %w", err)
	}
	previous := org.LogoKey // read before the write, which may update org in place
	key, err := s.blobs.Save(ctx, tenantID, "logo"+img.Extension, img.Reader())
	if err != nil {
		return nil, fmt.Errorf("membership.UploadOrganizationLogo: %w", err)
	}
	if err := s.orgWriter.UpdateOrganizationSettingsProfile(ctx, tenantID,
		map[string]interface{}{"logo_key": key, "logo_url": LogoPath}, nil, nil); err != nil {
		_ = s.blobs.Delete(ctx, key)
		return nil, fmt.Errorf("membership.UploadOrganizationLogo: %w", err)
	}
	if previous != "" && previous != key {
		_ = s.blobs.Delete(ctx, previous) // best-effort: an orphan file is not a failed upload
	}
	action := "organization.logo_uploaded"
	if previous != "" {
		action = "organization.logo_replaced"
	}
	s.record(ctx, tenantID, actorID, domain.AuditActionUpdate, "organization", tenantID.String(), action,
		nil, domain.JSONMap{"content_type": img.ContentType, "bytes": len(img.Data)})
	return s.GetOrganization(ctx, tenantID, canEdit)
}

// DeleteOrganizationLogo removes the logo. Removing none is not an error.
func (s *Service) DeleteOrganizationLogo(ctx context.Context, tenantID, actorID uuid.UUID, canEdit bool) (*OrganizationView, error) {
	org, err := s.editableOrg(ctx, tenantID, canEdit)
	if err != nil {
		return nil, fmt.Errorf("membership.DeleteOrganizationLogo: %w", err)
	}
	if org.LogoKey == "" {
		return s.GetOrganization(ctx, tenantID, canEdit)
	}
	previous := org.LogoKey
	if err := s.orgWriter.UpdateOrganizationSettingsProfile(ctx, tenantID,
		map[string]interface{}{"logo_key": "", "logo_url": ""}, nil, nil); err != nil {
		return nil, fmt.Errorf("membership.DeleteOrganizationLogo: %w", err)
	}
	_ = s.blobs.Delete(ctx, previous)
	s.record(ctx, tenantID, actorID, domain.AuditActionDelete, "organization", tenantID.String(),
		"organization.logo_removed", nil, nil)
	return s.GetOrganization(ctx, tenantID, canEdit)
}

// GetOrganizationLogo returns the caller's own organization logo. There is no
// id to choose, so a member can only ever read their own tenant's logo.
func (s *Service) GetOrganizationLogo(ctx context.Context, tenantID uuid.UUID) (*Logo, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("no organization in context")
	}
	notFound := domain.NewNotFoundError("logo", tenantID)
	if s.orgs == nil || s.blobs == nil {
		return nil, notFound
	}
	org, err := s.orgs.GetByID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("membership.GetOrganizationLogo: %w", err)
	}
	if org == nil || org.LogoKey == "" {
		return nil, notFound
	}
	rc, err := s.blobs.Open(ctx, org.LogoKey)
	if err != nil {
		return nil, notFound
	}
	defer rc.Close()
	data, err := io.ReadAll(io.LimitReader(rc, imageupload.MaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("membership.GetOrganizationLogo: %w", err)
	}
	ct, ok := imageupload.Sniff(data)
	if !ok {
		return nil, notFound
	}
	return &Logo{ContentType: ct, Data: data}, nil
}

// Branding is what every member's interface needs to look like their
// organization: its name, whether it has a logo, and its accent (#718).
type Branding struct {
	Name    string `json:"name"`
	HasLogo bool   `json:"has_logo"`
	Accent  string `json:"accent,omitempty"`
}

// GetBranding is readable by any member — unlike the full profile, which needs
// organization:read — because every member's interface wears the branding.
func (s *Service) GetBranding(ctx context.Context, tenantID uuid.UUID) (*Branding, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("no organization in context")
	}
	if s.orgs == nil {
		return nil, domain.NewInternalError("organization directory unavailable")
	}
	org, err := s.orgs.GetByID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("membership.GetBranding: %w", err)
	}
	if org == nil {
		return nil, domain.NewNotFoundError("organization", tenantID)
	}
	accent, _ := org.GetSettings()[domain.OrgSettingAccent].(string)
	if !domain.IsAccentPreset(accent) {
		accent = ""
	}
	return &Branding{Name: org.Name, HasLogo: org.LogoKey != "", Accent: accent}, nil
}
