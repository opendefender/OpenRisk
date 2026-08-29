// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package entity

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// AssetReader is the narrow port over the asset repository.
type AssetReader interface {
	GetByID(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) (*domain.Asset, error)
}

// AssetResolver serves both the asset and the vendor drawer.
//
// A vendor is an asset whose category is "vendor" (domain.CategoryVendor) — the
// product has no vendors table, and inventing one to satisfy a drawer would be
// exactly the fiction this wave forbids. The two share every query and differ in
// one place: a vendor resolver refuses an id whose row is not a vendor, so
// /entities/vendor/<a-server-id> is a 404 rather than a server rendered under a
// vendor heading.
type AssetResolver struct {
	assets    AssetReader
	relations RelationReader
	// vendorOnly restricts this resolver to the vendor category.
	vendorOnly bool
}

func NewAssetResolver(assets AssetReader, relations RelationReader) *AssetResolver {
	return &AssetResolver{assets: assets, relations: relations}
}

// NewVendorResolver is the same resolver restricted to the vendor category.
func NewVendorResolver(assets AssetReader, relations RelationReader) *AssetResolver {
	return &AssetResolver{assets: assets, relations: relations, vendorOnly: true}
}

func (r *AssetResolver) selfType() Type {
	if r.vendorOnly {
		return TypeVendor
	}
	return TypeAsset
}

// load is the single gate. Everything else in this resolver goes through it, so
// there is exactly one place where an id becomes a row — and exactly one place
// the tenant is applied.
func (r *AssetResolver) load(ctx context.Context, c Caller, id string) (*domain.Asset, uuid.UUID, error) {
	aid, err := uuid.Parse(id)
	if err != nil {
		// A malformed id is not-found, not a validation error: answering
		// differently for "not a uuid" and "not yours" tells a prober which of
		// their guesses had the right shape.
		return nil, uuid.Nil, domain.NewNotFoundError(string(r.selfType()), id)
	}
	if r.assets == nil {
		return nil, uuid.Nil, domain.NewNotFoundError(string(r.selfType()), id)
	}
	asset, err := r.assets.GetByID(ctx, aid, c.TenantID)
	if err != nil {
		return nil, uuid.Nil, err
	}
	if asset == nil {
		return nil, uuid.Nil, domain.NewNotFoundError(string(r.selfType()), id)
	}
	if r.vendorOnly && asset.Category != domain.CategoryVendor {
		return nil, uuid.Nil, domain.NewNotFoundError(string(TypeVendor), id)
	}
	return asset, aid, nil
}

func (r *AssetResolver) Summary(ctx context.Context, c Caller, id string) (*Summary, error) {
	asset, _, err := r.load(ctx, c, id)
	if err != nil {
		return nil, err
	}

	s := &Summary{
		ID:       asset.ID.String(),
		Title:    asset.Name,
		Subtitle: asset.Type,
		Severity: severityChip(string(asset.Criticality)),
		// An asset's "score" is its business criticality — the factor the Score
		// Engine multiplies into every risk on it. It is read from the column,
		// never derived here.
		Score: Score{
			Available: asset.Criticality != "",
			Key:       "criticality",
			Label:     "Business criticality",
			Value:     asset.Criticality.ScoreFactor(),
			Max:       3.0,
			Tone:      severityToneOf(string(asset.Criticality)),
			Basis:     "Asset criticality factor (Score Engine)",
		},
		CreatedAt: timePtr(asset.CreatedAt),
		UpdatedAt: timePtr(asset.UpdatedAt),
	}
	if asset.Criticality == "" {
		s.Score = unavailableScore("criticality", "Business criticality", "no criticality set on this asset")
	}
	if asset.Owner != "" {
		s.Owner = &Actor{Label: asset.Owner}
	}

	s.Fields = appendField(s.Fields, field("type", "Type", asset.Type, FieldText))
	s.Fields = appendField(s.Fields, field("category", "Category", string(asset.Category), FieldBadge))
	s.Fields = appendField(s.Fields, field("owner", "Owner", asset.Owner, FieldUser))
	s.Fields = appendField(s.Fields, field("source", "Source", asset.Source, FieldBadge))
	s.Fields = appendField(s.Fields, field("external_id", "External ID", asset.ExternalID, FieldText))
	s.Fields = appendField(s.Fields, field("cloud_resource_id", "Cloud resource", asset.CloudResourceID, FieldText))
	if len(asset.Hostnames) > 0 {
		s.Fields = append(s.Fields, Field{Key: "hostnames", Label: "Hostnames", Kind: FieldTagList, Values: asset.Hostnames})
	}
	if len(asset.IPAddresses) > 0 {
		s.Fields = append(s.Fields, Field{Key: "ip_addresses", Label: "IP addresses", Kind: FieldTagList, Values: asset.IPAddresses})
	}
	if len(asset.CPEs) > 0 {
		s.Fields = append(s.Fields, Field{Key: "cpes", Label: "Software (CPE)", Kind: FieldTagList, Values: asset.CPEs})
	}
	// Typed attributes are schema-validated on write, so whatever is in the bag
	// is a value the tenant's own schema admits.
	for key, value := range asset.Attributes {
		if str := attrString(value); str != "" {
			s.Fields = appendField(s.Fields, field("attr:"+key, title(key), str, FieldText))
		}
	}
	return s, nil
}

func (r *AssetResolver) Relations(ctx context.Context, c Caller, id string) ([]RelationGroup, error) {
	asset, aid, err := r.load(ctx, c, id)
	if err != nil {
		return nil, err
	}
	_ = asset
	if r.relations == nil {
		return []RelationGroup{}, nil
	}

	risks, riskTotal, riskErr := r.relations.RisksForAsset(ctx, c.TenantID, aid, relationCap)
	vulns, vulnTotal, vulnErr := r.relations.VulnerabilitiesForAsset(ctx, c.TenantID, aid, relationCap)
	finds, findTotal, findErr := r.relations.FindingsForAsset(ctx, c.TenantID, aid, relationCap)
	incs, incTotal, incErr := r.relations.IncidentsForAsset(ctx, c.TenantID, aid, relationCap)
	deps, depTotal, depErr := r.relations.DependenciesForAsset(ctx, c.TenantID, aid, relationCap)

	return []RelationGroup{
		group("risks", "Risks", TypeRisk, risks, riskTotal, riskErr, riskChips),
		group("vulnerabilities", "Vulnerabilities", TypeVulnerability, vulns, vulnTotal, vulnErr, vulnChips),
		group("findings", "Findings", TypeFinding, finds, findTotal, findErr, vulnChips),
		group("incidents", "Incidents", TypeIncident, incs, incTotal, incErr, incidentChips),
		group("dependencies", "Dependencies", TypeAsset, deps, depTotal, depErr, assetChips),
	}, nil
}

func (r *AssetResolver) Actions(ctx context.Context, c Caller, id string) []Action {
	self := r.selfType()
	base := fmt.Sprintf("/api/v1/assets/%s", id)
	out := []Action{}
	if c.Can("assets:update") {
		out = append(out, Action{
			Key: "edit", Label: "Edit", Kind: ActionPrimary,
			Method: "PATCH", Path: base, Permission: "assets:update",
		})
	}
	if c.Can("assets:update") && self == TypeAsset {
		out = append(out, Action{
			Key: "add_dependency", Label: "Add dependency", Kind: ActionSecondary,
			Method: "POST", Path: "/api/v1/asset-dependencies", Permission: "assets:update",
		})
	}
	if c.Can("risks:create") {
		out = append(out, Action{
			Key: "create_risk", Label: "Create risk", Kind: ActionSecondary,
			Method: "POST", Path: "/api/v1/risks", Permission: "risks:create",
		})
	}
	if c.Can("assets:delete") {
		out = append(out, Action{
			Key: "delete", Label: "Delete", Kind: ActionDanger,
			Method: "DELETE", Path: base, Permission: "assets:delete",
		})
	}
	return out
}

// --- target chip mappings --------------------------------------------------
//
// One function per TARGET type, shared by every resolver that can point at it.

func assetChips(r RelationRow) (*Chip, *Chip) { return nil, severityChip(r.Severity) }
func riskChips(r RelationRow) (*Chip, *Chip) {
	return riskStatusChip(r.Status), severityChip(r.Severity)
}
func vulnChips(r RelationRow) (*Chip, *Chip) {
	return vulnStatusChip(r.Status), severityChip(r.Severity)
}
func incidentChips(r RelationRow) (*Chip, *Chip) {
	return incidentStatusChip(r.Status), severityChip(r.Severity)
}
func controlChips(r RelationRow) (*Chip, *Chip)  { return controlStatusChip(r.Status), nil }
func evidenceChips(r RelationRow) (*Chip, *Chip) { return evidenceStatusChip(r.Status), nil }
func mitigationChips(r RelationRow) (*Chip, *Chip) {
	return mitigationStatusChip(r.Status), severityChip(r.Severity)
}

func severityToneOf(raw string) string {
	if c := severityChip(raw); c != nil {
		return c.Tone
	}
	return toneNeutral
}

// attrString renders a typed attribute value for display without asserting a
// type the schema does not promise.
func attrString(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		if t {
			return "Yes"
		}
		return "No"
	case float64:
		return strings.TrimSuffix(strings.TrimRight(fmt.Sprintf("%.2f", t), "0"), ".")
	default:
		return fmt.Sprintf("%v", t)
	}
}
