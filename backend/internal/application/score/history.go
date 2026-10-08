// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package score

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/domain/scoring"
)

// SnapshotStore persists the daily tenant score (#901).
type SnapshotStore interface {
	Upsert(ctx context.Context, s *domain.TenantScoreSnapshot) error
	ListSince(ctx context.Context, tenantID uuid.UUID, since time.Time) ([]domain.TenantScoreSnapshot, error)
}

// HistoryPoint is one month of the tenant score: the last snapshot taken in
// that month. A month with no snapshot is absent, never zero.
type HistoryPoint struct {
	Month string    `json:"month"` // YYYY-MM
	Day   time.Time `json:"day"`
	Value float64   `json:"value"`
	Band  string    `json:"band"`
}

// History is what GET /score/history returns.
type History struct {
	Months int            `json:"months"`
	Points []HistoryPoint `json:"points"`
	// Current is today's measured value, nil when the tenant has nothing to
	// score yet.
	Current *float64 `json:"current"`
	// Delta30d is today's value minus the value 30 days ago (the last snapshot
	// on or before that day). Nil when no snapshot is that old: a delta needs
	// two real measurements.
	Delta30d *float64 `json:"delta_30d"`
	// Since is the oldest snapshot kept for the tenant within the window, so
	// the UI can say how far back the line really goes.
	Since *time.Time `json:"since"`
}

// HistoryUseCase records and reads the daily tenant score history.
type HistoryUseCase struct {
	score *UseCase
	store SnapshotStore
	now   func() time.Time
}

func NewHistory(score *UseCase, store SnapshotStore) *HistoryUseCase {
	return &HistoryUseCase{score: score, store: store, now: func() time.Time { return time.Now().UTC() }}
}

// Record computes the tenant score and stores it as today's snapshot. An
// unmeasured score stores nothing.
func (h *HistoryUseCase) Record(ctx context.Context, tenantID uuid.UUID) (*scoring.Result, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewValidationError("tenant is required")
	}
	res, err := h.score.Execute(ctx, tenantID, scoring.ScopeTenant, uuid.Nil)
	if err != nil {
		return nil, err
	}
	if res == nil || !res.Measured {
		return res, nil
	}
	snap := &domain.TenantScoreSnapshot{
		TenantID:       tenantID,
		Day:            domain.SnapshotDay(h.now()),
		Value:          res.Value,
		Band:           string(res.Band),
		FormulaVersion: res.FormulaVersion,
	}
	if err := h.store.Upsert(ctx, snap); err != nil {
		return res, err
	}
	return res, nil
}

// Execute returns the monthly history over the last `months` months (1–24),
// recording today's snapshot first so the line always ends on the live value.
func (h *HistoryUseCase) Execute(ctx context.Context, tenantID uuid.UUID, months int) (*History, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewValidationError("tenant is required")
	}
	if months < 1 || months > 24 {
		return nil, domain.NewValidationError("months must be between 1 and 24")
	}
	if _, err := h.Record(ctx, tenantID); err != nil {
		return nil, err
	}

	today := domain.SnapshotDay(h.now())
	start := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -(months - 1), 0)
	// Reach 30 days further back than the window when needed, so the delta can
	// be computed even for a one-month window.
	from := start
	if d := today.AddDate(0, 0, -30); d.Before(from) {
		from = d
	}
	rows, err := h.store.ListSince(ctx, tenantID, from)
	if err != nil {
		return nil, err
	}

	out := &History{Months: months, Points: []HistoryPoint{}}
	byMonth := map[string]HistoryPoint{}
	var order []string
	for _, r := range rows {
		day := domain.SnapshotDay(r.Day)
		if day.Before(start) {
			continue
		}
		key := day.Format("2006-01")
		if _, seen := byMonth[key]; !seen {
			order = append(order, key)
		}
		// Rows come oldest first, so the last write wins: the month's closing value.
		byMonth[key] = HistoryPoint{Month: key, Day: day, Value: r.Value, Band: r.Band}
		if out.Since == nil {
			d := day
			out.Since = &d
		}
	}
	for _, k := range order {
		out.Points = append(out.Points, byMonth[k])
	}

	var current *domain.TenantScoreSnapshot
	var past *domain.TenantScoreSnapshot
	cutoff := today.AddDate(0, 0, -30)
	for i := range rows {
		day := domain.SnapshotDay(rows[i].Day)
		if day.Equal(today) {
			current = &rows[i]
		}
		if !day.After(cutoff) {
			past = &rows[i]
		}
	}
	if current != nil {
		v := current.Value
		out.Current = &v
		if past != nil {
			d := roundTenth(current.Value - past.Value)
			out.Delta30d = &d
		}
	}
	return out, nil
}

func roundTenth(v float64) float64 {
	if v < 0 {
		return -float64(int64(-v*10+0.5)) / 10
	}
	return float64(int64(v*10+0.5)) / 10
}
