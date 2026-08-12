// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package workers

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/opendefender/openrisk/internal/application/governance"
)

// ApprovalExpiryWorker closes approval requests nobody decided in time.
//
// Without it, "expires in 48h" is a label rather than a rule: the request stays
// pending forever and the deadline everyone agreed to means nothing. Expiry is
// its own terminal state, never a rejection — the requester needs to know that
// nobody refused, the window simply closed.
//
// Sweeps every 5 minutes: fine enough that a deadline is honoured close to the
// minute, coarse enough not to poll a mostly-empty table.
type ApprovalExpiryWorker struct {
	expire   *governance.ExpireApprovalsUseCase
	logger   zerolog.Logger
	interval time.Duration
}

// NewApprovalExpiryWorker builds the sweep.
func NewApprovalExpiryWorker(expire *governance.ExpireApprovalsUseCase, logger zerolog.Logger) *ApprovalExpiryWorker {
	return &ApprovalExpiryWorker{expire: expire, logger: logger, interval: 5 * time.Minute}
}

// WithInterval overrides the sweep cadence (tests).
func (w *ApprovalExpiryWorker) WithInterval(d time.Duration) *ApprovalExpiryWorker {
	if d > 0 {
		w.interval = d
	}
	return w
}

// Start runs until the context is cancelled.
func (w *ApprovalExpiryWorker) Start(ctx context.Context) {
	if w.expire == nil {
		return
	}
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	w.logger.Info().Dur("interval", w.interval).Msg("approval expiry worker started")
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			closed, err := w.expire.Execute(ctx, time.Now().UTC())
			if err != nil {
				w.logger.Warn().Err(err).Msg("approval expiry sweep failed")
				continue
			}
			if closed > 0 {
				w.logger.Info().Int("expired", closed).Msg("approval requests closed as expired")
			}
		}
	}
}
