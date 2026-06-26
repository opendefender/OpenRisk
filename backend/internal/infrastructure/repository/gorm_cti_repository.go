package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/opendefender/openrisk/pkg/cti"
)

// GormCTIRepository implements cti.Repository using GORM/PostgreSQL.
// RULE: Uses the injected *gorm.DB instance, never the global database.DB.
type GormCTIRepository struct {
	db *gorm.DB
}

// NewGormCTIRepository creates a new CTI repository backed by PostgreSQL.
func NewGormCTIRepository(db *gorm.DB) *GormCTIRepository {
	return &GormCTIRepository{db: db}
}

// UpsertVulnerabilities inserts or updates vulnerabilities atomically.
// Uses ON CONFLICT (cve_id) DO UPDATE for full idempotency.
func (r *GormCTIRepository) UpsertVulnerabilities(ctx context.Context, vulns []cti.CTIVulnerability) error {
	if len(vulns) == 0 {
		return nil
	}

	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}

	// Process in batches of 100 to avoid overwhelming PostgreSQL
	batchSize := 100
	for i := 0; i < len(vulns); i += batchSize {
		end := i + batchSize
		if end > len(vulns) {
			end = len(vulns)
		}
		batch := vulns[i:end]

		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "cve_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"cvss_v3", "severity", "description", "published_at",
				"cisa_known", "cisa_due_date", "mitre_tactics", "mitre_techniques",
				"affected_cpe", "remediation", "references", "last_updated_at", "updated_at",
			}),
		}).CreateInBatches(batch, len(batch)).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to upsert vulnerability batch: %w", err)
		}
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

// GetByCVE returns a vulnerability by CVE ID.
// Returns (nil, nil) if not found.
func (r *GormCTIRepository) GetByCVE(ctx context.Context, cveID string) (*cti.CTIVulnerability, error) {
	var v cti.CTIVulnerability
	err := r.db.WithContext(ctx).Where("cve_id = ?", cveID).First(&v).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get vulnerability %s: %w", cveID, err)
	}
	return &v, nil
}

// Search performs a filtered, paginated search on cti_vulnerabilities.
// Supports: query (ILIKE on cve_id + description), severity, cisa_known,
// published_after, and CPE overlap.
func (r *GormCTIRepository) Search(ctx context.Context, query string, filters cti.CTIFilter) ([]cti.CTIVulnerability, int64, error) {
	db := r.db.WithContext(ctx).Model(&cti.CTIVulnerability{})

	if query != "" {
		db = db.Where("cve_id ILIKE ? OR description ILIKE ?", "%"+query+"%", "%"+query+"%")
	}
	if filters.Severity != "" {
		db = db.Where("severity = ?", filters.Severity)
	}
	if filters.CISAKnown != nil {
		db = db.Where("cisa_known = ?", *filters.CISAKnown)
	}
	if filters.PublishedAfter != nil {
		db = db.Where("published_at >= ?", *filters.PublishedAfter)
	}
	if filters.CPE != "" {
		db = db.Where("affected_cpe && ?", pq.Array([]string{filters.CPE}))
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count vulnerabilities: %w", err)
	}

	limit := filters.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	offset := filters.Offset
	if offset < 0 {
		offset = 0
	}

	var results []cti.CTIVulnerability
	if err := db.Offset(offset).Limit(limit).Order("published_at DESC").Find(&results).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to search vulnerabilities: %w", err)
	}
	return results, total, nil
}

// MatchByAssetCPEs finds vulnerabilities whose affected_cpe overlaps with
// the provided CPEs, excluding CVEs that already have a risk created
// for the given tenant+asset combination.
//
// SQL: SELECT * FROM cti_vulnerabilities
//
//	WHERE affected_cpe && $1
//	AND cve_id NOT IN (
//	  SELECT source_cve_id FROM risks
//	  WHERE tenant_id = $2 AND asset_id = $3 AND source_cve_id IS NOT NULL AND deleted_at IS NULL
//	)
func (r *GormCTIRepository) MatchByAssetCPEs(ctx context.Context, tenantID, assetID uuid.UUID, cpes []string) ([]cti.CTIVulnerability, error) {
	if len(cpes) == 0 {
		return nil, nil
	}

	var results []cti.CTIVulnerability
	err := r.db.WithContext(ctx).
		Where("affected_cpe && ?", pq.Array(cpes)).
		Where("cve_id NOT IN (SELECT source_cve_id FROM risks WHERE tenant_id = ? AND asset_id = ? AND source_cve_id IS NOT NULL AND deleted_at IS NULL)", tenantID, assetID).
		Find(&results).Error
	if err != nil {
		return nil, fmt.Errorf("failed to match CPEs for asset %s: %w", assetID, err)
	}
	return results, nil
}
