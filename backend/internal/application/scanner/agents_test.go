// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package scanner

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
)

func TestRegisterAgent_Success(t *testing.T) {
	// Capture the persisted secrets at Create time — the use case strips the
	// in-memory struct afterward (a real GORM repo persists a copy first).
	var createdHash, createdEnc string
	var createdStatus domain.AgentStatus
	repo := &mockAgentRepo{createFunc: func(_ context.Context, a *domain.ScannerAgent) error {
		createdHash, createdEnc, createdStatus = a.TokenHash, a.PushSecretEnc, a.Status
		return nil
	}}
	uc := NewRegisterAgentUseCase(repo, testKeys(), testCipher())
	tenant, cfg := uuid.New(), uuid.New()

	res, err := uc.Execute(context.Background(), RegisterAgentInput{
		TenantID: tenant, ConfigID: &cfg, Name: "edge-01", Version: "1.0.0", Hostname: "edge-01", OS: "linux",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, res.Token)
	assert.NotEmpty(t, res.PushSecret)
	assert.True(t, res.RotateAfter.After(time.Now()))
	// Secrets are stored (hash/ciphertext) but stripped from the response body.
	assert.NotEmpty(t, createdHash)
	assert.NotEmpty(t, createdEnc)
	assert.Empty(t, res.Agent.TokenHash)
	assert.Empty(t, res.Agent.PushSecretEnc)
	assert.Equal(t, domain.AgentOnline, createdStatus)
}

func TestRegisterAgent_Unauthorized(t *testing.T) {
	uc := NewRegisterAgentUseCase(&mockAgentRepo{}, testKeys(), testCipher())
	_, err := uc.Execute(context.Background(), RegisterAgentInput{TenantID: uuid.Nil, Hostname: "x"})
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrUnauthorized)
}

func TestRegisterAgent_ReEnrollUpdatesInPlace(t *testing.T) {
	tenant, cfg := uuid.New(), uuid.New()
	existing := domain.ScannerAgent{ID: uuid.New(), TenantID: tenant, Hostname: "edge-01", RegistrationConfigID: &cfg, Status: domain.AgentOffline}
	var updated bool
	repo := &mockAgentRepo{
		listFunc: func(_ context.Context, _ uuid.UUID) ([]domain.ScannerAgent, error) {
			return []domain.ScannerAgent{existing}, nil
		},
		updateFunc: func(_ context.Context, _ *domain.ScannerAgent) error { updated = true; return nil },
		createFunc: func(_ context.Context, _ *domain.ScannerAgent) error {
			t.Fatal("should not create on re-enroll")
			return nil
		},
	}
	uc := NewRegisterAgentUseCase(repo, testKeys(), testCipher())
	res, err := uc.Execute(context.Background(), RegisterAgentInput{TenantID: tenant, ConfigID: &cfg, Hostname: "edge-01"})
	require.NoError(t, err)
	assert.True(t, updated)
	assert.Equal(t, existing.ID, res.Agent.ID) // same agent, rotated token
}

func TestRevokeAgent_Success(t *testing.T) {
	tenant, id := uuid.New(), uuid.New()
	agent := &domain.ScannerAgent{ID: id, TenantID: tenant, Status: domain.AgentOnline, TokenHash: "h"}
	var revoked *domain.ScannerAgent
	repo := &mockAgentRepo{
		getByIDFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.ScannerAgent, error) { return agent, nil },
		updateFunc:  func(_ context.Context, a *domain.ScannerAgent) error { revoked = a; return nil },
	}
	uc := NewRevokeAgentUseCase(repo, newFakeKV())
	require.NoError(t, uc.Execute(context.Background(), tenant, id))
	require.NotNil(t, revoked)
	assert.Equal(t, domain.AgentRevoked, revoked.Status)
	assert.Empty(t, revoked.TokenHash) // token killed
}

func TestRevokeAgent_NotFound(t *testing.T) {
	repo := &mockAgentRepo{getByIDFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.ScannerAgent, error) { return nil, nil }}
	uc := NewRevokeAgentUseCase(repo, newFakeKV())
	err := uc.Execute(context.Background(), uuid.New(), uuid.New())
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestRevokeAgent_Unauthorized(t *testing.T) {
	uc := NewRevokeAgentUseCase(&mockAgentRepo{}, newFakeKV())
	err := uc.Execute(context.Background(), uuid.Nil, uuid.New())
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrUnauthorized)
}

func TestListAgents_DowngradesStaleOnline(t *testing.T) {
	tenant := uuid.New()
	repo := &mockAgentRepo{listFunc: func(_ context.Context, _ uuid.UUID) ([]domain.ScannerAgent, error) {
		return []domain.ScannerAgent{
			{ID: uuid.New(), TenantID: tenant, Status: domain.AgentOnline, LastHeartbeat: time.Now().Add(-10 * time.Minute)},
			{ID: uuid.New(), TenantID: tenant, Status: domain.AgentOnline, LastHeartbeat: time.Now()},
		}, nil
	}}
	uc := NewListAgentsUseCase(repo)
	agents, err := uc.Execute(context.Background(), tenant)
	require.NoError(t, err)
	assert.Equal(t, domain.AgentOffline, agents[0].Status) // stale
	assert.Equal(t, domain.AgentOnline, agents[1].Status)  // fresh
}
