// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package workers

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/opendefender/openrisk/internal/domain"
)

// ScanTrigger is the slice of the scanner TriggerScanUseCase the scheduler needs.
type ScanTrigger interface {
	Execute(ctx context.Context, tenantID, triggeredBy, configID uuid.UUID) (*domain.ScanJob, error)
}

// ScanScheduler turns recurring ScanConfigs into scans. Every minute it finds
// due configs (across all tenants) and triggers each, then advances its
// NextRunAt by the configured interval — so vulnerability scanning runs on a
// cadence, not just on-demand. The TriggerScanUseCase still enforces the
// per-config lock and per-tenant concurrency cap, so a still-running scan is
// simply skipped until the next tick.
type ScanScheduler struct {
	configRepo domain.ScanConfigRepository
	trigger    ScanTrigger
	logger     zerolog.Logger
	interval   time.Duration
}

// systemUser marks scheduler-triggered scans (no human initiator).
var systemUser = uuid.Nil

func NewScanScheduler(configRepo domain.ScanConfigRepository, trigger ScanTrigger, logger zerolog.Logger) *ScanScheduler {
	return &ScanScheduler{configRepo: configRepo, trigger: trigger, logger: logger, interval: time.Minute}
}

// Start runs the scheduler loop until ctx is cancelled.
func (s *ScanScheduler) Start(ctx context.Context) {
	t := time.NewTicker(s.interval)
	defer t.Stop()
	s.logger.Info().Msg("Scan scheduler started (recurring scans)")
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.runDue(ctx)
		}
	}
}

func (s *ScanScheduler) runDue(ctx context.Context) {
	now := time.Now()
	due, err := s.configRepo.ListDueScheduled(ctx, now)
	if err != nil {
		s.logger.Warn().Err(err).Msg("scan scheduler: could not list due configs")
		return
	}
	for i := range due {
		cfg := due[i]
		// Advance the schedule FIRST so a slow/failing trigger can't cause the same
		// config to be picked up again next tick (steady cadence, no retry storm).
		next := now.Add(time.Duration(cfg.ScheduleMinutes) * time.Minute)
		if err := s.configRepo.UpdateNextRun(ctx, cfg.ID, cfg.TenantID, now, next); err != nil {
			s.logger.Warn().Err(err).Str("config_id", cfg.ID.String()).Msg("scan scheduler: could not advance schedule")
			continue
		}
		if _, err := s.trigger.Execute(ctx, cfg.TenantID, systemUser, cfg.ID); err != nil {
			// Conflicts (a scan already running / tenant cap) are expected and fine —
			// the next tick will catch up.
			s.logger.Info().Err(err).Str("config_id", cfg.ID.String()).Msg("scan scheduler: trigger skipped")
			continue
		}
		s.logger.Info().
			Str("config_id", cfg.ID.String()).
			Str("tenant_id", cfg.TenantID.String()).
			Int("interval_min", cfg.ScheduleMinutes).
			Msg("scan scheduler: triggered recurring scan")
	}
}
