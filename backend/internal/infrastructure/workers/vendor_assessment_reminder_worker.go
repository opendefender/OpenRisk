// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package workers

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/opendefender/openrisk/internal/application/tprm"
)

// VendorReminderSweeper runs one sweep of the vendor questionnaire reminders.
// Satisfied by tprm.SendVendorAssessmentRemindersUseCase.
type VendorReminderSweeper interface {
	Execute(ctx context.Context) ([]tprm.ReminderOutcome, error)
}

// VendorAssessmentReminderWorker sends the J-7 / J-3 / J-1 reminders for open
// vendor questionnaires (#672, ADR 0004 D6).
//
// It ticks hourly, like MitigationDueWorker, for the same reason: the rule is
// "past the threshold and not yet sent", so a missed tick (deploy, restart)
// still reminds instead of skipping — and the use case sends at most one
// reminder per questionnaire per sweep, so it never bursts either.
type VendorAssessmentReminderWorker struct {
	sweeper  VendorReminderSweeper
	logger   zerolog.Logger
	interval time.Duration
}

func NewVendorAssessmentReminderWorker(sweeper VendorReminderSweeper, logger zerolog.Logger) *VendorAssessmentReminderWorker {
	return &VendorAssessmentReminderWorker{sweeper: sweeper, logger: logger, interval: time.Hour}
}

// WithInterval overrides the sweep cadence (tests).
func (w *VendorAssessmentReminderWorker) WithInterval(d time.Duration) *VendorAssessmentReminderWorker {
	if d > 0 {
		w.interval = d
	}
	return w
}

func (w *VendorAssessmentReminderWorker) Start(ctx context.Context) {
	t := time.NewTicker(w.interval)
	defer t.Stop()
	w.logger.Info().Msg("Vendor assessment reminder worker started (J-7 / J-3 / J-1)")
	// Sweep once at boot so a deployment that spans a threshold does not skip it.
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

// Sweep runs one pass and logs every outcome. The log names the tenant, the
// assessment, the offset and the outcome — never a token or a link (RULE 6).
func (w *VendorAssessmentReminderWorker) Sweep(ctx context.Context) {
	outcomes, err := w.sweeper.Execute(ctx)
	if err != nil {
		w.logger.Warn().Err(err).Msg("vendor reminders: could not list questionnaires approaching their due date")
		return
	}
	for _, o := range outcomes {
		ev := w.logger.Info()
		switch o.Status {
		case tprm.ReminderMailFailed, tprm.ReminderError:
			ev = w.logger.Warn()
			if o.Err != nil {
				ev = ev.Str("error", o.Err.Error())
			}
		}
		ev.Str("tenant_id", o.TenantID.String()).
			Str("assessment_id", o.AssessmentID.String()).
			Int("offset_days", o.Offset).
			Str("outcome", string(o.Status)).
			Msg("vendor reminders: questionnaire reminder")
	}
}
