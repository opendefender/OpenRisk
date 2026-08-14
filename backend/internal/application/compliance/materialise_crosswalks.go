// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package compliance

import (
	"context"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	pkgcompliance "github.com/opendefender/openrisk/pkg/compliance"
)

// CrosswalkMaterialiser turns the curated catalog-level crosswalks into real
// rows between a tenant's own controls.
//
// Why materialise instead of resolving the curated table on every read: the
// crosswalks become the tenant's, which is the only way they can be edited,
// extended or deleted. A tenant who disagrees that their SOC 2 access evidence
// answers ISO A.5.15 must be able to remove that link and see the number change;
// a curated table consulted at read time would silently reassert it.
type CrosswalkMaterialiser struct {
	repo       domain.ComplianceRepository
	crosswalks domain.ControlCrosswalkRepository
}

func NewCrosswalkMaterialiser(repo domain.ComplianceRepository, crosswalks domain.ControlCrosswalkRepository) *CrosswalkMaterialiser {
	return &CrosswalkMaterialiser{repo: repo, crosswalks: crosswalks}
}

// MaterialiseResult reports what was created.
type MaterialiseResult struct {
	Created int `json:"created"`
	// Skipped covers links that already existed in either direction — importing
	// twice must not double anything.
	Skipped int `json:"skipped"`
	// Unmatched counts curated entries whose control could not be found in either
	// framework. Reported rather than swallowed: a silently unmatched crosswalk
	// is an inherited-coverage number that under-reports for no visible reason.
	Unmatched int `json:"unmatched"`
}

// ForFramework links a freshly imported framework to every other framework the
// tenant already holds, using the curated crosswalks for their catalogs.
//
// catalogOf tells us which catalog a framework was imported from. A framework
// built by hand has no catalog and simply takes part in nothing — there is
// nothing curated to say about controls the product has never seen.
func (m *CrosswalkMaterialiser) ForFramework(
	ctx context.Context,
	tenantID, frameworkID uuid.UUID,
	catalogKey string,
	catalogOf func(frameworkID uuid.UUID) string,
) (*MaterialiseResult, error) {
	res := &MaterialiseResult{}
	if catalogKey == "" || m.crosswalks == nil {
		return res, nil
	}

	newControls, err := m.repo.ListControlsByFramework(ctx, tenantID, frameworkID)
	if err != nil {
		return nil, err
	}
	newByCode := indexByCode(newControls)

	others, err := m.repo.ListFrameworks(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	for _, other := range others {
		if other.ID == frameworkID {
			continue
		}
		otherKey := catalogOf(other.ID)
		if otherKey == "" || otherKey == catalogKey {
			continue
		}

		entries := pkgcompliance.CrosswalksBetween(catalogKey, otherKey)
		if len(entries) == 0 {
			continue
		}

		otherControls, err := m.repo.ListControlsByFramework(ctx, tenantID, other.ID)
		if err != nil {
			return nil, err
		}
		otherByCode := indexByCode(otherControls)

		for _, e := range entries {
			src, okSrc := newByCode[e.SourceCode]
			tgt, okTgt := otherByCode[e.TargetCode]
			if !okSrc || !okTgt {
				// The catalogs are modelled at a level that may not include every
				// referenced control (a tenant can also have deleted one).
				res.Unmatched++
				continue
			}

			exists, err := m.crosswalks.Exists(ctx, tenantID, src.ID, tgt.ID)
			if err != nil {
				return nil, err
			}
			if exists {
				res.Skipped++
				continue
			}

			link := &domain.ControlCrosswalk{
				TenantID:        tenantID,
				SourceControlID: src.ID,
				TargetControlID: tgt.ID,
				Coverage:        domain.CrosswalkCoverage(e.Coverage),
				Rationale:       e.Rationale,
				Origin:          domain.CrosswalkOriginCurated,
			}
			if err := m.crosswalks.Create(ctx, link); err != nil {
				return nil, err
			}
			res.Created++
		}
	}

	return res, nil
}

func indexByCode(controls []domain.ComplianceControl) map[string]domain.ComplianceControl {
	out := make(map[string]domain.ComplianceControl, len(controls))
	for _, c := range controls {
		if c.ReferenceCode != "" {
			out[c.ReferenceCode] = c
		}
	}
	return out
}
