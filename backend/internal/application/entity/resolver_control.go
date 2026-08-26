// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package entity

import (
	"context"
	"strconv"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// ControlReader is the narrow port over the compliance repository.
type ControlReader interface {
	GetControlByID(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) (*domain.ComplianceControl, error)
	GetFrameworkByID(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) (*domain.ComplianceFramework, error)
}

// ControlResolver serves the compliance control drawer.
type ControlResolver struct {
	controls  ControlReader
	relations RelationReader
}

func NewControlResolver(controls ControlReader, relations RelationReader) *ControlResolver {
	return &ControlResolver{controls: controls, relations: relations}
}

func (r *ControlResolver) load(ctx context.Context, c Caller, id string) (*domain.ComplianceControl, uuid.UUID, error) {
	cid, err := uuid.Parse(id)
	if err != nil {
		return nil, uuid.Nil, domain.NewNotFoundError("control", id)
	}
	if r.controls == nil {
		return nil, uuid.Nil, domain.NewNotFoundError("control", id)
	}
	ctrl, err := r.controls.GetControlByID(ctx, cid, c.TenantID)
	if err != nil {
		return nil, uuid.Nil, err
	}
	if ctrl == nil {
		return nil, uuid.Nil, domain.NewNotFoundError("control", id)
	}
	return ctrl, cid, nil
}

func (r *ControlResolver) Summary(ctx context.Context, c Caller, id string) (*Summary, error) {
	ctrl, _, err := r.load(ctx, c, id)
	if err != nil {
		return nil, err
	}

	s := &Summary{
		ID:        ctrl.ID.String(),
		Title:     ctrl.Name,
		Subtitle:  ctrl.ReferenceCode,
		Status:    controlStatusChip(string(ctrl.Status)),
		CreatedAt: timePtr(ctrl.CreatedAt),
		UpdatedAt: timePtr(ctrl.UpdatedAt),
		// A control has no score. It has a state (implemented or not) and a body
		// of proof, and neither is a number — so the drawer says the score is
		// unavailable rather than inventing a percentage that would look like a
		// measurement (§13).
		Score: unavailableScore("coverage", "Score", "a control has an implementation status, not a score"),
	}

	// The framework is loaded through its OWN tenant-scoped read rather than
	// through a preload on the control. A preload would return whatever row the
	// foreign key points at; this asks the repository whether THIS tenant may
	// see that framework, which is the same question the compliance module asks.
	if ctrl.FrameworkID != uuid.Nil && r.controls != nil {
		if fw, err := r.controls.GetFrameworkByID(ctx, ctrl.FrameworkID, c.TenantID); err == nil && fw != nil {
			s.Fields = appendField(s.Fields, field("framework", "Framework", fw.Name+" "+fw.Version, FieldBadge))
		}
	}

	s.Fields = appendField(s.Fields, field("reference_code", "Reference", ctrl.ReferenceCode, FieldText))
	s.Fields = appendField(s.Fields, field("source_reference", "Source citation", ctrl.SourceReference, FieldText))
	s.Fields = appendField(s.Fields, field("description", "Description", ctrl.Description, FieldMultilne))
	s.Fields = append(s.Fields, Field{
		Key: "evidence_count", Label: "Covering evidence", Kind: FieldNumber,
		Value: strconv.Itoa(ctrl.EvidenceCount),
	})
	return s, nil
}

func (r *ControlResolver) Relations(ctx context.Context, c Caller, id string) ([]RelationGroup, error) {
	_, cid, err := r.load(ctx, c, id)
	if err != nil {
		return nil, err
	}
	if r.relations == nil {
		return []RelationGroup{}, nil
	}
	risks, riskTotal, riskErr := r.relations.RisksForControl(ctx, c.TenantID, cid, relationCap)
	evid, evTotal, evErr := r.relations.EvidenceForControl(ctx, c.TenantID, cid, relationCap)
	return []RelationGroup{
		group("risks", "Risks", TypeRisk, risks, riskTotal, riskErr, riskChips),
		group("evidence", "Evidence", TypeEvidence, evid, evTotal, evErr, evidenceChips),
	}, nil
}

func (r *ControlResolver) Actions(ctx context.Context, c Caller, id string) []Action {
	base := "/api/v1/compliance/controls/" + id
	out := []Action{}
	if c.Can("compliance:controls:update") {
		out = append(out, Action{
			Key: "set_status", Label: "Update status", Kind: ActionPrimary,
			Method: "PATCH", Path: base, Permission: "compliance:controls:update",
		})
	}
	if c.Can("compliance:evidences:create") {
		out = append(out, Action{
			Key: "attach_evidence", Label: "Attach evidence", Kind: ActionSecondary,
			Method: "POST", Path: base + "/evidences", Permission: "compliance:evidences:create",
		})
	}
	if c.Can("compliance:controls:delete") {
		out = append(out, Action{
			Key: "delete", Label: "Delete", Kind: ActionDanger,
			Method: "DELETE", Path: base, Permission: "compliance:controls:delete",
		})
	}
	return out
}
