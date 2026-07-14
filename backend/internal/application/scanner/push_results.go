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

	"github.com/opendefender/openrisk/internal/domain"
	scanpkg "github.com/opendefender/openrisk/internal/scanner"
)

// PushResultsInput carries what an authenticated Agent pushes back after running
// a job. The Agent is resolved by the auth middleware (bearer + HMAC), so it is
// passed in already-verified — never trusted from the body.
type PushResultsInput struct {
	Agent    *domain.ScannerAgent
	JobID    uuid.UUID
	Assets   []scanpkg.AssetDiscovery
	Findings []scanpkg.FindingDiscovery
	Errors   []string
}

// PushResultsUseCase ingests an Agent's scan results into a Redis preview. It
// claims the job (distributed lock — first agent wins), runs the pipeline
// (normalize→dedup→mitigation→preview→notify), and marks the job complete. It
// NEVER writes Assets/Risks: results stay in the preview until the user imports.
type PushResultsUseCase struct {
	agentRepo domain.ScannerAgentRepository
	jobRepo   domain.ScanJobRepository
	lock      *scanpkg.ScanLock
	pipeline  *scanpkg.Pipeline
}

func NewPushResultsUseCase(agentRepo domain.ScannerAgentRepository, jobRepo domain.ScanJobRepository, lock *scanpkg.ScanLock, pipeline *scanpkg.Pipeline) *PushResultsUseCase {
	return &PushResultsUseCase{agentRepo: agentRepo, jobRepo: jobRepo, lock: lock, pipeline: pipeline}
}

func (uc *PushResultsUseCase) Execute(ctx context.Context, in PushResultsInput) (*domain.ScanJob, error) {
	if in.Agent == nil || in.Agent.TenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("unauthenticated agent")
	}
	tenantID := in.Agent.TenantID

	job, err := uc.jobRepo.GetByID(ctx, in.JobID, tenantID)
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}
	if job == nil {
		return nil, domain.NewNotFoundError("scan job", in.JobID)
	}
	if !job.Provider.IsAgentBased() {
		return nil, domain.NewValidationError("this job is not an agent job")
	}
	if job.Status == domain.ScanCompleted {
		return nil, domain.NewConflictError("scan job", "already completed")
	}

	// Claim the job: the first agent to push wins the lock; others get a conflict.
	won, err := uc.lock.ClaimJob(ctx, tenantID, in.JobID, in.Agent.ID)
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}
	if !won {
		return nil, domain.NewConflictError("scan job", "already claimed by another agent")
	}

	now := time.Now()
	job.Status = domain.ScanRunning
	job.ClaimedByAgent = &in.Agent.ID
	job.StartedAt = &now
	if err := uc.jobRepo.Update(ctx, job); err != nil {
		return nil, domain.NewInternalError(err.Error())
	}

	meta := scanpkg.PreviewMeta{
		JobID:     job.ID,
		ConfigID:  job.ConfigID,
		TenantID:  tenantID,
		Provider:  job.Provider,
		AgentID:   &in.Agent.ID,
		AgentName: in.Agent.Name,
	}
	preview, err := uc.pipeline.Ingest(ctx, meta, in.Assets, in.Findings, in.Errors)
	if err != nil {
		job.Status = domain.ScanFailed
		job.Error = err.Error()
		completedAt := time.Now()
		job.CompletedAt = &completedAt
		_ = uc.jobRepo.Update(ctx, job)
		return nil, domain.NewInternalError(err.Error())
	}

	completedAt := time.Now()
	job.Status = domain.ScanCompleted
	job.CompletedAt = &completedAt
	job.AssetsFound = len(preview.Assets)
	job.FindingsFound = len(preview.Findings)
	job.PreviewKey = previewJobKey(tenantID, job.ID)
	if len(preview.Errors) > 0 {
		job.Error = strings.Join(preview.Errors, "; ")
	}
	if err := uc.jobRepo.Update(ctx, job); err != nil {
		return nil, domain.NewInternalError(err.Error())
	}

	// Agent bookkeeping: heartbeat + last job. Best-effort.
	in.Agent.Status = domain.AgentOnline
	in.Agent.LastHeartbeat = completedAt
	in.Agent.LastScanJobID = &job.ID
	_ = uc.agentRepo.Update(ctx, in.Agent)

	// Free the per-config lock the trigger held so the config can scan again.
	_ = uc.lock.ReleaseConfig(ctx, tenantID, job.ConfigID)

	return job, nil
}

// HeartbeatAgentUseCase updates an agent's liveness (called when it connects to
// the SSE stream and periodically thereafter).
type HeartbeatAgentUseCase struct {
	repo domain.ScannerAgentRepository
}

func NewHeartbeatAgentUseCase(repo domain.ScannerAgentRepository) *HeartbeatAgentUseCase {
	return &HeartbeatAgentUseCase{repo: repo}
}

func (uc *HeartbeatAgentUseCase) Execute(ctx context.Context, agent *domain.ScannerAgent, status domain.AgentStatus) error {
	if agent == nil || agent.TenantID == uuid.Nil {
		return domain.NewUnauthorizedError("unauthenticated agent")
	}
	agent.Status = status
	agent.LastHeartbeat = time.Now()
	if err := uc.repo.Update(ctx, agent); err != nil {
		return domain.NewInternalError(err.Error())
	}
	return nil
}
