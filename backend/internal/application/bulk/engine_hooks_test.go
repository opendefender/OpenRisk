// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package bulk

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
)

// #599 — the two extension points the risk register needs: a store-supplied
// prediction, and a hook that runs only once a batch has committed.

// predictingStore is a fakeStore whose change also moves a derived field, the
// way a risk status change moves its canonical state.
type predictingStore struct{ *fakeStore }

func (p predictingStore) PredictBulk(row domain.BulkRow, c domain.BulkChange) map[string]interface{} {
	after := c.PredictOn(row.Snapshot)
	after["derived"] = "moved-by-" + c.Status
	return after
}

func TestBulkEngine_Preview_UsesTheStorePrediction(t *testing.T) {
	store := predictingStore{newFakeStore()}
	tenant := uuid.New()
	id := store.seed(tenant, "r1", "open")
	e, _ := newEngine(store)

	p, err := e.Preview(context.Background(), Caller{TenantID: tenant, UserID: uuid.New()},
		Request{IDs: []uuid.UUID{id}, Change: statusChange("closed")})
	require.NoError(t, err)
	require.Len(t, p.Sample, 1)
	assert.Equal(t, "moved-by-closed", p.Sample[0].After["derived"])
	assert.Contains(t, p.Sample[0].ChangedFields, "derived")
}

func TestBulkEngine_AfterApply_RunsOnceAfterCommit(t *testing.T) {
	store := newFakeStore()
	tenant, actor := uuid.New(), uuid.New()
	a, b := store.seed(tenant, "a", "open"), store.seed(tenant, "b", "open")

	var calls int
	var seen []domain.BulkMutation
	var by Caller
	e, _ := newEngine(store)
	e.WithAfterApply(func(_ context.Context, c Caller, _ domain.BulkChange, m []domain.BulkMutation) {
		calls++
		seen, by = m, c
		// The hook sees the committed state, never the pre-commit one.
		assert.Equal(t, "closed", store.rows[a].status)
	})

	_, err := e.Apply(context.Background(), Caller{TenantID: tenant, UserID: actor},
		Request{IDs: []uuid.UUID{a, b}, Change: statusChange("closed")})
	require.NoError(t, err)
	assert.Equal(t, 1, calls)
	assert.Len(t, seen, 2)
	assert.Equal(t, actor, by.UserID)
}

func TestBulkEngine_AfterApply_NotFound_DoesNotRun(t *testing.T) {
	store := newFakeStore()
	tenant := uuid.New()
	a := store.seed(tenant, "a", "open")

	ran := false
	e, _ := newEngine(store)
	e.WithAfterApply(func(context.Context, Caller, domain.BulkChange, []domain.BulkMutation) { ran = true })

	_, err := e.Apply(context.Background(), Caller{TenantID: tenant, UserID: uuid.New()},
		Request{IDs: []uuid.UUID{a, uuid.New()}, Change: statusChange("closed")})
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrNotFound))
	assert.False(t, ran, "a failed batch must not reach the post-commit hook")
}

func TestBulkEngine_AfterApply_Unauthorized_DoesNotRun(t *testing.T) {
	store := newFakeStore()
	a := store.seed(uuid.New(), "a", "open")

	ran := false
	e, _ := newEngine(store)
	e.WithAfterApply(func(context.Context, Caller, domain.BulkChange, []domain.BulkMutation) { ran = true })

	_, err := e.Apply(context.Background(), Caller{TenantID: uuid.Nil, UserID: uuid.New()},
		Request{IDs: []uuid.UUID{a}, Change: statusChange("closed")})
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrForbidden))
	assert.False(t, ran)
}

// A tag action is refused by a register that does not declare it, exactly as
// change_status is refused by the delete-only asset register.
func TestBulkEngine_RefusesTagActionsOnARegisterWithoutTags(t *testing.T) {
	store := newFakeStore(domain.BulkActionChangeStatus, domain.BulkActionDelete)
	tenant := uuid.New()
	a := store.seed(tenant, "a", "open")
	e, _ := newEngine(store)

	_, err := e.Apply(context.Background(), Caller{TenantID: tenant, UserID: uuid.New()},
		Request{IDs: []uuid.UUID{a}, Change: domain.BulkChange{Action: domain.BulkActionAddTags, Tags: []string{"x"}}})
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrValidation))
}
