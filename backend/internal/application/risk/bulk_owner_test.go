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

	bulkapp "github.com/opendefender/openrisk/internal/application/bulk"
	"github.com/opendefender/openrisk/internal/domain"
)

// #913 — the governed set_owner: the membership validator and the one-notice-
// per-batch hook.

type membersOnly struct {
	members  map[uuid.UUID]map[uuid.UUID]bool
	notified []domain.OwnershipChange
	subject  domain.OwnershipSubject
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

func (m *membersOnly) Notify(_ context.Context, _ uuid.UUID, changes []domain.OwnershipChange, subject domain.OwnershipSubject) {
	m.notified = append(m.notified, changes...)
	m.subject = subject
}

type batchNotices struct{ got []domain.Notice }

func (b *batchNotices) Deliver(_ context.Context, n domain.Notice) error {
	b.got = append(b.got, n)
	return nil
}

type ownerFx struct {
	tenant, other, owner, actor uuid.UUID
	mgr                         *membersOnly
}

func newOwnerFx() ownerFx {
	f := ownerFx{tenant: uuid.New(), other: uuid.New(), owner: uuid.New(), actor: uuid.New()}
	f.mgr = &membersOnly{members: map[uuid.UUID]map[uuid.UUID]bool{
		f.tenant: {f.owner: true, f.actor: true},
		f.other:  {uuid.New(): true},
	}}
	return f
}

func setOwnerChange(owner uuid.UUID) domain.BulkChange {
	return domain.BulkChange{Action: domain.BulkActionSetOwner, OwnerID: &owner}
}

func TestBulkOwnerValidator_Success(t *testing.T) {
	f := newOwnerFx()
	v := BulkOwnerValidator(f.mgr)
	require.NoError(t, v(context.Background(), bulkapp.Caller{TenantID: f.tenant, UserID: f.actor}, setOwnerChange(f.owner)))

	// Other actions are not its business.
	require.NoError(t, v(context.Background(), bulkapp.Caller{TenantID: f.tenant, UserID: f.actor},
		domain.BulkChange{Action: domain.BulkActionAddTags, Tags: []string{"x"}}))
}

func TestBulkOwnerValidator_NotFound(t *testing.T) {
	// An unknown user, and a member of ANOTHER tenant, are the same refusal.
	f := newOwnerFx()
	var foreign uuid.UUID
	for id := range f.mgr.members[f.other] {
		foreign = id
	}
	v := BulkOwnerValidator(f.mgr)
	for name, owner := range map[string]uuid.UUID{"unknown": uuid.New(), "other tenant": foreign} {
		err := v(context.Background(), bulkapp.Caller{TenantID: f.tenant, UserID: f.actor}, setOwnerChange(owner))
		require.Error(t, err, name)
		assert.ErrorIs(t, err, domain.ErrValidation, name)
	}
}

func TestBulkOwnerValidator_Unauthorized(t *testing.T) {
	// The tenant checked is the caller's: a zero tenant has no members.
	f := newOwnerFx()
	err := BulkOwnerValidator(f.mgr)(context.Background(), bulkapp.Caller{UserID: f.actor}, setOwnerChange(f.owner))
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrValidation)

	// No manager: refused, never applied unchecked. No owner_id: refused.
	err = BulkOwnerValidator(nil)(context.Background(), bulkapp.Caller{TenantID: f.tenant}, setOwnerChange(f.owner))
	require.Error(t, err)
	err = BulkOwnerValidator(f.mgr)(context.Background(), bulkapp.Caller{TenantID: f.tenant}, domain.BulkChange{Action: domain.BulkActionSetOwner})
	assert.ErrorIs(t, err, domain.ErrValidation)
}

func mutations(n int, beforeOwner string) []domain.BulkMutation {
	out := make([]domain.BulkMutation, n)
	for i := range out {
		out[i] = domain.BulkMutation{ID: uuid.New(), Label: "R",
			Before: map[string]interface{}{"owner_id": beforeOwner}}
	}
	return out
}

func TestBulkOwnerNotices_OneNoticePerBatch(t *testing.T) {
	f := newOwnerFx()
	notices := &batchNotices{}
	hook := BulkOwnerNotices(f.mgr, notices)
	caller := bulkapp.Caller{TenantID: f.tenant, UserID: f.actor}

	hook(context.Background(), caller, setOwnerChange(f.owner), mutations(100, ""))
	require.Len(t, notices.got, 1, "a hundred risks, one notice")
	assert.Equal(t, f.owner, notices.got[0].UserID)
	assert.Equal(t, "100 risques vous ont été attribués", notices.got[0].Compose("fr").Subject)
	assert.Equal(t, "100 risks have been assigned to you", notices.got[0].Compose("en").Subject)
	assert.Empty(t, f.mgr.notified)
}

func TestBulkOwnerNotices_Rules(t *testing.T) {
	ctx := context.Background()

	t.Run("one risk keeps the single-risk notice", func(t *testing.T) {
		f := newOwnerFx()
		notices := &batchNotices{}
		m := mutations(1, "")
		BulkOwnerNotices(f.mgr, notices)(ctx, bulkapp.Caller{TenantID: f.tenant, UserID: f.actor}, setOwnerChange(f.owner), m)
		require.Len(t, f.mgr.notified, 1)
		assert.Equal(t, f.owner, *f.mgr.notified[0].To)
		assert.Equal(t, m[0].ID, f.mgr.subject.ResourceID)
		assert.Empty(t, notices.got)
	})

	t.Run("risks already owned by the target are not counted", func(t *testing.T) {
		f := newOwnerFx()
		notices := &batchNotices{}
		m := append(mutations(2, ""), mutations(3, f.owner.String())...)
		BulkOwnerNotices(f.mgr, notices)(ctx, bulkapp.Caller{TenantID: f.tenant, UserID: f.actor}, setOwnerChange(f.owner), m)
		require.Len(t, notices.got, 1)
		assert.Contains(t, notices.got[0].Compose("en").Subject, "2 risks")
	})

	t.Run("nothing changed, nobody told", func(t *testing.T) {
		f := newOwnerFx()
		notices := &batchNotices{}
		BulkOwnerNotices(f.mgr, notices)(ctx, bulkapp.Caller{TenantID: f.tenant, UserID: f.actor}, setOwnerChange(f.owner), mutations(2, f.owner.String()))
		assert.Empty(t, notices.got)
		assert.Empty(t, f.mgr.notified)
	})

	t.Run("nobody is told about their own action", func(t *testing.T) {
		f := newOwnerFx()
		notices := &batchNotices{}
		BulkOwnerNotices(f.mgr, notices)(ctx, bulkapp.Caller{TenantID: f.tenant, UserID: f.actor}, setOwnerChange(f.actor), mutations(3, ""))
		assert.Empty(t, notices.got)
		assert.Empty(t, f.mgr.notified)
	})

	t.Run("other actions are ignored", func(t *testing.T) {
		f := newOwnerFx()
		notices := &batchNotices{}
		BulkOwnerNotices(f.mgr, notices)(ctx, bulkapp.Caller{TenantID: f.tenant, UserID: f.actor},
			domain.BulkChange{Action: domain.BulkActionChangeStatus, Status: "closed"}, mutations(3, ""))
		assert.Empty(t, notices.got)
	})

	t.Run("no deliverer, a batch sends nothing rather than one per risk", func(t *testing.T) {
		f := newOwnerFx()
		BulkOwnerNotices(f.mgr, nil)(ctx, bulkapp.Caller{TenantID: f.tenant, UserID: f.actor}, setOwnerChange(f.owner), mutations(3, ""))
		assert.Empty(t, f.mgr.notified)
	})
}
