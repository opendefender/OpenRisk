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

// Plan works out the curated links a freshly imported framework gets against
// every other framework the tenant holds, without writing anything: the import
// applies them in its own transaction (ComplianceRepository.ApplyCatalogImport).
//
// newControls are the controls this import is about to create (their ids are
// already assigned); they join the framework's live controls as link ends.
//
// catalogOf tells us which catalog a framework was imported from. A framework
// built by hand has no catalog and simply takes part in nothing.
//
// A pair with a live link, or with a deleted link of this pack's provenance, is
// skipped: a deleted curated link is a tombstone, never re-created. Every lookup is tenant-scoped.
func (m *CrosswalkMaterialiser) Plan(
	ctx context.Context,
	tenantID, frameworkID uuid.UUID,
	catalogKey string,
	newControls []*domain.ComplianceControl,
	catalogOf func(frameworkID uuid.UUID) string,
) ([]*domain.ControlCrosswalk, *MaterialiseResult, error) {
	res := &MaterialiseResult{}
	if catalogKey == "" || m.crosswalks == nil {
		return nil, res, nil
	}

	live, err := m.repo.ListControlsByFramework(ctx, tenantID, frameworkID)
	if err != nil {
		return nil, nil, err
	}
	newByCode := indexByCode(live)
	fresh := make(map[uuid.UUID]bool, len(newControls))
	for _, c := range newControls {
		if c.ReferenceCode != "" {
			newByCode[c.ReferenceCode] = *c
			fresh[c.ID] = true
		}
	}

	others, err := m.repo.ListFrameworks(ctx, tenantID)
	if err != nil {
		return nil, nil, err
	}

	var links []*domain.ControlCrosswalk
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
			return nil, nil, err
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

			itemKey := pkgcompliance.CrosswalkItemKey(e)
			// A control created by this very import has no link yet; skip the
			// queries. Otherwise skip a live link of any origin (no duplicate) and
			// a deleted link that carries this pack's provenance (a tombstone). A
			// deleted manual link does not block.
			if !fresh[src.ID] {
				exists, err := m.crosswalks.Exists(ctx, tenantID, src.ID, tgt.ID)
				if err != nil {
					return nil, nil, err
				}
				if !exists {
					exists, err = m.crosswalks.PackTombstoneExists(ctx, tenantID, src.ID, tgt.ID, itemKey)
					if err != nil {
						return nil, nil, err
					}
				}
				if exists {
					res.Skipped++
					continue
				}
			}

			pb := pkgcompliance.CrosswalkPackBase(e)
			links = append(links, &domain.ControlCrosswalk{
				ID:              uuid.New(),
				TenantID:        tenantID,
				SourceControlID: src.ID,
				TargetControlID: tgt.ID,
				Coverage:        domain.CrosswalkCoverage(e.Coverage),
				Rationale:       e.Rationale,
				Origin:          domain.CrosswalkOriginCurated,
				PackKey:         pkgcompliance.CrosswalksPackKey,
				PackItemKey:     itemKey,
				PackVersion:     pkgcompliance.CrosswalksVersion,
				PackBase:        &pb.JSON,
				PackBaseHash:    &pb.Hash,
			})
			res.Created++
		}
	}

	return links, res, nil
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
