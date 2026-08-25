// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"context"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// ---------------------------------------------------------------------------
// OR26-03 — the administrator-facing half: "force MFA after N days".
// ---------------------------------------------------------------------------

// MFAPolicyStore reads and writes one tenant's policy.
type MFAPolicyStore interface {
	GetMFAPolicy(ctx context.Context, tenantID uuid.UUID) (*domain.MFAPolicy, error)
	SaveMFAPolicy(ctx context.Context, p *domain.MFAPolicy) error
}

// MFAPolicyCacheInvalidator drops cached decisions for a tenant.
//
// Optional. Without it a shortened window would not bite for up to a minute,
// and an administrator who set "0 days" would watch their colleagues keep
// working and conclude the setting is decorative.
type MFAPolicyCacheInvalidator interface {
	InvalidateTenant(tenantID uuid.UUID)
}

// MFAPolicyView is what the settings screen renders.
type MFAPolicyView struct {
	GraceDays int `json:"grace_days"`
	// Configured distinguishes a saved row from the shipped default, so the
	// screen can say "using the default" rather than implying somebody chose it.
	Configured  bool       `json:"configured"`
	UpdatedByID *uuid.UUID `json:"updated_by_id,omitempty"`
	// MinDays/MaxDays/DefaultDays are shipped so the form validates against the
	// server's bounds rather than a copy of them that can drift.
	MinDays     int `json:"min_days"`
	MaxDays     int `json:"max_days"`
	DefaultDays int `json:"default_days"`
	// PrivilegedOrgRoles / PrivilegedBusinessRoles name who the window applies
	// to, so the screen can state the consequence instead of leaving the admin
	// to guess whether it touches everyone.
	PrivilegedOrgRoles      []string `json:"privileged_org_roles"`
	PrivilegedBusinessRoles []string `json:"privileged_business_roles"`
}

// GetMFAPolicyUseCase reads the tenant's policy.
type GetMFAPolicyUseCase struct {
	store                   MFAPolicyStore
	privilegedOrgRoles      []string
	privilegedBusinessRoles []string
}

// NewGetMFAPolicyUseCase builds the reader.
func NewGetMFAPolicyUseCase(store MFAPolicyStore, orgRoles, businessRoles []string) *GetMFAPolicyUseCase {
	return &GetMFAPolicyUseCase{store: store, privilegedOrgRoles: orgRoles, privilegedBusinessRoles: businessRoles}
}

// Execute returns the tenant's policy, falling back to the shipped default.
func (uc *GetMFAPolicyUseCase) Execute(ctx context.Context, tenantID uuid.UUID) (*MFAPolicyView, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("no organization in context")
	}
	row, err := uc.store.GetMFAPolicy(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	view := &MFAPolicyView{
		GraceDays:               domain.MFAGraceDaysDefault,
		MinDays:                 domain.MFAGraceDaysMin,
		MaxDays:                 domain.MFAGraceDaysMax,
		DefaultDays:             domain.MFAGraceDaysDefault,
		PrivilegedOrgRoles:      uc.privilegedOrgRoles,
		PrivilegedBusinessRoles: uc.privilegedBusinessRoles,
	}
	if row != nil {
		view.GraceDays = row.EffectiveGraceDays()
		view.Configured = true
		view.UpdatedByID = row.UpdatedByID
	}
	return view, nil
}

// UpdateMFAPolicyInput is an administrator changing the window.
type UpdateMFAPolicyInput struct {
	TenantID  uuid.UUID
	ActorID   uuid.UUID
	GraceDays int
}

// UpdateMFAPolicyUseCase saves the tenant's policy.
//
// Authorization is the route's job (admin only) — this enforces the invariants
// the route cannot know: the value is in range, and it is written for the
// caller's own tenant and no other.
type UpdateMFAPolicyUseCase struct {
	store                   MFAPolicyStore
	cache                   MFAPolicyCacheInvalidator
	privilegedOrgRoles      []string
	privilegedBusinessRoles []string
}

// NewUpdateMFAPolicyUseCase builds the writer.
func NewUpdateMFAPolicyUseCase(store MFAPolicyStore, orgRoles, businessRoles []string) *UpdateMFAPolicyUseCase {
	return &UpdateMFAPolicyUseCase{store: store, privilegedOrgRoles: orgRoles, privilegedBusinessRoles: businessRoles}
}

// WithCacheInvalidator makes a saved policy take effect on the next request.
func (uc *UpdateMFAPolicyUseCase) WithCacheInvalidator(i MFAPolicyCacheInvalidator) *UpdateMFAPolicyUseCase {
	uc.cache = i
	return uc
}

// Execute validates and persists the policy.
func (uc *UpdateMFAPolicyUseCase) Execute(ctx context.Context, in UpdateMFAPolicyInput) (*MFAPolicyView, error) {
	if in.TenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("no organization in context")
	}

	policy := domain.MFAPolicy{TenantID: in.TenantID, GraceDays: in.GraceDays}
	if in.ActorID != uuid.Nil {
		actor := in.ActorID
		policy.UpdatedByID = &actor
	}
	// Validated here as well as in the repository: a bad value must be a 400 the
	// form can render, not a constraint violation surfacing as a 500.
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	if err := uc.store.SaveMFAPolicy(ctx, &policy); err != nil {
		return nil, err
	}

	if uc.cache != nil {
		uc.cache.InvalidateTenant(in.TenantID)
	}

	return &MFAPolicyView{
		GraceDays:               policy.EffectiveGraceDays(),
		Configured:              true,
		UpdatedByID:             policy.UpdatedByID,
		MinDays:                 domain.MFAGraceDaysMin,
		MaxDays:                 domain.MFAGraceDaysMax,
		DefaultDays:             domain.MFAGraceDaysDefault,
		PrivilegedOrgRoles:      uc.privilegedOrgRoles,
		PrivilegedBusinessRoles: uc.privilegedBusinessRoles,
	}, nil
}
