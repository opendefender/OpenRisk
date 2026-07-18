// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package workers

import (
	"context"
	"time"

	appauto "github.com/opendefender/openrisk/internal/application/automation"
	"github.com/rs/zerolog"
)

// SLAMonitor is the background scheduler that drives the SLA lifecycle: every
// tick it escalates overdue remediations past their escalation window and
// auto-closes trackers whose linked risk has been resolved (spec §10 steps
// 7 & 8). It runs cross-tenant; each tracker row carries its own tenant.
type SLAMonitor struct {
	sla      *appauto.SLAService
	logger   zerolog.Logger
	interval time.Duration
}

// NewSLAMonitor builds the monitor (default cadence: one minute).
func NewSLAMonitor(sla *appauto.SLAService, logger zerolog.Logger) *SLAMonitor {
	return &SLAMonitor{sla: sla, logger: logger, interval: time.Minute}
}

// Start runs the monitor loop until ctx is cancelled.
func (m *SLAMonitor) Start(ctx context.Context) {
	t := time.NewTicker(m.interval)
	defer t.Stop()
	m.logger.Info().Msg("SLA monitor started (escalation + auto-close on cadence)")
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			m.tick(ctx, now)
		}
	}
}

func (m *SLAMonitor) tick(ctx context.Context, now time.Time) {
	if n, err := m.sla.SweepEscalations(ctx, now); err != nil {
		m.logger.Warn().Err(err).Msg("sla monitor: escalation sweep failed")
	} else if n > 0 {
		m.logger.Info().Int("escalated", n).Msg("sla monitor: escalated overdue remediations")
	}
	if n, err := m.sla.SweepAutoClose(ctx); err != nil {
		m.logger.Warn().Err(err).Msg("sla monitor: auto-close sweep failed")
	} else if n > 0 {
		m.logger.Info().Int("closed", n).Msg("sla monitor: auto-closed resolved remediations")
	}
}
