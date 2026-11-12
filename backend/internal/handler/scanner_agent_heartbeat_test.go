// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	scanapp "github.com/opendefender/openrisk/internal/application/scanner"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/internal/middleware"
	scanpkg "github.com/opendefender/openrisk/internal/scanner"
	authpkg "github.com/opendefender/openrisk/pkg/auth"
)

// POST /scanner/agent/heartbeat (#1390).

// revokeAfterLookup revokes the agent right after authenticateAgent resolved
// it, which is the window the race lives in.
type revokeAfterLookup struct {
	*repository.GormScannerAgentRepository
	t *testing.T
}

func (r revokeAfterLookup) GetByTokenHash(ctx context.Context, hash string) (*domain.ScannerAgent, error) {
	agent, err := r.GormScannerAgentRepository.GetByTokenHash(ctx, hash)
	if agent != nil {
		revoke(r.t, r.GormScannerAgentRepository, agent)
	}
	return agent, err
}

func seedTokenAgent(t *testing.T, repo *repository.GormScannerAgentRepository, keys *authpkg.RSAKeys) (*domain.ScannerAgent, string) {
	t.Helper()
	agent := &domain.ScannerAgent{
		ID: uuid.New(), TenantID: uuid.New(), Name: "agent-1390",
		Status: domain.AgentOffline, PushSecretEnc: "secret", RegisteredAt: time.Now(),
	}
	token, err := scanpkg.MintAgentToken(keys, agent.TenantID, agent.ID)
	require.NoError(t, err)
	agent.TokenHash = scanpkg.HashToken(token)
	require.NoError(t, repo.Create(context.Background(), agent))
	return agent, token
}

func beat(t *testing.T, h *ScannerHandler, token, query string) int {
	t.Helper()
	app := fiber.New()
	app.Post("/scanner/agent/heartbeat", h.AgentHeartbeat)
	req := httptest.NewRequest("POST", "/scanner/agent/heartbeat"+query, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp.StatusCode
}

func TestAgentHeartbeat_Success(t *testing.T) {
	repo := agentTestRepo(t)
	keys := testSigningKeys(t)
	agent, token := seedTokenAgent(t, repo, keys)
	h := &ScannerHandler{agentRepo: repo, heartbeat: scanapp.NewHeartbeatAgentUseCase(repo), rsaKeys: keys}

	assert.Equal(t, 200, beat(t, h, token, "?status=scanning"))
	row, err := repo.GetByID(context.Background(), agent.ID, agent.TenantID)
	require.NoError(t, err)
	assert.Equal(t, domain.AgentScanning, row.Status)
	assert.Equal(t, agent.TokenHash, row.TokenHash)
	assert.Equal(t, "secret", row.PushSecretEnc)
}

func TestAgentHeartbeat_NotFound(t *testing.T) {
	repo := agentTestRepo(t)
	keys := testSigningKeys(t)
	h := &ScannerHandler{agentRepo: repo, heartbeat: scanapp.NewHeartbeatAgentUseCase(repo), rsaKeys: keys}
	// A validly signed token for an agent that does not exist.
	token, err := scanpkg.MintAgentToken(keys, uuid.New(), uuid.New())
	require.NoError(t, err)
	assert.Equal(t, 401, beat(t, h, token, ""))
}

func TestAgentHeartbeat_Unauthorized_RevokedAgent(t *testing.T) {
	repo := agentTestRepo(t)
	keys := testSigningKeys(t)
	agent, token := seedTokenAgent(t, repo, keys)
	h := &ScannerHandler{agentRepo: repo, heartbeat: scanapp.NewHeartbeatAgentUseCase(repo), rsaKeys: keys}

	revoke(t, repo, agent)
	assert.Equal(t, 401, beat(t, h, token, ""))
	assert.Equal(t, 401, beat(t, h, "", ""), "no token")
}

func TestAgentHeartbeat_RevocationBetweenAuthAndWrite(t *testing.T) {
	repo := agentTestRepo(t)
	keys := testSigningKeys(t)
	agent, token := seedTokenAgent(t, repo, keys)
	racing := revokeAfterLookup{repo, t}
	h := &ScannerHandler{agentRepo: racing, heartbeat: scanapp.NewHeartbeatAgentUseCase(repo), rsaKeys: keys}

	assert.Equal(t, 401, beat(t, h, token, ""))
	row, err := repo.GetByID(context.Background(), agent.ID, agent.TenantID)
	require.NoError(t, err)
	assert.Equal(t, domain.AgentRevoked, row.Status)
	assert.Empty(t, row.TokenHash)
	assert.Empty(t, row.PushSecretEnc)
}

func TestAgentHeartbeat_RateLimitedPerAgent(t *testing.T) {
	repo := agentTestRepo(t)
	keys := testSigningKeys(t)
	_, tokenA := seedTokenAgent(t, repo, keys)
	_, tokenB := seedTokenAgent(t, repo, keys)
	h := (&ScannerHandler{agentRepo: repo, heartbeat: scanapp.NewHeartbeatAgentUseCase(repo), rsaKeys: keys}).
		WithHeartbeatLimiter(middleware.NewRateLimitStore())

	for i := 1; i <= 12; i++ {
		require.Equal(t, 200, beat(t, h, tokenA, ""), "heartbeat %d", i)
	}
	assert.Equal(t, 429, beat(t, h, tokenA, ""), "the 13th inside a minute")
	assert.Equal(t, 200, beat(t, h, tokenB, ""), "another agent has its own budget")
}
