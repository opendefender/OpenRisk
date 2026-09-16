// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
)

// GormRegistrationRepository writes a whole new account in ONE transaction:
// the organization, its owner, and the owner's root membership (#687).
//
// Registration used to write those three rows with three independent calls.
// A failure of the second deleted the first best-effort; a failure of the third
// was printed and ignored, which left a user who could sign in but belonged to
// no organization — an account with no permissions and no way to fix itself.
// ABSOLUTE RULE #7 asks for a transaction on every multi-table operation, and
// this is the clearest case of it in the codebase.
type GormRegistrationRepository struct {
	db *gorm.DB
}

func NewGormRegistrationRepository(db *gorm.DB) *GormRegistrationRepository {
	return &GormRegistrationRepository{db: db}
}

// CreateAccount inserts the organization, the user and the membership together.
//
// The caller assigns the user's id before calling, so the organization can carry
// its real owner from the first insert rather than a placeholder that a later
// UPDATE had to correct — an update whose failure used to leave the row claiming
// an owner that never existed.
//
// Either all three rows are committed, or none is.
func (r *GormRegistrationRepository) CreateAccount(
	ctx context.Context,
	org *domain.Organization,
	user *domain.User,
	member *domain.OrganizationMember,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(org).Error; err != nil {
			return fmt.Errorf("create organization: %w", err)
		}
		if err := tx.Create(user).Error; err != nil {
			return fmt.Errorf("create user: %w", err)
		}
		if err := tx.Create(member).Error; err != nil {
			return fmt.Errorf("create membership: %w", err)
		}
		return nil
	})
}
