// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package controlstatus

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// DefaultBatch is how many controls one sweep claims (ADR 0012 point 5).
const DefaultBatch = 200

// SweepResult counts what one sweep did.
type SweepResult struct {
	Claimed    int
	Recomputed int
	Deleted    int
	Failed     int
}

// RecomputeControlsUseCase is the worker's use case: it recomputes the controls
// that are stale, due or never computed, and stores the result in the evaluation
// cache. It runs without a session, so it carries no Caller; every control it
// recomputes is scoped by the tenant its own claim row names, with the same loader
// as the live read.
type RecomputeControlsUseCase struct {
	loader Loader
	store  Store
	now    Clock
	batch  int
}

// NewRecomputeControlsUseCase builds the use case. batch <= 0 means DefaultBatch.
func NewRecomputeControlsUseCase(loader Loader, store Store, now Clock, batch int) *RecomputeControlsUseCase {
	if batch <= 0 {
		batch = DefaultBatch
	}
	return &RecomputeControlsUseCase{loader: loader, store: store, now: now, batch: batch}
}

// Sweep claims one batch and recomputes each control of it. A control that fails
// is counted and skipped: its claim lease expires and it is taken again. The first
// error of the claim itself is returned.
func (u *RecomputeControlsUseCase) Sweep(ctx context.Context) (SweepResult, error) {
	var res SweepResult
	refs, err := u.store.Claim(ctx, u.now().UTC(), u.batch)
	if err != nil {
		return res, fmt.Errorf("application.RecomputeControls: claim: %w", err)
	}
	res.Claimed = len(refs)
	for i, ref := range refs {
		if ctx.Err() != nil {
			// The rest are not recomputed: their lease puts them back.
			res.Failed += len(refs) - i
			return res, ctx.Err()
		}
		deleted, err := u.Recompute(ctx, ref.TenantID, ref.ControlID)
		switch {
		case err != nil:
			res.Failed++
		case deleted:
			res.Deleted++
		default:
			res.Recomputed++
		}
	}
	return res, nil
}

// Recompute computes one control of the tenant at the clock's instant and stores
// the evaluation. A control that is gone has its row deleted (deleted is true).
// The tenant filters every query of the loader and the store.
func (u *RecomputeControlsUseCase) Recompute(ctx context.Context, tenantID, controlID uuid.UUID) (deleted bool, err error) {
	if tenantID == uuid.Nil {
		return false, domain.ControlStatusError(domain.ErrUnauthorized, domain.CtlStatusUnauthorized)
	}
	at := u.now().UTC()
	snap, err := u.loader.Load(ctx, tenantID, controlID, at)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			if derr := u.store.Delete(ctx, tenantID, controlID); derr != nil {
				return false, wrap(derr, "application.RecomputeControl")
			}
			return true, nil
		}
		return false, wrap(err, "application.RecomputeControl")
	}
	if err := u.store.Save(ctx, tenantID, controlID, Compute(snap.Inputs, at), at); err != nil {
		return false, wrap(err, "application.RecomputeControl")
	}
	return false, nil
}
