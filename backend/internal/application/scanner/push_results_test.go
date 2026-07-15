// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package scanner

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
	scanpkg "github.com/opendefender/openrisk/internal/scanner"
)

func newPushUC(kv *fakeKV, agentRepo *mockAgentRepo, jobRepo *mockJobRepo) *PushResultsUseCase {
	pipeline := scanpkg.NewPipeline(scanpkg.NewRegistry(), scanpkg.NewPreviewStore(kv), scanpkg.NoopNotifier{}, zerolog.Nop())
	lock := scanpkg.NewScanLock(kv)
	return NewPushResultsUseCase(agentRepo, jobRepo, lock, pipeline)
}

func agentJob(tenant uuid.UUID) *domain.ScanJob {
	return &domain.ScanJob{ID: uuid.New(), TenantID: tenant, ConfigID: uuid.New(), Provider: domain.ProviderAgent, Status: domain.ScanQueued}
}

func TestPushResults_Success(t *testing.T) {
	tenant := uuid.New()
	agent := &domain.ScannerAgent{ID: uuid.New(), TenantID: tenant, Name: "edge-01"}
	job := agentJob(tenant)
	jobRepo := &mockJobRepo{getByIDFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.ScanJob, error) { return job, nil }}
	uc := newPushUC(newFakeKV(), &mockAgentRepo{}, jobRepo)

	got, err := uc.Execute(context.Background(), PushResultsInput{
		Agent: agent, JobID: job.ID,
		Assets: []scanpkg.AssetDiscovery{{ExternalID: "h1", Name: "host-1"}},
	})
	require.NoError(t, err)
	assert.Equal(t, domain.ScanCompleted, got.Status)
	assert.Equal(t, 1, got.AssetsFound)
	require.NotNil(t, got.ClaimedByAgent)
	assert.Equal(t, agent.ID, *got.ClaimedByAgent)
}

func TestPushResults_Unauthorized(t *testing.T) {
	uc := newPushUC(newFakeKV(), &mockAgentRepo{}, &mockJobRepo{})
	_, err := uc.Execute(context.Background(), PushResultsInput{Agent: nil, JobID: uuid.New()})
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrUnauthorized)
}

func TestPushResults_JobNotFound(t *testing.T) {
	tenant := uuid.New()
	jobRepo := &mockJobRepo{getByIDFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.ScanJob, error) { return nil, nil }}
	uc := newPushUC(newFakeKV(), &mockAgentRepo{}, jobRepo)
	_, err := uc.Execute(context.Background(), PushResultsInput{
		Agent: &domain.ScannerAgent{ID: uuid.New(), TenantID: tenant}, JobID: uuid.New(),
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestPushResults_WrongProvider(t *testing.T) {
	tenant := uuid.New()
	job := agentJob(tenant)
	job.Provider = domain.ProviderAWS // cloud job — agents can't push it
	jobRepo := &mockJobRepo{getByIDFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.ScanJob, error) { return job, nil }}
	uc := newPushUC(newFakeKV(), &mockAgentRepo{}, jobRepo)
	_, err := uc.Execute(context.Background(), PushResultsInput{Agent: &domain.ScannerAgent{ID: uuid.New(), TenantID: tenant}, JobID: job.ID})
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrValidation)
}

func TestPushResults_AlreadyCompleted(t *testing.T) {
	tenant := uuid.New()
	job := agentJob(tenant)
	job.Status = domain.ScanCompleted
	jobRepo := &mockJobRepo{getByIDFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.ScanJob, error) { return job, nil }}
	uc := newPushUC(newFakeKV(), &mockAgentRepo{}, jobRepo)
	_, err := uc.Execute(context.Background(), PushResultsInput{Agent: &domain.ScannerAgent{ID: uuid.New(), TenantID: tenant}, JobID: job.ID})
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrConflict)
}

func TestPushResults_ClaimLostToAnotherAgent(t *testing.T) {
	tenant := uuid.New()
	job := agentJob(tenant)
	kv := newFakeKV()
	// Pre-take the job lock so ClaimJob returns false (another agent won).
	_, _ = kv.SetNX(context.Background(), "scan:lock:job:"+tenant.String()+":"+job.ID.String(), "other", 0)
	jobRepo := &mockJobRepo{getByIDFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.ScanJob, error) { return job, nil }}
	uc := newPushUC(kv, &mockAgentRepo{}, jobRepo)
	_, err := uc.Execute(context.Background(), PushResultsInput{Agent: &domain.ScannerAgent{ID: uuid.New(), TenantID: tenant}, JobID: job.ID})
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrConflict)
}
