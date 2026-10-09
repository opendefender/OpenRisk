// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package workers

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/opendefender/openrisk/internal/domain/scoring"
)

// TenantIDLister lists every tenant the sweep should snapshot.
type TenantIDLister interface {
	ListTenantIDs(ctx context.Context) ([]uuid.UUID, error)
}

// ScoreRecorder computes and stores one tenant's score for today.
type ScoreRecorder interface {
	Record(ctx context.Context, tenantID uuid.UUID) (*scoring.Result, error)
}

// ScoreSnapshotWorker takes the daily snapshot of every tenant's exposure
// score (#901). The history endpoint records too when someone opens the
// dashboard; this sweep is what keeps a quiet tenant's line continuous.
// Re-running on the same day replaces that day's row, so the interval only
// decides how fresh the day's closing value is.
type ScoreSnapshotWorker struct {
	tenants  TenantIDLister
	recorder ScoreRecorder
	logger   zerolog.Logger
	interval time.Duration
}

func NewScoreSnapshotWorker(tenants TenantIDLister, recorder ScoreRecorder, logger zerolog.Logger) *ScoreSnapshotWorker {
	return &ScoreSnapshotWorker{tenants: tenants, recorder: recorder, logger: logger, interval: 6 * time.Hour}
}

// WithInterval overrides the sweep interval (tests).
func (w *ScoreSnapshotWorker) WithInterval(d time.Duration) *ScoreSnapshotWorker {
	if d > 0 {
		w.interval = d
	}
	return w
}

func (w *ScoreSnapshotWorker) Start(ctx context.Context) {
	t := time.NewTicker(w.interval)
	defer t.Stop()
	w.Sweep(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			w.Sweep(ctx)
		}
	}
}

// Sweep records every tenant once. One tenant failing never stops the others.
func (w *ScoreSnapshotWorker) Sweep(ctx context.Context) int {
	ids, err := w.tenants.ListTenantIDs(ctx)
	if err != nil {
		w.logger.Warn().Err(err).Msg("score snapshots: could not list tenants")
		return 0
	}
	recorded := 0
	for _, id := range ids {
		if ctx.Err() != nil {
			return recorded
		}
		if _, err := w.recorder.Record(ctx, id); err != nil {
			w.logger.Warn().Err(err).Str("tenant_id", id.String()).Msg("score snapshots: record failed")
			continue
		}
		recorded++
	}
	return recorded
}
