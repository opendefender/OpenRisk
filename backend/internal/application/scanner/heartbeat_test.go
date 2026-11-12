// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package scanner

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
)

// Heartbeat writes liveness only, through one conditional statement (#1390).

func TestHeartbeatAgent_Success(t *testing.T) {
	agent := &domain.ScannerAgent{ID: uuid.New(), TenantID: uuid.New(), TokenHash: "h", PushSecretEnc: "s"}
	var got []domain.AgentStatus
	var gotID, gotTenant uuid.UUID
	var gotHash string
	repo := &mockAgentRepo{
		livenessFunc: func(_ context.Context, id, tenant uuid.UUID, hash string, s domain.AgentStatus, at time.Time) error {
			gotID, gotTenant, gotHash = id, tenant, hash
			got = append(got, s)
			assert.WithinDuration(t, time.Now(), at, time.Minute)
			return nil
		},
		updateFunc: func(context.Context, *domain.ScannerAgent) error {
			t.Fatal("a heartbeat must not call Update")
			return nil
		},
	}
	uc := NewHeartbeatAgentUseCase(repo)
	require.NoError(t, uc.Execute(context.Background(), agent, domain.AgentOnline))
	require.NoError(t, uc.Execute(context.Background(), agent, domain.AgentScanning))
	assert.Equal(t, []domain.AgentStatus{domain.AgentOnline, domain.AgentScanning}, got)
	assert.Equal(t, agent.ID, gotID)
	assert.Equal(t, agent.TenantID, gotTenant)
	assert.Equal(t, "h", gotHash, "the hash read at authentication is the guard")
}

// A zero-row write cannot tell an unknown id from a revoked or replaced agent,
// so both answer Unauthorized, never NotFound (which would leak existence).
func TestHeartbeatAgent_NotFound(t *testing.T) {
	repo := &mockAgentRepo{livenessFunc: func(context.Context, uuid.UUID, uuid.UUID, string, domain.AgentStatus, time.Time) error {
		return domain.ErrNotFound
	}}
	err := NewHeartbeatAgentUseCase(repo).Execute(context.Background(),
		&domain.ScannerAgent{ID: uuid.New(), TenantID: uuid.New(), TokenHash: "h"}, domain.AgentOnline)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrUnauthorized)
	assert.Equal(t, 401, domain.HTTPStatusFromError(err))
}

func TestHeartbeatAgent_Unauthorized(t *testing.T) {
	uc := NewHeartbeatAgentUseCase(&mockAgentRepo{})
	assert.ErrorIs(t, uc.Execute(context.Background(), nil, domain.AgentOnline), domain.ErrUnauthorized)
	err := uc.Execute(context.Background(), &domain.ScannerAgent{ID: uuid.New()}, domain.AgentOnline)
	assert.ErrorIs(t, err, domain.ErrUnauthorized)
}

func TestHeartbeatAgent_StoreFailureIsInternal(t *testing.T) {
	repo := &mockAgentRepo{livenessFunc: func(context.Context, uuid.UUID, uuid.UUID, string, domain.AgentStatus, time.Time) error {
		return errors.New("db down")
	}}
	err := NewHeartbeatAgentUseCase(repo).Execute(context.Background(),
		&domain.ScannerAgent{ID: uuid.New(), TenantID: uuid.New(), TokenHash: "h"}, domain.AgentOnline)
	assert.ErrorIs(t, err, domain.ErrInternal)
}
