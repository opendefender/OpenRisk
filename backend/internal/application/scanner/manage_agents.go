// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package scanner

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	scanpkg "github.com/opendefender/openrisk/internal/scanner"
)

// agentOfflineAfter is how long without a heartbeat before List reports an agent
// as offline (computed on read; the stored status is only advisory).
const agentOfflineAfter = 90 * time.Second

// ListAgentsUseCase returns a tenant's registered agents, downgrading stale
// "online" agents to "offline" based on their last heartbeat.
type ListAgentsUseCase struct {
	repo domain.ScannerAgentRepository
}

func NewListAgentsUseCase(repo domain.ScannerAgentRepository) *ListAgentsUseCase {
	return &ListAgentsUseCase{repo: repo}
}

func (uc *ListAgentsUseCase) Execute(ctx context.Context, tenantID uuid.UUID) ([]domain.ScannerAgent, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("missing tenant")
	}
	agents, err := uc.repo.List(ctx, tenantID)
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}
	cutoff := time.Now().Add(-agentOfflineAfter)
	for i := range agents {
		agents[i].TokenHash = ""
		agents[i].PushSecretEnc = ""
		if agents[i].Status == domain.AgentOnline && agents[i].LastHeartbeat.Before(cutoff) {
			agents[i].Status = domain.AgentOffline
		}
	}
	return agents, nil
}

// RevokeAgentUseCase instantly disables an agent: its token hash is cleared (so
// GetByTokenHash can never authenticate it again) and its status set to revoked.
// A revoked agent must re-register to come back.
type RevokeAgentUseCase struct {
	repo domain.ScannerAgentRepository
	kv   scanpkg.KV
}

func NewRevokeAgentUseCase(repo domain.ScannerAgentRepository, kv scanpkg.KV) *RevokeAgentUseCase {
	return &RevokeAgentUseCase{repo: repo, kv: kv}
}

func (uc *RevokeAgentUseCase) Execute(ctx context.Context, tenantID, agentID uuid.UUID) error {
	if tenantID == uuid.Nil {
		return domain.NewUnauthorizedError("missing tenant")
	}
	agent, err := uc.repo.GetByID(ctx, agentID, tenantID)
	if err != nil {
		return domain.NewInternalError(err.Error())
	}
	if agent == nil {
		return domain.NewNotFoundError("agent", agentID)
	}
	agent.Status = domain.AgentRevoked
	agent.TokenHash = ""     // kill the current token
	agent.PushSecretEnc = "" // kill the push secret
	if err := uc.repo.Update(ctx, agent); err != nil {
		return domain.NewInternalError(err.Error())
	}
	// Tell the agent (if connected) to shut its stream down.
	_ = uc.kv.Publish(ctx, scanpkg.AgentJobChannel(tenantID), scanpkg.AgentJobDispatch{
		Type:     "agent.revoked",
		TenantID: tenantID,
	})
	return nil
}
