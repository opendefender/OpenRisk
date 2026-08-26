// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package entity

import (
	"context"

	"github.com/google/uuid"
)

// RelationRow is the light projection a relation query returns.
//
// It is deliberately not a domain entity. Opening a risk's drawer should not
// load twelve full assets with their attribute bags and preloaded risk graphs to
// render twelve chips; the query selects the few columns a chip shows and stops
// (§45). Anything more is one click away — the chip opens that entity's own
// drawer, which loads it properly.
type RelationRow struct {
	ID       string
	Title    string
	Subtitle string
	// Status and Severity are raw domain values; the resolver turns them into
	// Chips so the tone mapping lives in one place per type.
	Status   string
	Severity string
	// Label names the edge when the edge itself carries meaning — a dependency's
	// "runs_on", a control mapping's framework name.
	Label string
}

// RelationReader is the narrow port the resolvers use for cross-module edges.
//
// Every method takes the tenant and MUST filter on it. These queries are the
// most dangerous surface in the drawer: they cross module boundaries, so a
// missing predicate here leaks another tenant's inventory into a drawer whose
// own entity was correctly scoped. That is why the port is stated as one
// interface with the tenant in every signature rather than as a grab-bag of
// repository methods.
//
// Each method returns (rows, total) with rows capped at limit. A relation list
// is a preview, not a report: an asset with 4 000 findings must not turn a
// drawer open into a 4 000-row read, and the client is told the list was cut.
type RelationReader interface {
	// --- asset-centred -----------------------------------------------------
	RisksForAsset(ctx context.Context, tenantID, assetID uuid.UUID, limit int) ([]RelationRow, int, error)
	VulnerabilitiesForAsset(ctx context.Context, tenantID, assetID uuid.UUID, limit int) ([]RelationRow, int, error)
	FindingsForAsset(ctx context.Context, tenantID, assetID uuid.UUID, limit int) ([]RelationRow, int, error)
	IncidentsForAsset(ctx context.Context, tenantID, assetID uuid.UUID, limit int) ([]RelationRow, int, error)
	DependenciesForAsset(ctx context.Context, tenantID, assetID uuid.UUID, limit int) ([]RelationRow, int, error)

	// --- risk-centred ------------------------------------------------------
	AssetsForRisk(ctx context.Context, tenantID, riskID uuid.UUID, limit int) ([]RelationRow, int, error)
	MitigationsForRisk(ctx context.Context, tenantID, riskID uuid.UUID, limit int) ([]RelationRow, int, error)
	IncidentsForRisk(ctx context.Context, tenantID, riskID uuid.UUID, limit int) ([]RelationRow, int, error)
	ControlsForRisk(ctx context.Context, tenantID, riskID uuid.UUID, limit int) ([]RelationRow, int, error)
	VulnerabilitiesForRisk(ctx context.Context, tenantID, riskID uuid.UUID, limit int) ([]RelationRow, int, error)

	// --- control-centred ---------------------------------------------------
	RisksForControl(ctx context.Context, tenantID, controlID uuid.UUID, limit int) ([]RelationRow, int, error)
	EvidenceForControl(ctx context.Context, tenantID, controlID uuid.UUID, limit int) ([]RelationRow, int, error)

	// --- evidence-centred --------------------------------------------------
	ControlsForEvidence(ctx context.Context, tenantID, evidenceID uuid.UUID, limit int) ([]RelationRow, int, error)

	// --- vulnerability-centred ---------------------------------------------
	AssetsForVulnerability(ctx context.Context, tenantID, vulnID uuid.UUID, limit int) ([]RelationRow, int, error)
	RisksForVulnerability(ctx context.Context, tenantID, vulnID uuid.UUID, limit int) ([]RelationRow, int, error)

	// --- incident-centred --------------------------------------------------
	RisksForIncident(ctx context.Context, tenantID uuid.UUID, incidentID uint, limit int) ([]RelationRow, int, error)
	AssetsForIncident(ctx context.Context, tenantID uuid.UUID, incidentID uint, limit int) ([]RelationRow, int, error)
}

// relationCap is how many rows a relation group carries before it reports itself
// truncated.
const relationCap = 25

// chipFn turns a raw relation row into the status/severity chips of the TARGET
// type. It is per-target-type, not per-source, so an asset chip looks the same
// whether it was reached from a risk or from an incident.
type chipFn func(RelationRow) (status *Chip, severity *Chip)

// group builds one relation group, turning a query failure into a degraded group
// rather than a failed drawer (§27).
func group(key, label string, target Type, rows []RelationRow, total int, err error, chip chipFn) RelationGroup {
	g := RelationGroup{GroupKey: key, Label: label, TargetType: target, Items: []Relation{}}
	if err != nil {
		g.Error = "could not be loaded"
		return g
	}
	for _, r := range rows {
		rel := Relation{
			Ref:           Ref{Type: target, ID: r.ID},
			Title:         r.Title,
			Subtitle:      r.Subtitle,
			RelationLabel: r.Label,
			URL:           DeepLink(target, r.ID),
		}
		if chip != nil {
			rel.Status, rel.Severity = chip(r)
		}
		g.Items = append(g.Items, rel)
	}
	g.Total = total
	g.Truncated = total > len(rows)
	return g
}
