// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package controlchain

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain/canonical"
)

// splitActorID is provenance.actor.id of everything the D7 split writes
// (A10.2): compliance_controls records no author, so the actor is the split.
const splitActorID = "control_split"

// splitConfidence is the user_entered default (D3). The status the split copies
// was a person's claim; nothing about it was derived or observed.
const splitConfidence = 0.70

// RequirementRow is what the split reads of one compliance_controls row,
// deleted rows included.
type RequirementRow struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	Name        string
	Description string
	Status      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

// SplitControl is the D7 split of one requirement row (A10.1, A10.2): a Control
// whose id is canonical.ControlIDFor(row.ID), openrisk_user, confidence 0.70,
// provenance method method, actor {system, control_split}, with the row's
// status copied once into declared_status. A not_applicable row becomes
// applicability = not_applicable, declared_status = not_implemented, an empty
// justification and the read-only pending flag. defaulted is true when the row's
// status is outside the four the model knows: it is read as not_implemented
// rather than failing the boot, and the reconciler lists the row.
func SplitControl(row RequirementRow, method canonical.Method) (c canonical.Control, defaulted bool) {
	id := canonical.ControlIDFor(row.ID)
	c = canonical.Control{
		Envelope: canonical.Envelope{
			ID:         id,
			TenantID:   row.TenantID,
			Source:     canonical.SourceOpenRiskUser,
			SourceID:   id.String(),
			CreatedAt:  row.CreatedAt,
			UpdatedAt:  row.UpdatedAt,
			Confidence: splitConfidence,
			Provenance: splitProvenance(method),
		},
		Name:          row.Name,
		Applicability: canonical.Applicable,
		Tags:          canonical.List[string]{},
	}
	if strings.TrimSpace(row.Description) != "" {
		d := row.Description
		c.Description = &d
	}
	switch row.Status {
	case "not_implemented":
		c.DeclaredStatus = canonical.DeclaredNotImplemented
	case "in_progress":
		c.DeclaredStatus = canonical.DeclaredInProgress
	case "implemented":
		c.DeclaredStatus = canonical.DeclaredImplemented
	case "not_applicable":
		c.Applicability = canonical.NotApplicable
		c.DeclaredStatus = canonical.DeclaredNotImplemented
		c = canonical.WithApplicabilityReviewPending(c, true)
	default:
		c.DeclaredStatus = canonical.DeclaredNotImplemented
		defaulted = true
	}
	return c, defaulted
}

func splitProvenance(method canonical.Method) canonical.Provenance {
	return canonical.Provenance{
		Method:         method,
		Actor:          canonical.Actor{Kind: canonical.ActorSystem, ID: splitActorID},
		ConfidenceRule: canonical.ConfidenceRule,
	}
}

// ControlToRow is the persistence shape of a canonical Control. The caller has
// validated it.
func ControlToRow(c canonical.Control, deletedAt *time.Time) (ControlRow, error) {
	prov, err := json.Marshal(c.Provenance)
	if err != nil {
		return ControlRow{}, fmt.Errorf("controlchain: provenance: %w", err)
	}
	tags, err := json.Marshal(c.Tags)
	if err != nil {
		return ControlRow{}, fmt.Errorf("controlchain: tags: %w", err)
	}
	return ControlRow{
		ID: c.ID, TenantID: c.TenantID, Source: string(c.Source), SourceID: c.SourceID,
		SourceVersion: c.SourceVersion, Confidence: c.Confidence, Provenance: string(prov),
		ObservedAt: c.ObservedAt, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt, DeletedAt: deletedAt,
		Name: c.Name, Description: c.Description, Objective: c.Objective,
		OwnerID: c.OwnerID, AssigneeID: c.AssigneeID, ReviewerID: c.ReviewerID,
		Applicability: string(c.Applicability), ApplicabilityJustification: c.ApplicabilityJustification,
		ApplicabilityReviewPending: c.ApplicabilityReviewPending(),
		DeclaredStatus:             string(c.DeclaredStatus), Tags: string(tags),
	}, nil
}

// Edge builds one canonical edge written by the reconciler (D-108 Q11, A10.2):
// openrisk_user, confidence 0.70, method migration, actor {system,
// control_split}, source_id the legacy row it was carried from. The target's
// tenant is the edge's by construction: the caller resolved both ends in that
// tenant before building it.
func Edge(tenant uuid.UUID, fromType canonical.EntityType, fromID uuid.UUID, verb canonical.Verb,
	toType canonical.EntityType, toID uuid.UUID, attrs canonical.EdgeAttributes, sourceID string, at time.Time,
) canonical.Edge {
	return canonical.Edge{
		ID:         uuid.New(),
		TenantID:   tenant,
		From:       canonical.RecordRef{Type: fromType, ID: fromID},
		Verb:       verb,
		To:         canonical.EdgeTarget{Type: toType, ID: toID, TenantID: tenant},
		Attributes: attrs,
		Source:     canonical.SourceOpenRiskUser,
		SourceID:   sourceID,
		Confidence: splitConfidence,
		Provenance: splitProvenance(canonical.MethodMigration),
		CreatedAt:  at,
		UpdatedAt:  at,
	}
}

// EdgeToRow is the persistence shape of a canonical Edge.
func EdgeToRow(e canonical.Edge) (CanonicalEdgeRow, error) {
	prov, err := json.Marshal(e.Provenance)
	if err != nil {
		return CanonicalEdgeRow{}, fmt.Errorf("controlchain: provenance: %w", err)
	}
	attrs, err := json.Marshal(e.Attributes)
	if err != nil {
		return CanonicalEdgeRow{}, fmt.Errorf("controlchain: attributes: %w", err)
	}
	k := e.Key()
	return CanonicalEdgeRow{
		ID: e.ID, TenantID: e.TenantID,
		FromType: string(e.From.Type), FromID: e.From.ID, Verb: string(e.Verb),
		ToType: string(e.To.Type), ToID: e.To.ID, Qualifier: k.Qualifier,
		Attributes: string(attrs), Source: string(e.Source), SourceID: e.SourceID,
		Confidence: e.Confidence, Provenance: string(prov), CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt,
	}, nil
}
