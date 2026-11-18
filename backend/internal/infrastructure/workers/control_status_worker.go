// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package workers

import (
	"context"
	"errors"
	"time"

	"github.com/rs/zerolog"

	statusapp "github.com/opendefender/openrisk/internal/application/controlstatus"
)

// ControlStatusSweeper is what the worker drives: one claim-and-recompute pass.
// Satisfied by controlstatus.RecomputeControlsUseCase. The pass is cross-tenant by
// necessity (a scheduled job has no session): the claim is the one declared
// cross-tenant read, and every control it returns is recomputed under the tenant
// its own row names.
type ControlStatusSweeper interface {
	Sweep(ctx context.Context) (statusapp.SweepResult, error)
}

// Defaults of the control status worker (ADR 0012 point 5).
const (
	// ControlStatusInterval is how often a pass runs.
	ControlStatusInterval = 60 * time.Second
	// controlStatusMaxPasses bounds the passes of one tick: a pass that claims a full
	// batch is followed at once by another, so a cold fill of a large catalogue does
	// not take one batch a minute, and no tick runs unbounded.
	controlStatusMaxPasses = 10
	// controlStatusTickBudget bounds the time of one tick. Past it, the claimed rows
	// that were not recomputed are leased and come back.
	controlStatusTickBudget = 50 * time.Second
)

// ControlStatusWorker keeps the evaluation cache of the control status engine
// (#555) current: it recomputes the controls a write marked stale, those whose
// clock-driven change has come due (an exception ending, a window closing) and
// those never computed, in batches of 200, every 60 seconds.
//
// It is not what a user reads: the status endpoint computes live. The cache is what
// the score readers may join later (#1465), and what makes an incremental
// recompute observable.
type ControlStatusWorker struct {
	sweeper  ControlStatusSweeper
	logger   zerolog.Logger
	interval time.Duration
	done     chan struct{}
}

// NewControlStatusWorker builds the worker.
func NewControlStatusWorker(sweeper ControlStatusSweeper, logger zerolog.Logger) *ControlStatusWorker {
	return &ControlStatusWorker{sweeper: sweeper, logger: logger, interval: ControlStatusInterval, done: make(chan struct{})}
}

// WithInterval overrides the pass cadence (tests).
func (w *ControlStatusWorker) WithInterval(d time.Duration) *ControlStatusWorker {
	if d > 0 {
		w.interval = d
	}
	return w
}

// Start runs until ctx is cancelled. A pass in flight when ctx is cancelled is
// finished (up to its tick budget): it runs on a context that is not cancelled with
// ctx, so a shutdown does not abandon a half-recomputed batch. Done is closed when
// Start returns.
func (w *ControlStatusWorker) Start(ctx context.Context) {
	defer close(w.done)
	t := time.NewTicker(w.interval)
	defer t.Stop()
	w.logger.Info().Dur("interval", w.interval).Msg("Control status worker started (stale, due and never-computed controls, in batches)")
	w.Tick(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			w.Tick(ctx)
		}
	}
}

// Tick runs the passes of one tick. Exported so tests drive it without a ticker.
func (w *ControlStatusWorker) Tick(parent context.Context) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), controlStatusTickBudget)
	defer cancel()
	var total statusapp.SweepResult
	for pass := 0; pass < controlStatusMaxPasses; pass++ {
		res, err := w.sweeper.Sweep(ctx)
		total.Claimed += res.Claimed
		total.Recomputed += res.Recomputed
		total.Deleted += res.Deleted
		total.Failed += res.Failed
		if err != nil {
			if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
				w.logger.Warn().Err(err).Msg("control status: a pass failed")
			}
			break
		}
		if res.Claimed < statusapp.DefaultBatch {
			break // the backlog is drained
		}
		if parent.Err() != nil {
			break // shutting down: no further pass
		}
	}
	if total.Failed > 0 {
		w.logger.Warn().Int("failed", total.Failed).Msg("control status: some controls could not be recomputed; they are retried when their claim lease ends")
	}
	if total.Claimed > 0 {
		w.logger.Info().Int("claimed", total.Claimed).Int("recomputed", total.Recomputed).Int("deleted", total.Deleted).
			Msg("control status: controls recomputed")
	}
}

// Drain waits for Start to return, up to ctx. Call it after cancelling Start's
// context, as the other workers do at shutdown.
func (w *ControlStatusWorker) Drain(ctx context.Context) error {
	select {
	case <-w.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
