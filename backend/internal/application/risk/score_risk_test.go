// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package risk

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	pkgscoring "github.com/opendefender/openrisk/pkg/scoring"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeAssetStore is an in-memory RiskAssetStore holding assets of several
// tenants, recording what the use case asked it to write.
type fakeAssetStore struct {
	assets  []*domain.Asset
	saved   *domain.Risk
	linked  []*domain.Asset
	created bool
	calls   int
}

func (f *fakeAssetStore) FindByIDs(_ context.Context, tenantID uuid.UUID, ids []uuid.UUID) ([]*domain.Asset, error) {
	out := []*domain.Asset{}
	for _, id := range ids {
		for _, a := range f.assets {
			if a.ID == id && a.TenantID == tenantID {
				out = append(out, a)
			}
		}
	}
	return out, nil
}

func (f *fakeAssetStore) SaveWithAssets(_ context.Context, r *domain.Risk, assets []*domain.Asset, create bool) error {
	f.calls++
	f.saved, f.linked, f.created = r, assets, create
	return nil
}

func (f *fakeAssetStore) add(tenant uuid.UUID, crit domain.AssetCriticality) uuid.UUID {
	a := &domain.Asset{ID: uuid.New(), TenantID: tenant, Name: string(crit), Criticality: crit}
	f.assets = append(f.assets, a)
	return a.ID
}

// assertConsistent runs GET /risks/:id/score-working's use case on the risk as
// stored and requires it to agree with the stored score — the #792 contract.
func assertConsistent(t *testing.T, tenant uuid.UUID, stored *domain.Risk) {
	t.Helper()
	w, err := NewGetScoreWorkingUseCase(repoReturning(stored), nil, pkgscoring.NewEngine()).
		Execute(context.Background(), tenant, stored.ID, false)
	require.NoError(t, err)
	assert.True(t, w.Consistent, "stored %.3f, working computes %.3f", w.Stored, w.Computed)
}

func TestCreateRisk_ScoresWithAssetCriticality(t *testing.T) {
	tenant := uuid.New()
	cases := []struct {
		name  string
		crits []domain.AssetCriticality
		want  float64
	}{
		// P=0.5, I=2 throughout: the issue's live case is the "one critical" row.
		{"zero assets", nil, 1.0},
		{"one critical asset", []domain.AssetCriticality{domain.CriticalityCritical}, 3.0},
		{"several assets", []domain.AssetCriticality{domain.CriticalityLow, domain.CriticalityHigh, domain.CriticalityCritical}, 2.0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &fakeAssetStore{}
			var ids []uuid.UUID
			for _, c := range tc.crits {
				ids = append(ids, store.add(tenant, c))
			}
			var repoCreated *domain.Risk
			repo := &MockRiskRepository{createFunc: func(_ context.Context, r *domain.Risk) error {
				repoCreated = r
				return nil
			}}

			got, err := NewCreateRiskUseCase(repo).WithAssets(store).Execute(context.Background(), tenant, CreateRiskInput{
				Title: "t", Probability: 0.5, Impact: 2, AssetIDs: ids,
			})
			require.NoError(t, err)

			assert.InDelta(t, tc.want, got.Score, 1e-9)
			assert.Equal(t, domain.CriticalityFromScore(tc.want), got.Criticality)
			if len(ids) == 0 {
				assert.Same(t, got, repoCreated, "no asset: plain create")
				assert.Zero(t, store.calls)
			} else {
				assert.Nil(t, repoCreated, "links and row are written together")
				require.Equal(t, 1, store.calls)
				assert.True(t, store.created)
				assert.Len(t, store.linked, len(ids))
			}
			assertConsistent(t, tenant, got)
		})
	}
}

func TestCreateRisk_ForeignAssetIsNotLinkedNorScored(t *testing.T) {
	tenant, other := uuid.New(), uuid.New()
	store := &fakeAssetStore{}
	foreign := store.add(other, domain.CriticalityCritical)

	got, err := NewCreateRiskUseCase(&MockRiskRepository{}).WithAssets(store).Execute(context.Background(), tenant, CreateRiskInput{
		Title: "t", Probability: 0.5, Impact: 2, AssetIDs: []uuid.UUID{foreign},
	})
	require.NoError(t, err)
	assert.Empty(t, got.Assets)
	assert.InDelta(t, 1.0, got.Score, 1e-9, "another tenant's asset must not weigh on the score")
	assert.Zero(t, store.calls)
}

func TestUpdateRisk_ScoresWithAssetCriticality(t *testing.T) {
	tenant := uuid.New()
	cases := []struct {
		name     string
		existing []domain.AssetCriticality // links already on the row
		relink   []domain.AssetCriticality // nil = the update sends no asset_ids
		want     float64
	}{
		{"zero assets", nil, nil, 1.0},
		{"one asset already linked, edit P only", []domain.AssetCriticality{domain.CriticalityCritical}, nil, 3.0},
		{"several assets sent", nil, []domain.AssetCriticality{domain.CriticalityLow, domain.CriticalityHigh, domain.CriticalityCritical}, 2.0},
		{"links replaced", []domain.AssetCriticality{domain.CriticalityCritical}, []domain.AssetCriticality{domain.CriticalityLow}, 0.5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &fakeAssetStore{}
			existing := &domain.Risk{ID: uuid.New(), TenantID: tenant, Title: "t", Probability: 0.9, Impact: 2, Score: 1.8}
			for _, c := range tc.existing {
				existing.Assets = append(existing.Assets, &domain.Asset{ID: uuid.New(), TenantID: tenant, Criticality: c})
			}
			var relinkIDs []uuid.UUID
			if tc.relink != nil {
				relinkIDs = []uuid.UUID{}
				for _, c := range tc.relink {
					relinkIDs = append(relinkIDs, store.add(tenant, c))
				}
			}
			repoUpdated := false
			repo := &MockRiskRepository{
				getByIDFunc: func(_ context.Context, id, tid uuid.UUID) (*domain.Risk, error) {
					if id == existing.ID && tid == tenant {
						return existing, nil
					}
					return nil, nil
				},
				updateFunc: func(context.Context, *domain.Risk) error { repoUpdated = true; return nil },
			}
			p := 0.5

			got, err := NewUpdateRiskUseCase(repo).WithAssets(store).Execute(context.Background(), tenant, existing.ID, UpdateRiskInput{
				Probability: &p, AssetIDs: relinkIDs,
			})
			require.NoError(t, err)

			assert.InDelta(t, tc.want, got.Score, 1e-9)
			assert.Equal(t, domain.CriticalityFromScore(tc.want), got.Criticality)
			if tc.relink == nil {
				assert.True(t, repoUpdated)
				assert.Zero(t, store.calls)
			} else {
				assert.False(t, repoUpdated)
				require.Equal(t, 1, store.calls)
				assert.False(t, store.created)
				assert.Len(t, store.linked, len(tc.relink))
			}
			assertConsistent(t, tenant, got)
		})
	}
}

func TestUpdateRisk_WithAssets_NotFound(t *testing.T) {
	store := &fakeAssetStore{}
	repo := &MockRiskRepository{getByIDFunc: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Risk, error) { return nil, nil }}

	_, err := NewUpdateRiskUseCase(repo).WithAssets(store).Execute(context.Background(), uuid.New(), uuid.New(), UpdateRiskInput{
		AssetIDs: []uuid.UUID{store.add(uuid.New(), domain.CriticalityHigh)},
	})
	assert.True(t, errors.Is(err, domain.ErrNotFound), "got %v", err)
	assert.Zero(t, store.calls)
}

// Another tenant's risk reads as not found through the tenant-scoped GetByID,
// and nothing — links included — is written.
func TestUpdateRisk_WithAssets_Unauthorized(t *testing.T) {
	owner, intruder := uuid.New(), uuid.New()
	store := &fakeAssetStore{}
	riskID := uuid.New()
	repo := &MockRiskRepository{getByIDFunc: func(_ context.Context, id, tid uuid.UUID) (*domain.Risk, error) {
		if tid == owner {
			return &domain.Risk{ID: id, TenantID: owner, Title: "t"}, nil
		}
		return nil, nil
	}}

	_, err := NewUpdateRiskUseCase(repo).WithAssets(store).Execute(context.Background(), intruder, riskID, UpdateRiskInput{
		AssetIDs: []uuid.UUID{store.add(intruder, domain.CriticalityCritical)},
	})
	assert.True(t, errors.Is(err, domain.ErrNotFound), "got %v", err)
	assert.Zero(t, store.calls)
}

func TestRiskAssetCriticality(t *testing.T) {
	assert.Equal(t, domain.NoAssetCriticalityFactor, domain.RiskAssetCriticality(nil))
	assert.InDelta(t, 3.0, domain.RiskAssetCriticality([]domain.AssetCriticality{domain.CriticalityCritical}), 1e-9)
	assert.InDelta(t, 2.0, domain.RiskAssetCriticality([]domain.AssetCriticality{
		domain.CriticalityLow, domain.CriticalityHigh, domain.CriticalityCritical,
	}), 1e-9)
}
