// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package cti

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/rs/zerolog"
)

// ====================================================================
// MITRE ATT&CK Static Data
// ====================================================================

// MITREMapping maps a CVE to MITRE ATT&CK techniques and tactics.
type MITREMapping struct {
	CVEID      string   `json:"cve_id"`
	Techniques []string `json:"techniques"`
	Tactics    []string `json:"tactics"`
}

// MITREData holds the static MITRE ATT&CK mapping loaded from JSON.
type MITREData struct {
	Mappings []MITREMapping `json:"mappings"`
}

// ====================================================================
// Sync Worker
// ====================================================================

// SyncWorker performs periodic synchronization of CTI sources (NVD, CISA KEV).
// Launched as a goroutine at server startup via DI.
//
// Schedule:
//   - NVD: every 1 hour (at minute 5: 00:05, 01:05, ...)
//   - CISA KEV: every 6 hours
//
// After each successful NVD sync, MatchCVEsToAllTenantAssets() is invoked.
type SyncWorker struct {
	repo       Repository
	client     *ExternalClient
	matcher    Matcher
	logger     zerolog.Logger
	mitreData  *MITREData
	nvdTick    time.Duration
	cisaTick   time.Duration
	stopCh     chan struct{}
}

// NewSyncWorker creates a new CTI sync worker.
func NewSyncWorker(
	repo Repository,
	client *ExternalClient,
	matcher Matcher,
	logger zerolog.Logger,
) *SyncWorker {
	w := &SyncWorker{
		repo:     repo,
		client:   client,
		matcher:  matcher,
		logger:   logger.With().Str("component", "cti_sync_worker").Logger(),
		nvdTick:  1 * time.Hour,
		cisaTick: 6 * time.Hour,
		stopCh:   make(chan struct{}),
	}

	// Load MITRE ATT&CK data from embedded JSON
	w.loadMITREData()

	return w
}

// Start launches both sync loops in background goroutines.
// Blocks until ctx is cancelled or Stop() is called.
func (w *SyncWorker) Start(ctx context.Context) {
	// Run initial sync immediately
	w.runNVDSync(ctx)
	w.runCISASync(ctx)

	nvdTicker := time.NewTicker(w.nvdTick)
	cisaTicker := time.NewTicker(w.cisaTick)
	defer nvdTicker.Stop()
	defer cisaTicker.Stop()

	w.logger.Info().
		Dur("nvd_interval", w.nvdTick).
		Dur("cisa_interval", w.cisaTick).
		Msg("CTI sync worker started")

	for {
		select {
		case <-ctx.Done():
			w.logger.Info().Msg("CTI sync worker shutting down (context cancelled)")
			return
		case <-w.stopCh:
			w.logger.Info().Msg("CTI sync worker stopped")
			return
		case <-nvdTicker.C:
			w.runNVDSync(ctx)
		case <-cisaTicker.C:
			w.runCISASync(ctx)
		}
	}
}

// Stop signals the worker to stop.
func (w *SyncWorker) Stop() {
	close(w.stopCh)
}

// ====================================================================
// NVD Sync (every 1 hour)
// ====================================================================

func (w *SyncWorker) runNVDSync(ctx context.Context) {
	start := time.Now()
	w.logger.Info().Msg("starting NVD sync")

	// Fetch last 2 hours window to ensure no gaps
	now := time.Now().UTC()
	pubEnd := now.Format("2006-01-02T15:04:05.000")
	pubStart := now.Add(-2 * time.Hour).Format("2006-01-02T15:04:05.000")

	var vulns []CTIVulnerability
	var lastErr error

	// Retry 3× with exponential backoff (1s → 3s → 9s)
	backoff := time.Second
	for attempt := 1; attempt <= 3; attempt++ {
		var err error
		vulns, err = w.client.FetchNVDCVEs(ctx, pubStart, pubEnd)
		if err == nil {
			lastErr = nil
			break
		}
		lastErr = err
		w.logger.Warn().
			Err(err).
			Int("attempt", attempt).
			Msg("NVD fetch attempt failed")

		if attempt < 3 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			backoff *= 3
		}
	}

	if lastErr != nil {
		w.logger.Error().
			Err(lastErr).
			Int64("duration_ms", time.Since(start).Milliseconds()).
			Msg("NVD sync failed after all retries")
		return
	}

	// Enrich with MITRE ATT&CK data
	w.enrichWithMITRE(vulns)

	// Upsert into database
	if err := w.repo.UpsertVulnerabilities(ctx, vulns); err != nil {
		w.logger.Error().
			Err(err).
			Int("cves_fetched", len(vulns)).
			Msg("NVD upsert failed")
		return
	}

	w.logger.Info().
		Int("cves_synced", len(vulns)).
		Int64("duration_ms", time.Since(start).Milliseconds()).
		Msg("NVD sync completed")

	// After successful NVD sync → trigger matching
	if w.matcher != nil {
		matchStart := time.Now()
		if err := w.matcher.MatchCVEsToAllTenantAssets(ctx); err != nil {
			w.logger.Error().
				Err(err).
				Msg("post-NVD matching failed")
		} else {
			w.logger.Info().
				Int64("match_duration_ms", time.Since(matchStart).Milliseconds()).
				Msg("post-NVD matching completed")
		}
	}
}

// ====================================================================
// CISA KEV Sync (every 6 hours)
// ====================================================================

func (w *SyncWorker) runCISASync(ctx context.Context) {
	start := time.Now()
	w.logger.Info().Msg("starting CISA KEV sync")

	var vulns []CTIVulnerability
	var lastErr error

	backoff := time.Second
	for attempt := 1; attempt <= 3; attempt++ {
		var err error
		vulns, err = w.client.FetchCISAKEV(ctx)
		if err == nil {
			lastErr = nil
			break
		}
		lastErr = err
		w.logger.Warn().
			Err(err).
			Int("attempt", attempt).
			Msg("CISA KEV fetch attempt failed")

		if attempt < 3 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			backoff *= 3
		}
	}

	if lastErr != nil {
		w.logger.Error().
			Err(lastErr).
			Int64("duration_ms", time.Since(start).Milliseconds()).
			Msg("CISA KEV sync failed after all retries")
		return
	}

	// Enrich with MITRE data
	w.enrichWithMITRE(vulns)

	if err := w.repo.UpsertVulnerabilities(ctx, vulns); err != nil {
		w.logger.Error().
			Err(err).
			Int("cves_fetched", len(vulns)).
			Msg("CISA KEV upsert failed")
		return
	}

	w.logger.Info().
		Int("cves_synced", len(vulns)).
		Int64("duration_ms", time.Since(start).Milliseconds()).
		Msg("CISA KEV sync completed")
}

// ====================================================================
// MITRE ATT&CK Enrichment
// ====================================================================

func (w *SyncWorker) loadMITREData() {
	// Determine path relative to this source file
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		w.logger.Warn().Msg("could not determine MITRE data path, skipping enrichment")
		return
	}
	dataPath := filepath.Join(filepath.Dir(filename), "data", "mitre_attack.json")

	data, err := os.ReadFile(dataPath)
	if err != nil {
		w.logger.Warn().
			Str("path", dataPath).
			Msg("MITRE ATT&CK data file not found, enrichment disabled")
		return
	}

	var mitreData MITREData
	if err := json.Unmarshal(data, &mitreData); err != nil {
		w.logger.Error().
			Err(err).
			Msg("failed to parse MITRE ATT&CK data")
		return
	}

	w.mitreData = &mitreData
	w.logger.Info().
		Int("mappings", len(mitreData.Mappings)).
		Msg("MITRE ATT&CK data loaded")
}

func (w *SyncWorker) enrichWithMITRE(vulns []CTIVulnerability) {
	if w.mitreData == nil {
		return
	}

	// Build lookup map
	lookup := make(map[string]MITREMapping, len(w.mitreData.Mappings))
	for _, m := range w.mitreData.Mappings {
		lookup[m.CVEID] = m
	}

	for i := range vulns {
		if mapping, ok := lookup[vulns[i].CVEID]; ok {
			vulns[i].MitreTactics = mapping.Tactics
			vulns[i].MitreTechniques = mapping.Techniques
		}
	}
}

// SyncAll performs a full synchronization of all CTI sources.
// Convenience method that triggers both NVD and CISA syncs.
func (w *SyncWorker) SyncAll(ctx context.Context) error {
	w.runNVDSync(ctx)
	w.runCISASync(ctx)
	return nil
}

// ====================================================================
// MITRE Data Helper — create empty initial data file
// ====================================================================

// EnsureMITREDataFile creates the MITRE data directory and an empty JSON file
// if they don't exist. This is called during initialization.
func EnsureMITREDataFile(basePath string) error {
	dataDir := filepath.Join(basePath, "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create MITRE data directory: %w", err)
	}

	dataFile := filepath.Join(dataDir, "mitre_attack.json")
	if _, err := os.Stat(dataFile); os.IsNotExist(err) {
		emptyData := MITREData{Mappings: []MITREMapping{}}
		jsonData, _ := json.MarshalIndent(emptyData, "", "  ")
		if err := os.WriteFile(dataFile, jsonData, 0644); err != nil {
			return fmt.Errorf("failed to create MITRE data file: %w", err)
		}
	}
	return nil
}
