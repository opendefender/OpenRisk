// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package notification

import (
	"context"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// Settings → Notifications → "E-mails d'avis" (#997): how long notice e-mails
// about one item are grouped.

// NoticeEmailPolicyStore reads and writes one tenant's policy
// (repository.GormNoticeEmailPolicyRepository).
type NoticeEmailPolicyStore interface {
	NoticeEmailPolicyReader
	SaveNoticeEmailPolicy(ctx context.Context, p *domain.NoticeEmailPolicy) error
}

// NoticeEmailPolicyView is what the settings panel renders. The options and
// the default are sent so the form offers exactly what the server accepts.
type NoticeEmailPolicyView struct {
	WindowMinutes int `json:"window_minutes"`
	// Configured tells a saved choice from the shipped default.
	Configured     bool       `json:"configured"`
	UpdatedByID    *uuid.UUID `json:"updated_by_id,omitempty"`
	Options        []int      `json:"options"`
	DefaultMinutes int        `json:"default_minutes"`
}

func policyView(p *domain.NoticeEmailPolicy) *NoticeEmailPolicyView {
	v := &NoticeEmailPolicyView{
		WindowMinutes:  domain.NoticeEmailWindowDefault,
		Options:        append([]int(nil), domain.NoticeEmailWindows...),
		DefaultMinutes: domain.NoticeEmailWindowDefault,
	}
	if p != nil {
		v.WindowMinutes = int(p.Window().Minutes())
		v.Configured = true
		v.UpdatedByID = p.UpdatedByID
	}
	return v
}

// GetNoticeEmailPolicyUseCase reads the tenant's policy, or the default.
type GetNoticeEmailPolicyUseCase struct {
	store NoticeEmailPolicyStore
}

func NewGetNoticeEmailPolicyUseCase(store NoticeEmailPolicyStore) *GetNoticeEmailPolicyUseCase {
	return &GetNoticeEmailPolicyUseCase{store: store}
}

func (uc *GetNoticeEmailPolicyUseCase) Execute(ctx context.Context, tenantID uuid.UUID) (*NoticeEmailPolicyView, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("no organization in context")
	}
	row, err := uc.store.GetNoticeEmailPolicy(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	return policyView(row), nil
}

// UpdateNoticeEmailPolicyInput is an administrator choosing a window.
type UpdateNoticeEmailPolicyInput struct {
	TenantID      uuid.UUID
	ActorID       uuid.UUID
	WindowMinutes int
}

// UpdateNoticeEmailPolicyUseCase saves the tenant's policy. Who may call it is
// the route's job (admin, root); this checks the value and the tenant.
type UpdateNoticeEmailPolicyUseCase struct {
	store NoticeEmailPolicyStore
}

func NewUpdateNoticeEmailPolicyUseCase(store NoticeEmailPolicyStore) *UpdateNoticeEmailPolicyUseCase {
	return &UpdateNoticeEmailPolicyUseCase{store: store}
}

func (uc *UpdateNoticeEmailPolicyUseCase) Execute(ctx context.Context, in UpdateNoticeEmailPolicyInput) (*NoticeEmailPolicyView, error) {
	if in.TenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("no organization in context")
	}
	p := domain.NoticeEmailPolicy{TenantID: in.TenantID, WindowMinutes: in.WindowMinutes}
	if in.ActorID != uuid.Nil {
		actor := in.ActorID
		p.UpdatedByID = &actor
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	if err := uc.store.SaveNoticeEmailPolicy(ctx, &p); err != nil {
		return nil, err
	}
	return policyView(&p), nil
}
