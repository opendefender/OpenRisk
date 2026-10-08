// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package workers

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

	"github.com/opendefender/openrisk/internal/domain/scoring"
)

type listTenants struct {
	ids []uuid.UUID
	err error
}

func (l listTenants) ListTenantIDs(context.Context) ([]uuid.UUID, error) { return l.ids, l.err }

type recordSpy struct {
	seen []uuid.UUID
	fail uuid.UUID
}

func (r *recordSpy) Record(_ context.Context, id uuid.UUID) (*scoring.Result, error) {
	r.seen = append(r.seen, id)
	if id == r.fail {
		return nil, errors.New("boom")
	}
	return &scoring.Result{Measured: true}, nil
}

// Every tenant is recorded once, each under its own id.
func TestScoreSnapshotSweep_Success(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	spy := &recordSpy{}
	n := NewScoreSnapshotWorker(listTenants{ids: []uuid.UUID{a, b}}, spy, zerolog.Nop()).Sweep(context.Background())
	assert.Equal(t, 2, n)
	assert.Equal(t, []uuid.UUID{a, b}, spy.seen)
}

// One tenant failing does not stop the sweep for the others.
func TestScoreSnapshotSweep_OneFailure(t *testing.T) {
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	spy := &recordSpy{fail: b}
	n := NewScoreSnapshotWorker(listTenants{ids: []uuid.UUID{a, b, c}}, spy, zerolog.Nop()).Sweep(context.Background())
	assert.Equal(t, 2, n)
	assert.Len(t, spy.seen, 3)
}

// No tenants listed (or the listing failed): nothing is recorded.
func TestScoreSnapshotSweep_NotFound(t *testing.T) {
	spy := &recordSpy{}
	assert.Equal(t, 0, NewScoreSnapshotWorker(listTenants{err: errors.New("db")}, spy, zerolog.Nop()).Sweep(context.Background()))
	assert.Empty(t, spy.seen)
}
