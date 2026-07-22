// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package scanmitigation implements scanner.MitigationAutoDetector: it turns a
// remediated finding (a CVE that a follow-up scan no longer detects) into an
// auto-completed mitigation sub-action on the linked risk, and publishes the
// mitigation.auto_completed event for the SSE stream.
package scanmitigation

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	appmitigation "github.com/opendefender/openrisk/internal/application/mitigation"
	"github.com/opendefender/openrisk/internal/domain"
	redisinfra "github.com/opendefender/openrisk/internal/infrastructure/redis"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/internal/scanner"
)

// Detector wires the scanner's remediation diff to the mitigation sub-actions.
type Detector struct {
	db           *gorm.DB
	autoComplete *appmitigation.AutoCompleteSubActionUseCase
	subRepo      repository.MitigationSubActionRepository
	redis        *redisinfra.Client
	logger       zerolog.Logger
}

// NewDetector builds the detector.
func NewDetector(
	db *gorm.DB,
	autoComplete *appmitigation.AutoCompleteSubActionUseCase,
	subRepo repository.MitigationSubActionRepository,
	redis *redisinfra.Client,
	logger zerolog.Logger,
) *Detector {
	return &Detector{db: db, autoComplete: autoComplete, subRepo: subRepo, redis: redis, logger: logger}
}

// OnRemediated implements scanner.MitigationAutoDetector. For each remediated CVE
// it walks: scanner finding → asset (by external id) → risk(s) with that
// source_cve_id → mitigation plan(s) → incomplete sub-action(s), auto-completing
// each (CompletedSource=scanner, CompletedBy=nil, AutoDetectedAt=now) and
// publishing mitigation.auto_completed. Best-effort: individual failures are
// logged and skipped so one bad link never derails a scan.
func (d *Detector) OnRemediated(ctx context.Context, tenantID, scanJobID uuid.UUID, remediated []scanner.AutoMitigation) {
	for _, m := range remediated {
		if m.CVE == nil || *m.CVE == "" {
			continue // only CVE-linked remediations map to a risk
		}
		cve := *m.CVE

		// scanner finding → asset (by tenant + external id).
		var asset domain.Asset
		if err := d.db.WithContext(ctx).
			Where("tenant_id = ? AND external_id = ? AND deleted_at IS NULL", tenantID, m.AssetExternalID).
			First(&asset).Error; err != nil {
			continue
		}

		// asset + CVE → risk(s) auto-created from that CVE.
		var risks []domain.Risk
		if err := d.db.WithContext(ctx).
			Where("tenant_id = ? AND asset_id = ? AND source_cve_id = ? AND deleted_at IS NULL", tenantID, asset.ID, cve).
			Find(&risks).Error; err != nil {
			continue
		}

		for _, risk := range risks {
			var mits []domain.Mitigation
			if err := d.db.WithContext(ctx).Where("risk_id = ?", risk.ID).Find(&mits).Error; err != nil {
				continue
			}
			for _, mit := range mits {
				subs, err := d.subRepo.List(tenantID.String(), mit.ID)
				if err != nil {
					continue
				}
				for _, sub := range subs {
					if sub.Completed {
						continue // never override a manual/prior completion
					}
					evidence := fmt.Sprintf("Scanner %s: %s", scanJobID.String(), m.Evidence)

					if err := d.autoComplete.Execute(appmitigation.AutoCompleteSubActionInput{
						TenantID:     tenantID,
						SubActionID:  sub.ID,
						ScannerJobID: scanJobID.String(),
						Evidence:     evidence,
					}); err != nil {
						d.logger.Warn().Err(err).Str("sub_action_id", sub.ID.String()).Msg("scanner auto-complete failed")
						continue
					}

					// Publish mitigation.auto_completed (tenant_id, plan_id,
					// sub_action_id, scanner_run_id) → SSE stream + event worker.
					if err := d.redis.Publish(ctx, "mitigation.auto_completed", &domain.MitigationAutoCompleted{
						TenantID:     tenantID,
						PlanID:       mit.ID,
						SubActionID:  sub.ID,
						ScannerJobID: scanJobID.String(),
						Evidence:     evidence,
					}); err != nil {
						d.logger.Warn().Err(err).Msg("failed to publish mitigation.auto_completed")
					}

					d.logger.Info().
						Str("cve", cve).
						Str("plan_id", mit.ID.String()).
						Str("sub_action_id", sub.ID.String()).
						Msg("scanner auto-completed mitigation sub-action")
				}
			}
		}
	}
}
