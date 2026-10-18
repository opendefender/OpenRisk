// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package workers

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	riskapp "github.com/opendefender/openrisk/internal/application/risk"
)

// AcceptanceSweeper is the one pass the worker drives.
type AcceptanceSweeper interface {
	Execute(ctx context.Context) (riskapp.SweepResult, error)
}

// AcceptanceExpiryWorker keeps residual-risk acceptances time-bound (#811).
//
// Hourly, like the evidence sweep: the rule is "inside the window and not yet
// stamped", not "exactly on the day", so a missed tick during a deploy costs
// nothing, and two ticks never send a notice twice.
type AcceptanceExpiryWorker struct {
	sweeper  AcceptanceSweeper
	logger   zerolog.Logger
	interval time.Duration
}

func NewAcceptanceExpiryWorker(sweeper AcceptanceSweeper, logger zerolog.Logger) *AcceptanceExpiryWorker {
	return &AcceptanceExpiryWorker{sweeper: sweeper, logger: logger, interval: time.Hour}
}

func (w *AcceptanceExpiryWorker) Start(ctx context.Context) {
	t := time.NewTicker(w.interval)
	defer t.Stop()
	w.logger.Info().Msg("Acceptance expiry worker started (residual-risk acceptances are time-bound)")
	// Sweep once at boot so a deployment spanning a threshold does not skip it.
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

// Sweep runs one pass and logs what it did.
func (w *AcceptanceExpiryWorker) Sweep(ctx context.Context) {
	res, err := w.sweeper.Execute(ctx)
	if err != nil {
		w.logger.Warn().Err(err).Msg("acceptance expiry: sweep incomplete")
	}
	if res.Reminded > 0 || res.Expired > 0 {
		w.logger.Info().
			Int("reminded", res.Reminded).
			Int("expired", res.Expired).
			Int("notices", res.Notices).
			Msg("acceptance expiry: sweep done")
	}
}
