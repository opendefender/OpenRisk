// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/tenantguard"
)

// GormOAuthLinkRepository stores which providers sign a user in.
//
// No tenant predicate, and that is correct: a link is resolved during the OAuth
// callback, before any session exists and therefore before any tenant is known.
// The scoping key is (provider, provider_user_id) — the provider's own stable
// identity for the person — and the link then determines which user, and hence
// which tenant, the session belongs to.
type GormOAuthLinkRepository struct {
	db *gorm.DB
}

// NewGormOAuthLinkRepository builds the repository.
func NewGormOAuthLinkRepository(db *gorm.DB) *GormOAuthLinkRepository {
	return &GormOAuthLinkRepository{db: db}
}

// FindByProviderSubject looks up a link by the provider's stable user ID.
//
// Returns (nil, nil) when absent: a first-time sign-in is the normal path here,
// not an error.
func (r *GormOAuthLinkRepository) FindByProviderSubject(ctx context.Context, provider, subject string) (*domain.OAuthProvider, error) {
	if provider == "" || subject == "" {
		return nil, nil
	}
	var link domain.OAuthProvider
	err := tenantguard.CrossTenant(r.db.WithContext(ctx), "OAuth sign-in: the provider subject resolves the link's tenant; no session yet").
		Where("provider = ? AND provider_user_id = ?", provider, subject).
		First(&link).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &link, nil
}

// ListByUser returns every provider linked to a user.
func (r *GormOAuthLinkRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]domain.OAuthProvider, error) {
	var links []domain.OAuthProvider
	if err := tenantguard.CrossTenant(r.db.WithContext(ctx), "a user's own provider links span every organisation they belong to").Where("user_id = ?", userID).Find(&links).Error; err != nil {
		return nil, err
	}
	return links, nil
}

// Create links a provider to a user.
func (r *GormOAuthLinkRepository) Create(ctx context.Context, link *domain.OAuthProvider) error {
	return r.db.WithContext(ctx).Create(link).Error
}

// TouchLogin records a successful sign-in through a link.
//
// Best-effort by contract: the caller ignores the error, because failing to
// update a timestamp must not fail an otherwise valid sign-in.
func (r *GormOAuthLinkRepository) TouchLogin(ctx context.Context, tenantID, id uuid.UUID, at time.Time) error {
	return r.db.WithContext(ctx).
		Model(&domain.OAuthProvider{}).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Update("last_login_at", at).Error
}
