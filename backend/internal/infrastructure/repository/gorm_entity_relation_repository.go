// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/application/entity"
)

// GormEntityRelationRepository answers the universal drawer's relation queries.
//
// It exists as ONE type rather than as methods added to each module's repository
// for a reason worth stating: relations are the drawer's cross-tenant risk
// surface. An asset's drawer reaches into risks, vulnerabilities and incidents; a
// risk's reaches into controls and mitigations. Scattering those joins across
// five repositories would scatter five chances to forget a tenant predicate, and
// the six cross-tenant leaks this project has already fixed were every one of
// them exactly that. Here they sit in one file, each starting from the tenant.
//
// Every method:
//   - filters the TARGET table on tenant_id, not merely the source entity's;
//   - selects only the columns a relation chip renders (§45 — no overfetch);
//   - returns (rows, total) so the drawer can say "showing 25 of 312".
type GormEntityRelationRepository struct {
	db *gorm.DB
}

func NewGormEntityRelationRepository(db *gorm.DB) *GormEntityRelationRepository {
	return &GormEntityRelationRepository{db: db}
}

// Compile-time proof that this satisfies the port the resolvers depend on.
var _ entity.RelationReader = (*GormEntityRelationRepository)(nil)

// relScan is the row shape every query below selects into.
type relScan struct {
	ID       string
	Title    string
	Subtitle string
	Status   string
	Severity string
	Label    string
}

func toRelationRows(scans []relScan) []entity.RelationRow {
	out := make([]entity.RelationRow, 0, len(scans))
	for _, s := range scans {
		out = append(out, entity.RelationRow{
			ID: s.ID, Title: s.Title, Subtitle: s.Subtitle,
			Status: s.Status, Severity: s.Severity, Label: s.Label,
		})
	}
	return out
}

// fetchRelations runs the count and the capped page off one prepared query.
func fetchRelations(q *gorm.DB, limit int) ([]entity.RelationRow, int, error) {
	var total int64
	if err := q.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []entity.RelationRow{}, 0, nil
	}
	if limit <= 0 {
		limit = 25
	}
	var scans []relScan
	if err := q.Session(&gorm.Session{}).Limit(limit).Scan(&scans).Error; err != nil {
		return nil, 0, err
	}
	return toRelationRows(scans), int(total), nil
}

// jsonContainsID matches an id inside a jsonb array of ids.
//
// Incident.RiskIDs and Incident.AssetIDs are jsonb string arrays. A containment
// operator (@>) would be the right tool on Postgres and does not exist on the
// sqlite the repository tests run against, so the predicate casts the column to
// text and looks for the quoted id. A quoted uuid is 38 characters of fixed
// shape; a false positive would need another id to contain it verbatim, which
// uuids cannot do. Stated here rather than left to be rediscovered.
func jsonContainsID(column, id string) (string, string) {
	return fmt.Sprintf("CAST(%s AS TEXT) LIKE ?", column), "%\"" + id + "\"%"
}

// =============================================================================
// Asset-centred
// =============================================================================

// RisksForAsset returns the risks attached to an asset through BOTH linkage
// mechanisms: the risk_assets many-to-many and the legacy risks.asset_id column.
// Both carry live data, so reading only one would silently under-report.
func (r *GormEntityRelationRepository) RisksForAsset(ctx context.Context, tenantID, assetID uuid.UUID, limit int) ([]entity.RelationRow, int, error) {
	q := r.db.WithContext(ctx).
		Table("risks").
		Select("DISTINCT CAST(risks.id AS TEXT) AS id, risks.name AS title, risks.description AS subtitle, risks.status AS status, risks.criticality AS severity").
		Joins("LEFT JOIN risk_assets ON risk_assets.risk_id = risks.id").
		Where("risks.tenant_id = ?", tenantID).
		Where("risks.deleted_at IS NULL").
		Where("(risk_assets.asset_id = ? OR risks.asset_id = ?)", assetID, assetID)
	return fetchRelations(q, limit)
}

// VulnerabilitiesForAsset returns the findings attributed to an asset.
func (r *GormEntityRelationRepository) VulnerabilitiesForAsset(ctx context.Context, tenantID, assetID uuid.UUID, limit int) ([]entity.RelationRow, int, error) {
	q := r.db.WithContext(ctx).
		Table("vulnerabilities").
		Select("CAST(vulnerabilities.id AS TEXT) AS id, vulnerabilities.title AS title, vulnerabilities.cve_id AS subtitle, vulnerabilities.status AS status, vulnerabilities.severity AS severity").
		Where("vulnerabilities.tenant_id = ?", tenantID).
		Where("vulnerabilities.deleted_at IS NULL").
		Where("vulnerabilities.asset_id = ?", assetID).
		Order("vulnerabilities.priority_score DESC")
	return fetchRelations(q, limit)
}

// FindingsForAsset is the finding alias of VulnerabilitiesForAsset — same rows,
// same table (see entity.VulnerabilityResolver for why they are one thing).
func (r *GormEntityRelationRepository) FindingsForAsset(ctx context.Context, tenantID, assetID uuid.UUID, limit int) ([]entity.RelationRow, int, error) {
	return r.VulnerabilitiesForAsset(ctx, tenantID, assetID, limit)
}

// IncidentsForAsset returns incidents whose asset_ids name this asset.
func (r *GormEntityRelationRepository) IncidentsForAsset(ctx context.Context, tenantID, assetID uuid.UUID, limit int) ([]entity.RelationRow, int, error) {
	pred, arg := jsonContainsID("incidents.asset_ids", assetID.String())
	q := r.db.WithContext(ctx).
		Table("incidents").
		Select("CAST(incidents.id AS TEXT) AS id, incidents.title AS title, incidents.incident_type AS subtitle, incidents.status AS status, incidents.severity AS severity").
		Where("incidents.tenant_id = ?", tenantID.String()).
		Where("incidents.deleted_at IS NULL").
		Where(pred, arg).
		Order("incidents.created_at DESC")
	return fetchRelations(q, limit)
}

// DependenciesForAsset returns the assets this one depends on and the assets
// that depend on it, as one list labelled by the edge's type.
//
// The union is deliberate: "what does this touch" is one question to someone
// looking at an outage, and splitting it in two makes them read both lists to
// answer it. The Label carries the dependency type so nothing is lost.
func (r *GormEntityRelationRepository) DependenciesForAsset(ctx context.Context, tenantID, assetID uuid.UUID, limit int) ([]entity.RelationRow, int, error) {
	q := r.db.WithContext(ctx).
		Table("asset_dependencies AS d").
		Select(`CASE WHEN d.source_asset_id = ? THEN CAST(a_target.id AS TEXT) ELSE CAST(a_source.id AS TEXT) END AS id,
		        CASE WHEN d.source_asset_id = ? THEN a_target.name ELSE a_source.name END AS title,
		        CASE WHEN d.source_asset_id = ? THEN a_target.type ELSE a_source.type END AS subtitle,
		        CASE WHEN d.source_asset_id = ? THEN a_target.criticality ELSE a_source.criticality END AS severity,
		        d.dependency_type AS label`, assetID, assetID, assetID, assetID).
		Joins("LEFT JOIN assets AS a_target ON a_target.id = d.target_asset_id AND a_target.tenant_id = ? AND a_target.deleted_at IS NULL", tenantID).
		Joins("LEFT JOIN assets AS a_source ON a_source.id = d.source_asset_id AND a_source.tenant_id = ? AND a_source.deleted_at IS NULL", tenantID).
		Where("d.tenant_id = ?", tenantID).
		Where("d.deleted_at IS NULL").
		Where("(d.source_asset_id = ? OR d.target_asset_id = ?)", assetID, assetID).
		// The joins above are tenant-scoped, so an edge pointing at another
		// tenant's row joins to NULL and is dropped here rather than rendered as
		// a nameless chip.
		Where("(CASE WHEN d.source_asset_id = ? THEN a_target.id ELSE a_source.id END) IS NOT NULL", assetID)
	return fetchRelations(q, limit)
}

// =============================================================================
// Risk-centred
// =============================================================================

func (r *GormEntityRelationRepository) AssetsForRisk(ctx context.Context, tenantID, riskID uuid.UUID, limit int) ([]entity.RelationRow, int, error) {
	q := r.db.WithContext(ctx).
		Table("assets").
		Select("DISTINCT CAST(assets.id AS TEXT) AS id, assets.name AS title, assets.type AS subtitle, assets.criticality AS severity").
		Joins("LEFT JOIN risk_assets ON risk_assets.asset_id = assets.id").
		Joins("LEFT JOIN risks ON risks.asset_id = assets.id AND risks.id = ? AND risks.tenant_id = ? AND risks.deleted_at IS NULL", riskID, tenantID).
		Where("assets.tenant_id = ?", tenantID).
		Where("assets.deleted_at IS NULL").
		Where("(risk_assets.risk_id = ? OR risks.id IS NOT NULL)", riskID)
	return fetchRelations(q, limit)
}

func (r *GormEntityRelationRepository) MitigationsForRisk(ctx context.Context, tenantID, riskID uuid.UUID, limit int) ([]entity.RelationRow, int, error) {
	q := r.db.WithContext(ctx).
		Table("mitigations").
		Select("CAST(mitigations.id AS TEXT) AS id, mitigations.title AS title, mitigations.description AS subtitle, mitigations.status AS status, mitigations.priority AS severity").
		Where("mitigations.tenant_id = ?", tenantID).
		Where("mitigations.deleted_at IS NULL").
		Where("mitigations.risk_id = ?", riskID).
		Order("mitigations.created_at DESC")
	return fetchRelations(q, limit)
}

func (r *GormEntityRelationRepository) IncidentsForRisk(ctx context.Context, tenantID, riskID uuid.UUID, limit int) ([]entity.RelationRow, int, error) {
	pred, arg := jsonContainsID("incidents.risk_ids", riskID.String())
	q := r.db.WithContext(ctx).
		Table("incidents").
		Select("CAST(incidents.id AS TEXT) AS id, incidents.title AS title, incidents.incident_type AS subtitle, incidents.status AS status, incidents.severity AS severity").
		Where("incidents.tenant_id = ?", tenantID.String()).
		Where("incidents.deleted_at IS NULL").
		Where(pred, arg).
		Order("incidents.created_at DESC")
	return fetchRelations(q, limit)
}

// ControlsForRisk reads the risk_control_mappings crosswalk.
//
// A mapping may name a framework without naming a control — that is what the
// 0046 data migration could honestly infer from free-text framework names. Those
// rows are skipped rather than rendered as a chip that deep-links nowhere.
func (r *GormEntityRelationRepository) ControlsForRisk(ctx context.Context, tenantID, riskID uuid.UUID, limit int) ([]entity.RelationRow, int, error) {
	q := r.db.WithContext(ctx).
		Table("risk_control_mappings AS m").
		Select("CAST(c.id AS TEXT) AS id, c.name AS title, c.reference_code AS subtitle, c.status AS status, f.name AS label").
		Joins("JOIN compliance_controls AS c ON c.id = m.control_id AND c.tenant_id = ? AND c.deleted_at IS NULL", tenantID).
		Joins("LEFT JOIN compliance_frameworks AS f ON f.id = c.framework_id AND f.tenant_id = ?", tenantID).
		Where("m.tenant_id = ?", tenantID).
		Where("m.deleted_at IS NULL").
		Where("m.risk_id = ?", riskID).
		Where("m.control_id IS NOT NULL")
	return fetchRelations(q, limit)
}

// VulnerabilitiesForRisk returns findings that point back at this risk — the
// link the vulnerability ingest writes when it auto-creates a risk for a P1/KEV.
func (r *GormEntityRelationRepository) VulnerabilitiesForRisk(ctx context.Context, tenantID, riskID uuid.UUID, limit int) ([]entity.RelationRow, int, error) {
	q := r.db.WithContext(ctx).
		Table("vulnerabilities").
		Select("CAST(vulnerabilities.id AS TEXT) AS id, vulnerabilities.title AS title, vulnerabilities.cve_id AS subtitle, vulnerabilities.status AS status, vulnerabilities.severity AS severity").
		Where("vulnerabilities.tenant_id = ?", tenantID).
		Where("vulnerabilities.deleted_at IS NULL").
		Where("vulnerabilities.risk_id = ?", riskID).
		Order("vulnerabilities.priority_score DESC")
	return fetchRelations(q, limit)
}

// =============================================================================
// Control-centred
// =============================================================================

func (r *GormEntityRelationRepository) RisksForControl(ctx context.Context, tenantID, controlID uuid.UUID, limit int) ([]entity.RelationRow, int, error) {
	q := r.db.WithContext(ctx).
		Table("risk_control_mappings AS m").
		Select("CAST(r.id AS TEXT) AS id, r.name AS title, r.description AS subtitle, r.status AS status, r.criticality AS severity").
		Joins("JOIN risks AS r ON r.id = m.risk_id AND r.tenant_id = ? AND r.deleted_at IS NULL", tenantID).
		Where("m.tenant_id = ?", tenantID).
		Where("m.deleted_at IS NULL").
		Where("m.control_id = ?", controlID)
	return fetchRelations(q, limit)
}

// EvidenceForControl reads the evidence↔control link table. Evidence is reusable
// across controls, so it hangs off a link table rather than a foreign key.
func (r *GormEntityRelationRepository) EvidenceForControl(ctx context.Context, tenantID, controlID uuid.UUID, limit int) ([]entity.RelationRow, int, error) {
	q := r.db.WithContext(ctx).
		Table("evidence_control_links AS l").
		Select("CAST(e.id AS TEXT) AS id, e.title AS title, e.filename AS subtitle, e.review AS status").
		Joins("JOIN evidences AS e ON e.id = l.evidence_id AND e.tenant_id = ? AND e.deleted_at IS NULL", tenantID).
		Where("l.tenant_id = ?", tenantID).
		Where("l.control_id = ?", controlID).
		Order("e.collected_at DESC")
	return fetchRelations(q, limit)
}

// =============================================================================
// Evidence-centred
// =============================================================================

func (r *GormEntityRelationRepository) ControlsForEvidence(ctx context.Context, tenantID, evidenceID uuid.UUID, limit int) ([]entity.RelationRow, int, error) {
	q := r.db.WithContext(ctx).
		Table("evidence_control_links AS l").
		Select("CAST(c.id AS TEXT) AS id, c.name AS title, c.reference_code AS subtitle, c.status AS status, f.name AS label").
		Joins("JOIN compliance_controls AS c ON c.id = l.control_id AND c.tenant_id = ? AND c.deleted_at IS NULL", tenantID).
		Joins("LEFT JOIN compliance_frameworks AS f ON f.id = c.framework_id AND f.tenant_id = ?", tenantID).
		Where("l.tenant_id = ?", tenantID).
		Where("l.evidence_id = ?", evidenceID)
	return fetchRelations(q, limit)
}

// =============================================================================
// Vulnerability-centred
// =============================================================================

func (r *GormEntityRelationRepository) AssetsForVulnerability(ctx context.Context, tenantID, vulnID uuid.UUID, limit int) ([]entity.RelationRow, int, error) {
	q := r.db.WithContext(ctx).
		Table("assets").
		Select("CAST(assets.id AS TEXT) AS id, assets.name AS title, assets.type AS subtitle, assets.criticality AS severity").
		Joins("JOIN vulnerabilities AS v ON v.asset_id = assets.id AND v.tenant_id = ? AND v.deleted_at IS NULL", tenantID).
		Where("assets.tenant_id = ?", tenantID).
		Where("assets.deleted_at IS NULL").
		Where("v.id = ?", vulnID)
	return fetchRelations(q, limit)
}

func (r *GormEntityRelationRepository) RisksForVulnerability(ctx context.Context, tenantID, vulnID uuid.UUID, limit int) ([]entity.RelationRow, int, error) {
	q := r.db.WithContext(ctx).
		Table("risks").
		Select("CAST(risks.id AS TEXT) AS id, risks.name AS title, risks.description AS subtitle, risks.status AS status, risks.criticality AS severity").
		Joins("JOIN vulnerabilities AS v ON v.risk_id = risks.id AND v.tenant_id = ? AND v.deleted_at IS NULL", tenantID).
		Where("risks.tenant_id = ?", tenantID).
		Where("risks.deleted_at IS NULL").
		Where("v.id = ?", vulnID)
	return fetchRelations(q, limit)
}

// =============================================================================
// Incident-centred
// =============================================================================

// RisksForIncident resolves the incident's risk_ids into real risks.
//
// The ids are read off the incident row first and the risks are then loaded with
// a tenant predicate, rather than joined blindly on the id list. An incident's
// jsonb id list is data, and data can name anything; resolving it through a
// tenant-scoped read is what stops a hand-edited list from becoming a window
// into another tenant.
func (r *GormEntityRelationRepository) RisksForIncident(ctx context.Context, tenantID uuid.UUID, incidentID uint, limit int) ([]entity.RelationRow, int, error) {
	ids, err := r.incidentLinkIDs(ctx, tenantID, incidentID, "risk_ids")
	if err != nil || len(ids) == 0 {
		return []entity.RelationRow{}, 0, err
	}
	q := r.db.WithContext(ctx).
		Table("risks").
		Select("CAST(risks.id AS TEXT) AS id, risks.name AS title, risks.description AS subtitle, risks.status AS status, risks.criticality AS severity").
		Where("risks.tenant_id = ?", tenantID).
		Where("risks.deleted_at IS NULL").
		Where("risks.id IN ?", ids)
	return fetchRelations(q, limit)
}

func (r *GormEntityRelationRepository) AssetsForIncident(ctx context.Context, tenantID uuid.UUID, incidentID uint, limit int) ([]entity.RelationRow, int, error) {
	ids, err := r.incidentLinkIDs(ctx, tenantID, incidentID, "asset_ids")
	if err != nil || len(ids) == 0 {
		return []entity.RelationRow{}, 0, err
	}
	q := r.db.WithContext(ctx).
		Table("assets").
		Select("CAST(assets.id AS TEXT) AS id, assets.name AS title, assets.type AS subtitle, assets.criticality AS severity").
		Where("assets.tenant_id = ?", tenantID).
		Where("assets.deleted_at IS NULL").
		Where("assets.id IN ?", ids)
	return fetchRelations(q, limit)
}

// incidentLinkIDs reads one of the incident's jsonb id lists, tenant-scoped, and
// returns the entries that parse as uuids. A malformed entry is dropped rather
// than passed into a query.
func (r *GormEntityRelationRepository) incidentLinkIDs(ctx context.Context, tenantID uuid.UUID, incidentID uint, column string) ([]uuid.UUID, error) {
	var row struct {
		RiskIDs  []byte `gorm:"column:risk_ids"`
		AssetIDs []byte `gorm:"column:asset_ids"`
	}
	err := r.db.WithContext(ctx).
		Table("incidents").
		Select("risk_ids, asset_ids").
		Where("id = ? AND tenant_id = ? AND deleted_at IS NULL", incidentID, tenantID.String()).
		Take(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	raw := row.RiskIDs
	if column == "asset_ids" {
		raw = row.AssetIDs
	}
	if len(raw) == 0 {
		return nil, nil
	}
	var strs []string
	if err := json.Unmarshal(raw, &strs); err != nil {
		return nil, nil
	}
	out := make([]uuid.UUID, 0, len(strs))
	for _, s := range strs {
		if id, err := uuid.Parse(s); err == nil && id != uuid.Nil {
			out = append(out, id)
		}
	}
	return out, nil
}
