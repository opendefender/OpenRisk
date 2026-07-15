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

func newTriggerUC(kv *fakeKV, configRepo *mockConfigRepo, jobRepo *mockJobRepo) *TriggerScanUseCase {
	reg := testRegistry()
	pipeline := scanpkg.NewPipeline(reg, scanpkg.NewPreviewStore(kv), scanpkg.NoopNotifier{}, zerolog.Nop())
	lock := scanpkg.NewScanLock(kv)
	return NewTriggerScanUseCase(configRepo, jobRepo, lock, reg, pipeline, testCipher(), kv, zerolog.Nop())
}

func agentConfig(tenantID uuid.UUID) *domain.ScanConfig {
	return &domain.ScanConfig{
		ID: uuid.New(), TenantID: tenantID, Provider: domain.ProviderAgent,
		Enabled: true, Targets: []string{"10.0.0.0/24"},
	}
}

func TestTriggerScan_Success_AgentDispatches(t *testing.T) {
	tenant := uuid.New()
	cfg := agentConfig(tenant)
	kv := newFakeKV()
	configRepo := &mockConfigRepo{getByIDFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.ScanConfig, error) { return cfg, nil }}
	jobRepo := &mockJobRepo{}
	uc := newTriggerUC(kv, configRepo, jobRepo)

	job, err := uc.Execute(context.Background(), tenant, uuid.New(), cfg.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.ScanQueued, job.Status)
	assert.Equal(t, 1, kv.messages) // dispatched to agents over SSE
}

func TestTriggerScan_Unauthorized(t *testing.T) {
	uc := newTriggerUC(newFakeKV(), &mockConfigRepo{}, &mockJobRepo{})
	_, err := uc.Execute(context.Background(), uuid.Nil, uuid.New(), uuid.New())
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrUnauthorized)
}

func TestTriggerScan_NotFound(t *testing.T) {
	configRepo := &mockConfigRepo{getByIDFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.ScanConfig, error) { return nil, nil }}
	uc := newTriggerUC(newFakeKV(), configRepo, &mockJobRepo{})
	_, err := uc.Execute(context.Background(), uuid.New(), uuid.New(), uuid.New())
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestTriggerScan_Disabled(t *testing.T) {
	tenant := uuid.New()
	cfg := agentConfig(tenant)
	cfg.Enabled = false
	configRepo := &mockConfigRepo{getByIDFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.ScanConfig, error) { return cfg, nil }}
	uc := newTriggerUC(newFakeKV(), configRepo, &mockJobRepo{})
	_, err := uc.Execute(context.Background(), tenant, uuid.New(), cfg.ID)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrValidation)
}

func TestTriggerScan_ConcurrencyCap(t *testing.T) {
	tenant := uuid.New()
	cfg := agentConfig(tenant)
	configRepo := &mockConfigRepo{getByIDFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.ScanConfig, error) { return cfg, nil }}
	jobRepo := &mockJobRepo{countActiveFunc: func(_ context.Context, _ uuid.UUID) (int64, error) {
		return scanpkg.MaxConcurrentScansPerTenant, nil
	}}
	uc := newTriggerUC(newFakeKV(), configRepo, jobRepo)
	_, err := uc.Execute(context.Background(), tenant, uuid.New(), cfg.ID)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrConflict)
}

func TestTriggerScan_ConfigLockHeld(t *testing.T) {
	tenant := uuid.New()
	cfg := agentConfig(tenant)
	kv := newFakeKV()
	// Pre-take the config lock so AcquireConfig returns false.
	_, _ = kv.SetNX(context.Background(), "scan:lock:config:"+tenant.String()+":"+cfg.ID.String(), "held", 0)
	configRepo := &mockConfigRepo{getByIDFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.ScanConfig, error) { return cfg, nil }}
	uc := newTriggerUC(kv, configRepo, &mockJobRepo{})
	_, err := uc.Execute(context.Background(), tenant, uuid.New(), cfg.ID)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrConflict)
}
