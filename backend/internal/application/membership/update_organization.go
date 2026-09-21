// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package membership

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// OrganizationProfileWriter applies a profile edit to the caller's own
// organization. Satisfied by GormOrganizationRepository.
//
// columns are organizations columns; setKeys and clearKeys address keys of
// organizations.settings. The write must merge settings server-side so keys it
// does not name — the display currency among them — survive untouched.
type OrganizationProfileWriter interface {
	UpdateOrganizationSettingsProfile(ctx context.Context, orgID uuid.UUID, columns map[string]interface{}, setKeys map[string]string, clearKeys []string) error
}

// WithOrganizationWriter enables UpdateOrganization.
func (s *Service) WithOrganizationWriter(w OrganizationProfileWriter) *Service {
	s.orgWriter = w
	return s
}

// UpdateOrganization edits the caller's own organization profile (#299).
//
// The organization is the tenant: there is no id to choose, so one tenant can
// never address another. The permission check (organization:update) is made
// by the route guard and repeated here through canEdit, so the use case is not
// safe only because of where it happens to be mounted.
func (s *Service) UpdateOrganization(ctx context.Context, tenantID, actorID uuid.UUID, canEdit bool, patch domain.OrganizationProfilePatch) (*OrganizationView, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("no organization in context")
	}
	if !canEdit {
		return nil, domain.NewForbiddenError("organization:update is required")
	}
	if s.orgs == nil || s.orgWriter == nil {
		return nil, domain.NewInternalError("organization directory unavailable")
	}
	if err := patch.Normalize(); err != nil {
		return nil, err
	}

	org, err := s.orgs.GetByID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("membership.UpdateOrganization: %w", err)
	}
	if org == nil {
		return nil, domain.NewNotFoundError("organization", tenantID)
	}
	if patch.IsEmpty() {
		return s.GetOrganization(ctx, tenantID, canEdit)
	}

	settings := org.GetSettings()
	before, after := domain.JSONMap{}, domain.JSONMap{}
	columns := map[string]interface{}{}
	setKeys := map[string]string{}
	var clearKeys []string

	column := func(name string, current string, next *string) {
		if next == nil || *next == current {
			return
		}
		columns[name] = *next
		before[name], after[name] = current, *next
	}
	column("name", org.Name, patch.Name)
	column("industry", org.Industry, patch.Industry)
	column("size", string(org.Size), patch.Size)

	setting := func(key string, next *string) {
		if next == nil {
			return
		}
		current, _ := settings[key].(string)
		if *next == current {
			return
		}
		if *next == "" {
			clearKeys = append(clearKeys, key)
		} else {
			setKeys[key] = *next
		}
		before[key], after[key] = current, *next
	}
	setting(domain.OrgSettingWebsite, patch.Website)
	setting(domain.OrgSettingDescription, patch.Description)
	setting(domain.OrgSettingTimezone, patch.Timezone)
	setting(domain.OrgSettingDefaultLocale, patch.DefaultLocale)
	setting(domain.OrgSettingDateFormat, patch.DateFormat)
	setting(domain.OrgSettingAccent, patch.Accent)

	if len(after) == 0 {
		return s.GetOrganization(ctx, tenantID, canEdit)
	}
	if err := s.orgWriter.UpdateOrganizationSettingsProfile(ctx, tenantID, columns, setKeys, clearKeys); err != nil {
		return nil, fmt.Errorf("membership.UpdateOrganization: %w", err)
	}

	s.record(ctx, tenantID, actorID, domain.AuditActionUpdate, "organization", tenantID.String(),
		"organization.updated", before, after)
	return s.GetOrganization(ctx, tenantID, canEdit)
}
