// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package score

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
)

// memSnapshots is an in-memory SnapshotStore keyed like the real table.
type memSnapshots struct {
	rows      map[string]domain.TenantScoreSnapshot
	upsertErr error
}

func newMemSnapshots() *memSnapshots {
	return &memSnapshots{rows: map[string]domain.TenantScoreSnapshot{}}
}

func (m *memSnapshots) key(t uuid.UUID, d time.Time) string {
	return t.String() + "|" + domain.SnapshotDay(d).Format("2006-01-02")
}

func (m *memSnapshots) Upsert(_ context.Context, s *domain.TenantScoreSnapshot) error {
	if m.upsertErr != nil {
		return m.upsertErr
	}
	m.rows[m.key(s.TenantID, s.Day)] = *s
	return nil
}

func (m *memSnapshots) ListSince(_ context.Context, tenantID uuid.UUID, since time.Time) ([]domain.TenantScoreSnapshot, error) {
	var out []domain.TenantScoreSnapshot
	for _, r := range m.rows {
		if r.TenantID == tenantID && !domain.SnapshotDay(r.Day).Before(domain.SnapshotDay(since)) {
			out = append(out, r)
		}
	}
	// oldest first, as the repository orders
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].Day.Before(out[i].Day) {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out, nil
}

func (m *memSnapshots) put(t uuid.UUID, day string, v float64) {
	d, _ := time.Parse("2006-01-02", day)
	m.rows[m.key(t, d)] = domain.TenantScoreSnapshot{TenantID: t, Day: d, Value: v, Band: "medium"}
}

// measuredUseCase scores a tenant with two critical and three high risks, which
// is enough data for a measured result.
func measuredUseCase() *UseCase {
	rc := &stubRiskCounts{counts: map[string]int{"critical": 2, "high": 3, "medium": 1}}
	return New().WithRiskCounts(rc)
}

func fixedNow(day string) func() time.Time {
	d, _ := time.Parse("2006-01-02", day)
	return func() time.Time { return d.Add(15 * time.Hour) }
}

func TestScoreHistory_Success(t *testing.T) {
	store := newMemSnapshots()
	tenant := uuid.New()
	// Two readings in August (the later one closes the month), one in
	// September exactly 30 days before "today".
	store.put(tenant, "2026-08-03", 70)
	store.put(tenant, "2026-08-28", 66)
	store.put(tenant, "2026-09-08", 61)

	uc := measuredUseCase()
	uc.now = fixedNow("2026-10-08")
	h := NewHistory(uc, store)
	h.now = fixedNow("2026-10-08")

	out, err := h.Execute(context.Background(), tenant, 12)
	require.NoError(t, err)
	require.NotNil(t, out.Current, "today is recorded before the history is read")

	months := make([]string, 0, len(out.Points))
	for _, p := range out.Points {
		months = append(months, p.Month)
	}
	assert.Equal(t, []string{"2026-08", "2026-09", "2026-10"}, months)
	assert.Equal(t, 66.0, out.Points[0].Value, "a month reads as its last snapshot")
	require.NotNil(t, out.Delta30d)
	assert.InDelta(t, *out.Current-61, *out.Delta30d, 0.051)
	require.NotNil(t, out.Since)
	assert.Equal(t, "2026-08-03", out.Since.Format("2006-01-02"))
}

// A tenant with no history yet still gets today's point, but no delta: a
// movement needs two real measurements.
func TestScoreHistory_NotFound(t *testing.T) {
	store := newMemSnapshots()
	uc := measuredUseCase()
	h := NewHistory(uc, store)

	out, err := h.Execute(context.Background(), uuid.New(), 12)
	require.NoError(t, err)
	assert.Len(t, out.Points, 1)
	assert.NotNil(t, out.Current)
	assert.Nil(t, out.Delta30d)
}

// An unmeasured tenant stores nothing and reports no current value.
func TestScoreHistory_UnmeasuredStoresNothing(t *testing.T) {
	store := newMemSnapshots()
	uc, _, _, _, _ := emptyTenantUseCase()
	h := NewHistory(uc, store)

	out, err := h.Execute(context.Background(), uuid.New(), 12)
	require.NoError(t, err)
	assert.Empty(t, out.Points)
	assert.Nil(t, out.Current)
	assert.Empty(t, store.rows)
}

// No tenant, no history; and one tenant never reads another's snapshots.
func TestScoreHistory_Unauthorized(t *testing.T) {
	store := newMemSnapshots()
	h := NewHistory(measuredUseCase(), store)

	_, err := h.Execute(context.Background(), uuid.Nil, 12)
	assert.True(t, errors.Is(err, domain.ErrValidation))

	other, mine := uuid.New(), uuid.New()
	store.put(other, "2026-09-01", 90)
	out, err := h.Execute(context.Background(), mine, 12)
	require.NoError(t, err)
	for _, p := range out.Points {
		assert.NotEqual(t, 90.0, p.Value, "another tenant's snapshot leaked into the history")
	}
}

func TestScoreHistory_MonthsValidation(t *testing.T) {
	h := NewHistory(measuredUseCase(), newMemSnapshots())
	for _, m := range []int{0, 25, -1} {
		_, err := h.Execute(context.Background(), uuid.New(), m)
		assert.True(t, errors.Is(err, domain.ErrValidation), "months=%d", m)
	}
}

func TestScoreHistory_StoreFailureSurfaces(t *testing.T) {
	store := newMemSnapshots()
	store.upsertErr = errors.New("db down")
	h := NewHistory(measuredUseCase(), store)
	_, err := h.Execute(context.Background(), uuid.New(), 12)
	assert.Error(t, err)
}

type fixedFigures struct{ f Figures }

func (s fixedFigures) Figures(context.Context, uuid.UUID) Figures { return s.f }

// #903: today's snapshot carries the executive figures, and the history
// returns the last snapshot from before the current quarter as the baseline.
func TestScoreHistory_QuarterBaseline(t *testing.T) {
	store := newMemSnapshots()
	tenant := uuid.New()
	ale, crit, comp := 117.2e6, 2, 73.0
	store.put(tenant, "2026-08-14", 60)
	store.put(tenant, "2026-09-29", 57) // last reading of Q3
	store.put(tenant, "2026-10-02", 55) // already Q4: not the baseline
	d, _ := time.Parse("2006-01-02", "2026-09-29")
	r := store.rows[store.key(tenant, d)]
	oldALE, oldCrit := 129.6e6, 3
	r.ALEXAF, r.CriticalRisks = &oldALE, &oldCrit
	store.rows[store.key(tenant, d)] = r

	uc := measuredUseCase()
	uc.now = fixedNow("2026-10-08")
	h := NewHistory(uc, store).WithFigures(fixedFigures{Figures{ALEXAF: &ale, CriticalRisks: &crit, CompliancePct: &comp}})
	h.now = fixedNow("2026-10-08")

	out, err := h.Execute(context.Background(), tenant, 12)
	require.NoError(t, err)
	require.NotNil(t, out.QuarterBaseline)
	assert.Equal(t, "2026-Q3", out.QuarterBaseline.Quarter)
	assert.Equal(t, 57.0, out.QuarterBaseline.Value)
	assert.Equal(t, oldALE, *out.QuarterBaseline.ALEXAF)
	assert.Equal(t, oldCrit, *out.QuarterBaseline.CriticalRisks)
	assert.Nil(t, out.QuarterBaseline.CompliancePct, "a figure not read that day stays a gap")

	today, _ := time.Parse("2006-01-02", "2026-10-08")
	snap := store.rows[store.key(tenant, today)]
	require.NotNil(t, snap.ALEXAF)
	assert.Equal(t, ale, *snap.ALEXAF)
	assert.Equal(t, crit, *snap.CriticalRisks)
	assert.Equal(t, comp, *snap.CompliancePct)
}

// No snapshot before the quarter: no baseline, so no delta is invented.
func TestScoreHistory_NoQuarterBaseline(t *testing.T) {
	store := newMemSnapshots()
	tenant := uuid.New()
	store.put(tenant, "2026-10-02", 55)
	uc := measuredUseCase()
	uc.now = fixedNow("2026-10-08")
	h := NewHistory(uc, store)
	h.now = fixedNow("2026-10-08")
	out, err := h.Execute(context.Background(), tenant, 12)
	require.NoError(t, err)
	assert.Nil(t, out.QuarterBaseline)
}
