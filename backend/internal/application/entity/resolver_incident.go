// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package entity

import (
	"context"
	"strconv"

	"github.com/opendefender/openrisk/internal/domain"
)

// IncidentReader is the narrow port over the incident service.
//
// The tenant is a string here, not a uuid, because domain.Incident stores it as
// one — the incident module predates the uuid convention and was never migrated.
// The port states the truth rather than hiding it behind a conversion that would
// have to be undone one layer down.
type IncidentReader interface {
	GetIncident(tenantID string, id uint) (*domain.Incident, error)
}

// IncidentResolver serves the incident drawer.
//
// Incidents are the one type with a SEQUENTIAL integer id. That makes them the
// easiest entity in the product to enumerate — /entities/incident/1, 2, 3 — so
// the tenant predicate below is not a formality: it is the only thing standing
// between a curious user and another tenant's breach register.
type IncidentResolver struct {
	incidents IncidentReader
	relations RelationReader
}

func NewIncidentResolver(incidents IncidentReader, relations RelationReader) *IncidentResolver {
	return &IncidentResolver{incidents: incidents, relations: relations}
}

func (r *IncidentResolver) load(ctx context.Context, c Caller, id string) (*domain.Incident, uint, error) {
	n, err := strconv.ParseUint(id, 10, 64)
	if err != nil || n == 0 {
		return nil, 0, domain.NewNotFoundError("incident", id)
	}
	if r.incidents == nil {
		return nil, 0, domain.NewNotFoundError("incident", id)
	}
	inc, err := r.incidents.GetIncident(c.TenantID.String(), uint(n))
	if err != nil || inc == nil {
		// The service already answers not-found for another tenant's id; any
		// other failure is reported the same way rather than leaking whether the
		// row exists.
		return nil, 0, domain.NewNotFoundError("incident", id)
	}
	return inc, uint(n), nil
}

func (r *IncidentResolver) Summary(ctx context.Context, c Caller, id string) (*Summary, error) {
	inc, _, err := r.load(ctx, c, id)
	if err != nil {
		return nil, err
	}

	s := &Summary{
		ID:        strconv.FormatUint(uint64(inc.ID), 10),
		Title:     inc.Title,
		Subtitle:  inc.Description,
		Status:    incidentStatusChip(inc.Status),
		Severity:  severityChip(inc.Severity),
		CreatedAt: timePtr(inc.CreatedAt),
		UpdatedAt: timePtr(inc.UpdatedAt),
		// An incident's magnitude is its severity band, which is already the
		// header chip. There is no numeric incident score in this domain and the
		// drawer does not manufacture one.
		Score: unavailableScore("severity", "Score", "an incident carries a severity band, not a score"),
	}

	s.Owner = actorFor(inc.OwnerID)
	if s.Owner == nil && inc.AssignedTo != "" {
		s.Owner = &Actor{Label: inc.AssignedTo}
	}

	s.Fields = appendField(s.Fields, field("incident_type", "Type", title(inc.IncidentType), FieldBadge))
	s.Fields = appendField(s.Fields, field("reported_by", "Reported by", inc.ReportedBy, FieldUser))
	s.Fields = appendField(s.Fields, field("source", "Source", title(inc.Source), FieldBadge))
	s.Fields = appendField(s.Fields, field("external_id", "External reference", inc.ExternalID, FieldText))

	// Provenance. An incident that appeared on its own is unnerving until you can
	// see what opened it, so the origin is a first-class summary field with the
	// rule that produced it named.
	s.Fields = appendField(s.Fields, field("origin", "Origin", title(inc.Origin), FieldBadge))
	s.Fields = appendField(s.Fields, field("origin_rule", "Opened by rule", inc.OriginRuleName, FieldText))
	s.Fields = appendField(s.Fields, field("origin_detail", "Origin detail", inc.OriginDetail, FieldMultilne))

	if inc.ResolvedAt != nil {
		s.Fields = append(s.Fields, Field{
			Key: "resolved_at", Label: "Resolved", Kind: FieldDate,
			Value: inc.ResolvedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	s.Fields = appendField(s.Fields, field("resolution", "Resolution", inc.Resolution, FieldMultilne))
	return s, nil
}

func (r *IncidentResolver) Relations(ctx context.Context, c Caller, id string) ([]RelationGroup, error) {
	_, n, err := r.load(ctx, c, id)
	if err != nil {
		return nil, err
	}
	if r.relations == nil {
		return []RelationGroup{}, nil
	}
	risks, riskTotal, riskErr := r.relations.RisksForIncident(ctx, c.TenantID, n, relationCap)
	assets, assetTotal, assetErr := r.relations.AssetsForIncident(ctx, c.TenantID, n, relationCap)
	return []RelationGroup{
		group("risks", "Risks", TypeRisk, risks, riskTotal, riskErr, riskChips),
		group("assets", "Assets", TypeAsset, assets, assetTotal, assetErr, assetChips),
	}, nil
}

func (r *IncidentResolver) Actions(ctx context.Context, c Caller, id string) []Action {
	base := "/api/v1/incidents/" + id
	out := []Action{}
	if c.Can("incidents:update") {
		out = append(out,
			Action{Key: "update", Label: "Update", Kind: ActionPrimary,
				Method: "PUT", Path: base, Permission: "incidents:update"},
			Action{Key: "add_action", Label: "Add action", Kind: ActionSecondary,
				Method: "POST", Path: base + "/actions", Permission: "incidents:update"},
			Action{Key: "post_mortem", Label: "Post-mortem", Kind: ActionSecondary,
				Method: "PUT", Path: base + "/post-mortem", Permission: "incidents:update"},
		)
	}
	if c.Can("incidents:delete") {
		out = append(out, Action{
			Key: "delete", Label: "Delete", Kind: ActionDanger,
			Method: "DELETE", Path: base, Permission: "incidents:delete",
		})
	}
	return out
}
