// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// PreviewTTL is how long a scan preview lives in Redis before it self-destructs.
// The user imports/ignores within this window; after it, the preview is gone and
// nothing was ever written to the DB.
const PreviewTTL = 48 * time.Hour

// latestPointerTTL keeps the "last preview for this config" pointer a bit longer
// than a single preview so auto-mitigation detection can diff against it even if
// the previous preview itself just expired.
const latestPointerTTL = 72 * time.Hour

// KV is the minimal Redis surface the preview store needs. *redis.Client
// satisfies it, but depending on the interface keeps this package unit-testable
// with an in-memory fake and free of an infrastructure import.
type KV interface {
	Set(ctx context.Context, key, value string, ttl time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Del(ctx context.Context, keys ...string) error
	Publish(ctx context.Context, channel string, payload interface{}) error
}

// PreviewStore persists ScanPreviews in Redis, tenant-scoped by key.
type PreviewStore struct {
	kv KV
}

func NewPreviewStore(kv KV) *PreviewStore { return &PreviewStore{kv: kv} }

// previewKey is intentionally tenant-scoped in the key itself: a preview can
// only be read back by supplying the owning tenant, so one tenant can never load
// another's preview even with a guessed job ID.
func previewKey(tenantID, jobID uuid.UUID) string {
	return fmt.Sprintf("scan:preview:%s:%s", tenantID, jobID)
}

func latestKey(tenantID, configID uuid.UUID) string {
	return fmt.Sprintf("scan:latest:%s:%s", tenantID, configID)
}

// Store writes the preview (48h TTL) and updates the per-config "latest" pointer
// used for auto-mitigation diffing.
func (s *PreviewStore) Store(ctx context.Context, p *ScanPreview) error {
	data, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("marshal preview: %w", err)
	}
	if err := s.kv.Set(ctx, previewKey(p.TenantID, p.JobID), string(data), PreviewTTL); err != nil {
		return fmt.Errorf("store preview: %w", err)
	}
	// Best-effort latest pointer; a failure here only weakens the NEXT scan's
	// mitigation diff, so it must not fail the scan.
	_ = s.kv.Set(ctx, latestKey(p.TenantID, p.ConfigID), p.JobID.String(), latestPointerTTL)
	return nil
}

// Load returns the preview for (tenant, job) or (nil, nil) if it expired / never
// existed.
func (s *PreviewStore) Load(ctx context.Context, tenantID, jobID uuid.UUID) (*ScanPreview, error) {
	raw, err := s.kv.Get(ctx, previewKey(tenantID, jobID))
	if err != nil {
		return nil, fmt.Errorf("load preview: %w", err)
	}
	if raw == "" {
		return nil, nil
	}
	var p ScanPreview
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return nil, fmt.Errorf("unmarshal preview: %w", err)
	}
	return &p, nil
}

// LoadLatestForConfig returns the most recent stored preview for a config, used
// to diff findings for auto-mitigation detection. Returns (nil, nil) when there
// is no prior scan to compare against.
func (s *PreviewStore) LoadLatestForConfig(ctx context.Context, tenantID, configID uuid.UUID) (*ScanPreview, error) {
	jobStr, err := s.kv.Get(ctx, latestKey(tenantID, configID))
	if err != nil || jobStr == "" {
		return nil, err
	}
	jobID, err := uuid.Parse(jobStr)
	if err != nil {
		return nil, nil
	}
	return s.Load(ctx, tenantID, jobID)
}

// Delete removes a preview once the user has imported or ignored it.
func (s *PreviewStore) Delete(ctx context.Context, tenantID, jobID uuid.UUID) error {
	return s.kv.Del(ctx, previewKey(tenantID, jobID))
}
