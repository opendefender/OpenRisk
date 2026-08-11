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

// AuditRetentionWorker applies each tenant's configured audit-retention window.
//
// It sweeps every 6 hours rather than continuously: retention is a
// day-granularity policy, and a sweep that deletes evidence is not something to
// run more often than it needs to. Tenants with no window (the default) are
// untouched — nothing is ever deleted unless someone configured it.
//
// Pruning seals what it removes (domain.AuditChainSeal), so verification of the
// surviving chain still passes and can account for the gap.
type AuditRetentionWorker struct {
	prune    *governance.PruneAuditTrailUseCase
	logger   zerolog.Logger
	interval time.Duration
}

// NewAuditRetentionWorker builds the sweep.
func NewAuditRetentionWorker(prune *governance.PruneAuditTrailUseCase, logger zerolog.Logger) *AuditRetentionWorker {
	return &AuditRetentionWorker{prune: prune, logger: logger, interval: 6 * time.Hour}
}

// WithInterval overrides the sweep cadence (tests).
func (w *AuditRetentionWorker) WithInterval(d time.Duration) *AuditRetentionWorker {
	if d > 0 {
		w.interval = d
	}
	return w
}

// Start runs the sweep until the context is cancelled.
func (w *AuditRetentionWorker) Start(ctx context.Context) {
	if w.prune == nil {
		return
	}
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	w.logger.Info().Dur("interval", w.interval).Msg("audit retention worker started")
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.sweep(ctx)
		}
	}
}

func (w *AuditRetentionWorker) sweep(ctx context.Context) {
	results, err := w.prune.ExecuteAll(ctx, time.Now().UTC())
	if err != nil {
		w.logger.Warn().Err(err).Msg("audit retention sweep failed")
		return
	}
	var total int64
	for _, r := range results {
		total += r.Pruned
		if r.Pruned > 0 {
			w.logger.Info().
				Str("tenant", r.TenantID.String()).
				Int64("pruned", r.Pruned).
				Msg("audit retention: entries pruned and sealed")
		}
	}
	if total > 0 {
		w.logger.Info().Int64("pruned", total).Int("tenants", len(results)).Msg("audit retention sweep complete")
	}
}
