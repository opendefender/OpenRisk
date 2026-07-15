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

// RiskReviewStore is the cross-tenant slice of risk data the review worker needs.
type RiskReviewStore interface {
	ListDueForReview(ctx context.Context, now time.Time) ([]domain.Risk, error)
	BumpNextReview(ctx context.Context, id uuid.UUID, next time.Time) error
}

// NotifyReviewFunc raises the "review due" reminder for a risk's owner (in-app +
// e-mail). Wired in the composition root so the worker doesn't depend on the
// notification use case directly.
type NotifyReviewFunc func(ctx context.Context, tenantID, ownerID, riskID uuid.UUID, riskTitle string)

// RiskReviewWorker nudges risk owners when a review is due, keeping the risk
// register "updated regularly" with an enforced cadence. Each due risk is
// notified once per interval (NextReviewAt is bumped forward after notifying);
// marking the risk reviewed resets the cadence from that point.
type RiskReviewWorker struct {
	store    RiskReviewStore
	notify   NotifyReviewFunc
	logger   zerolog.Logger
	interval time.Duration
}

func NewRiskReviewWorker(store RiskReviewStore, notify NotifyReviewFunc, logger zerolog.Logger) *RiskReviewWorker {
	return &RiskReviewWorker{store: store, notify: notify, logger: logger, interval: time.Minute}
}

func (w *RiskReviewWorker) Start(ctx context.Context) {
	t := time.NewTicker(w.interval)
	defer t.Stop()
	w.logger.Info().Msg("Risk review worker started (owner reminders on cadence)")
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			w.runDue(ctx)
		}
	}
}

func (w *RiskReviewWorker) runDue(ctx context.Context) {
	now := time.Now()
	due, err := w.store.ListDueForReview(ctx, now)
	if err != nil {
		w.logger.Warn().Err(err).Msg("risk review: could not list due risks")
		return
	}
	for i := range due {
		r := due[i]
		owner := reviewOwner(&r)
		// Push the next review out first so a due risk is nudged once per cadence,
		// regardless of whether an owner exists to notify.
		next := now.Add(time.Duration(r.ReviewIntervalDays) * 24 * time.Hour)
		if err := w.store.BumpNextReview(ctx, r.ID, next); err != nil {
			w.logger.Warn().Err(err).Str("risk_id", r.ID.String()).Msg("risk review: could not bump next review")
			continue
		}
		if owner == uuid.Nil {
			w.logger.Info().Str("risk_id", r.ID.String()).Msg("risk review: due but no owner to notify — skipped")
			continue
		}
		if w.notify != nil {
			w.notify(ctx, r.TenantID, owner, r.ID, r.Title)
		}
		w.logger.Info().
			Str("risk_id", r.ID.String()).
			Str("owner", owner.String()).
			Int("interval_days", r.ReviewIntervalDays).
			Msg("risk review: owner reminded")
	}
}

// reviewOwner resolves who is responsible for reviewing a risk: the reviewer, then
// the assignee, then the creator. Returns uuid.Nil if none is set.
func reviewOwner(r *domain.Risk) uuid.UUID {
	if r.ReviewerID != nil && *r.ReviewerID != uuid.Nil {
		return *r.ReviewerID
	}
	if r.AssignedTo != nil && *r.AssignedTo != uuid.Nil {
		return *r.AssignedTo
	}
	return r.CreatedBy
}
