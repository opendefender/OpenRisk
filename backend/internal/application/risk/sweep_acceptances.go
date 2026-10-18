// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package risk

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// AcceptanceReminderStage names which notice a reminder stamp records.
type AcceptanceReminderStage int

const (
	AcceptanceReminder30 AcceptanceReminderStage = 30
	AcceptanceReminder7  AcceptanceReminderStage = 7
)

// AcceptanceStore is the cross-tenant slice the expiry sweep needs.
//
// Cross-tenant by necessity — a scheduled job has no session — so the list
// reads every tenant and every write that follows names the row's own
// tenant_id in its WHERE clause. Both writes are conditional: they report
// false when another tick (or another replica) got there first, which is what
// makes two ticks produce one notification and one transition.
type AcceptanceStore interface {
	// ListAcceptancesDue returns accepted risks whose end date falls on or
	// before `until`.
	ListAcceptancesDue(ctx context.Context, until time.Time, limit int) ([]domain.Risk, error)
	// MarkAcceptanceReminded stamps a reminder if it is not stamped yet and the
	// risk is still accepted. False means nothing was stamped.
	MarkAcceptanceReminded(ctx context.Context, tenantID, riskID uuid.UUID, stage AcceptanceReminderStage, at time.Time) (bool, error)
	// ExpireAcceptance moves a risk whose acceptance has lapsed from
	// RESIDUAL_ACCEPTED to ASSESSED and stamps acceptance_expired_at. False
	// means the risk was no longer accepted or not yet due.
	ExpireAcceptance(ctx context.Context, tenantID, riskID uuid.UUID, at time.Time) (bool, error)
}

// NotifyAcceptanceFunc raises one notice for one person. Wired in the
// composition root so this package never depends on the notification module.
type NotifyAcceptanceFunc func(ctx context.Context, tenantID, userID, riskID uuid.UUID, subject, message string)

// SweepResult counts what one pass did, for the log line and the tests.
type SweepResult struct {
	Reminded int
	Expired  int
	Notices  int
}

// SweepAcceptancesUseCase reminds and expires residual-risk acceptances (#811).
//
// At D-30 and D-7 the owner and the approver are told the acceptance is about
// to lapse. At the end date the risk leaves RESIDUAL_ACCEPTED through a SYSTEM
// transition (actor: the acceptance-expiry job, reason acceptance_expired) and
// lands in ASSESSED, where it shows in the owner's Action Center until someone
// re-assesses it. Each step is stamped before anything is sent: a missed
// notice is a smaller harm than an hourly one.
type SweepAcceptancesUseCase struct {
	store   AcceptanceStore
	journal AuditJournal
	notify  NotifyAcceptanceFunc
	now     func() time.Time
}

func NewSweepAcceptancesUseCase(store AcceptanceStore, journal AuditJournal, notify NotifyAcceptanceFunc) *SweepAcceptancesUseCase {
	return &SweepAcceptancesUseCase{store: store, journal: journal, notify: notify}
}

// WithClock pins the clock (tests).
func (uc *SweepAcceptancesUseCase) WithClock(now func() time.Time) *SweepAcceptancesUseCase {
	uc.now = now
	return uc
}

// Execute runs one pass. A row that fails is skipped and the pass goes on;
// the first error is returned so the worker can log it.
func (uc *SweepAcceptancesUseCase) Execute(ctx context.Context) (SweepResult, error) {
	now := time.Now().UTC()
	if uc.now != nil {
		now = uc.now().UTC()
	}
	var res SweepResult
	rows, err := uc.store.ListAcceptancesDue(ctx, now.Add(domain.AcceptanceReminderFar), 500)
	if err != nil {
		return res, fmt.Errorf("application.SweepAcceptances: %w", err)
	}

	var firstErr error
	for i := range rows {
		r := &rows[i]
		if r.AcceptanceExpiresAt == nil || r.State() != domain.StateResidualAccepted {
			continue
		}
		left := r.AcceptanceExpiresAt.Sub(now)

		switch {
		case left <= 0:
			done, err := uc.store.ExpireAcceptance(ctx, r.TenantID, r.ID, now)
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			if !done {
				continue
			}
			res.Expired++
			uc.journalExpiry(ctx, r, now)
			res.Notices += uc.tell(ctx, r, expiredCopy(r))

		case left <= domain.AcceptanceReminderNear:
			if r.AcceptanceReminder7At != nil {
				continue
			}
			done, err := uc.store.MarkAcceptanceReminded(ctx, r.TenantID, r.ID, AcceptanceReminder7, now)
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			if !done {
				continue
			}
			// The D-30 notice is obsolete once D-7 has gone out; stamp it so a
			// late sweep never sends the two out of order.
			if r.AcceptanceReminder30At == nil {
				_, _ = uc.store.MarkAcceptanceReminded(ctx, r.TenantID, r.ID, AcceptanceReminder30, now)
			}
			res.Reminded++
			res.Notices += uc.tell(ctx, r, reminderCopy(r, now))

		default:
			if r.AcceptanceReminder30At != nil {
				continue
			}
			done, err := uc.store.MarkAcceptanceReminded(ctx, r.TenantID, r.ID, AcceptanceReminder30, now)
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			if !done {
				continue
			}
			res.Reminded++
			res.Notices += uc.tell(ctx, r, reminderCopy(r, now))
		}
	}
	return res, firstErr
}

// AcceptanceRecipients are the owner and the approver, once each. The owner
// falls back to the assignee, then to the creator: a lapsing acceptance
// nobody hears about is the failure this feature exists to prevent.
func AcceptanceRecipients(r *domain.Risk) []uuid.UUID {
	out := make([]uuid.UUID, 0, 2)
	add := func(id *uuid.UUID) {
		if id == nil || *id == uuid.Nil {
			return
		}
		for _, seen := range out {
			if seen == *id {
				return
			}
		}
		out = append(out, *id)
	}
	switch {
	case r.OwnerID != nil && *r.OwnerID != uuid.Nil:
		add(r.OwnerID)
	case r.AssigneeID != nil && *r.AssigneeID != uuid.Nil:
		add(r.AssigneeID)
	case r.CreatedBy != uuid.Nil:
		c := r.CreatedBy
		add(&c)
	}
	add(r.AcceptanceApprovedBy)
	return out
}

type notice struct{ subject, message string }

func (uc *SweepAcceptancesUseCase) tell(ctx context.Context, r *domain.Risk, n notice) int {
	if uc.notify == nil {
		return 0
	}
	sent := 0
	for _, to := range AcceptanceRecipients(r) {
		uc.notify(ctx, r.TenantID, to, r.ID, n.subject, n.message)
		sent++
	}
	return sent
}

// journalExpiry records the system transition in the hash-chained trail,
// attributed to the job by name rather than to a blank actor.
func (uc *SweepAcceptancesUseCase) journalExpiry(ctx context.Context, r *domain.Risk, now time.Time) {
	if uc.journal == nil {
		return
	}
	before := r.AcceptanceSnapshot()
	after := map[string]interface{}{
		"lifecycle_state":       string(domain.StateAssessed),
		"reason":                domain.AcceptanceExpiredReason,
		"acceptance_expired_at": now.Format(time.RFC3339),
	}
	if r.AcceptanceExpiresAt != nil {
		after["acceptance_expires_at"] = r.AcceptanceExpiresAt.UTC().Format(time.RFC3339)
	}
	ev := &domain.AuditEvent{
		ID:            uuid.New(),
		TenantID:      r.TenantID,
		ActorType:     domain.AuditActorJob,
		ActorLabel:    domain.AuditJobAcceptanceExpiry,
		Action:        domain.AuditActionUpdate,
		EntityType:    "risk",
		EntityID:      r.ID.String(),
		Summary:       "residual risk acceptance expired: risk re-opened for assessment (" + domain.AcceptanceExpiredReason + ")",
		Before:        domain.JSONMap(before),
		After:         domain.JSONMap(after),
		ChangedFields: domain.StringList{"lifecycle_state", "status", "lifecycle_phase", "acceptance_expired_at"},
		Source:        domain.AuditSourceExplicit,
		CreatedAt:     now,
	}
	_ = uc.journal.Append(ctx, ev)
}

func riskLabel(r *domain.Risk) string {
	if r.Title != "" {
		return r.Title
	}
	return r.Name
}

func reminderCopy(r *domain.Risk, now time.Time) notice {
	days := int(r.AcceptanceExpiresAt.Sub(now).Hours()/24 + 0.999)
	if days < 1 {
		days = 1
	}
	date := r.AcceptanceExpiresAt.UTC().Format("02/01/2006")
	return notice{
		subject: fmt.Sprintf("Acceptation de risque à renouveler sous %d jour(s) : %s", days, riskLabel(r)),
		message: fmt.Sprintf(
			"L'acceptation du risque résiduel « %s » expire le %s. Passé cette date, le risque sera rouvert pour réévaluation. Pour la prolonger, soumettez une nouvelle demande d'approbation dans Gouvernance.",
			riskLabel(r), date),
	}
}

func expiredCopy(r *domain.Risk) notice {
	return notice{
		subject: fmt.Sprintf("Acceptation de risque expirée : %s", riskLabel(r)),
		message: fmt.Sprintf(
			"L'acceptation du risque résiduel « %s » a expiré. Le risque est revenu à l'état « Évalué » et doit être réévalué : traitez-le ou soumettez une nouvelle acceptation.",
			riskLabel(r)),
	}
}
