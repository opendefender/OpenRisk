// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package workers

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/opendefender/openrisk/internal/domain"
)

// MitigationDueStore is the cross-tenant slice of mitigation data the reminder
// sweep needs. Cross-tenant by necessity — a scheduled job has no session — so
// every row carries its own tenant_id and the notification is addressed with it.
type MitigationDueStore interface {
	// ListDueForReminder returns unfinished plans with a due date inside the
	// widest reminder window that have not had that reminder sent yet.
	ListDueForReminder(ctx context.Context, now time.Time) ([]domain.Mitigation, error)
	// MarkReminderSent stamps the reminder so the next tick does not resend it.
	MarkReminderSent(ctx context.Context, id uuid.UUID, offset int, at time.Time) error
}

// NotifyDueFunc raises the deadline reminder for a mitigation's assignee.
// Wired in the composition root, so the worker does not depend on the
// notification use case directly.
type NotifyDueFunc func(ctx context.Context, tenantID, userID, mitigationID uuid.UUID, subject, message string)

// MitigationDueWorker sends the D-7 and D-1 deadline nudges.
//
// It ticks hourly rather than by the minute: a day-granularity reminder does not
// need a per-minute sweep, and an hourly cadence keeps a missed tick (deploy,
// restart) from mattering — the rule is "past the threshold and not yet sent",
// not "exactly on the day".
type MitigationDueWorker struct {
	store    MitigationDueStore
	notify   NotifyDueFunc
	logger   zerolog.Logger
	interval time.Duration
}

func NewMitigationDueWorker(store MitigationDueStore, notify NotifyDueFunc, logger zerolog.Logger) *MitigationDueWorker {
	return &MitigationDueWorker{store: store, notify: notify, logger: logger, interval: time.Hour}
}

// WithInterval overrides the sweep cadence (tests).
func (w *MitigationDueWorker) WithInterval(d time.Duration) *MitigationDueWorker {
	if d > 0 {
		w.interval = d
	}
	return w
}

func (w *MitigationDueWorker) Start(ctx context.Context) {
	t := time.NewTicker(w.interval)
	defer t.Stop()
	w.logger.Info().Msg("Mitigation due worker started (D-7 / D-1 deadline reminders)")
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

// Sweep sends every reminder that is due right now. Exported so the boot path
// and tests can drive one pass without a ticker.
func (w *MitigationDueWorker) Sweep(ctx context.Context) {
	now := time.Now()
	rows, err := w.store.ListDueForReminder(ctx, now)
	if err != nil {
		w.logger.Warn().Err(err).Msg("mitigation due: could not list plans approaching their deadline")
		return
	}

	for i := range rows {
		m := rows[i]
		offset, due := m.DueReminderDue(now)
		if !due {
			continue
		}

		// Stamp FIRST. A plan whose notification fails must not be re-notified on
		// every tick for the rest of the week; a missed nudge is a smaller harm
		// than an hourly one.
		if err := w.store.MarkReminderSent(ctx, m.ID, offset, now); err != nil {
			w.logger.Warn().Err(err).Str("mitigation_id", m.ID.String()).
				Msg("mitigation due: could not stamp reminder — skipped to avoid resending in a loop")
			continue
		}

		recipient := dueRecipient(&m)
		if recipient == uuid.Nil {
			w.logger.Info().Str("mitigation_id", m.ID.String()).
				Msg("mitigation due: deadline approaching but nobody is assigned — nothing to notify")
			continue
		}
		if w.notify != nil {
			subject, message := dueReminderCopy(&m, offset, now)
			w.notify(ctx, m.TenantID, recipient, m.ID, subject, message)
		}
		w.logger.Info().
			Str("mitigation_id", m.ID.String()).
			Str("recipient", recipient.String()).
			Int("offset_days", offset).
			Msg("mitigation due: deadline reminder sent")
	}
}

// dueRecipient picks who to nudge: the person doing the work, then the person
// answering for it, then whoever created it. Notifying nobody is better than
// notifying everybody, which is how reminders get muted.
func dueRecipient(m *domain.Mitigation) uuid.UUID {
	if m.AssigneeID != nil && *m.AssigneeID != uuid.Nil {
		return *m.AssigneeID
	}
	if m.OwnerID != nil && *m.OwnerID != uuid.Nil {
		return *m.OwnerID
	}
	if len(m.AssignedTo) > 0 && m.AssignedTo[0] != uuid.Nil {
		return m.AssignedTo[0] // legacy jsonb array
	}
	return m.CreatedBy
}

// dueReminderCopy words the nudge. It states the deadline concretely rather than
// "soon", and says plainly when the plan is already late.
func dueReminderCopy(m *domain.Mitigation, offset int, now time.Time) (string, string) {
	days, _ := m.DaysUntilDue(now)
	when := ""
	if m.DueDate != nil {
		when = m.DueDate.Format("02/01/2006")
	}

	switch {
	case days < 0:
		return fmt.Sprintf("Mitigation en retard : %s", m.Title),
			fmt.Sprintf("« %s » devait être terminée le %s, il y a %d jour(s). Progression : %d %%.",
				m.Title, when, -days, m.Progress)
	case days == 0:
		return fmt.Sprintf("Échéance aujourd'hui : %s", m.Title),
			fmt.Sprintf("« %s » est due aujourd'hui (%s). Progression : %d %%.", m.Title, when, m.Progress)
	case offset == 1:
		return fmt.Sprintf("Échéance demain : %s", m.Title),
			fmt.Sprintf("« %s » est due le %s. Progression : %d %%.", m.Title, when, m.Progress)
	default:
		return fmt.Sprintf("Échéance dans %d jours : %s", days, m.Title),
			fmt.Sprintf("« %s » est due le %s. Progression : %d %%.", m.Title, when, m.Progress)
	}
}
