package cti

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Service is the main API for the CTI module defining all use cases
type Service interface {
	SyncAll(ctx context.Context) error
	GetVulnerability(ctx context.Context, cveID string) (*CTIVulnerability, error)
	Search(ctx context.Context, query string, filters CTIFilter) ([]CTIVulnerability, error)
	MatchAsset(ctx context.Context, tenantID, assetID uuid.UUID) ([]CTIVulnerability, error)
}

// CTIFilter contains search filters for vulnerability queries
type CTIFilter struct {
	Severity       string
	CISAKnown      *bool
	PublishedAfter *time.Time
	CPE            string
	Limit          int
	Offset         int
}

// Matcher performs the matching logic
type Matcher interface {
	MatchCVEsToAllTenantAssets(ctx context.Context) error
}

// Repository defines data access functions for CTI vulnerabilities
type Repository interface {
	UpsertVulnerabilities(ctx context.Context, vulns []CTIVulnerability) error
	GetByCVE(ctx context.Context, cveID string) (*CTIVulnerability, error)
	Search(ctx context.Context, query string, filters CTIFilter) ([]CTIVulnerability, int64, error)
	MatchByAssetCPEs(ctx context.Context, tenantID, assetID uuid.UUID, cpes []string) ([]CTIVulnerability, error)
}
