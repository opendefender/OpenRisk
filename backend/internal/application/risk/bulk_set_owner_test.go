// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package risk

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
)

// #913 — bulk set_owner writes owner_id (the register's Owner column), never
// the legacy assigned_to that assign_to writes.

// membersOnly is an OwnershipManager with a fixed member set per tenant, like
// the real service: a user outside the tenant is a validation error.
type membersOnly struct {
	members  map[uuid.UUID]map[uuid.UUID]bool
	notified []domain.OwnershipChange
}

func (m *membersOnly) Apply(_ context.Context, tenant uuid.UUID, block *domain.Ownership, patch domain.OwnershipPatch, actor uuid.UUID) ([]domain.OwnershipChange, error) {
	for _, id := range patch.CandidateIDs() {
		if !m.members[tenant][id] {
			return nil, domain.NewValidationError("user is not a member of this organization")
		}
	}
	before := *block
	patch.Apply(block)
	return domain.DiffOwnership(before, *block, actor), nil
}

func (m *membersOnly) Notify(_ context.Context, _ uuid.UUID, changes []domain.OwnershipChange, _ domain.OwnershipSubject) {
	m.notified = append(m.notified, changes...)
}

type batchNotices struct{ got []domain.Notice }

func (b *batchNotices) Deliver(_ context.Context, n domain.Notice) error {
	b.got = append(b.got, n)
	return nil
}

func ownerFixture() (tenant, other, newOwner, actor uuid.UUID, mgr *membersOnly) {
	tenant, other, newOwner, actor = uuid.New(), uuid.New(), uuid.New(), uuid.New()
	mgr = &membersOnly{members: map[uuid.UUID]map[uuid.UUID]bool{
		tenant: {newOwner: true, actor: true},
		other:  {uuid.New(): true},
	}}
	return
}

func TestBulkSetOwner_Success(t *testing.T) {
	tenant, _, newOwner, actor, mgr := ownerFixture()
	a, b := seedRisk(tenant, "A"), seedRisk(tenant, "B")
	legacy := uuid.New()
	a.AssignedTo = &legacy
	repo := newBulkRepo(tenant, a, b)
	uc, journal := newUC(repo)
	notices := &batchNotices{}
	uc.WithOwnership(mgr).WithNotices(notices)

	res, err := uc.Execute(context.Background(), tenant, BulkActionRequest{
		Type: BulkActionSetOwner, RiskIDs: []uuid.UUID{a.ID, b.ID}, OwnerID: &newOwner,
	}, actor)
	require.NoError(t, err)
	assert.Equal(t, 2, res.Applied)
	assert.Equal(t, 2, res.Audited)

	for _, r := range []*domain.Risk{repo.risks[a.ID], repo.risks[b.ID]} {
		require.NotNil(t, r.OwnerID)
		assert.Equal(t, newOwner, *r.OwnerID)
	}
	require.NotNil(t, repo.risks[a.ID].AssignedTo)
	assert.Equal(t, legacy, *repo.risks[a.ID].AssignedTo, "set_owner leaves the legacy assigned_to alone")
	assert.Nil(t, repo.risks[b.ID].AssignedTo)

	require.Len(t, journal.events, 2)
	assert.Contains(t, []string(journal.events[0].ChangedFields), "owner_id", "the trail shows the owner change")
	assert.Empty(t, mgr.notified, "no per-risk notice for a batch")
	require.Len(t, notices.got, 1, "one notice for the whole batch, not one per risk")
	assert.Equal(t, newOwner, notices.got[0].UserID)
	assert.Equal(t, "2 risques vous ont été attribués", notices.got[0].Compose("fr").Subject)
	assert.Equal(t, "2 risks have been assigned to you", notices.got[0].Compose("en").Subject)
}

func TestBulkSetOwner_AssignToStillWritesTheLegacyColumn(t *testing.T) {
	tenant, _, newOwner, actor, mgr := ownerFixture()
	a := seedRisk(tenant, "A")
	repo := newBulkRepo(tenant, a)
	uc, _ := newUC(repo)
	uc.WithOwnership(mgr)

	_, err := uc.Execute(context.Background(), tenant, BulkActionRequest{
		Type: BulkActionAssignTo, RiskIDs: []uuid.UUID{a.ID}, AssignToID: &newOwner,
	}, actor)
	require.NoError(t, err)
	assert.Nil(t, repo.risks[a.ID].OwnerID)
	require.NotNil(t, repo.risks[a.ID].AssignedTo)
}

func TestBulkSetOwner_NotFound(t *testing.T) {
	tenant, _, newOwner, actor, mgr := ownerFixture()
	a := seedRisk(tenant, "A")
	repo := newBulkRepo(tenant, a)
	uc, journal := newUC(repo)
	uc.WithOwnership(mgr)

	_, err := uc.Execute(context.Background(), tenant, BulkActionRequest{
		Type: BulkActionSetOwner, RiskIDs: []uuid.UUID{a.ID, uuid.New()}, OwnerID: &newOwner,
	}, actor)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
	assert.Nil(t, repo.risks[a.ID].OwnerID, "one unknown id fails the batch: nothing written")
	assert.Empty(t, journal.events)
	assert.Empty(t, mgr.notified)
}

func TestBulkSetOwner_RejectsAnOwnerOutsideTheTenant(t *testing.T) {
	tenant, other, _, actor, mgr := ownerFixture()
	var foreign uuid.UUID
	for id := range mgr.members[other] {
		foreign = id
	}
	stranger := uuid.New()
	a := seedRisk(tenant, "A")

	for name, owner := range map[string]uuid.UUID{"member of another tenant": foreign, "unknown user": stranger} {
		t.Run(name, func(t *testing.T) {
			owner := owner
			repo := newBulkRepo(tenant, a)
			uc, journal := newUC(repo)
			uc.WithOwnership(mgr)
			_, err := uc.Execute(context.Background(), tenant, BulkActionRequest{
				Type: BulkActionSetOwner, RiskIDs: []uuid.UUID{a.ID}, OwnerID: &owner,
			}, actor)
			require.Error(t, err)
			assert.ErrorIs(t, err, domain.ErrValidation)
			assert.Equal(t, 0, repo.applyCalls, "rejected before any transaction opens")
			assert.Nil(t, repo.risks[a.ID].OwnerID)
			assert.Empty(t, journal.events)
		})
	}
}

func TestBulkSetOwner_Unauthorized(t *testing.T) {
	tenant, _, newOwner, actor, mgr := ownerFixture()
	a := seedRisk(tenant, "A")
	repo := newBulkRepo(tenant, a)
	uc, _ := newUC(repo)
	uc.WithOwnership(mgr)

	_, err := uc.Execute(context.Background(), uuid.Nil, BulkActionRequest{
		Type: BulkActionSetOwner, RiskIDs: []uuid.UUID{a.ID}, OwnerID: &newOwner,
	}, actor)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrForbidden)
	assert.Nil(t, repo.risks[a.ID].OwnerID)

	// Another tenant's risk answers 404, like a fabricated id.
	otherTenant := uuid.New()
	mgr.members[otherTenant] = map[uuid.UUID]bool{newOwner: true}
	_, err = uc.Execute(context.Background(), otherTenant, BulkActionRequest{
		Type: BulkActionSetOwner, RiskIDs: []uuid.UUID{a.ID}, OwnerID: &newOwner,
	}, actor)
	assert.ErrorIs(t, err, domain.ErrNotFound)
	assert.Nil(t, repo.risks[a.ID].OwnerID)
}

func TestBulkSetOwner_ValidatesParameters(t *testing.T) {
	tenant, _, newOwner, actor, mgr := ownerFixture()
	a := seedRisk(tenant, "A")
	uc, _ := newUC(newBulkRepo(tenant, a))

	_, err := uc.WithOwnership(mgr).Execute(context.Background(), tenant, BulkActionRequest{
		Type: BulkActionSetOwner, RiskIDs: []uuid.UUID{a.ID},
	}, actor)
	assert.ErrorIs(t, err, domain.ErrValidation, "owner_id is required")

	uc2, _ := newUC(newBulkRepo(tenant, a))
	_, err = uc2.Execute(context.Background(), tenant, BulkActionRequest{
		Type: BulkActionSetOwner, RiskIDs: []uuid.UUID{a.ID}, OwnerID: &newOwner,
	}, actor)
	require.Error(t, err, "without the membership check the action is refused, not applied unchecked")
}

func TestBulkSetOwner_NoticeRules(t *testing.T) {
	ctx := context.Background()

	t.Run("one risk keeps the single-risk notice", func(t *testing.T) {
		tenant, _, newOwner, actor, mgr := ownerFixture()
		a := seedRisk(tenant, "A")
		uc, _ := newUC(newBulkRepo(tenant, a))
		notices := &batchNotices{}
		uc.WithOwnership(mgr).WithNotices(notices)
		_, err := uc.Execute(ctx, tenant, BulkActionRequest{Type: BulkActionSetOwner, RiskIDs: []uuid.UUID{a.ID}, OwnerID: &newOwner}, actor)
		require.NoError(t, err)
		assert.Len(t, mgr.notified, 1)
		assert.Empty(t, notices.got)
	})

	t.Run("a hundred risks, one notice", func(t *testing.T) {
		tenant, _, newOwner, actor, mgr := ownerFixture()
		var risks []*domain.Risk
		var ids []uuid.UUID
		for i := 0; i < 100; i++ {
			r := seedRisk(tenant, "R")
			risks, ids = append(risks, r), append(ids, r.ID)
		}
		uc, _ := newUC(newBulkRepo(tenant, risks...))
		notices := &batchNotices{}
		uc.WithOwnership(mgr).WithNotices(notices)
		_, err := uc.Execute(ctx, tenant, BulkActionRequest{Type: BulkActionSetOwner, RiskIDs: ids, OwnerID: &newOwner}, actor)
		require.NoError(t, err)
		require.Len(t, notices.got, 1)
		assert.Contains(t, notices.got[0].Compose("fr").Subject, "100 risques")
		assert.Empty(t, mgr.notified)
	})

	t.Run("risks already owned by the target are not counted", func(t *testing.T) {
		tenant, _, newOwner, actor, mgr := ownerFixture()
		a, b, c := seedRisk(tenant, "A"), seedRisk(tenant, "B"), seedRisk(tenant, "C")
		a.OwnerID = &newOwner
		uc, _ := newUC(newBulkRepo(tenant, a, b, c))
		notices := &batchNotices{}
		uc.WithOwnership(mgr).WithNotices(notices)
		_, err := uc.Execute(ctx, tenant, BulkActionRequest{Type: BulkActionSetOwner, RiskIDs: []uuid.UUID{a.ID, b.ID, c.ID}, OwnerID: &newOwner}, actor)
		require.NoError(t, err)
		require.Len(t, notices.got, 1)
		assert.Contains(t, notices.got[0].Compose("en").Subject, "2 risks")
	})

	t.Run("nobody is told about their own action", func(t *testing.T) {
		tenant, _, _, actor, mgr := ownerFixture()
		a, b := seedRisk(tenant, "A"), seedRisk(tenant, "B")
		uc, _ := newUC(newBulkRepo(tenant, a, b))
		notices := &batchNotices{}
		uc.WithOwnership(mgr).WithNotices(notices)
		_, err := uc.Execute(ctx, tenant, BulkActionRequest{Type: BulkActionSetOwner, RiskIDs: []uuid.UUID{a.ID, b.ID}, OwnerID: &actor}, actor)
		require.NoError(t, err)
		assert.Empty(t, notices.got)
		assert.Empty(t, mgr.notified)
	})

	t.Run("a rejected batch tells nobody", func(t *testing.T) {
		tenant, _, newOwner, actor, mgr := ownerFixture()
		a := seedRisk(tenant, "A")
		uc, _ := newUC(newBulkRepo(tenant, a))
		notices := &batchNotices{}
		uc.WithOwnership(mgr).WithNotices(notices)
		_, err := uc.Execute(ctx, tenant, BulkActionRequest{Type: BulkActionSetOwner, RiskIDs: []uuid.UUID{a.ID, uuid.New()}, OwnerID: &newOwner}, actor)
		require.Error(t, err)
		assert.Empty(t, notices.got)
		assert.Empty(t, mgr.notified)
	})
}
