// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package orgdeletion implements the danger-zone organization erasure with the
// guardrails RGPD and Cameroon law 2024/017 both favour: a mandatory full export
// BEFORE anything is scheduled, exact-name confirmation, an MFA check for enrolled
// admins, a 30-day cancelable grace window, a notification to every admin, and a
// logged purge. Nothing is destroyed synchronously — a worker purges only after
// the grace window elapses, and any admin can call it off until then.
package orgdeletion

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

var (
	// ErrNameMismatch — the operator did not type the exact organization name.
	ErrNameMismatch = errors.New("org deletion: confirmation name does not match the organization name")
	// ErrMFARequired — the enrolled admin did not supply a valid MFA code.
	ErrMFARequired = errors.New("org deletion: a valid MFA code is required")
	// ErrAlreadyPending — a deletion is already scheduled for this org.
	ErrAlreadyPending = errors.New("org deletion: a deletion is already scheduled")
	// ErrNoActiveRequest — nothing to cancel.
	ErrNoActiveRequest = errors.New("org deletion: no active deletion request")
)

// Store persists deletion requests (tenant-scoped).
type Store interface {
	GetActive(ctx context.Context, tenant uuid.UUID) (*domain.OrgDeletionRequest, error)
	Create(ctx context.Context, req *domain.OrgDeletionRequest) error
	Cancel(ctx context.Context, tenant uuid.UUID, by uuid.UUID, at time.Time) error
	ListDue(ctx context.Context, now time.Time) ([]domain.OrgDeletionRequest, error)
	MarkCompleted(ctx context.Context, id uuid.UUID, at time.Time) error
}

// OrgReader returns the organization's exact name (for the typed confirmation).
type OrgReader interface {
	Name(ctx context.Context, tenant uuid.UUID) (string, error)
}

// Exporter produces the mandatory pre-deletion export and returns its path.
type Exporter interface {
	Export(ctx context.Context, tenant uuid.UUID) (path string, err error)
}

// MFAGate verifies an MFA code for an admin who has MFA enrolled. It returns nil
// for an admin with no MFA (exempt — you cannot force TOTP on someone without an
// authenticator), and an error when an enrolled admin's code is wrong/missing.
type MFAGate interface {
	VerifyRequired(ctx context.Context, user, tenant uuid.UUID, code string) error
}

// Purger irreversibly erases a tenant's data.
type Purger interface {
	PurgeTenant(ctx context.Context, tenant uuid.UUID) error
}

// AdminNotifier notifies every admin of the org (best-effort, optional).
type AdminNotifier interface {
	NotifyAdmins(ctx context.Context, tenant uuid.UUID, subject, body string) error
}

// Service orchestrates the danger-zone flow.
type Service struct {
	store    Store
	orgs     OrgReader
	exporter Exporter
	mfa      MFAGate
	purger   Purger
	notifier AdminNotifier
	now      func() time.Time
}

func NewService(store Store, orgs OrgReader, exporter Exporter, mfa MFAGate, purger Purger) *Service {
	return &Service{store: store, orgs: orgs, exporter: exporter, mfa: mfa, purger: purger, now: time.Now}
}

// WithNotifier attaches the admin notifier (optional).
func (s *Service) WithNotifier(n AdminNotifier) *Service { s.notifier = n; return s }

// GetActive returns the pending deletion request for a tenant, or nil.
func (s *Service) GetActive(ctx context.Context, tenant uuid.UUID) (*domain.OrgDeletionRequest, error) {
	return s.store.GetActive(ctx, tenant)
}

// Request schedules a deletion. Order matters: verify name, verify MFA, EXPORT,
// then schedule — so a scheduled deletion always has a completed export behind it.
func (s *Service) Request(ctx context.Context, tenant, requestedBy uuid.UUID, confirmName, mfaCode, reason string) (*domain.OrgDeletionRequest, error) {
	name, err := s.orgs.Name(ctx, tenant)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(confirmName) != strings.TrimSpace(name) {
		return nil, ErrNameMismatch
	}

	if s.mfa != nil {
		if err := s.mfa.VerifyRequired(ctx, requestedBy, tenant, mfaCode); err != nil {
			return nil, ErrMFARequired
		}
	}

	if existing, err := s.store.GetActive(ctx, tenant); err != nil {
		return nil, err
	} else if existing != nil {
		return nil, ErrAlreadyPending
	}

	// Mandatory full export BEFORE scheduling.
	exportPath, err := s.exporter.Export(ctx, tenant)
	if err != nil {
		return nil, fmt.Errorf("pre-deletion export failed, deletion NOT scheduled: %w", err)
	}

	now := s.now()
	req := &domain.OrgDeletionRequest{
		OrganizationID:   tenant,
		RequestedBy:      requestedBy,
		Status:           domain.DeletionPending,
		Reason:           reason,
		ConfirmedName:    confirmName,
		ExportPath:       exportPath,
		ScheduledPurgeAt: now.Add(time.Duration(domain.OrgDeletionGraceDays) * 24 * time.Hour),
	}
	if err := s.store.Create(ctx, req); err != nil {
		return nil, err
	}

	if s.notifier != nil {
		_ = s.notifier.NotifyAdmins(ctx, tenant,
			"Suppression d'organisation programmée",
			fmt.Sprintf("La suppression de « %s » a été programmée pour le %s (délai de grâce de %d jours). Toute personne administratrice peut l'annuler avant cette date.",
				name, req.ScheduledPurgeAt.Format("2006-01-02"), domain.OrgDeletionGraceDays))
	}
	return req, nil
}

// Cancel calls off a pending deletion during its grace window.
func (s *Service) Cancel(ctx context.Context, tenant, by uuid.UUID) error {
	active, err := s.store.GetActive(ctx, tenant)
	if err != nil {
		return err
	}
	if active == nil {
		return ErrNoActiveRequest
	}
	if err := s.store.Cancel(ctx, tenant, by, s.now()); err != nil {
		return err
	}
	if s.notifier != nil {
		_ = s.notifier.NotifyAdmins(ctx, tenant, "Suppression d'organisation annulée",
			"La suppression programmée de votre organisation a été annulée.")
	}
	return nil
}

// RunDuePurges purges every tenant whose grace window has elapsed. Called by a
// scheduled worker. Best-effort per tenant: one failure does not stop the sweep.
func (s *Service) RunDuePurges(ctx context.Context) (int, error) {
	due, err := s.store.ListDue(ctx, s.now())
	if err != nil {
		return 0, err
	}
	purged := 0
	for i := range due {
		req := due[i]
		if err := s.purger.PurgeTenant(ctx, req.OrganizationID); err != nil {
			continue
		}
		if err := s.store.MarkCompleted(ctx, req.ID, s.now()); err != nil {
			continue
		}
		purged++
	}
	return purged, nil
}
