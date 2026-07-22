// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package cti

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// service is the concrete implementation of Service
type service struct {
	repo   Repository
	client *ExternalClient
}

// NewService constructs a new CTI service
func NewService(repo Repository, client *ExternalClient) Service {
	return &service{repo: repo, client: client}
}

// SyncAll fetches NVD and CISA feeds and upserts vulnerabilities.
// Fetches a 24h NVD window; ExternalClient already returns parsed
// []CTIVulnerability, so no manual JSON parsing is needed here.
func (s *service) SyncAll(ctx context.Context) error {
	now := time.Now().UTC()
	pubEnd := now.Format("2006-01-02T15:04:05.000")
	pubStart := now.Add(-24 * time.Hour).Format("2006-01-02T15:04:05.000")

	nvdVulns, err := s.client.FetchNVDCVEs(ctx, pubStart, pubEnd)
	if err != nil {
		return fmt.Errorf("nvd fetch failed: %w", err)
	}
	if err := s.repo.UpsertVulnerabilities(ctx, nvdVulns); err != nil {
		return fmt.Errorf("nvd upsert failed: %w", err)
	}

	kevVulns, err := s.client.FetchCISAKEV(ctx)
	if err != nil {
		return fmt.Errorf("cisa kev fetch failed: %w", err)
	}
	if err := s.repo.UpsertVulnerabilities(ctx, kevVulns); err != nil {
		return fmt.Errorf("cisa kev upsert failed: %w", err)
	}

	return nil
}

func (s *service) GetVulnerability(ctx context.Context, cveID string) (*CTIVulnerability, error) {
	return s.repo.GetByCVE(ctx, cveID)
}

func (s *service) Search(ctx context.Context, query string, filters CTIFilter) ([]CTIVulnerability, error) {
	if filters.Limit <= 0 {
		filters.Limit = 50
	}
	res, _, err := s.repo.Search(ctx, query, filters)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (s *service) MatchAsset(ctx context.Context, tenantID uuid.UUID, assetID uuid.UUID) ([]CTIVulnerability, error) {
	// The matching logic (Repository.MatchByAssetCPEs) lives behind CPEMatcher
	// in matcher.go; it needs the asset's CPE list, which this service does not
	// have (asset retrieval is an external dependency, not part of pkg/cti).
	return nil, fmt.Errorf("MatchAsset requires asset CPEs from asset repository; use CPEMatcher")
}
