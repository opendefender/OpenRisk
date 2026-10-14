// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/opendefender/openrisk/internal/application/threatintel"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/cti"
)

// GormThreatRepository is the tenant side of the threat list (#911): the
// tenant's assets with CPEs, its findings that name a CVE, and its dismissals.
// Every query is filtered by tenant_id.
type GormThreatRepository struct {
	db *gorm.DB
}

// NewGormThreatRepository builds the repository.
func NewGormThreatRepository(db *gorm.DB) *GormThreatRepository {
	return &GormThreatRepository{db: db}
}

var _ threatintel.TenantSide = (*GormThreatRepository)(nil)

// AssetsWithCPEs lists the tenant's live assets that declare at least one CPE.
func (r *GormThreatRepository) AssetsWithCPEs(ctx context.Context, tenantID uuid.UUID) ([]threatintel.AssetCPEs, error) {
	var rows []struct {
		ID          uuid.UUID
		Name        string
		Criticality string
		CPEs        pq.StringArray `gorm:"column:cpes;type:text[]"`
	}
	err := r.db.WithContext(ctx).Model(&domain.Asset{}).
		Select("id, name, criticality, cpes").
		Where("tenant_id = ?", tenantID).
		Where("cpes IS NOT NULL").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]threatintel.AssetCPEs, 0, len(rows))
	for _, a := range rows {
		if len(a.CPEs) == 0 {
			continue
		}
		out = append(out, threatintel.AssetCPEs{
			ID: a.ID, Name: a.Name, Criticality: domain.AssetCriticality(a.Criticality), CPEs: a.CPEs,
		})
	}
	return out, nil
}

// FindingsWithCVE lists the tenant's unresolved findings that name a CVE. A
// finding accepted or remediated no longer makes the CVE a live threat.
func (r *GormThreatRepository) FindingsWithCVE(ctx context.Context, tenantID uuid.UUID) ([]threatintel.Finding, error) {
	var rows []struct {
		CVEID            string `gorm:"column:cve_id"`
		Title            string
		AssetID          *uuid.UUID
		AssetName        string
		AssetCriticality string
		EPSS             float64 `gorm:"column:epss"`
		Source           string
	}
	err := r.db.WithContext(ctx).Model(&domain.Vulnerability{}).
		Select("cve_id, title, asset_id, asset_name, asset_criticality, epss, source").
		Where("tenant_id = ?", tenantID).
		Where("cve_id <> '' AND status IN ?", domain.UnresolvedVulnStatuses).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]threatintel.Finding, 0, len(rows))
	for _, f := range rows {
		out = append(out, threatintel.Finding{
			CVEID: f.CVEID, Title: f.Title, AssetID: f.AssetID, AssetName: f.AssetName,
			Criticality: domain.AssetCriticality(f.AssetCriticality), EPSS: f.EPSS,
			Source: domain.VulnSource(f.Source),
		})
	}
	return out, nil
}

// Dismissals maps the tenant's dismissed CVEs to when they were dismissed.
func (r *GormThreatRepository) Dismissals(ctx context.Context, tenantID uuid.UUID) (map[string]time.Time, error) {
	var rows []domain.CTIThreatDismissal
	if err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string]time.Time, len(rows))
	for _, d := range rows {
		out[strings.ToUpper(d.CVEID)] = d.CreatedAt
	}
	return out, nil
}

// Dismiss records the dismissal; a second one for the same CVE is a no-op.
func (r *GormThreatRepository) Dismiss(ctx context.Context, d *domain.CTIThreatDismissal) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "cve_id"}},
			DoNothing: true,
		}).
		Create(d).Error
}

// Restore deletes the tenant's dismissal and says whether there was one.
func (r *GormThreatRepository) Restore(ctx context.Context, tenantID uuid.UUID, cveID string) (bool, error) {
	res := r.db.WithContext(ctx).
		Where("tenant_id = ? AND cve_id = ?", tenantID, cveID).
		Delete(&domain.CTIThreatDismissal{})
	return res.RowsAffected > 0, res.Error
}

// ByCPEs returns the catalogue entries whose affected CPEs intersect the list:
// the rule of MatchByAssetCPEs, without its per-asset risk exclusion. The
// catalogue is public reference data and carries no tenant_id; the CPEs come
// from one tenant's assets.
func (r *GormCTIRepository) ByCPEs(ctx context.Context, cpes []string) ([]cti.CTIVulnerability, error) {
	if len(cpes) == 0 {
		return nil, nil
	}
	var out []cti.CTIVulnerability
	// Tenant-agnostic: cti_vulnerabilities is public NVD/KEV data with no tenant_id.
	err := r.db.WithContext(ctx).Where("affected_cpe && ?", pq.Array(cpes)).Find(&out).Error
	return out, err
}

// ByCVEs returns the catalogue entries for the given identifiers.
func (r *GormCTIRepository) ByCVEs(ctx context.Context, cveIDs []string) ([]cti.CTIVulnerability, error) {
	if len(cveIDs) == 0 {
		return nil, nil
	}
	var out []cti.CTIVulnerability
	// Tenant-agnostic: cti_vulnerabilities is public NVD/KEV data with no tenant_id.
	err := r.db.WithContext(ctx).Where("UPPER(cve_id) IN ?", cveIDs).Find(&out).Error
	return out, err
}
