// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package risk

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/crq"
)

// --- score trend on the list (#913) -----------------------------------------

type fakeTrendReader struct {
	byTenant map[uuid.UUID]map[uuid.UUID]domain.TrendSnapshots
	calls    int
	cutoff   time.Time
	err      error
}

func (f *fakeTrendReader) TrendSnapshots(_ context.Context, tenantID uuid.UUID, ids []uuid.UUID, cutoff time.Time) (map[uuid.UUID]domain.TrendSnapshots, error) {
	f.calls++
	f.cutoff = cutoff
	if f.err != nil {
		return nil, f.err
	}
	out := map[uuid.UUID]domain.TrendSnapshots{}
	for _, id := range ids {
		if s, ok := f.byTenant[tenantID][id]; ok {
			out[id] = s
		}
	}
	return out, nil
}

func listOf(tenant uuid.UUID, scores ...float64) (*MockRiskRepository, []uuid.UUID) {
	risks := make([]domain.Risk, len(scores))
	ids := make([]uuid.UUID, len(scores))
	for i, s := range scores {
		ids[i] = uuid.New()
		risks[i] = domain.Risk{ID: ids[i], TenantID: tenant, Score: s}
	}
	repo := &MockRiskRepository{
		listFunc: func(_ context.Context, _ uuid.UUID, _ domain.RiskQuery) (*domain.PaginatedResult[domain.Risk], error) {
			return &domain.PaginatedResult[domain.Risk]{Data: risks, Total: int64(len(risks))}, nil
		},
	}
	return repo, ids
}

func TestListRisksTrends_Success(t *testing.T) {
	tenant := uuid.New()
	repo, ids := listOf(tenant, 8, 3, 4.4, 5)
	at := 6.0
	reader := &fakeTrendReader{byTenant: map[uuid.UUID]map[uuid.UUID]domain.TrendSnapshots{
		tenant: {
			ids[0]: {AtCutoff: &at, Oldest: 1},  // 8 vs 6   -> up
			ids[1]: {Oldest: 5},                 // 3 vs 5   -> down
			ids[2]: {Oldest: 4},                 // 4.4 vs 4 -> flat
			ids[3]: {AtCutoff: &at, Oldest: 20}, // 5 vs 6   -> down (cutoff beats oldest)
		},
	}}
	now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	uc := NewListRisksUseCase(repo).WithTrends(reader)
	uc.now = func() time.Time { return now }

	res, err := uc.Execute(context.Background(), tenant, domain.NewRiskQuery())
	require.NoError(t, err)

	want := []domain.RiskTrend{domain.RiskTrendUp, domain.RiskTrendDown, domain.RiskTrendFlat, domain.RiskTrendDown}
	for i, w := range want {
		require.NotNil(t, res.Data[i].ScoreTrend, "row %d", i)
		assert.Equal(t, w, *res.Data[i].ScoreTrend, "row %d", i)
	}
	assert.Equal(t, 1, reader.calls, "ONE batch read for the page, not one per row")
	assert.Equal(t, domain.TrendBaselineCutoff(now), reader.cutoff)
}

func TestListRisksTrends_NotFound(t *testing.T) {
	// No snapshot -> null, which is not "flat".
	tenant := uuid.New()
	repo, _ := listOf(tenant, 8)
	reader := &fakeTrendReader{}
	res, err := NewListRisksUseCase(repo).WithTrends(reader).Execute(context.Background(), tenant, domain.NewRiskQuery())
	require.NoError(t, err)
	assert.Nil(t, res.Data[0].ScoreTrend)

	// Without a reader the trend stays null too.
	res, err = NewListRisksUseCase(repo).Execute(context.Background(), tenant, domain.NewRiskQuery())
	require.NoError(t, err)
	assert.Nil(t, res.Data[0].ScoreTrend)

	// A failing read degrades to null, the list still answers.
	failing := &fakeTrendReader{err: errors.New("boom")}
	res, err = NewListRisksUseCase(repo).WithTrends(failing).Execute(context.Background(), tenant, domain.NewRiskQuery())
	require.NoError(t, err)
	assert.Nil(t, res.Data[0].ScoreTrend)
}

func TestListRisksTrends_Unauthorized(t *testing.T) {
	// A nil tenant never reaches the snapshot reader.
	repo, ids := listOf(uuid.Nil, 8)
	reader := &fakeTrendReader{byTenant: map[uuid.UUID]map[uuid.UUID]domain.TrendSnapshots{
		uuid.Nil: {ids[0]: {Oldest: 1}},
	}}
	res, err := NewListRisksUseCase(repo).WithTrends(reader).Execute(context.Background(), uuid.Nil, domain.NewRiskQuery())
	require.NoError(t, err)
	assert.Zero(t, reader.calls)
	assert.Nil(t, res.Data[0].ScoreTrend)
}

func TestListRisksTrends_CrossTenant(t *testing.T) {
	// B's snapshots for a risk id that A's page happens to carry are not used.
	a, b := uuid.New(), uuid.New()
	repo, ids := listOf(a, 8)
	reader := &fakeTrendReader{byTenant: map[uuid.UUID]map[uuid.UUID]domain.TrendSnapshots{
		b: {ids[0]: {Oldest: 1}},
	}}
	res, err := NewListRisksUseCase(repo).WithTrends(reader).Execute(context.Background(), a, domain.NewRiskQuery())
	require.NoError(t, err)
	assert.Nil(t, res.Data[0].ScoreTrend)
}

// --- GET /risks/summary ------------------------------------------------------

type fakeSummaryRepo struct {
	// per tenant data: a second tenant's numbers must never appear in the first's.
	views    map[uuid.UUID]map[domain.RiskView]int64
	bands    map[uuid.UUID]map[domain.CriticalityLevel]int64
	exposure map[uuid.UUID][]domain.RiskExposureInput

	gotView      domain.RiskView
	gotSearch    string
	gotUser      uuid.UUID
	exposureRead bool
	err          error
}

func (f *fakeSummaryRepo) ViewCounts(_ context.Context, tenant, user uuid.UUID) (map[domain.RiskView]int64, error) {
	f.gotUser = user
	return f.views[tenant], f.err
}

func (f *fakeSummaryRepo) CriticalityCounts(_ context.Context, tenant, _ uuid.UUID, view domain.RiskView, search string) (map[domain.CriticalityLevel]int64, error) {
	f.gotView, f.gotSearch = view, search
	return f.bands[tenant], f.err
}

func (f *fakeSummaryRepo) CountActive(_ context.Context, tenant uuid.UUID) (int64, error) {
	return int64(len(f.exposure[tenant])), f.err
}

func (f *fakeSummaryRepo) ActiveExposureInputs(_ context.Context, tenant uuid.UUID) ([]domain.RiskExposureInput, error) {
	f.exposureRead = true
	return f.exposure[tenant], f.err
}

type fakeEntitlement struct {
	allowed bool
	err     error
}

func (f fakeEntitlement) HasFinancialQuantification(context.Context, uuid.UUID) (bool, error) {
	return f.allowed, f.err
}

func f64(v float64) *float64 { return &v }

func summaryFixture() (*fakeSummaryRepo, uuid.UUID, uuid.UUID) {
	a, b := uuid.New(), uuid.New()
	repo := &fakeSummaryRepo{
		views: map[uuid.UUID]map[domain.RiskView]int64{
			a: {domain.RiskViewAll: 3, domain.RiskViewMine: 1, domain.RiskViewHot: 2, domain.RiskViewReviewDue: 1, domain.RiskViewAccepted: 0},
			b: {domain.RiskViewAll: 99},
		},
		bands: map[uuid.UUID]map[domain.CriticalityLevel]int64{
			a: {domain.CriticalityCriticalNew: 1, domain.CriticalityHighNew: 1, domain.CriticalityMediumNew: 1, domain.CriticalityLowNew: 0},
			b: {domain.CriticalityCriticalNew: 99},
		},
		exposure: map[uuid.UUID][]domain.RiskExposureInput{
			a: {
				{SLEXAF: f64(1000), ARO: f64(2), Criticality: domain.CriticalityHighNew}, // explicit: 2000
				{Criticality: domain.CriticalityCriticalNew},                             // reference
				{Criticality: domain.CriticalityLevel("LOW")},                            // legacy upper-case, reference
			},
			b: {{SLEXAF: f64(1e9), ARO: f64(1)}},
		},
	}
	return repo, a, b
}

func TestRiskSummary_Success(t *testing.T) {
	repo, a, _ := summaryFixture()
	q := crq.NewQuantifier(600, crq.DefaultReference())
	uc := NewGetRiskSummaryUseCase(repo, q, fakeEntitlement{allowed: true})
	me := uuid.New()

	got, err := uc.Execute(context.Background(), a, me, domain.RiskViewHot, "log")
	require.NoError(t, err)

	assert.Equal(t, int64(3), got.Views[domain.RiskViewAll])
	assert.Equal(t, int64(2), got.Views[domain.RiskViewHot])
	assert.Equal(t, int64(1), got.ByCriticality[domain.CriticalityCriticalNew])
	assert.Equal(t, int64(3), got.Active.Count)

	// The sum is what the list computes per row, with the same quantifier.
	want := 2000.0
	ref := crq.DefaultReference()
	want += ref.For("critical") + ref.For("low")
	require.NotNil(t, got.Active.ALEXAF)
	assert.InDelta(t, want, *got.Active.ALEXAF, 0.001)

	assert.Equal(t, domain.RiskViewHot, repo.gotView, "bands are counted inside the selected view")
	assert.Equal(t, "log", repo.gotSearch, "and inside the search")
	assert.Equal(t, me, repo.gotUser, "the user comes from the caller, to resolve mine")
}

func TestRiskSummary_NotFound(t *testing.T) {
	// There is no object to be missing here: a tenant with an empty register
	// gets a well-formed summary of zeros, not an error.
	empty := uuid.New()
	repo := &fakeSummaryRepo{
		views: map[uuid.UUID]map[domain.RiskView]int64{empty: {}},
		bands: map[uuid.UUID]map[domain.CriticalityLevel]int64{empty: {}},
	}
	uc := NewGetRiskSummaryUseCase(repo, crq.NewQuantifier(600, crq.DefaultReference()), fakeEntitlement{allowed: true})

	got, err := uc.Execute(context.Background(), empty, uuid.New(), "", "")
	require.NoError(t, err)
	assert.Zero(t, got.Active.Count)
	require.NotNil(t, got.Active.ALEXAF)
	assert.Zero(t, *got.Active.ALEXAF, "entitled and empty is a real zero")
	assert.Equal(t, domain.RiskViewAll, repo.gotView, "no view means all")
}

func TestRiskSummary_Unauthorized(t *testing.T) {
	repo, _, _ := summaryFixture()
	uc := NewGetRiskSummaryUseCase(repo, crq.NewQuantifier(600, crq.DefaultReference()), fakeEntitlement{allowed: true})

	got, err := uc.Execute(context.Background(), uuid.Nil, uuid.New(), domain.RiskViewAll, "")
	require.Error(t, err)
	assert.Nil(t, got)
	assert.ErrorIs(t, err, domain.ErrUnauthorized)
	assert.Equal(t, 401, domain.HTTPStatusFromError(err))
	assert.False(t, repo.exposureRead, "nothing is read for a nil tenant")
}

func TestRiskSummary_CrossTenant(t *testing.T) {
	repo, a, b := summaryFixture()
	uc := NewGetRiskSummaryUseCase(repo, crq.NewQuantifier(600, crq.DefaultReference()), fakeEntitlement{allowed: true})

	got, err := uc.Execute(context.Background(), a, uuid.New(), domain.RiskViewAll, "")
	require.NoError(t, err)
	assert.NotEqual(t, int64(99), got.Views[domain.RiskViewAll])
	assert.NotEqual(t, int64(99), got.ByCriticality[domain.CriticalityCriticalNew])
	assert.Less(t, *got.Active.ALEXAF, 1e9, "tenant B's exposure is not in tenant A's sum")

	gotB, err := uc.Execute(context.Background(), b, uuid.New(), domain.RiskViewAll, "")
	require.NoError(t, err)
	assert.Equal(t, int64(99), gotB.Views[domain.RiskViewAll])
}

func TestRiskSummary_ALEIsNullWithoutTheEntitlement(t *testing.T) {
	q := crq.NewQuantifier(600, crq.DefaultReference())
	cases := []struct {
		name string
		uc   func(*fakeSummaryRepo) *GetRiskSummaryUseCase
	}{
		{"plan without financial quantification", func(r *fakeSummaryRepo) *GetRiskSummaryUseCase {
			return NewGetRiskSummaryUseCase(r, q, fakeEntitlement{allowed: false})
		}},
		{"entitlement unreadable fails closed", func(r *fakeSummaryRepo) *GetRiskSummaryUseCase {
			return NewGetRiskSummaryUseCase(r, q, fakeEntitlement{allowed: true, err: errors.New("down")})
		}},
		{"no quantifier", func(r *fakeSummaryRepo) *GetRiskSummaryUseCase {
			return NewGetRiskSummaryUseCase(r, nil, fakeEntitlement{allowed: true})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo, a, _ := summaryFixture()
			got, err := tc.uc(repo).Execute(context.Background(), a, uuid.New(), domain.RiskViewAll, "")
			require.NoError(t, err)
			assert.Nil(t, got.Active.ALEXAF, "null, never 0")
			assert.Equal(t, int64(3), got.Active.Count, "the count is not an entitlement")
			assert.False(t, repo.exposureRead, "the ALE inputs are not even read")
		})
	}

	// No plan gate at all (a deployment without the entitlement service) serves the figure.
	repo, a, _ := summaryFixture()
	got, err := NewGetRiskSummaryUseCase(repo, q, nil).Execute(context.Background(), a, uuid.New(), domain.RiskViewAll, "")
	require.NoError(t, err)
	assert.NotNil(t, got.Active.ALEXAF)
}

func TestRiskSummary_RepositoryErrorsAreInternal(t *testing.T) {
	repo, a, _ := summaryFixture()
	repo.err = errors.New("db down")
	uc := NewGetRiskSummaryUseCase(repo, crq.NewQuantifier(600, crq.DefaultReference()), fakeEntitlement{allowed: true})
	_, err := uc.Execute(context.Background(), a, uuid.New(), domain.RiskViewAll, "")
	require.Error(t, err)
	assert.Equal(t, 500, domain.HTTPStatusFromError(err))
	assert.NotContains(t, err.Error(), "db down", "internals stay out of the message")
}
