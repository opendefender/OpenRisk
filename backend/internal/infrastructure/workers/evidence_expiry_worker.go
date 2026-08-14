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

// EvidenceExpiryStore is the cross-tenant slice the sweep needs. Cross-tenant by
// necessity — a scheduled job has no session — so every row carries its own
// tenant_id and each reminder is addressed with it.
type EvidenceExpiryStore interface {
	ListExpiring(ctx context.Context, now time.Time, window time.Duration, limit int) ([]domain.Evidence, error)
	MarkReminded(ctx context.Context, id uuid.UUID, at time.Time) error
}

// NotifyExpiryFunc raises the reminder for whoever owns the artifact. Wired in
// the composition root so the worker never depends on the notification use case.
type NotifyExpiryFunc func(ctx context.Context, tenantID, userID, evidenceID uuid.UUID, subject, message string)

// EvidenceExpiryWorker warns the owner before proof goes stale.
//
// This is the half of the module that makes expiry matter. Recording a
// valid_until date only relocates the problem: without a nudge, the first person
// to notice a lapsed certificate is the auditor reading the register. The
// register would still be honest — the control shows as unevidenced — but the
// organisation finds out at the worst possible moment.
//
// Hourly, like the mitigation deadline sweep, and for the same reason: the rule
// is "inside the window and not yet reminded", not "exactly on the day", so a
// missed tick during a deploy costs nothing.
type EvidenceExpiryWorker struct {
	store    EvidenceExpiryStore
	notify   NotifyExpiryFunc
	logger   zerolog.Logger
	interval time.Duration
	window   time.Duration
}

func NewEvidenceExpiryWorker(store EvidenceExpiryStore, notify NotifyExpiryFunc, logger zerolog.Logger) *EvidenceExpiryWorker {
	return &EvidenceExpiryWorker{
		store:    store,
		notify:   notify,
		logger:   logger,
		interval: time.Hour,
		window:   domain.EvidenceExpiryWindow,
	}
}

// WithInterval overrides the sweep cadence (tests).
func (w *EvidenceExpiryWorker) WithInterval(d time.Duration) *EvidenceExpiryWorker {
	if d > 0 {
		w.interval = d
	}
	return w
}

func (w *EvidenceExpiryWorker) Start(ctx context.Context) {
	t := time.NewTicker(w.interval)
	defer t.Stop()
	w.logger.Info().Msg("Evidence expiry worker started (renewal reminders before proof goes stale)")
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

// Sweep sends every reminder due right now. Exported so boot and tests can drive
// a single pass without a ticker.
func (w *EvidenceExpiryWorker) Sweep(ctx context.Context) {
	now := time.Now()
	rows, err := w.store.ListExpiring(ctx, now, w.window, 500)
	if err != nil {
		w.logger.Warn().Err(err).Msg("evidence expiry: could not list artifacts approaching their expiry")
		return
	}

	for i := range rows {
		ev := rows[i]

		// Stamp FIRST. An artifact whose notification fails must not be re-notified
		// every hour until someone renews it; a missed nudge is a smaller harm than
		// an hourly one, and it is the same trade the mitigation sweep makes.
		if err := w.store.MarkReminded(ctx, ev.ID, now); err != nil {
			w.logger.Warn().Err(err).Str("evidence_id", ev.ID.String()).
				Msg("evidence expiry: could not stamp reminder — skipped to avoid resending in a loop")
			continue
		}

		recipient := expiryRecipient(&ev)
		if recipient == uuid.Nil {
			// Worth a line: proof with nobody attached to it is exactly the proof
			// that lapses, and the missing-evidence view is the only thing that will
			// catch it.
			w.logger.Info().Str("evidence_id", ev.ID.String()).
				Msg("evidence expiry: proof is expiring but nobody owns it — nothing to notify")
			continue
		}

		if w.notify != nil {
			subject, message := expiryReminderCopy(&ev, now)
			w.notify(ctx, ev.TenantID, recipient, ev.ID, subject, message)
		}
		w.logger.Info().
			Str("evidence_id", ev.ID.String()).
			Str("recipient", recipient.String()).
			Msg("evidence expiry: renewal reminder sent")
	}
}

// expiryRecipient picks who to nudge: the person who must refresh the proof,
// then the person who answers for it, then whoever collected it.
func expiryRecipient(ev *domain.Evidence) uuid.UUID {
	if ev.Ownership.AssigneeID != nil && *ev.Ownership.AssigneeID != uuid.Nil {
		return *ev.Ownership.AssigneeID
	}
	if ev.Ownership.OwnerID != nil && *ev.Ownership.OwnerID != uuid.Nil {
		return *ev.Ownership.OwnerID
	}
	if ev.CollectedBy != nil {
		return *ev.CollectedBy
	}
	return uuid.Nil
}

// expiryReminderCopy writes the nudge. It leads with the consequence rather than
// the date, because "renew this or the control stops being evidenced" is the part
// that gets a certificate reordered.
func expiryReminderCopy(ev *domain.Evidence, now time.Time) (subject, message string) {
	title := ev.Title
	if title == "" {
		title = ev.Filename
	}

	days := 0
	if d := ev.DaysUntil(now); d != nil {
		days = *d
	}

	switch {
	case days < 0:
		subject = fmt.Sprintf("Preuve expirée : %s", title)
		message = fmt.Sprintf(
			"La preuve « %s » a expiré il y a %d jour(s). Elle ne justifie plus les contrôles auxquels elle est rattachée : ceux-ci apparaissent désormais comme non couverts. Remplacez-la ou prolongez sa validité.",
			title, -days)
	case days == 0:
		subject = fmt.Sprintf("Preuve expirant aujourd'hui : %s", title)
		message = fmt.Sprintf(
			"La preuve « %s » expire aujourd'hui. Passé ce jour, les contrôles qu'elle justifie ne seront plus couverts.",
			title)
	default:
		subject = fmt.Sprintf("Preuve à renouveler sous %d jour(s) : %s", days, title)
		message = fmt.Sprintf(
			"La preuve « %s » expire dans %d jour(s). Renouvelez-la avant cette date pour que les contrôles qu'elle justifie restent couverts.",
			title, days)
	}
	return subject, message
}
