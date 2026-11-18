// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package controlstatus

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	ctlchain "github.com/opendefender/openrisk/internal/application/controlchain"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/domain/canonical"
)

// FreshnessView is the freshness of a control's signals.
type FreshnessView struct {
	Percent float64 `json:"percent"`
	Fresh   int     `json:"fresh"`
	Counted int     `json:"counted"`
}

// SourceView is one signal that fed the status, so the status is an audit trail
// and not a bare label.
type SourceView struct {
	Kind       SourceKind `json:"kind"`
	ID         uuid.UUID  `json:"id"`
	TestID     *uuid.UUID `json:"test_id,omitempty"`
	ResultID   *uuid.UUID `json:"result_id,omitempty"`
	ObservedAt *time.Time `json:"observed_at"`
	Fresh      bool       `json:"fresh"`
	Via        Via        `json:"via"`
}

// StatusView is the status of a control as the API returns it. It carries no
// tenant and no stored value: it is computed from the chain at EvaluatedAt.
type StatusView struct {
	ControlID          uuid.UUID                    `json:"control_id"`
	Status             domain.ComputedControlStatus `json:"status"`
	UnderlyingStatus   domain.ComputedControlStatus `json:"underlying_status"`
	DeclaredStatus     canonical.DeclaredStatus     `json:"declared_status"`
	EvaluatedAt        time.Time                    `json:"evaluated_at"`
	Freshness          FreshnessView                `json:"freshness"`
	OpenExceptionCount int                          `json:"open_exception_count"`
	OpenRiskCount      int                          `json:"open_risk_count"`
	Sources            []SourceView                 `json:"sources"`
	Truncated          bool                         `json:"truncated"`
	NextChangeAt       *time.Time                   `json:"next_change_at"`
}

// GetControlStatusUseCase computes the status of one control of the caller's
// tenant, live, at the instant of the request.
type GetControlStatusUseCase struct {
	loader Loader
	now    Clock
}

// NewGetControlStatusUseCase builds the use case.
func NewGetControlStatusUseCase(loader Loader, now Clock) *GetControlStatusUseCase {
	return &GetControlStatusUseCase{loader: loader, now: now}
}

// Execute checks the caller (no tenant or user is Unauthorized, a missing
// compliance:controls:read is Forbidden), reads the control's inputs in one
// tenant-scoped read-only snapshot and computes. It never reads the evaluation
// cache, so it can never serve a status the clock has already overtaken. A control
// that is not live in the caller's tenant, another tenant's included, is
// ErrNotFound.
func (u *GetControlStatusUseCase) Execute(ctx context.Context, c ctlchain.Caller, controlID uuid.UUID) (*StatusView, error) {
	if err := c.Authorize(ctlchain.PermissionRead); err != nil {
		return nil, statusAuthError(err)
	}
	if controlID == uuid.Nil {
		return nil, domain.ControlStatusError(domain.ErrNotFound, domain.CtlStatusNotFound)
	}
	now := u.now().UTC()
	snap, err := u.loader.Load(ctx, c.TenantID, controlID, now)
	if err != nil {
		return nil, wrap(err, "application.GetControlStatus")
	}
	ev := Compute(snap.Inputs, now)
	return viewOf(controlID, snap, ev, now), nil
}

func viewOf(controlID uuid.UUID, snap *Snapshot, ev Evaluation, now time.Time) *StatusView {
	sources := make([]SourceView, len(ev.Sources))
	for i, s := range ev.Sources {
		var observed *time.Time
		if !s.ObservedAt.IsZero() {
			o := s.ObservedAt.UTC()
			observed = &o
		}
		sources[i] = SourceView{Kind: s.Kind, ID: s.ID, TestID: s.TestID, ResultID: s.ResultID, ObservedAt: observed, Fresh: s.Fresh, Via: s.Via}
	}
	return &StatusView{
		ControlID: controlID, Status: ev.Status, UnderlyingStatus: ev.Underlying, DeclaredStatus: snap.DeclaredStatus,
		EvaluatedAt: now, Freshness: FreshnessView{Percent: ev.Percent, Fresh: ev.Fresh, Counted: ev.Counted},
		OpenExceptionCount: ev.OpenExceptionCount, OpenRiskCount: snap.OpenRiskCount,
		Sources: sources, Truncated: ev.Truncated, NextChangeAt: ev.NextChangeAt,
	}
}

// statusAuthError re-codes the library's authorization refusal with the status
// endpoint's own stable codes.
func statusAuthError(err error) error {
	switch {
	case errors.Is(err, domain.ErrUnauthorized):
		return domain.ControlStatusError(domain.ErrUnauthorized, domain.CtlStatusUnauthorized)
	case errors.Is(err, domain.ErrForbidden):
		return domain.ControlStatusError(domain.ErrForbidden, domain.CtlStatusForbidden)
	}
	return err
}

// wrap returns typed application errors unchanged (the central mapper answers
// them) and wraps anything else with the operation.
func wrap(err error, op string) error {
	var app *domain.AppError
	if errors.As(err, &app) {
		return err
	}
	return fmt.Errorf("%s: %w", op, err)
}
