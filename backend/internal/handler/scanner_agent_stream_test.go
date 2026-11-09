// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"bufio"
	"context"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	scanapp "github.com/opendefender/openrisk/internal/application/scanner"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
)

// ---------------------------------------------------------------------------
// Agent revocation on a live stream (#962).
//
// The server's absolute WriteTimeout used to cut every agent stream within
// seconds, and that cut was, by accident, the only thing that ended a revoked
// agent's stream: agent tokens carry no jti, so the session revocation check
// never applies to them. With the rolling deadline the stream lives as long as
// the client reads, so the agent is now re-read on every keepalive tick.
// ---------------------------------------------------------------------------

// agentTestRepo is the production GORM repository on in-memory SQLite, so the
// tests exercise the real Update column list — the one that used to write a
// stale token hash back over a revocation.
func agentTestRepo(t *testing.T) *repository.GormScannerAgentRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	// Postgres DDL (gen_random_uuid) does not run on SQLite; the columns the
	// repository reads and writes are what matters here.
	require.NoError(t, db.Exec(`CREATE TABLE scanner_agents (
		id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, name TEXT NOT NULL,
		version TEXT, status TEXT, last_heartbeat DATETIME, ip TEXT,
		hostname TEXT, os TEXT, registered_at DATETIME, token_hash TEXT,
		push_secret_enc TEXT, registration_config_id TEXT,
		last_scan_job_id TEXT, token_rotated_at DATETIME,
		created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`).Error)
	return repository.NewGormScannerAgentRepository(db)
}

func seedAgent(t *testing.T, repo *repository.GormScannerAgentRepository) *domain.ScannerAgent {
	t.Helper()
	agent := &domain.ScannerAgent{
		ID:            uuid.New(),
		TenantID:      uuid.New(),
		Name:          "agent-962",
		Status:        domain.AgentOnline,
		TokenHash:     "hash-at-connect",
		PushSecretEnc: "secret-at-connect",
		RegisteredAt:  time.Now(),
	}
	require.NoError(t, repo.Create(context.Background(), agent))
	return agent
}

// revoke does what RevokeAgentUseCase does to the row.
func revoke(t *testing.T, repo *repository.GormScannerAgentRepository, agent *domain.ScannerAgent) {
	t.Helper()
	row, err := repo.GetByID(context.Background(), agent.ID, agent.TenantID)
	require.NoError(t, err)
	row.Status = domain.AgentRevoked
	row.TokenHash = ""
	row.PushSecretEnc = ""
	require.NoError(t, repo.Update(context.Background(), row))
}

func TestAgentStillValid_Success(t *testing.T) {
	repo := agentTestRepo(t)
	agent := seedAgent(t, repo)
	h := &ScannerHandler{agentRepo: repo}

	got, ok := h.agentStillValid(context.Background(), agent.ID, agent.TenantID, "hash-at-connect")
	require.True(t, ok)
	assert.Equal(t, agent.ID, got.ID)
}

func TestAgentStillValid_NotFound(t *testing.T) {
	repo := agentTestRepo(t)
	agent := seedAgent(t, repo)
	h := &ScannerHandler{agentRepo: repo}

	_, ok := h.agentStillValid(context.Background(), uuid.New(), agent.TenantID, "hash-at-connect")
	assert.False(t, ok, "an unknown agent must not keep a stream")
	// Same agent id under another tenant: the lookup is tenant-scoped.
	_, ok = h.agentStillValid(context.Background(), agent.ID, uuid.New(), "hash-at-connect")
	assert.False(t, ok, "an agent read through another tenant must not keep a stream")
}

func TestAgentStillValid_Unauthorized(t *testing.T) {
	repo := agentTestRepo(t)
	h := &ScannerHandler{agentRepo: repo}

	t.Run("revoked", func(t *testing.T) {
		agent := seedAgent(t, repo)
		revoke(t, repo, agent)
		_, ok := h.agentStillValid(context.Background(), agent.ID, agent.TenantID, "hash-at-connect")
		assert.False(t, ok)
	})
	t.Run("re-registered with a new token", func(t *testing.T) {
		agent := seedAgent(t, repo)
		row, err := repo.GetByID(context.Background(), agent.ID, agent.TenantID)
		require.NoError(t, err)
		row.TokenHash = "a-newer-hash"
		require.NoError(t, repo.Update(context.Background(), row))
		_, ok := h.agentStillValid(context.Background(), agent.ID, agent.TenantID, "hash-at-connect")
		assert.False(t, ok)
	})
}

// The stream's close handler used to write the row it captured at connect back
// over the database, token hash and push secret included. After a revocation
// that restored the credentials the revocation had just cleared.
func TestMarkAgentStreamClosed_DoesNotUndoRevocation(t *testing.T) {
	repo := agentTestRepo(t)
	agent := seedAgent(t, repo)
	h := &ScannerHandler{agentRepo: repo, heartbeat: scanapp.NewHeartbeatAgentUseCase(repo)}

	revoke(t, repo, agent)
	h.markAgentStreamClosed(context.Background(), agent.ID, agent.TenantID, "hash-at-connect")

	row, err := repo.GetByID(context.Background(), agent.ID, agent.TenantID)
	require.NoError(t, err)
	assert.Equal(t, domain.AgentRevoked, row.Status)
	assert.Empty(t, row.TokenHash, "the revoked token came back")
	assert.Empty(t, row.PushSecretEnc, "the revoked push secret came back")
	// And the agent cannot authenticate with its old token again.
	byHash, err := repo.GetByTokenHash(context.Background(), "hash-at-connect")
	require.NoError(t, err)
	assert.Nil(t, byHash)
}

func TestMarkAgentStreamClosed_MarksALiveAgentOffline(t *testing.T) {
	repo := agentTestRepo(t)
	agent := seedAgent(t, repo)
	h := &ScannerHandler{agentRepo: repo, heartbeat: scanapp.NewHeartbeatAgentUseCase(repo)}

	h.markAgentStreamClosed(context.Background(), agent.ID, agent.TenantID, "hash-at-connect")

	row, err := repo.GetByID(context.Background(), agent.ID, agent.TenantID)
	require.NoError(t, err)
	assert.Equal(t, domain.AgentOffline, row.Status)
	assert.Equal(t, "hash-at-connect", row.TokenHash)
}

// readFrames returns the stream's lines and closes the channel at EOF.
func readFrames(t *testing.T, url string) (<-chan string, func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	lines := make(chan string, 256)
	go func() {
		defer close(lines)
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			lines <- sc.Text()
		}
	}()
	stop := func() { cancel(); _ = resp.Body.Close() }
	t.Cleanup(stop)
	return lines, stop
}

// awaitClose drains the stream until EOF and reports whether a revocation frame
// came first.
func awaitClose(t *testing.T, lines <-chan string, within time.Duration) (revoked bool) {
	t.Helper()
	timer := time.NewTimer(within)
	defer timer.Stop()
	for {
		select {
		case line, ok := <-lines:
			if !ok {
				return revoked
			}
			if line == "event: stream.revoked" {
				revoked = true
			}
		case <-timer.C:
			t.Fatalf("the stream was still open after %s", within)
			return false
		}
	}
}

func TestStreamChannel_RevokedMidStreamIsClosed(t *testing.T) {
	setSSEWriteTimeout(t, deadlineTestWriteTimeout)
	var authorized atomic.Bool
	authorized.Store(true)
	url := streamChannelServerWith(t, channelStream{stillAuthorized: authorized.Load})

	lines, _ := readFrames(t, url)
	// Alive across several ticks and past the server's WriteTimeout.
	time.Sleep(deadlineTestWriteTimeout + 2*deadlineTestKeepalive)
	authorized.Store(false)

	assert.True(t, awaitClose(t, lines, 3*deadlineTestKeepalive), "no stream.revoked frame before the close")
}

func TestStreamChannel_MaxLifetimeEndsTheStream(t *testing.T) {
	setSSEWriteTimeout(t, deadlineTestWriteTimeout)
	url := streamChannelServerWith(t, channelStream{maxLifetime: 1500 * time.Millisecond})

	lines, _ := readFrames(t, url)
	started := time.Now()
	awaitClose(t, lines, 3*time.Second)
	assert.GreaterOrEqual(t, time.Since(started), time.Second, "closed before its lifetime")
}
