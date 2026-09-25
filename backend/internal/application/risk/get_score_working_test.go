// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package risk

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	pkgscoring "github.com/opendefender/openrisk/pkg/scoring"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeAuditTrail serves pre-sealed entries per (entity_type, entity_id), newest
// first, and records every tenant it was asked about.
type fakeAuditTrail struct {
	byEntity map[string][]domain.AuditEvent
	tenants  []uuid.UUID
}

func (f *fakeAuditTrail) Append(context.Context, *domain.AuditEvent) error { return nil }
func (f *fakeAuditTrail) List(_ context.Context, tenantID uuid.UUID, flt domain.AuditEventFilter) ([]domain.AuditEvent, int64, error) {
	f.tenants = append(f.tenants, tenantID)
	var out []domain.AuditEvent
	for _, t := range flt.EntityTypes {
		out = append(out, f.byEntity[t+"/"+flt.EntityID]...)
	}
	return out, int64(len(out)), nil
}

type fakeEmails map[uuid.UUID]string

func (f fakeEmails) EmailsByIDs(context.Context, []uuid.UUID) (map[uuid.UUID]string, error) {
	return f, nil
}

func sealed(tenant uuid.UUID, seq int64, ev domain.AuditEvent) domain.AuditEvent {
	ev.TenantID = tenant
	ev.CreatedAt = time.Date(2026, 9, 20, 10, int(seq), 0, 0, time.UTC)
	ev.SealChain(seq, "")
	return ev
}

func scoreWorkingFixture(t *testing.T) (uuid.UUID, *domain.Risk, *fakeAuditTrail, uuid.UUID) {
	t.Helper()
	tenant, analyst := uuid.New(), uuid.New()
	asset := domain.Asset{ID: uuid.New(), TenantID: tenant, Name: "Core banking", Criticality: domain.CriticalityCritical}
	r := &domain.Risk{
		ID: uuid.New(), TenantID: tenant, Title: "Ransomware",
		Probability: 0.5, Impact: 4, Score: 6.0, Criticality: "high",
		Assets: []*domain.Asset{&asset},
	}
	rk, ak := "risk/"+r.ID.String(), "asset/"+asset.ID.String()
	trail := &fakeAuditTrail{byEntity: map[string][]domain.AuditEvent{
		rk: {
			sealed(tenant, 3, *domain.ScoreEngineAuditEvent(tenant, r.ID, 4.0, domain.ScoreEngineTerms{
				Probability: 0.5, Impact: 4, AssetCriticality: 3, Score: 6, Criticality: "high",
			})),
			sealed(tenant, 2, domain.AuditEvent{ID: uuid.New(), ActorID: &analyst, ActorType: domain.AuditActorUser,
				Action: domain.AuditActionUpdate, EntityType: "risk", EntityID: r.ID.String(),
				Before: domain.JSONMap{"impact": 2.0}, After: domain.JSONMap{"impact": 4.0, "probability": 0.5},
				ChangedFields: domain.StringList{"impact"}}),
			sealed(tenant, 1, domain.AuditEvent{ID: uuid.New(), ActorID: &analyst, ActorType: domain.AuditActorUser,
				Action: domain.AuditActionCreate, EntityType: "risk", EntityID: r.ID.String(),
				After: domain.JSONMap{"impact": 2.0, "probability": 0.5}}),
		},
		ak: {
			sealed(tenant, 0, domain.AuditEvent{ID: uuid.New(), ActorID: &analyst, Action: domain.AuditActionUpdate,
				EntityType: "asset", EntityID: asset.ID.String(),
				Before: domain.JSONMap{"criticality": "HIGH"}, After: domain.JSONMap{"criticality": "CRITICAL"},
				ChangedFields: domain.StringList{"criticality"}}),
		},
	}}
	return tenant, r, trail, analyst
}

func repoReturning(r *domain.Risk) *MockRiskRepository {
	return &MockRiskRepository{getByIDFunc: func(_ context.Context, id, tenantID uuid.UUID) (*domain.Risk, error) {
		if id == r.ID && tenantID == r.TenantID {
			return r, nil
		}
		return nil, nil
	}}
}

func TestGetScoreWorking_Success(t *testing.T) {
	tenant, r, trail, analyst := scoreWorkingFixture(t)
	uc := NewGetScoreWorkingUseCase(repoReturning(r), trail, pkgscoring.NewEngine()).
		WithUserLookup(fakeEmails{analyst: "analyst@bank.example"})

	w, err := uc.Execute(context.Background(), tenant, r.ID, true)
	require.NoError(t, err)

	// The arithmetic: 0.5 × 4 × 3.0 (one critical asset) = 6.0, which is what is stored.
	assert.Equal(t, domain.ScoreWorkingFormula, w.Formula)
	require.Len(t, w.Terms, 3)
	assert.InDelta(t, 3.0, w.Terms[2].Value, 1e-9)
	assert.InDelta(t, 6.0, w.Computed, 1e-9)
	assert.True(t, w.Consistent)
	assert.True(t, w.SourcesVisible)
	assert.False(t, w.AssetCriticalityDefaulted)

	// Each term cites the entry that set it.
	require.NotNil(t, w.Terms[1].Source, "impact must cite its entry")
	assert.Equal(t, int64(2), w.Terms[1].Source.Sequence)
	assert.Equal(t, "analyst@bank.example", w.Terms[1].Source.ActorEmail)
	assert.True(t, w.Terms[1].Source.HashValid)
	require.NotNil(t, w.Terms[0].Source, "probability falls back to the create")
	assert.Equal(t, int64(1), w.Terms[0].Source.Sequence)
	require.NotNil(t, w.Terms[2].Source, "with one asset its criticality entry is the term's source")
	assert.Equal(t, "CRITICAL", w.Terms[2].Source.Value)
	require.NotNil(t, w.ScoreSource)
	assert.Equal(t, domain.AuditActorJob, w.ScoreSource.ActorType)
	assert.Equal(t, domain.AuditJobScoreEngine, w.ScoreSource.ActorLabel)

	// Every audit read stayed inside the caller's tenant.
	for _, tn := range trail.tenants {
		assert.Equal(t, tenant, tn)
	}
}

func TestGetScoreWorking_TamperedSourceIsFlagged(t *testing.T) {
	tenant, r, trail, _ := scoreWorkingFixture(t)
	rk := "risk/" + r.ID.String()
	// Someone edits the stored after-image of the impact entry in the database.
	trail.byEntity[rk][1].After["impact"] = 9.0

	w, err := NewGetScoreWorkingUseCase(repoReturning(r), trail, pkgscoring.NewEngine()).
		Execute(context.Background(), tenant, r.ID, true)
	require.NoError(t, err)
	require.NotNil(t, w.Terms[1].Source)
	assert.False(t, w.Terms[1].Source.HashValid, "an edited source entry must not verify")
}

func TestGetScoreWorking_InconsistentStoredScoreIsSaidSo(t *testing.T) {
	tenant, r, trail, _ := scoreWorkingFixture(t)
	r.Score = 2.0 // e.g. a writer that left out asset criticality
	w, err := NewGetScoreWorkingUseCase(repoReturning(r), trail, pkgscoring.NewEngine()).
		Execute(context.Background(), tenant, r.ID, true)
	require.NoError(t, err)
	assert.False(t, w.Consistent)
	assert.InDelta(t, 2.0, w.Stored, 1e-9)
	assert.InDelta(t, 6.0, w.Computed, 1e-9)
}

func TestGetScoreWorking_NoAssetUsesTheDocumentedDefault(t *testing.T) {
	tenant, r, trail, _ := scoreWorkingFixture(t)
	r.Assets = nil
	w, err := NewGetScoreWorkingUseCase(repoReturning(r), trail, pkgscoring.NewEngine()).
		Execute(context.Background(), tenant, r.ID, true)
	require.NoError(t, err)
	assert.True(t, w.AssetCriticalityDefaulted)
	assert.InDelta(t, domain.CriticalityMedium.ScoreFactor(), w.Terms[2].Value, 1e-9)
	assert.Nil(t, w.Terms[2].Source)
}

func TestGetScoreWorking_NotFound(t *testing.T) {
	tenant, r, trail, _ := scoreWorkingFixture(t)
	uc := NewGetScoreWorkingUseCase(repoReturning(r), trail, pkgscoring.NewEngine())

	_, err := uc.Execute(context.Background(), tenant, uuid.New(), true)
	assert.ErrorIs(t, err, domain.ErrNotFound)

	// Another tenant asking for this risk's id gets the same answer.
	_, err = uc.Execute(context.Background(), uuid.New(), r.ID, true)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestGetScoreWorking_Unauthorized(t *testing.T) {
	tenant, r, trail, _ := scoreWorkingFixture(t)
	uc := NewGetScoreWorkingUseCase(repoReturning(r), trail, pkgscoring.NewEngine())

	// Without the audit-read permission: the arithmetic, and no provenance.
	w, err := uc.Execute(context.Background(), tenant, r.ID, false)
	require.NoError(t, err)
	assert.False(t, w.SourcesVisible)
	assert.InDelta(t, 6.0, w.Computed, 1e-9)
	for _, term := range w.Terms {
		assert.Nil(t, term.Source)
	}
	assert.Nil(t, w.ScoreSource)
	assert.Empty(t, trail.tenants, "the trail must not even be read")

	// No tenant at all is refused outright.
	_, err = uc.Execute(context.Background(), uuid.Nil, r.ID, true)
	assert.ErrorIs(t, err, domain.ErrForbidden)
}
