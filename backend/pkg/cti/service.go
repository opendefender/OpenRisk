// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package cti

import (
	"context"
	"encoding/json"
	"fmt"

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
func (s *service) SyncAll(ctx context.Context) error {
	// Example: fetch last 24h window from NVD (caller can craft URL)
	// The detailed param assembly and JSON parsing is implemented here at a minimal level.
	nvdURL := "https://services.nvd.nist.gov/rest/json/cves/2.0?resultsPerPage=2000"
	data, err := s.client.FetchNVD(ctx, nvdURL)
	if err != nil {
		return fmt.Errorf("nvd fetch failed: %w", err)
	}

	// Parse NVD response minimally to extract CVE items.
	// For now we store raw parsing placeholder: callers should extend JSON parsing to map fields.
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return fmt.Errorf("failed to parse nvd payload: %w", err)
	}

	// TODO: transform parsed into []CTIVulnerability. For now, no-op.
	_ = parsed

	// CISA KEV
	cisaURL := "https://www.cisa.gov/sites/default/files/feeds/known_exploited_vulnerabilities.json"
	_, _ = s.client.FetchCISAKEV(ctx, cisaURL)

	// Upsert is a no-op placeholder until parser implemented
	return nil
}

func (s *service) GetVulnerability(ctx context.Context, cveID string) (*CTIVulnerability, error) {
	return s.repo.GetByCVE(ctx, cveID)
}

func (s *service) Search(ctx context.Context, query string, filters CTIFilter) ([]CTIVulnerability, error) {
	res, _, err := s.repo.Search(ctx, query, filters, 50, 0)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (s *service) MatchAsset(ctx context.Context, tenantID uuid.UUID, assetID uuid.UUID) ([]CTIVulnerability, error) {
	// The matcher logic lives in matcher.go but we expose a convenience method
	// For now we call repo.FindByCPEOverlap with asset CPEs — asset CPE retrieval is external
	return nil, fmt.Errorf("MatchAsset requires asset CPEs from asset repository; use matcher package")
}
