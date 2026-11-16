// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package controlchain holds the use cases of the internal control library
// (#1414, #553 slice S3, D-108 Q5): create, read, edit and delete a control of
// the ADR 0008 chain, and link it to the requirements it addresses.
//
// Every use case takes a Caller built from the signed session, checks the
// permission itself (the route middleware is the first gate, not the only one),
// and puts the caller's tenant in every repository call. A write and its audit
// entry are one transaction, owned by the repository. The envelope of a control
// (id, tenant, source, confidence, provenance) is set here and never read from a
// request: the inputs below have no field for it.
package controlchain

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/domain/canonical"
)

// Permissions the use cases require. They are the strings the routes require.
const (
	PermissionRead   = "compliance:controls:read"
	PermissionCreate = "compliance:controls:create"
	PermissionUpdate = "compliance:controls:update"
	PermissionDelete = "compliance:controls:delete"
)

// Origin describes the request for the audit entry.
type Origin struct {
	// ActorType is domain.AuditActorUser or domain.AuditActorServiceToken.
	ActorType  string
	ActorLabel string
	IPAddress  string
	UserAgent  string
	RequestID  string
}

// Caller is the authenticated identity a use case runs as. TenantID and UserID
// come from the signed session, never from a request body.
type Caller struct {
	TenantID    uuid.UUID
	UserID      uuid.UUID
	Permissions []string
	Origin      Origin
}

// can mirrors the route middleware's matching: an exact permission, "*", or a
// "resource:*" wildcard.
func (c Caller) can(required string) bool {
	for _, p := range c.Permissions {
		if p == required || p == "*" {
			return true
		}
		if len(p) > 2 && strings.HasSuffix(p, ":*") {
			prefix := p[:len(p)-1]
			if len(required) > len(prefix) && strings.HasPrefix(required, prefix) {
				return true
			}
		}
	}
	return false
}

// authorize is the use cases' first step: no tenant or no user is Unauthorized
// (nothing is read or written), a missing permission is Forbidden.
func (c Caller) authorize(permission string) error {
	if c.TenantID == uuid.Nil || c.UserID == uuid.Nil {
		return domain.InternalControlError(domain.ErrUnauthorized, domain.IntCtlUnauthorized)
	}
	if !c.can(permission) {
		return domain.InternalControlError(domain.ErrForbidden, domain.IntCtlForbidden)
	}
	return nil
}

// Repository is the persistence the use cases need. Every method is scoped by the
// tenant it is given; another tenant's id reads as absent (ErrNotFound).
type Repository interface {
	// CreateControl stores the control and its audit entry in one transaction.
	CreateControl(ctx context.Context, c canonical.Control, audit *domain.AuditEvent) error
	GetControl(ctx context.Context, tenantID, id uuid.UUID) (*canonical.Control, error)
	ListControls(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]canonical.Control, error)
	// UpdateControl loads the live control under lock and lets apply decide: it
	// gets the stored control and a probe that tells, inside the transaction,
	// whether the control has live, accepted, unexpired evidence at asOf (D-108
	// Q16). apply returns the control to store and the audit entry of the change;
	// a nil entry means nothing changed and nothing is written. An error from
	// apply rolls everything back.
	UpdateControl(ctx context.Context, tenantID, id uuid.UUID,
		apply func(cur canonical.Control, evidenced func(asOf time.Time) (bool, error)) (canonical.Control, *domain.AuditEvent, error),
	) (*canonical.Control, error)
	// DeleteControl soft-deletes the live control after apply accepted it.
	DeleteControl(ctx context.Context, tenantID, id uuid.UUID, apply func(cur canonical.Control) (*domain.AuditEvent, error)) error
	// LinkRequirement resolves both ends of the addressed_by edge in the tenant
	// (ErrNotFound otherwise) and inserts it idempotently. created is false when
	// the edge already existed; the audit entry is written only when it did not.
	LinkRequirement(ctx context.Context, tenantID uuid.UUID, edge canonical.Edge, audit *domain.AuditEvent) (created bool, err error)
	// UnlinkRequirement deletes the addressed_by edge. A control or an edge that
	// is not in the tenant is ErrNotFound.
	UnlinkRequirement(ctx context.Context, tenantID, controlID, requirementID uuid.UUID, audit *domain.AuditEvent) error
	// ListRequirementsOfControl returns the live requirements the control addresses.
	ListRequirementsOfControl(ctx context.Context, tenantID, controlID uuid.UUID) ([]domain.InternalControlRequirement, error)
	// ListControlsOfRequirement returns the live controls that address the
	// requirement, split controls included.
	ListControlsOfRequirement(ctx context.Context, tenantID, requirementID uuid.UUID) ([]canonical.Control, error)
}

// OwnerValidator checks that a user may own work in the tenant: an active member
// of it. Satisfied by ownership.Service.
type OwnerValidator interface {
	Validate(ctx context.Context, tenantID uuid.UUID, patch domain.OwnershipPatch) error
}

// Clock returns the caller's instant. It is injected so a test pins it.
type Clock func() time.Time

// ControlView is a control as the API returns it: its content and its envelope
// (source, confidence, provenance), so a reader can tell a control the tenant
// wrote from one the legacy split made. The tenant is deliberately not part of it.
type ControlView struct {
	ID                         uuid.UUID                `json:"id"`
	Name                       string                   `json:"name"`
	Description                *string                  `json:"description"`
	Objective                  *string                  `json:"objective"`
	OwnerID                    *uuid.UUID               `json:"owner_id"`
	AssigneeID                 *uuid.UUID               `json:"assignee_id"`
	ReviewerID                 *uuid.UUID               `json:"reviewer_id"`
	Applicability              canonical.Applicability  `json:"applicability"`
	ApplicabilityJustification *string                  `json:"applicability_justification"`
	ApplicabilityReviewPending bool                     `json:"applicability_review_pending"`
	DeclaredStatus             canonical.DeclaredStatus `json:"declared_status"`
	Tags                       []string                 `json:"tags"`
	Source                     canonical.Source         `json:"source"`
	SourceID                   string                   `json:"source_id"`
	Confidence                 float64                  `json:"confidence"`
	Provenance                 canonical.Provenance     `json:"provenance"`
	CreatedAt                  time.Time                `json:"created_at"`
	UpdatedAt                  time.Time                `json:"updated_at"`
}

// ViewOf renders a control.
func ViewOf(c canonical.Control) ControlView {
	tags := []string(c.Tags)
	if tags == nil {
		tags = []string{}
	}
	return ControlView{
		ID: c.ID, Name: c.Name, Description: c.Description, Objective: c.Objective,
		OwnerID: c.OwnerID, AssigneeID: c.AssigneeID, ReviewerID: c.ReviewerID,
		Applicability: c.Applicability, ApplicabilityJustification: c.ApplicabilityJustification,
		ApplicabilityReviewPending: c.ApplicabilityReviewPending(), DeclaredStatus: c.DeclaredStatus,
		Tags: tags, Source: c.Source, SourceID: c.SourceID, Confidence: c.Confidence, Provenance: c.Provenance,
		CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
	}
}

func viewsOf(cs []canonical.Control) []ControlView {
	out := make([]ControlView, len(cs))
	for i := range cs {
		out[i] = ViewOf(cs[i])
	}
	return out
}

// Limits of a tag list.
const (
	maxTags   = 50
	maxTagLen = 64
)

func cleanTags(in []string) (canonical.List[string], error) {
	if len(in) > maxTags {
		return nil, domain.InternalControlError(domain.ErrValidation, domain.IntCtlInvalidTags)
	}
	out := make(canonical.List[string], 0, len(in))
	for _, t := range in {
		t = strings.TrimSpace(t)
		if t == "" || len([]rune(t)) > maxTagLen {
			return nil, domain.InternalControlError(domain.ErrValidation, domain.IntCtlInvalidTags)
		}
		out = append(out, t)
	}
	return out, nil
}

// validation maps a canonical refusal onto domain.ErrValidation (400). The
// canonical messages name the field and the rule, never the value.
func validation(err error) error {
	if err == nil || !errors.Is(err, canonical.ErrValidation) {
		return err
	}
	return domain.NewValidationError(strings.TrimPrefix(err.Error(), canonical.ErrValidation.Error()+": "))
}

// userProvenance is the provenance of a write a person made through the API:
// manual_entry by that user (never the split's {system, control_split}).
func userProvenance(userID uuid.UUID) canonical.Provenance {
	return canonical.Provenance{
		Method:         canonical.MethodManualEntry,
		Actor:          canonical.Actor{Kind: canonical.ActorUser, ID: userID.String()},
		ConfidenceRule: canonical.ConfidenceRule,
	}
}

func userConfidence() (float64, error) {
	return canonical.DefaultConfidence(canonical.SourceOpenRiskUser, canonical.ReviewContext{})
}

// validateOwners checks every user the patch assigns is a member of the tenant.
// Another tenant's user and a deactivated one read the same.
func validateOwners(ctx context.Context, owners OwnerValidator, tenantID uuid.UUID, patch domain.OwnershipPatch) error {
	if len(patch.CandidateIDs()) == 0 {
		return nil
	}
	if owners == nil {
		return domain.InternalControlError(domain.ErrInternal, domain.IntCtlUnavailable)
	}
	if err := owners.Validate(ctx, tenantID, patch); err != nil {
		if errors.Is(err, domain.ErrValidation) {
			return domain.InternalControlError(domain.ErrValidation, domain.IntCtlOwnerInvalid)
		}
		return err
	}
	return nil
}

const auditEntityType = "control"

// snapshot is the audited image of a control: its content and who owns it. No
// credential is part of it.
func snapshot(c canonical.Control) domain.JSONMap {
	m := domain.JSONMap{
		"id":                           c.ID.String(),
		"tenant_id":                    c.TenantID.String(),
		"name":                         c.Name,
		"applicability":                string(c.Applicability),
		"applicability_review_pending": c.ApplicabilityReviewPending(),
		"declared_status":              string(c.DeclaredStatus),
		"source":                       string(c.Source),
		"tags":                         []string(c.Tags),
	}
	for k, v := range map[string]*string{
		"description": c.Description, "objective": c.Objective, "applicability_justification": c.ApplicabilityJustification,
	} {
		if v != nil {
			m[k] = *v
		}
	}
	for k, v := range map[string]*uuid.UUID{"owner_id": c.OwnerID, "assignee_id": c.AssigneeID, "reviewer_id": c.ReviewerID} {
		if v != nil {
			m[k] = v.String()
		}
	}
	return m
}

// changedFields lists the snapshot keys whose value differs.
func changedFields(before, after domain.JSONMap) domain.StringList {
	var out domain.StringList
	seen := map[string]bool{}
	for k, v := range after {
		seen[k] = true
		if !sameValue(before[k], v) {
			out = append(out, k)
		}
	}
	for k := range before {
		if !seen[k] {
			out = append(out, k)
		}
	}
	// Deterministic order for the trail.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

func sameValue(a, b any) bool {
	as, aok := a.([]string)
	bs, bok := b.([]string)
	if aok && bok {
		return strings.Join(as, "\x00") == strings.Join(bs, "\x00")
	}
	return a == b
}

// auditEvent builds the entry written in the write's transaction.
func auditEvent(c Caller, action domain.AuditAction, summary string, before, after domain.JSONMap,
	changed domain.StringList, entityID uuid.UUID, now time.Time) *domain.AuditEvent {
	actor := c.UserID
	return &domain.AuditEvent{
		ID:            uuid.New(),
		TenantID:      c.TenantID,
		ActorID:       &actor,
		ActorType:     c.Origin.ActorType,
		ActorLabel:    c.Origin.ActorLabel,
		Action:        action,
		EntityType:    auditEntityType,
		EntityID:      entityID.String(),
		Summary:       summary,
		Before:        before,
		After:         after,
		ChangedFields: changed,
		IPAddress:     c.Origin.IPAddress,
		UserAgent:     c.Origin.UserAgent,
		RequestID:     c.Origin.RequestID,
		Source:        domain.AuditSourceExplicit,
		CreatedAt:     now.UTC(),
	}
}
