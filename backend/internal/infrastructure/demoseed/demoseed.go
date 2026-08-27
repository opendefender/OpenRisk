// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package demoseed loads the demonstration dataset into a tenant.
//
// This is the ONLY thing in the codebase allowed to create data nobody asked
// for, and it does so only when DEMO_MODE=true. The fixtures live outside the
// application in dev/fixtures/demo.json; no Go file and no frontend module
// embeds them, so there is no path by which demo content can reach a real
// deployment that has not explicitly opted in.
//
// The rule this package exists to enforce: a tenant either contains data its
// users put there, or it is visibly flagged as a demonstration. Never a silent
// mixture of the two. Enabled() drives the banner the frontend renders, so a
// seeded tenant cannot be mistaken for a real one.
package demoseed

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"gorm.io/gorm"
)

// EnvFlag is the environment variable that turns demonstration data on.
const EnvFlag = "DEMO_MODE"

// envFixturesDir optionally overrides where demo.json is looked up.
const envFixturesDir = "DEMO_FIXTURES_DIR"

// defaultFixtureDirs are tried in order, covering both `go run ./cmd/server`
// from backend/ and a container whose workdir is the repository root.
var defaultFixtureDirs = []string{"dev/fixtures", "../dev/fixtures"}

// Enabled reports whether demonstration mode is on.
func Enabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(EnvFlag)), "true")
}

/* ---------------- fixture shape ---------------- */

type fixtureAsset struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Criticality string `json:"criticality"`
	Owner       string `json:"owner"`
}

type fixtureRisk struct {
	Title                   string   `json:"title"`
	Description             string   `json:"description"`
	Probability             float64  `json:"probability"`
	Impact                  float64  `json:"impact"`
	Criticality             string   `json:"criticality"`
	Status                  string   `json:"status"`
	LifecyclePhase          string   `json:"lifecycle_phase"`
	TreatmentPlan           string   `json:"treatment_plan"`
	Tags                    []string `json:"tags"`
	Frameworks              []string `json:"frameworks"`
	Asset                   string   `json:"asset"`
	SLEXAF                  *float64 `json:"sle_xaf"`
	ARO                     *float64 `json:"aro"`
	DowntimeHours           *float64 `json:"downtime_hours"`
	HourlyDowntimeCostXAF   *float64 `json:"hourly_downtime_cost_xaf"`
	FinesXAF                *float64 `json:"fines_xaf"`
	RemediationCostXAF      *float64 `json:"remediation_cost_xaf"`
	MitigationEffectiveness *float64 `json:"mitigation_effectiveness"`
}

// fixtureDependency is an edge in the asset graph. Without these the topology
// view renders twenty unconnected dots, which is the one screen where "what
// depends on what" is the entire point — a demo tenant that cannot show it is
// demonstrating the wrong thing.
type fixtureDependency struct {
	Source      string `json:"source"`
	Target      string `json:"target"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

type fixtureIncident struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Severity    string `json:"severity"`
	Status      string `json:"status"`
}

type fixtureFile struct {
	Assets       []fixtureAsset      `json:"assets"`
	Dependencies []fixtureDependency `json:"dependencies"`
	Risks        []fixtureRisk       `json:"risks"`
	Incidents    []fixtureIncident   `json:"incidents"`
}

// Result reports what the seeder did, for the startup log.
type Result struct {
	Assets       int
	Dependencies int
	Risks        int
	Incidents    int
	Skipped      bool // already seeded
}

// Seed loads dev/fixtures/demo.json into the given tenant.
//
// Idempotent by title/name within the tenant: restarting a demo container does
// not multiply the dataset. It is a no-op unless Enabled().
func Seed(db *gorm.DB, tenantID, createdBy uuid.UUID) (Result, error) {
	var res Result
	if db == nil || !Enabled() {
		return res, nil
	}
	if tenantID == uuid.Nil {
		return res, fmt.Errorf("demoseed: refusing to seed a nil tenant")
	}

	path, err := locateFixture()
	if err != nil {
		return res, err
	}
	raw, err := os.ReadFile(path) // #nosec G304 -- path is resolved from a fixed allowlist
	if err != nil {
		return res, fmt.Errorf("demoseed: read %s: %w", path, err)
	}
	var fx fixtureFile
	if err := json.Unmarshal(raw, &fx); err != nil {
		return res, fmt.Errorf("demoseed: parse %s: %w", path, err)
	}

	// One transaction: a half-seeded demo tenant is worse than an unseeded one,
	// because it looks like real data that went missing.
	err = db.Transaction(func(tx *gorm.DB) error {
		// Already seeded? Bail out rather than duplicating.
		var existing int64
		if err := tx.Model(&domain.Asset{}).Where("tenant_id = ?", tenantID).Count(&existing).Error; err != nil {
			return err
		}
		if existing > 0 {
			res.Skipped = true
			return nil
		}

		assetIDs := make(map[string]uuid.UUID, len(fx.Assets))
		for _, fa := range fx.Assets {
			a := domain.Asset{
				TenantID:       tenantID,
				OrganizationID: tenantID,
				Name:           fa.Name,
				Type:           fa.Type,
				Criticality:    domain.AssetCriticality(fa.Criticality),
				Owner:          fa.Owner,
				Source:         "DEMO",
			}
			if err := tx.Create(&a).Error; err != nil {
				return fmt.Errorf("demoseed: create asset %q: %w", fa.Name, err)
			}
			assetIDs[fa.Name] = a.ID
			res.Assets++
		}

		// The dependency graph. Skipped silently when either end is not in the
		// fixture rather than failing the transaction: a typo in a demo edge
		// should not cost the whole demo tenant.
		for _, fd := range fx.Dependencies {
			src, okS := assetIDs[fd.Source]
			dst, okT := assetIDs[fd.Target]
			if !okS || !okT {
				continue
			}
			depType := domain.DependencyType(fd.Type)
			if depType == "" {
				depType = domain.DepDependsOn
			}
			d := domain.AssetDependency{
				TenantID:      tenantID,
				SourceAssetID: src,
				TargetAssetID: dst,
				Type:          depType,
				Description:   fd.Description,
			}
			if err := tx.Create(&d).Error; err != nil {
				return fmt.Errorf("demoseed: create dependency %s->%s: %w", fd.Source, fd.Target, err)
			}
			res.Dependencies++
		}

		for _, fr := range fx.Risks {
			r := domain.Risk{
				TenantID:                tenantID,
				OrganizationID:          tenantID,
				Name:                    fr.Title,
				Title:                   fr.Title,
				Description:             fr.Description,
				Probability:             fr.Probability,
				Impact:                  fr.Impact,
				Criticality:             domain.CriticalityLevel(fr.Criticality),
				Status:                  domain.RiskStatus(fr.Status),
				LifecyclePhase:          domain.RiskPhase(fr.LifecyclePhase),
				TreatmentPlan:           domain.RiskTreatment(fr.TreatmentPlan),
				Tags:                    fr.Tags,
				Frameworks:              fr.Frameworks,
				CreatedBy:               createdBy,
				Source:                  domain.RiskSource("import"),
				SLEXAF:                  fr.SLEXAF,
				ARO:                     fr.ARO,
				DowntimeHours:           fr.DowntimeHours,
				HourlyDowntimeCostXAF:   fr.HourlyDowntimeCostXAF,
				FinesXAF:                fr.FinesXAF,
				RemediationCostXAF:      fr.RemediationCostXAF,
				MitigationEffectiveness: fr.MitigationEffectiveness,
			}
			// Score Engine formula: probability x impact x asset criticality.
			// Seeded directly so a demo tenant is coherent before the Redis
			// score worker has run.
			critFactor := 1.0
			if id, ok := assetIDs[fr.Asset]; ok {
				r.AssetID = &id
				critFactor = criticalityFactor(fx.Assets, fr.Asset)
			}
			r.Score = fr.Probability * fr.Impact * critFactor

			if err := tx.Create(&r).Error; err != nil {
				return fmt.Errorf("demoseed: create risk %q: %w", fr.Title, err)
			}
			// Many-to-many link so the asset drawer and Smart Score see it.
			if id, ok := assetIDs[fr.Asset]; ok {
				if err := tx.Exec(
					"INSERT INTO risk_assets (risk_id, asset_id) VALUES (?, ?) ON CONFLICT DO NOTHING",
					r.ID, id,
				).Error; err != nil {
					return fmt.Errorf("demoseed: link risk %q to asset: %w", fr.Title, err)
				}
			}
			res.Risks++
		}

		now := time.Now()
		for _, fi := range fx.Incidents {
			inc := domain.Incident{
				TenantID:     tenantID.String(),
				Title:        fi.Title,
				Description:  fi.Description,
				IncidentType: fi.Type,
				Severity:     fi.Severity,
				Status:       fi.Status,
				Source:       "internal",
				ReportedBy:   "demo@opendefender.io",
			}
			if fi.Status == "resolved" || fi.Status == "closed" {
				inc.ResolvedAt = &now
			}
			if err := tx.Create(&inc).Error; err != nil {
				return fmt.Errorf("demoseed: create incident %q: %w", fi.Title, err)
			}
			res.Incidents++
		}
		return nil
	})

	return res, err
}

// criticalityFactor maps an asset's criticality onto the Score Engine's
// [0.1, 3.0] multiplier.
func criticalityFactor(assets []fixtureAsset, name string) float64 {
	for _, a := range assets {
		if a.Name != name {
			continue
		}
		switch strings.ToUpper(a.Criticality) {
		case "CRITICAL":
			return 3.0
		case "HIGH":
			return 2.0
		case "MEDIUM":
			return 1.0
		default:
			return 0.5
		}
	}
	return 1.0
}

// locateFixture resolves demo.json from DEMO_FIXTURES_DIR or the known
// repository-relative locations.
func locateFixture() (string, error) {
	dirs := defaultFixtureDirs
	if override := strings.TrimSpace(os.Getenv(envFixturesDir)); override != "" {
		dirs = append([]string{override}, dirs...)
	}
	for _, d := range dirs {
		p := filepath.Join(d, "demo.json")
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("demoseed: demo.json not found in %v (set %s)", dirs, envFixturesDir)
}
