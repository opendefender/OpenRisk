// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package scanner

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	entapp "github.com/opendefender/openrisk/internal/application/entitlements"
	"github.com/opendefender/openrisk/internal/domain"
	scanpkg "github.com/opendefender/openrisk/internal/scanner"
	ent "github.com/opendefender/openrisk/pkg/entitlements"
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
	// No type asked for: the vulnerability scan every scan ran before #1164.
	assert.Equal(t, domain.ScanVulnerability, job.ScanType)
	assert.Equal(t, "vulnerability", kv.last.(scanpkg.AgentJobDispatch).ScanType)
}

func TestTriggerScan_Success_BothTypes(t *testing.T) {
	for _, st := range []domain.ScanType{domain.ScanDiscovery, domain.ScanVulnerability} {
		t.Run(string(st), func(t *testing.T) {
			tenant := uuid.New()
			cfg := agentConfig(tenant)
			kv := newFakeKV()
			configRepo := &mockConfigRepo{getByIDFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.ScanConfig, error) { return cfg, nil }}
			var created *domain.ScanJob
			jobRepo := &mockJobRepo{createFunc: func(_ context.Context, j *domain.ScanJob) error { created = j; return nil }}
			uc := newTriggerUC(kv, configRepo, jobRepo)

			job, err := uc.ExecuteWithType(context.Background(), tenant, uuid.New(), cfg.ID, st)
			require.NoError(t, err)
			assert.Equal(t, st, job.ScanType)
			require.NotNil(t, created)
			assert.Equal(t, st, created.ScanType) // persisted, not only returned
			assert.Equal(t, string(st), kv.last.(scanpkg.AgentJobDispatch).ScanType)
		})
	}
}

func TestTriggerScan_InvalidType(t *testing.T) {
	tenant := uuid.New()
	cfg := agentConfig(tenant)
	configRepo := &mockConfigRepo{getByIDFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.ScanConfig, error) { return cfg, nil }}
	uc := newTriggerUC(newFakeKV(), configRepo, &mockJobRepo{})
	_, err := uc.ExecuteWithType(context.Background(), tenant, uuid.New(), cfg.ID, "compliance")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrValidation)
}

func TestTriggerScan_TypeOnCloudProvider(t *testing.T) {
	tenant := uuid.New()
	cfg := &domain.ScanConfig{ID: uuid.New(), TenantID: tenant, Provider: domain.ProviderAWS, Enabled: true}
	configRepo := &mockConfigRepo{getByIDFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.ScanConfig, error) { return cfg, nil }}
	jobRepo := &mockJobRepo{createFunc: func(_ context.Context, _ *domain.ScanJob) error {
		t.Fatal("no job may be created when the type is refused")
		return nil
	}}
	uc := newTriggerUC(newFakeKV(), configRepo, jobRepo)
	_, err := uc.ExecuteWithType(context.Background(), tenant, uuid.New(), cfg.ID, domain.ScanDiscovery)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrValidation)
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

// fixedScanQuota answers the monthly scan cap with fixed numbers.
type fixedScanQuota struct{ limit, used int }

func (q fixedScanQuota) Capacity(_ context.Context, _ uuid.UUID, _ ent.LimitKey) (bool, int, int, ent.Plan, error) {
	return q.used < q.limit, q.limit, q.used, ent.PlanFree, nil
}

// TestTriggerScanQuota_* (D-078): the monthly cap lives in the use case, so the
// scheduler and automation, which call Execute directly, are held to it too.
func TestTriggerScanQuota_Success(t *testing.T) {
	tenant := uuid.New()
	cfg := agentConfig(tenant)
	configRepo := &mockConfigRepo{getByIDFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.ScanConfig, error) { return cfg, nil }}
	uc := newTriggerUC(newFakeKV(), configRepo, &mockJobRepo{}).WithQuota(fixedScanQuota{limit: 4, used: 3})

	_, err := uc.Execute(context.Background(), tenant, uuid.New(), cfg.ID)
	require.NoError(t, err)
}

func TestTriggerScanQuota_Unauthorized(t *testing.T) {
	tenant := uuid.New()
	cfg := agentConfig(tenant)
	configRepo := &mockConfigRepo{getByIDFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.ScanConfig, error) { return cfg, nil }}
	kv := newFakeKV()
	uc := newTriggerUC(kv, configRepo, &mockJobRepo{}).WithQuota(fixedScanQuota{limit: 4, used: 4})

	_, err := uc.Execute(context.Background(), tenant, uuid.Nil, cfg.ID)
	var lim *domain.LimitReachedError
	require.ErrorAs(t, err, &lim)
	assert.Equal(t, "scans_per_month", lim.Key)
	assert.Equal(t, 4, lim.Limit)
	assert.Equal(t, 0, kv.messages, "a refused scan dispatches nothing")
}

func TestTriggerScanQuota_NotFound(t *testing.T) {
	// The config is looked up first: an unknown config is NotFound even at the cap.
	configRepo := &mockConfigRepo{getByIDFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.ScanConfig, error) { return nil, nil }}
	uc := newTriggerUC(newFakeKV(), configRepo, &mockJobRepo{}).WithQuota(fixedScanQuota{limit: 4, used: 4})
	_, err := uc.Execute(context.Background(), uuid.New(), uuid.New(), uuid.New())
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

// TestTriggerScanQuota_EveryCaller_Unauthorized (#783 criterion 6): the console
// passes the user; the scheduler (workers.systemUser) and the automation scan
// action both pass uuid.Nil. Each at 4/4 is refused before a job row is written.
func TestTriggerScanQuota_EveryCaller_Unauthorized(t *testing.T) {
	for caller, actor := range map[string]uuid.UUID{
		"console":                uuid.New(),
		"scheduler / automation": uuid.Nil,
	} {
		tenant := uuid.New()
		cfg := agentConfig(tenant)
		configRepo := &mockConfigRepo{getByIDFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.ScanConfig, error) { return cfg, nil }}
		created := 0
		jobRepo := &mockJobRepo{createFunc: func(context.Context, *domain.ScanJob) error { created++; return nil }}
		uc := newTriggerUC(newFakeKV(), configRepo, jobRepo).WithQuota(fixedScanQuota{limit: 4, used: 4})

		_, err := uc.Execute(context.Background(), tenant, actor, cfg.ID)
		var lim *domain.LimitReachedError
		require.ErrorAs(t, err, &lim, caller)
		assert.Equal(t, 0, created, "%s: a refused scan writes no job row", caller)
	}
}

// TestTriggerScanQuota_SelfHosted_Success (#783 criterion 8): through the real
// entitlements service, a self-hosted install is never held to Free's four.
func TestTriggerScanQuota_SelfHosted_Success(t *testing.T) {
	tenant := uuid.New()
	cfg := agentConfig(tenant)
	configRepo := &mockConfigRepo{getByIDFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.ScanConfig, error) { return cfg, nil }}
	svc := entapp.NewService(freeOrg{}).WithUsage(scanUsage(10000)).WithDeployment(ent.DeploymentSelfHosted)
	uc := newTriggerUC(newFakeKV(), configRepo, &mockJobRepo{}).WithQuota(svc)

	_, err := uc.Execute(context.Background(), tenant, uuid.New(), cfg.ID)
	require.NoError(t, err)

	hosted := newTriggerUC(newFakeKV(), configRepo, &mockJobRepo{}).
		WithQuota(entapp.NewService(freeOrg{}).WithUsage(scanUsage(4)))
	_, err = hosted.Execute(context.Background(), tenant, uuid.New(), cfg.ID)
	var lim *domain.LimitReachedError
	require.ErrorAs(t, err, &lim, "the same stored Free plan, hosted, is refused at 4")
}

type freeOrg struct{}

func (freeOrg) PlanAndRegion(context.Context, uuid.UUID) (string, string, error) {
	return "free", "eu", nil
}

type scanUsage int

func (u scanUsage) Count(context.Context, uuid.UUID, ent.LimitKey) (int, error) { return int(u), nil }
