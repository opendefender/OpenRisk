// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package scanner

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/opendefender/openrisk/internal/domain"
)

// PreviewMeta is the identity of a scan run, attached to the preview it produces.
type PreviewMeta struct {
	JobID       uuid.UUID
	ConfigID    uuid.UUID
	TenantID    uuid.UUID
	Provider    domain.ScannerProvider
	AgentID     *uuid.UUID
	AgentName   string
	TriggeredBy uuid.UUID
}

// Pipeline turns raw discoveries into a stored preview:
//
//	Validate → Scan → Normalize → Deduplicate → StorePreview → Notify
//
// It NEVER creates Assets or Risks in the database. The result lives only in the
// Redis preview until the user imports/ignores it. Cloud scans call Run (which
// drives a registered Scanner); agent pushes call Ingest with already-collected
// discoveries. Both converge on finalize().
type Pipeline struct {
	registry *Registry
	preview  *PreviewStore
	notifier Notifier
	logger   zerolog.Logger
	now      func() time.Time
}

func NewPipeline(reg *Registry, preview *PreviewStore, notifier Notifier, logger zerolog.Logger) *Pipeline {
	if notifier == nil {
		notifier = NoopNotifier{}
	}
	return &Pipeline{
		registry: reg,
		preview:  preview,
		notifier: notifier,
		logger:   logger,
		now:      time.Now,
	}
}

// Run validates the config, resolves the provider's Scanner, drains its three
// channels, and finalises a preview. Used for cloud scans executed in-process.
// It refuses agent-based providers — those are pushed by the Agent, not run here.
func (p *Pipeline) Run(ctx context.Context, sc ScanConfig, meta PreviewMeta) (*ScanPreview, error) {
	s, ok := p.registry.Get(sc.Provider)
	if !ok {
		return nil, domain.NewValidationError(fmt.Sprintf("no scanner registered for provider %q", sc.Provider))
	}
	if s.IsAgentBased() {
		return nil, domain.NewValidationError("agent-based providers push results; they are not run in the backend")
	}
	if err := s.Validate(ctx, sc); err != nil {
		return nil, err
	}

	assetCh, findingCh, errCh := s.Scan(ctx, sc)
	assets, findings, scanErrs := drain(ctx, assetCh, findingCh, errCh)
	return p.finalize(ctx, meta, assets, findings, scanErrs)
}

// Ingest finalises a preview from discoveries an Agent already collected and
// pushed. Same normalize→dedup→mitigation→store→notify tail as Run.
func (p *Pipeline) Ingest(ctx context.Context, meta PreviewMeta, assets []AssetDiscovery, findings []FindingDiscovery, scanErrs []string) (*ScanPreview, error) {
	return p.finalize(ctx, meta, assets, findings, scanErrs)
}

// finalize is the shared tail. Pure-ish: only touches Redis (preview + notify),
// never the SQL DB.
func (p *Pipeline) finalize(ctx context.Context, meta PreviewMeta, assets []AssetDiscovery, findings []FindingDiscovery, scanErrs []string) (*ScanPreview, error) {
	// Normalize.
	for i := range assets {
		assets[i] = normalizeAsset(assets[i])
		assets[i].ScanJobID = meta.JobID
		assets[i].AgentID = meta.AgentID
	}
	for i := range findings {
		findings[i] = normalizeFinding(findings[i])
		findings[i].ScanJobID = meta.JobID
		findings[i].AgentID = meta.AgentID
	}

	// Deduplicate (by ExternalID + tenant — the pipeline is per-tenant already).
	assets = dedupeAssets(assets)
	findings = dedupeFindings(findings)

	// Auto-mitigation: diff against the last preview for this config.
	var mitigations []AutoMitigation
	if prev, err := p.preview.LoadLatestForConfig(ctx, meta.TenantID, meta.ConfigID); err != nil {
		p.logger.Warn().Err(err).Msg("scanner: could not load previous preview for mitigation diff")
	} else if prev != nil && prev.JobID != meta.JobID {
		mitigations = detectMitigations(prev.Findings, findings, p.now())
	}

	now := p.now()
	preview := &ScanPreview{
		JobID:       meta.JobID,
		ConfigID:    meta.ConfigID,
		TenantID:    meta.TenantID,
		Provider:    meta.Provider,
		AgentID:     meta.AgentID,
		AgentName:   meta.AgentName,
		TriggeredBy: meta.TriggeredBy,
		Assets:      assets,
		Findings:    findings,
		Mitigations: mitigations,
		Errors:      scanErrs,
		CreatedAt:   now,
		ExpiresAt:   now.Add(PreviewTTL),
	}

	// StorePreview (Redis, 48h TTL). This is the ONLY persistence the pipeline does.
	if err := p.preview.Store(ctx, preview); err != nil {
		return nil, err
	}

	// Notify (SSE + in-app). Best-effort, never blocks the caller's result.
	p.notifier.ScanCompleted(ctx, preview)

	p.logger.Info().
		Str("tenant_id", meta.TenantID.String()).
		Str("job_id", meta.JobID.String()).
		Str("provider", string(meta.Provider)).
		Int("assets", len(assets)).
		Int("findings", len(findings)).
		Int("mitigations", len(mitigations)).
		Msg("scanner: preview stored")

	return preview, nil
}

// drain reads all three scanner channels until each is closed, respecting
// context cancellation. Errors are collected as strings (non-fatal per item).
func drain(ctx context.Context, assetCh <-chan AssetDiscovery, findingCh <-chan FindingDiscovery, errCh <-chan error) ([]AssetDiscovery, []FindingDiscovery, []string) {
	var (
		mu       sync.Mutex
		assets   []AssetDiscovery
		findings []FindingDiscovery
		errs     []string
		wg       sync.WaitGroup
	)

	wg.Add(3)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case a, ok := <-assetCh:
				if !ok {
					return
				}
				mu.Lock()
				assets = append(assets, a)
				mu.Unlock()
			}
		}
	}()
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case f, ok := <-findingCh:
				if !ok {
					return
				}
				mu.Lock()
				findings = append(findings, f)
				mu.Unlock()
			}
		}
	}()
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case e, ok := <-errCh:
				if !ok {
					return
				}
				if e != nil {
					mu.Lock()
					errs = append(errs, e.Error())
					mu.Unlock()
				}
			}
		}
	}()
	wg.Wait()

	if ctx.Err() != nil {
		errs = append(errs, ctx.Err().Error())
	}
	return assets, findings, errs
}
