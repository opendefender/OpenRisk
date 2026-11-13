// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	scanapp "github.com/opendefender/openrisk/internal/application/scanner"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/testsupport/testdb"
)

// pgAgentFixture seeds a live agent in the real scanner_agents table. Not a
// temp table: the concurrent test needs several connections to see one row.
func pgAgentFixture(t *testing.T) (*GormScannerAgentRepository, *domain.ScannerAgent) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	db, err := testdb.Open(t, postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	repo := NewGormScannerAgentRepository(db)
	agent := &domain.ScannerAgent{
		ID: uuid.New(), TenantID: uuid.New(), Name: "agent-1390", Status: domain.AgentOffline,
		TokenHash: "hash-1390", PushSecretEnc: "secret-1390", RegisteredAt: time.Now(),
	}
	require.NoError(t, repo.Create(context.Background(), agent))
	t.Cleanup(func() {
		db.Unscoped().Where("id = ? AND tenant_id = ?", agent.ID, agent.TenantID).Delete(&domain.ScannerAgent{})
	})
	return repo, agent
}

// revokeRow does what RevokeAgentUseCase does to the row.
func revokeRow(t *testing.T, repo *GormScannerAgentRepository, agent *domain.ScannerAgent) {
	t.Helper()
	require.NoError(t, tryRevokeRow(repo, agent))
}

// tryRevokeRow returns its error instead of failing the test, so a goroutine
// can call it: FailNow is only valid on the test's own goroutine.
func tryRevokeRow(repo *GormScannerAgentRepository, agent *domain.ScannerAgent) error {
	row, err := repo.GetByID(context.Background(), agent.ID, agent.TenantID)
	if err != nil {
		return err
	}
	if row == nil {
		return errors.New("agent row not found")
	}
	row.Status, row.TokenHash, row.PushSecretEnc = domain.AgentRevoked, "", ""
	return repo.Update(context.Background(), row)
}

func assertStillRevoked(t *testing.T, repo *GormScannerAgentRepository, agent *domain.ScannerAgent) {
	t.Helper()
	row, err := repo.GetByID(context.Background(), agent.ID, agent.TenantID)
	require.NoError(t, err)
	assert.Equal(t, domain.AgentRevoked, row.Status)
	assert.Empty(t, row.TokenHash)
	assert.Empty(t, row.PushSecretEnc)
}

func TestUpdateLiveness_LiveAgentChangesOnlyLiveness_Postgres(t *testing.T) {
	repo, agent := pgAgentFixture(t)
	ctx := context.Background()
	at := time.Now().Add(time.Hour).Truncate(time.Second)

	require.NoError(t, repo.UpdateLiveness(ctx, agent.ID, agent.TenantID, agent.TokenHash, domain.AgentScanning, at))
	row, err := repo.GetByID(ctx, agent.ID, agent.TenantID)
	require.NoError(t, err)
	assert.Equal(t, domain.AgentScanning, row.Status)
	assert.WithinDuration(t, at, row.LastHeartbeat, time.Second)
	assert.Equal(t, "hash-1390", row.TokenHash)
	assert.Equal(t, "secret-1390", row.PushSecretEnc)
}

func TestUpdateLiveness_OtherTenantOrHashTouchesNothing_Postgres(t *testing.T) {
	repo, agent := pgAgentFixture(t)
	ctx := context.Background()
	now := time.Now()

	err := repo.UpdateLiveness(ctx, agent.ID, uuid.New(), agent.TokenHash, domain.AgentOnline, now)
	assert.ErrorIs(t, err, domain.ErrNotFound, "another tenant")
	err = repo.UpdateLiveness(ctx, agent.ID, agent.TenantID, "a-stale-hash", domain.AgentOnline, now)
	assert.ErrorIs(t, err, domain.ErrNotFound, "a replaced token")
	err = repo.UpdateLiveness(ctx, agent.ID, agent.TenantID, "", domain.AgentOnline, now)
	assert.ErrorIs(t, err, domain.ErrNotFound, "an empty hash never matches")
	err = repo.UpdateLiveness(ctx, uuid.New(), agent.TenantID, agent.TokenHash, domain.AgentOnline, now)
	assert.ErrorIs(t, err, domain.ErrNotFound, "an unknown agent")

	row, err := repo.GetByID(ctx, agent.ID, agent.TenantID)
	require.NoError(t, err)
	assert.Equal(t, domain.AgentOffline, row.Status, "no call above may touch the row")
	assert.True(t, row.LastHeartbeat.IsZero() || row.LastHeartbeat.Before(now.Add(-time.Second)))
}

// Criterion 1: the struct was read before the revocation, the heartbeat runs
// after it commits.
func TestHeartbeat_AfterRevocationKeepsItRevoked_Postgres(t *testing.T) {
	repo, agent := pgAgentFixture(t)
	stale, err := repo.GetByID(context.Background(), agent.ID, agent.TenantID)
	require.NoError(t, err)

	revokeRow(t, repo, agent)

	err = scanapp.NewHeartbeatAgentUseCase(repo).Execute(context.Background(), stale, domain.AgentOnline)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrUnauthorized)
	assertStillRevoked(t, repo, agent)
}

// Criterion 2: one revocation racing 50 heartbeats from the pre-revocation copy.
func TestHeartbeat_RacingARevocationNeverRestoresIt_Postgres(t *testing.T) {
	repo, agent := pgAgentFixture(t)
	stale, err := repo.GetByID(context.Background(), agent.ID, agent.TenantID)
	require.NoError(t, err)
	uc := scanapp.NewHeartbeatAgentUseCase(repo)

	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			// Each call gets its own copy: Execute must not mutate shared state.
			cp := *stale
			err := uc.Execute(context.Background(), &cp, domain.AgentOnline)
			if err != nil && !errors.Is(err, domain.ErrUnauthorized) {
				t.Errorf("heartbeat %d: unexpected error %v", i, err)
			}
		}(i)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		if err := tryRevokeRow(repo, agent); err != nil {
			t.Errorf("revocation: %v", err)
		}
	}()
	close(start)
	wg.Wait()

	assertStillRevoked(t, repo, agent)
}

func TestRecordJobCompletion_Postgres(t *testing.T) {
	repo, agent := pgAgentFixture(t)
	ctx := context.Background()
	jobID := uuid.New()
	now := time.Now().Truncate(time.Second)

	require.NoError(t, repo.RecordJobCompletion(ctx, agent.ID, agent.TenantID, agent.TokenHash, jobID, now))
	row, err := repo.GetByID(ctx, agent.ID, agent.TenantID)
	require.NoError(t, err)
	assert.Equal(t, domain.AgentOnline, row.Status)
	require.NotNil(t, row.LastScanJobID)
	assert.Equal(t, jobID, *row.LastScanJobID)
	assert.Equal(t, "hash-1390", row.TokenHash)
	assert.Equal(t, "secret-1390", row.PushSecretEnc)

	assert.ErrorIs(t, repo.RecordJobCompletion(ctx, agent.ID, uuid.New(), agent.TokenHash, jobID, now), domain.ErrNotFound)
	assert.ErrorIs(t, repo.RecordJobCompletion(ctx, agent.ID, agent.TenantID, "stale", jobID, now), domain.ErrNotFound)

	// A push landing after the revocation restores nothing.
	revokeRow(t, repo, agent)
	assert.ErrorIs(t, repo.RecordJobCompletion(ctx, agent.ID, agent.TenantID, agent.TokenHash, uuid.New(), now), domain.ErrNotFound)
	assertStillRevoked(t, repo, agent)
}
