// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package asset

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListAssetSnapshots_Success(t *testing.T) {
	tenantID := uuid.New()
	assetID := uuid.New()
	repo := &MockAssetRepository{
		getByIDFunc: func(ctx context.Context, id, tid uuid.UUID) (*domain.Asset, error) {
			return &domain.Asset{ID: id, TenantID: tid}, nil
		},
		listSnapshotsFunc: func(ctx context.Context, aid, tid uuid.UUID) ([]domain.AssetSnapshot, error) {
			assert.Equal(t, assetID, aid)
			assert.Equal(t, tenantID, tid)
			return []domain.AssetSnapshot{{Criticality: domain.CriticalityLow}, {Criticality: domain.CriticalityHigh}}, nil
		},
	}
	uc := NewListAssetSnapshotsUseCase(repo)

	got, err := uc.Execute(context.Background(), tenantID, assetID)

	require.NoError(t, err)
	assert.Len(t, got, 2)
}

func TestListAssetSnapshots_AssetNotFound(t *testing.T) {
	repo := &MockAssetRepository{
		getByIDFunc: func(ctx context.Context, id, tid uuid.UUID) (*domain.Asset, error) {
			return nil, nil
		},
	}
	uc := NewListAssetSnapshotsUseCase(repo)

	_, err := uc.Execute(context.Background(), uuid.New(), uuid.New())

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestListAssetSnapshots_CrossTenantReturnsNotFound(t *testing.T) {
	repo := &MockAssetRepository{
		getByIDFunc: func(ctx context.Context, id, tid uuid.UUID) (*domain.Asset, error) {
			return nil, nil
		},
	}
	uc := NewListAssetSnapshotsUseCase(repo)

	_, err := uc.Execute(context.Background(), uuid.New(), uuid.New())

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

// mockUserLookup is a stub assetapp.UserLookup that records the IDs it was
// asked to resolve and returns a fixed email map (or an error).
type mockUserLookup struct {
	emails    map[uuid.UUID]string
	err       error
	askedWith []uuid.UUID
}

func (m *mockUserLookup) EmailsByIDs(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	m.askedWith = ids
	if m.err != nil {
		return nil, m.err
	}
	return m.emails, nil
}

func TestListAssetSnapshots_ResolvesActorEmails(t *testing.T) {
	alice, bob := uuid.New(), uuid.New()
	repo := &MockAssetRepository{
		getByIDFunc: func(ctx context.Context, id, tid uuid.UUID) (*domain.Asset, error) {
			return &domain.Asset{ID: id, TenantID: tid}, nil
		},
		listSnapshotsFunc: func(ctx context.Context, aid, tid uuid.UUID) ([]domain.AssetSnapshot, error) {
			return []domain.AssetSnapshot{
				{ChangedBy: alice, Reason: "update"},
				{ChangedBy: bob, Reason: "update"},
				{ChangedBy: alice, Reason: "delete"}, // duplicate actor
				{ChangedBy: uuid.Nil, Reason: "update"}, // system/legacy — skipped
			}, nil
		},
	}
	lookup := &mockUserLookup{emails: map[uuid.UUID]string{alice: "alice@acme.io", bob: "bob@acme.io"}}
	uc := NewListAssetSnapshotsUseCase(repo).WithUserLookup(lookup)

	got, err := uc.Execute(context.Background(), uuid.New(), uuid.New())

	require.NoError(t, err)
	require.Len(t, got, 4)
	assert.Equal(t, "alice@acme.io", got[0].ChangedByEmail)
	assert.Equal(t, "bob@acme.io", got[1].ChangedByEmail)
	assert.Equal(t, "alice@acme.io", got[2].ChangedByEmail)
	assert.Empty(t, got[3].ChangedByEmail, "nil actor must not resolve to an email")
	// Distinct, non-nil IDs only (alice once, bob once).
	assert.ElementsMatch(t, []uuid.UUID{alice, bob}, lookup.askedWith)
}

func TestListAssetSnapshots_LookupErrorDegradesGracefully(t *testing.T) {
	actor := uuid.New()
	repo := &MockAssetRepository{
		getByIDFunc: func(ctx context.Context, id, tid uuid.UUID) (*domain.Asset, error) {
			return &domain.Asset{ID: id, TenantID: tid}, nil
		},
		listSnapshotsFunc: func(ctx context.Context, aid, tid uuid.UUID) ([]domain.AssetSnapshot, error) {
			return []domain.AssetSnapshot{{ChangedBy: actor, Reason: "update"}}, nil
		},
	}
	lookup := &mockUserLookup{err: assert.AnError}
	uc := NewListAssetSnapshotsUseCase(repo).WithUserLookup(lookup)

	got, err := uc.Execute(context.Background(), uuid.New(), uuid.New())

	require.NoError(t, err, "a failed email lookup must never fail the history request")
	require.Len(t, got, 1)
	assert.Empty(t, got[0].ChangedByEmail)
	assert.Equal(t, actor, got[0].ChangedBy, "raw actor UUID is still available")
}
