// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package scanner

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/opendefender/openrisk/internal/domain"
	scanpkg "github.com/opendefender/openrisk/internal/scanner"
)

// CloudScanTimeout bounds an in-process cloud scan (mirrors the Agent's 15-minute
// hard timeout for on-prem scans).
const CloudScanTimeout = 15 * time.Minute

// TriggerScanUseCase queues (and, for cloud, runs) a scan for a ScanConfig.
//
// Invariants:
//   - max 1 active scan per config (per-config Redis lock);
//   - max 3 concurrent scans per tenant;
//   - the pipeline NEVER writes Assets/Risks — cloud runs land in a Redis
//     preview; agent jobs are dispatched over SSE and land in a preview on push.
type TriggerScanUseCase struct {
	configRepo domain.ScanConfigRepository
	jobRepo    domain.ScanJobRepository
	lock       *scanpkg.ScanLock
	registry   *scanpkg.Registry
	pipeline   *scanpkg.Pipeline
	cipher     *CredentialCipher
	kv         scanpkg.KV
	logger     zerolog.Logger
}

func NewTriggerScanUseCase(
	configRepo domain.ScanConfigRepository,
	jobRepo domain.ScanJobRepository,
	lock *scanpkg.ScanLock,
	registry *scanpkg.Registry,
	pipeline *scanpkg.Pipeline,
	cipher *CredentialCipher,
	kv scanpkg.KV,
	logger zerolog.Logger,
) *TriggerScanUseCase {
	return &TriggerScanUseCase{
		configRepo: configRepo, jobRepo: jobRepo, lock: lock, registry: registry,
		pipeline: pipeline, cipher: cipher, kv: kv, logger: logger,
	}
}

func (uc *TriggerScanUseCase) Execute(ctx context.Context, tenantID, triggeredBy, configID uuid.UUID) (*domain.ScanJob, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("missing tenant")
	}
	cfg, err := uc.configRepo.GetByID(ctx, configID, tenantID)
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}
	if cfg == nil {
		return nil, domain.NewNotFoundError("scan config", configID)
	}
	if !cfg.Enabled {
		return nil, domain.NewValidationError("scan config is disabled")
	}

	// Per-tenant concurrency cap.
	active, err := uc.jobRepo.CountActiveByTenant(ctx, tenantID)
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}
	if active >= scanpkg.MaxConcurrentScansPerTenant {
		return nil, domain.NewConflictError("scan", "max concurrent scans reached for tenant")
	}

	// Per-config lock: only one active scan per config.
	jobID := uuid.New()
	acquired, err := uc.lock.AcquireConfig(ctx, tenantID, configID, jobID)
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}
	if !acquired {
		return nil, domain.NewConflictError("scan", "a scan is already running for this config")
	}

	job := &domain.ScanJob{
		ID:          jobID,
		TenantID:    tenantID,
		ConfigID:    configID,
		Provider:    cfg.Provider,
		Status:      domain.ScanQueued,
		Targets:     cfg.Targets,
		TriggeredBy: triggeredBy,
	}
	if err := uc.jobRepo.Create(ctx, job); err != nil {
		_ = uc.lock.ReleaseConfig(ctx, tenantID, configID)
		return nil, domain.NewInternalError(err.Error())
	}

	if cfg.Provider.IsAgentBased() {
		// Dispatch to the tenant's on-prem Agents over SSE; the first to claim it
		// (Redis lock, on push) runs it. The config lock is held until the job
		// completes on push or its TTL lapses.
		uc.dispatchToAgents(ctx, cfg, job)
		return job, nil
	}

	// Cloud: run in the background (detached from the request context) and update
	// the job when done. Return the queued job immediately.
	now := time.Now()
	job.Status = domain.ScanRunning
	job.StartedAt = &now
	if err := uc.jobRepo.Update(ctx, job); err != nil {
		uc.logger.Warn().Err(err).Msg("scanner: could not mark job running")
	}
	go uc.runCloudJob(cfg, job)
	return job, nil
}

// dispatchToAgents publishes the queued job to the tenant's agent SSE channel.
func (uc *TriggerScanUseCase) dispatchToAgents(ctx context.Context, cfg *domain.ScanConfig, job *domain.ScanJob) {
	dispatch := scanpkg.AgentJobDispatch{
		Type:     "scan.job",
		JobID:    job.ID,
		ConfigID: cfg.ID,
		TenantID: cfg.TenantID,
		Provider: string(cfg.Provider),
		Targets:  cfg.Targets,
		AgentIDs: cfg.AgentIDs,
	}
	if err := uc.kv.Publish(ctx, scanpkg.AgentJobChannel(cfg.TenantID), dispatch); err != nil {
		uc.logger.Warn().Err(err).Msg("scanner: could not dispatch job to agents")
	}
}

// runCloudJob executes a cloud scan and finalises its preview. It runs on its
// own background context with a hard timeout and always releases the per-config
// lock. NEVER writes assets/risks — pipeline.Run stores a Redis preview only.
func (uc *TriggerScanUseCase) runCloudJob(cfg *domain.ScanConfig, job *domain.ScanJob) {
	ctx, cancel := context.WithTimeout(context.Background(), CloudScanTimeout)
	defer cancel()
	defer func() { _ = uc.lock.ReleaseConfig(ctx, cfg.TenantID, cfg.ID) }()

	creds, err := uc.cipher.DecryptCredentials(cfg.EncryptedCredentials)
	if err != nil {
		uc.failJob(ctx, job, "decrypt credentials failed")
		return
	}

	runtime := scanpkg.ScanConfig{
		ConfigID:    cfg.ID,
		TenantID:    cfg.TenantID,
		ScanJobID:   job.ID,
		Provider:    cfg.Provider,
		Credentials: creds,
		Regions:     cfg.Regions,
		Options:     nil,
	}
	meta := scanpkg.PreviewMeta{
		JobID:       job.ID,
		ConfigID:    cfg.ID,
		TenantID:    cfg.TenantID,
		Provider:    cfg.Provider,
		TriggeredBy: job.TriggeredBy,
	}

	preview, err := uc.pipeline.Run(ctx, runtime, meta)
	if err != nil {
		uc.failJob(ctx, job, err.Error())
		return
	}

	now := time.Now()
	job.Status = domain.ScanCompleted
	job.CompletedAt = &now
	job.AssetsFound = len(preview.Assets)
	job.FindingsFound = len(preview.Findings)
	job.PreviewKey = previewJobKey(cfg.TenantID, job.ID)
	if len(preview.Errors) > 0 {
		job.Error = strings.Join(preview.Errors, "; ")
	}
	if err := uc.jobRepo.Update(ctx, job); err != nil {
		uc.logger.Error().Err(err).Str("job_id", job.ID.String()).Msg("scanner: could not persist completed job")
	}
}

func (uc *TriggerScanUseCase) failJob(ctx context.Context, job *domain.ScanJob, reason string) {
	now := time.Now()
	job.Status = domain.ScanFailed
	job.CompletedAt = &now
	job.Error = reason
	if err := uc.jobRepo.Update(ctx, job); err != nil {
		uc.logger.Error().Err(err).Msg("scanner: could not persist failed job")
	}
}

// previewJobKey mirrors scanner.previewKey (unexported) so the job can point at
// its Redis preview. Kept in lockstep with internal/scanner/preview.go.
func previewJobKey(tenantID, jobID uuid.UUID) string {
	return "scan:preview:" + tenantID.String() + ":" + jobID.String()
}
