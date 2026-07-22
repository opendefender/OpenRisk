// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package scanner

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// MaxConcurrentScansPerTenant caps how many scans a tenant can run at once
// (enforced together with the per-config lock). Master Prompt V5: 1 active scan
// per config + max 3 simultaneous per tenant.
const MaxConcurrentScansPerTenant = 3

// ConfigLockTTL bounds a per-config lock so a crashed runner can't wedge a
// config forever. A little over the agent's 15-minute hard timeout.
const ConfigLockTTL = 20 * time.Minute

// JobLockTTL bounds the per-job claim lock (only the first agent that claims a
// job may push its results).
const JobLockTTL = 20 * time.Minute

// Locker is the atomic SET-NX / DEL surface the scan locks need. *redis.Client
// satisfies it.
type Locker interface {
	SetNX(ctx context.Context, key, value string, ttl time.Duration) (bool, error)
	Del(ctx context.Context, keys ...string) error
}

// ScanLock provides the two distributed locks the engine relies on: one per
// config (only one active scan per config) and one per job (only one agent
// claims a queued job).
type ScanLock struct{ locker Locker }

func NewScanLock(l Locker) *ScanLock { return &ScanLock{locker: l} }

func configLockKey(tenantID, configID uuid.UUID) string {
	return fmt.Sprintf("scan:lock:config:%s:%s", tenantID, configID)
}

func jobLockKey(tenantID, jobID uuid.UUID) string {
	return fmt.Sprintf("scan:lock:job:%s:%s", tenantID, jobID)
}

// AcquireConfig tries to take the per-config lock. Returns false (no error) when
// a scan for that config is already running.
func (l *ScanLock) AcquireConfig(ctx context.Context, tenantID, configID, jobID uuid.UUID) (bool, error) {
	return l.locker.SetNX(ctx, configLockKey(tenantID, configID), jobID.String(), ConfigLockTTL)
}

// ReleaseConfig frees the per-config lock (best-effort; the TTL is the backstop).
func (l *ScanLock) ReleaseConfig(ctx context.Context, tenantID, configID uuid.UUID) error {
	return l.locker.Del(ctx, configLockKey(tenantID, configID))
}

// ClaimJob tries to claim a queued job for an agent. Returns false when another
// agent already claimed it (the distributed lock that makes "first available
// agent takes the job" safe).
func (l *ScanLock) ClaimJob(ctx context.Context, tenantID, jobID, agentID uuid.UUID) (bool, error) {
	return l.locker.SetNX(ctx, jobLockKey(tenantID, jobID), agentID.String(), JobLockTTL)
}

// ReleaseJob frees the per-job claim lock.
func (l *ScanLock) ReleaseJob(ctx context.Context, tenantID, jobID uuid.UUID) error {
	return l.locker.Del(ctx, jobLockKey(tenantID, jobID))
}
