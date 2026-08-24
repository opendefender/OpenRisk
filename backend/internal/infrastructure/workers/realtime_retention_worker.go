// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package workers

import (
	"context"
	"time"

	"github.com/rs/zerolog"
)

// RealtimeEventPurger is the retention side of the durable event log.
type RealtimeEventPurger interface {
	PurgeBefore(ctx context.Context, cutoff time.Time) (int64, error)
}

// RealtimeRetentionWorker keeps the replay window at its stated size.
//
// This worker is what makes the replay promise honest in both directions. The
// stream tells a client "reconnect within the window and you will be caught up",
// and it tells a client whose cursor predates the window to resynchronise. Both
// statements are only true if something actually enforces the window: without
// this sweep the table grows for the life of the deployment and the window is
// whatever the disk allows, which is a promise nobody made and nobody can keep.
//
// Unlike the audit trail's retention, this one is not per tenant and not
// configurable per tenant. The event log is a replay buffer, not evidence: the
// business state its events describe lives in the tables they were derived from,
// so there is nothing here a tenant could need kept longer, and an instance-wide
// window is the only thing that bounds the table.
type RealtimeRetentionWorker struct {
	purger    RealtimeEventPurger
	retention time.Duration
	interval  time.Duration
	logger    zerolog.Logger
}

// NewRealtimeRetentionWorker builds the sweep.
func NewRealtimeRetentionWorker(purger RealtimeEventPurger, retention time.Duration, logger zerolog.Logger) *RealtimeRetentionWorker {
	// Hourly, not daily: an hourly sweep keeps the table's size close to the
	// window at all times, while a daily one would let it swell to a day's
	// worth of surplus before collapsing — and the surplus is largest exactly
	// when the system is busiest.
	return &RealtimeRetentionWorker{
		purger:    purger,
		retention: retention,
		interval:  time.Hour,
		logger:    logger,
	}
}

// WithInterval overrides the sweep cadence (tests).
func (w *RealtimeRetentionWorker) WithInterval(d time.Duration) *RealtimeRetentionWorker {
	if d > 0 {
		w.interval = d
	}
	return w
}

// Start sweeps until the context is cancelled.
func (w *RealtimeRetentionWorker) Start(ctx context.Context) {
	if w == nil || w.purger == nil || w.retention <= 0 {
		return
	}
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	w.logger.Info().
		Dur("retention", w.retention).
		Dur("interval", w.interval).
		Msg("realtime retention worker started")

	// Sweep once at boot: a deployment that was down over a weekend would
	// otherwise serve an oversized window for a full interval after coming back.
	w.sweep(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.sweep(ctx)
		}
	}
}

func (w *RealtimeRetentionWorker) sweep(ctx context.Context) {
	cutoff := time.Now().UTC().Add(-w.retention)
	removed, err := w.purger.PurgeBefore(ctx, cutoff)
	if err != nil {
		// Logged, never fatal. A failed sweep costs disk; a worker that exits on
		// a transient database error costs the window forever.
		w.logger.Error().Err(err).Time("cutoff", cutoff).Msg("realtime retention sweep failed")
		return
	}
	if removed > 0 {
		w.logger.Info().Int64("removed", removed).Time("cutoff", cutoff).Msg("realtime events purged")
	}
}
