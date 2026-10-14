// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package inventory

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	scanpkg "github.com/opendefender/openrisk/internal/scanner"
)

// TriggerKV is what the manual trigger needs from Redis: a per-tenant lock and
// the agents' job channel.
type TriggerKV interface {
	SetNX(ctx context.Context, key, value string, ttl time.Duration) (bool, error)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
	Publish(ctx context.Context, channel string, payload interface{}) error
}

// PendingKey marks an inventory job an agent has not answered yet. Redis
// pub/sub is fire-and-forget and an agent's stream reconnects now and then, so
// the stream replays a pending job on connect (as it replays queued scan jobs)
// and the agent's inventory clears the mark. Found in the #910 live run: a job
// published during a reconnect was otherwise lost.
func PendingKey(agentID string) string { return "inventory:pending:" + agentID }

// AgentLister lists a tenant's agents.
type AgentLister interface {
	List(ctx context.Context, tenantID uuid.UUID) ([]domain.ScannerAgent, error)
}

// ScanCooldown is the minimum time between two manual scans of a tenant.
const ScanCooldown = 10 * time.Minute

// TriggerResult says how many agents were asked for an inventory.
type TriggerResult struct {
	Agents int `json:"agents"`
}

// TriggerScanUseCase is "Lancer un scan" (#910): every online agent of the
// tenant reports its host's packages, each inventory is then matched as it
// arrives. At most one per tenant every ScanCooldown.
type TriggerScanUseCase struct {
	agents AgentLister
	kv     TriggerKV
}

// NewTriggerScanUseCase builds the use case.
func NewTriggerScanUseCase(a AgentLister, kv TriggerKV) *TriggerScanUseCase {
	return &TriggerScanUseCase{agents: a, kv: kv}
}

// ErrScanCooldown is returned when a scan was started less than ScanCooldown ago.
var ErrScanCooldown = &domain.AppError{
	Err:     domain.ErrConflict,
	Message: "a scan was started less than 10 minutes ago",
	Code:    http.StatusTooManyRequests,
}

// Execute dispatches an inventory job to the tenant's online agents.
func (uc *TriggerScanUseCase) Execute(ctx context.Context, tenantID uuid.UUID) (*TriggerResult, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("no tenant")
	}
	agents, err := uc.agents.List(ctx, tenantID)
	if err != nil {
		return nil, domain.NewInternalError("could not list agents: " + err.Error())
	}
	var online []string
	for _, a := range agents {
		if a.TenantID == tenantID && (a.Status == domain.AgentOnline || a.Status == domain.AgentScanning) {
			online = append(online, a.ID.String())
		}
	}
	if len(online) == 0 {
		return nil, domain.NewNotFoundError("online agent", tenantID)
	}
	won, err := uc.kv.SetNX(ctx, "inventory:scan:"+tenantID.String(), "1", ScanCooldown)
	if err != nil {
		return nil, domain.NewInternalError("could not take the scan lock: " + err.Error())
	}
	if !won {
		return nil, ErrScanCooldown
	}
	dispatch := scanpkg.AgentJobDispatch{
		Type: "inventory.job", JobID: uuid.New(), TenantID: tenantID, AgentIDs: online,
	}
	for _, id := range online {
		if err := uc.kv.Set(ctx, PendingKey(id), dispatch.JobID.String(), ScanCooldown); err != nil {
			return nil, domain.NewInternalError("could not queue the job: " + err.Error())
		}
	}
	if err := uc.kv.Publish(ctx, scanpkg.AgentJobChannel(tenantID), dispatch); err != nil {
		return nil, domain.NewInternalError("could not reach the agents: " + err.Error())
	}
	return &TriggerResult{Agents: len(online)}, nil
}

// RematchAllUseCase is the daily job: every stored inventory is matched again,
// so advisories published since land without the agents doing anything.
type RematchAllUseCase struct {
	repo    domain.HostInventoryRepository
	matcher Matcher
}

// NewRematchAllUseCase builds the job.
func NewRematchAllUseCase(r domain.HostInventoryRepository, m Matcher) *RematchAllUseCase {
	return &RematchAllUseCase{repo: r, matcher: m}
}

// Execute walks tenant by tenant; one host's failure never stops the others.
// It returns how many inventories were matched without error.
func (uc *RematchAllUseCase) Execute(ctx context.Context) int {
	tenants, err := uc.repo.TenantsWithInventories(ctx)
	if err != nil {
		return 0
	}
	ok := 0
	for _, t := range tenants {
		invs, err := uc.repo.List(ctx, t)
		if err != nil {
			continue
		}
		for _, inv := range invs {
			if inv.PackageCount == 0 || inv.MatchError == domain.InventoryErrNoOsquery {
				continue
			}
			if ctx.Err() != nil {
				return ok
			}
			if uc.matcher.MatchInventory(ctx, t, inv.ID) == nil {
				ok++
			}
		}
	}
	return ok
}

// ListAssetInventoriesUseCase returns the inventories attached to one of the
// tenant's assets, for the asset drawer: what was collected, when, and the
// state of its last match.
type ListAssetInventoriesUseCase struct {
	repo domain.HostInventoryRepository
}

// NewListAssetInventoriesUseCase builds the use case.
func NewListAssetInventoriesUseCase(r domain.HostInventoryRepository) *ListAssetInventoriesUseCase {
	return &ListAssetInventoriesUseCase{repo: r}
}

// Execute lists them; another tenant's asset id simply has none.
func (uc *ListAssetInventoriesUseCase) Execute(ctx context.Context, tenantID, assetID uuid.UUID) ([]domain.HostInventory, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("no tenant")
	}
	out, err := uc.repo.ListByAsset(ctx, tenantID, assetID)
	if err != nil {
		return nil, domain.NewInternalError("could not read inventories: " + err.Error())
	}
	if out == nil {
		out = []domain.HostInventory{}
	}
	return out, nil
}
