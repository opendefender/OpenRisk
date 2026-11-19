// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
)

// Revoke and Reenroll (#1402). Same fixture as the liveness tests: a real
// scanner_agents row, because the race test needs several connections.

// reenrolled is a copy of the agent as a registration would write it.
func reenrolled(a *domain.ScannerAgent) *domain.ScannerAgent {
	cp := *a
	cp.Status = domain.AgentOnline
	cp.TokenHash = "new-hash-1402"
	cp.PushSecretEnc = "new-secret-1402"
	cp.LastHeartbeat = time.Now()
	cp.TokenRotatedAt = time.Now()
	return &cp
}

func TestRevoke_ClearsCredentialsAndIsIdempotent_Postgres(t *testing.T) {
	repo, agent := pgAgentFixture(t)
	ctx := context.Background()
	require.NoError(t, repo.Revoke(ctx, agent.ID, agent.TenantID))
	assertStillRevoked(t, repo, agent)
	require.NoError(t, repo.Revoke(ctx, agent.ID, agent.TenantID), "revoking twice succeeds")
	assertStillRevoked(t, repo, agent)
	assert.ErrorIs(t, repo.Revoke(ctx, uuid.New(), agent.TenantID), domain.ErrNotFound)
}

// Criterion 6: another tenant's call touches no row.
func TestRevokeAndReenroll_OtherTenantTouchesNothing_Postgres(t *testing.T) {
	repo, agent := pgAgentFixture(t)
	ctx := context.Background()

	assert.ErrorIs(t, repo.Revoke(ctx, agent.ID, uuid.New()), domain.ErrNotFound)
	foreign := reenrolled(agent)
	foreign.TenantID = uuid.New()
	assert.ErrorIs(t, repo.Reenroll(ctx, foreign), domain.ErrConflict)

	row, err := repo.GetByID(ctx, agent.ID, agent.TenantID)
	require.NoError(t, err)
	assert.Equal(t, domain.AgentOffline, row.Status)
	assert.Equal(t, "hash-1390", row.TokenHash)
	assert.Equal(t, "secret-1390", row.PushSecretEnc)
}

func TestReenroll_LiveAgentRotatesCredentials_Postgres(t *testing.T) {
	repo, agent := pgAgentFixture(t)
	require.NoError(t, repo.Reenroll(context.Background(), reenrolled(agent)))
	row, err := repo.GetByID(context.Background(), agent.ID, agent.TenantID)
	require.NoError(t, err)
	assert.Equal(t, domain.AgentOnline, row.Status)
	assert.Equal(t, "new-hash-1402", row.TokenHash)
	assert.Equal(t, "new-secret-1402", row.PushSecretEnc)
}

// Criterion 1: the registration read a live agent, the revocation commits, then
// the registration's write runs.
func TestReenroll_AfterRevocationReturnsConflictAndKeepsItRevoked_Postgres(t *testing.T) {
	repo, agent := pgAgentFixture(t)
	stale := reenrolled(agent)
	require.NoError(t, repo.Revoke(context.Background(), agent.ID, agent.TenantID))

	err := repo.Reenroll(context.Background(), stale)
	assert.ErrorIs(t, err, domain.ErrConflict)
	assertStillRevoked(t, repo, agent)
}

// Criterion 2: one revocation racing 50 Reenroll calls. Goroutines never call
// require (FailNow is only valid on the test goroutine); errors are collected
// and asserted after Wait.
func TestReenroll_RacingARevocationNeverRestoresIt_Postgres(t *testing.T) {
	repo, agent := pgAgentFixture(t)
	ctx := context.Background()

	const n = 50
	errs := make(chan error, n+1)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if err := repo.Reenroll(ctx, reenrolled(agent)); err != nil && !errors.Is(err, domain.ErrConflict) {
				errs <- err
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		if err := repo.Revoke(ctx, agent.ID, agent.TenantID); err != nil {
			errs <- err
		}
	}()
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("unexpected error: %v", err)
	}

	assertStillRevoked(t, repo, agent)
}
