// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package controlchain

import (
	"context"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/domain/canonical"
)

// Implementations of a control (#1415, #553 slice S4). An implementation says
// where and how one control is in place: a description, a lifecycle status, an
// owner, and the assets it applies to (applies_to edges, assets only in v1).
//
// Delete semantics, consistent with the control's (D-116 point 5): a delete is
// soft (deleted_at, tenant_id in the WHERE) and keeps the applies_to edges, which
// readers filter on the liveness of both ends (A10.3). Deleting an implementation
// also soft-deletes its live tests, in the same transaction, one audit entry
// each, so a test never outlives the implementation it checks as a live record.
// Deleting a control does not cascade (that use case is S3's and unchanged):
// every implementation and test read resolves its parent chain live, in the
// tenant, so the children of a deleted control read as absent (ErrNotFound).
// Replacing the targets of an implementation hard-deletes the removed
// applies_to edges; the audit entry of that write carries the before and after
// target sets, which is the journal A10.3 asks for.

// MaxAppliesTo bounds the targets of one implementation.
const MaxAppliesTo = 100

// ImplementationRecord is an implementation and the live assets it applies to.
type ImplementationRecord struct {
	Implementation canonical.ControlImplementation
	// AppliesTo is the ids of the live assets of the tenant the implementation
	// applies to, sorted.
	AppliesTo []uuid.UUID
	// StaleTargets is filled by UpdateImplementation only: the targets of the
	// implementation's applies_to edges that are not live (their asset was
	// deleted). They are invisible to readers but still stored, so a replacement
	// must delete and journal them too.
	StaleTargets []uuid.UUID
}

// ImplementationRepository is the persistence the implementation use cases need.
// Every method is scoped by the tenant it is given; another tenant's id (of an
// implementation, a control or an asset) reads as absent (ErrNotFound).
type ImplementationRepository interface {
	// CreateImplementation resolves the control and every edge target in the
	// tenant, then stores the implementation, its applies_to edges and the audit
	// entry in one transaction.
	CreateImplementation(ctx context.Context, tenantID uuid.UUID, impl canonical.ControlImplementation, edges []canonical.Edge, audit *domain.AuditEvent) error
	GetImplementation(ctx context.Context, tenantID, id uuid.UUID) (*ImplementationRecord, error)
	// ListImplementations returns the live implementations of a live control.
	ListImplementations(ctx context.Context, tenantID, controlID uuid.UUID, limit, offset int) ([]ImplementationRecord, error)
	// UpdateImplementation loads the live implementation under lock and lets apply
	// decide: it returns the implementation to store, the applies_to edges to add
	// (their targets are resolved in the tenant) and the target asset ids whose
	// edge to delete, and the audit entry. A nil entry means nothing changed and
	// nothing is written. An error from apply rolls everything back.
	UpdateImplementation(ctx context.Context, tenantID, id uuid.UUID,
		apply func(cur ImplementationRecord) (next canonical.ControlImplementation, add []canonical.Edge, remove []uuid.UUID, ev *domain.AuditEvent, err error),
	) (*ImplementationRecord, error)
	// DeleteImplementation soft-deletes the live implementation and its live
	// tests after apply accepted it. apply gets those tests (full rows, for the audit before image) and returns
	// the audit entries to append, in the same transaction.
	DeleteImplementation(ctx context.Context, tenantID, id uuid.UUID,
		apply func(cur ImplementationRecord, tests []canonical.ControlTest) ([]*domain.AuditEvent, error)) error
}

// TargetRef names one applies_to target. Only assets in v1: System and Service
// have no model yet (#1217).
type TargetRef struct {
	Type string
	ID   uuid.UUID
}

// TargetView is a target as the API returns it.
type TargetView struct {
	Type canonical.EntityType `json:"type"`
	ID   uuid.UUID            `json:"id"`
}

// ImplementationView is an implementation as the API returns it: its content and
// its envelope (source, confidence, provenance). The tenant is not part of it.
type ImplementationView struct {
	ID           uuid.UUID                      `json:"id"`
	ControlID    uuid.UUID                      `json:"control_id"`
	Description  string                         `json:"description"`
	Status       canonical.ImplementationStatus `json:"status"`
	InPlaceSince *canonical.Date                `json:"in_place_since"`
	OwnerID      *uuid.UUID                     `json:"owner_id"`
	AssigneeID   *uuid.UUID                     `json:"assignee_id"`
	ReviewerID   *uuid.UUID                     `json:"reviewer_id"`
	AppliesTo    []TargetView                   `json:"applies_to"`
	Source       canonical.Source               `json:"source"`
	SourceID     string                         `json:"source_id"`
	Confidence   float64                        `json:"confidence"`
	Provenance   canonical.Provenance           `json:"provenance"`
	ObservedAt   *time.Time                     `json:"observed_at"`
	CreatedAt    time.Time                      `json:"created_at"`
	UpdatedAt    time.Time                      `json:"updated_at"`
}

// ImplementationViewOf renders an implementation record.
func ImplementationViewOf(r ImplementationRecord) ImplementationView {
	ci := r.Implementation
	targets := make([]TargetView, len(r.AppliesTo))
	for i, id := range r.AppliesTo {
		targets[i] = TargetView{Type: canonical.TypeAsset, ID: id}
	}
	return ImplementationView{
		ID: ci.ID, ControlID: ci.ControlID, Description: ci.Description, Status: ci.Status, InPlaceSince: ci.InPlaceSince,
		OwnerID: ci.OwnerID, AssigneeID: ci.AssigneeID, ReviewerID: ci.ReviewerID, AppliesTo: targets,
		Source: ci.Source, SourceID: ci.SourceID, Confidence: ci.Confidence, Provenance: ci.Provenance,
		ObservedAt: ci.ObservedAt, CreatedAt: ci.CreatedAt, UpdatedAt: ci.UpdatedAt,
	}
}

func implementationViewsOf(rs []ImplementationRecord) []ImplementationView {
	out := make([]ImplementationView, len(rs))
	for i := range rs {
		out[i] = ImplementationViewOf(rs[i])
	}
	return out
}

const auditImplementationType = "control_implementation"

// cleanTargets turns the requested targets into distinct, sorted asset ids. At
// least one is required (AC2, D-108 Q12) and only assets are accepted in v1.
func cleanTargets(in []TargetRef) ([]uuid.UUID, error) {
	if len(in) == 0 {
		return nil, domain.InternalControlError(domain.ErrValidation, domain.IntCtlAppliesToRequired)
	}
	if len(in) > MaxAppliesTo {
		return nil, domain.InternalControlError(domain.ErrValidation, domain.IntCtlAppliesToInvalid)
	}
	seen := make(map[uuid.UUID]bool, len(in))
	out := make([]uuid.UUID, 0, len(in))
	for _, t := range in {
		if (t.Type != "" && t.Type != string(canonical.TypeAsset)) || t.ID == uuid.Nil {
			return nil, domain.InternalControlError(domain.ErrValidation, domain.IntCtlAppliesToInvalid)
		}
		if !seen[t.ID] {
			seen[t.ID] = true
			out = append(out, t.ID)
		}
	}
	sortIDs(out)
	return out, nil
}

func sortIDs(ids []uuid.UUID) {
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
}

func idStrings(ids []uuid.UUID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	return out
}

// appliesToEdge builds the applies_to edge of one asset with the server-set
// user_entered envelope. The target's tenant is the edge's by construction: the
// repository resolves the asset in that tenant before it inserts the edge.
func appliesToEdge(c Caller, implID, assetID uuid.UUID, conf float64, now time.Time) (canonical.Edge, error) {
	edgeID := uuid.New()
	e := canonical.Edge{
		ID: edgeID, TenantID: c.TenantID,
		From:       canonical.RecordRef{Type: canonical.TypeControlImplementation, ID: implID},
		Verb:       canonical.VerbAppliesTo,
		To:         canonical.EdgeTarget{Type: canonical.TypeAsset, ID: assetID, TenantID: c.TenantID},
		Source:     canonical.SourceOpenRiskUser,
		SourceID:   edgeID.String(),
		Confidence: conf,
		Provenance: userProvenance(c.UserID),
		CreatedAt:  now, UpdatedAt: now,
	}
	if err := validation(e.Validate()); err != nil {
		return canonical.Edge{}, err
	}
	return e, nil
}

// implementationSnapshot is the audited image of an implementation: its content,
// who owns it and where it applies. The target set is what journals an edge
// hard delete.
func implementationSnapshot(ci canonical.ControlImplementation, appliesTo []uuid.UUID) domain.JSONMap {
	m := domain.JSONMap{
		"id":          ci.ID.String(),
		"tenant_id":   ci.TenantID.String(),
		"control_id":  ci.ControlID.String(),
		"description": ci.Description,
		"status":      string(ci.Status),
		"source":      string(ci.Source),
		"applies_to":  idStrings(appliesTo),
	}
	if ci.InPlaceSince != nil {
		m["in_place_since"] = string(*ci.InPlaceSince)
	}
	for k, v := range map[string]*uuid.UUID{"owner_id": ci.OwnerID, "assignee_id": ci.AssigneeID, "reviewer_id": ci.ReviewerID} {
		if v != nil {
			m[k] = v.String()
		}
	}
	return m
}

// newEnvelope is the server-set envelope of a user-written record.
func newEnvelope(c Caller, id uuid.UUID, conf float64, now time.Time) canonical.Envelope {
	return canonical.Envelope{
		ID: id, TenantID: c.TenantID, Source: canonical.SourceOpenRiskUser, SourceID: id.String(),
		CreatedAt: now, UpdatedAt: now, Confidence: conf, Provenance: userProvenance(c.UserID),
	}
}
