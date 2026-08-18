// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/crq"
)

// Narrow, targeted writes for the onboarding wizard's organization and profile
// steps.
//
// These are CONCRETE methods, deliberately kept off domain.UserRepository /
// auth.OrganizationRepository — the same convention as ListRisksForFinancial and
// CountRisksByCriticality. Widening a shared port for one caller forces every
// mock in the codebase to grow a method it will never use.
//
// Both are partial updates: an empty field means "the user did not answer that
// question", and must not blank out a value they already had.

// UpdateOrganizationProfile applies the wizard's organization step.
func (r *GormOrganizationRepository) UpdateOrganizationProfile(ctx context.Context, orgID uuid.UUID, name, industry, size string) error {
	if orgID == uuid.Nil {
		return nil
	}

	updates := map[string]interface{}{}
	if v := strings.TrimSpace(name); v != "" {
		updates["name"] = v
	}
	if v := strings.TrimSpace(industry); v != "" {
		updates["industry"] = v
	}
	if v := strings.TrimSpace(size); v != "" {
		updates["size"] = v
	}
	if len(updates) == 0 {
		return nil
	}

	return r.db.WithContext(ctx).
		Model(&domain.Organization{}).
		Where("id = ?", orgID).
		Updates(updates).Error
}

// SetOrganizationCurrency stores the tenant's display currency inside the
// organization's settings jsonb. The value is validated against the supported
// set; an unsupported/empty code is ignored (leaves the current setting intact),
// so a blank onboarding answer can't blank out a real choice. Load-merge-save is
// safe here (onboarding / settings screen are low-concurrency).
func (r *GormOrganizationRepository) SetOrganizationCurrency(ctx context.Context, orgID uuid.UUID, currency string) error {
	if orgID == uuid.Nil {
		return nil
	}
	code := strings.ToUpper(strings.TrimSpace(currency))
	if !crq.IsSupportedCurrency(code) {
		return nil
	}
	var org domain.Organization
	if err := r.db.WithContext(ctx).Where("id = ?", orgID).First(&org).Error; err != nil {
		return err
	}
	settings := org.GetSettings()
	settings["currency"] = code
	if err := org.SetSettings(settings); err != nil {
		return err
	}
	return r.db.WithContext(ctx).
		Model(&domain.Organization{}).
		Where("id = ?", orgID).
		Update("settings", org.Settings).Error
}

// OrgCurrency reads the tenant's display currency from settings, falling back to
// XAF (the base) when unset or unknown. Concrete method (off any shared port) so
// mocks stay valid — same convention as ListRisksForFinancial.
func (r *GormOrganizationRepository) OrgCurrency(ctx context.Context, orgID uuid.UUID) (string, error) {
	if orgID == uuid.Nil {
		return string(crq.CurrencyXAF), nil
	}
	var org domain.Organization
	if err := r.db.WithContext(ctx).Where("id = ?", orgID).First(&org).Error; err != nil {
		return string(crq.CurrencyXAF), err
	}
	if v, ok := org.GetSettings()["currency"].(string); ok && crq.IsSupportedCurrency(v) {
		return strings.ToUpper(strings.TrimSpace(v)), nil
	}
	return string(crq.CurrencyXAF), nil
}

// UpdateUserProfile applies the wizard's profile step.
//
// The write is scoped to the acting user's own id — the wizard can only ever
// edit the profile of the person walking it.
//
// Language is NOT written here: domain.User has no language column, and the
// display language is already a client preference. It stays in the wizard's
// stored answers, where the client reads it back, rather than being silently
// dropped into a column that does not exist.
func (r *GormUserRepository) UpdateUserProfile(ctx context.Context, userID uuid.UUID, fullName, jobTitle, avatarURL string) error {
	if userID == uuid.Nil {
		return nil
	}

	updates := map[string]interface{}{}
	if v := strings.TrimSpace(fullName); v != "" {
		updates["full_name"] = v
	}
	if v := strings.TrimSpace(jobTitle); v != "" {
		// The User model calls this "department" — it is the free-text org
		// affiliation field, which is what the wizard's "fonction" answers.
		updates["department"] = v
	}
	if v := strings.TrimSpace(avatarURL); v != "" {
		updates["avatar_url"] = v
	}
	if len(updates) == 0 {
		return nil
	}

	return r.db.WithContext(ctx).
		Model(&domain.User{}).
		Where("id = ?", userID).
		Updates(updates).Error
}
