// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package controlexception holds the use cases of control exceptions (#1428,
// #555 slice 2): create, list and revoke a time-boxed, owned, justified waiver of
// one control.
//
// Every use case takes a Caller built from the signed session, checks the
// permission itself (the route middleware is the first gate, not the only one),
// and puts the caller's tenant in every repository call. A write and its audit
// entry are one transaction, owned by the repository.
package controlexception

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// Permissions the use cases require. They are the strings the routes require.
const (
	PermissionRead   = "compliance:controls:read"
	PermissionUpdate = "compliance:controls:update"
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
		return domain.ControlExceptionError(domain.ErrUnauthorized, domain.CtlExcUnauthorized)
	}
	if !c.can(permission) {
		return domain.ControlExceptionError(domain.ErrForbidden, domain.CtlExcForbidden)
	}
	return nil
}

// Repository is the persistence the use cases need. Every method is scoped by the
// tenant it is given.
type Repository interface {
	// ControlExists: the canonical control exists, undeleted, in the tenant.
	ControlExists(ctx context.Context, tenantID, controlID uuid.UUID) (bool, error)
	// Create stores the exception and appends audit in the same transaction.
	Create(ctx context.Context, ex *domain.ControlException, audit *domain.AuditEvent) error
	// ListByControl returns the control's exceptions in the tenant, newest first.
	ListByControl(ctx context.Context, tenantID, controlID uuid.UUID) ([]domain.ControlException, error)
	// Revoke loads the exception by (tenant, id), runs apply on it and stores the
	// result and the audit entry apply returned in one transaction. A missing or
	// foreign id is ErrNotFound; an error from apply rolls everything back.
	Revoke(ctx context.Context, tenantID, id uuid.UUID,
		apply func(current *domain.ControlException) (*domain.AuditEvent, error)) (*domain.ControlException, error)
}

// OwnerValidator checks that a user may own work in the tenant: an active member
// of it. Satisfied by ownership.Service.
type OwnerValidator interface {
	Validate(ctx context.Context, tenantID uuid.UUID, patch domain.OwnershipPatch) error
}

// Clock returns the caller's instant. It is injected so a test pins it.
type Clock func() time.Time

// View is an exception as the API returns it, with the status derived at the
// instant of the request.
type View struct {
	ID           uuid.UUID                     `json:"id"`
	ControlID    uuid.UUID                     `json:"control_id"`
	OwnerID      uuid.UUID                     `json:"owner_id"`
	Reason       string                        `json:"reason"`
	ApprovedBy   uuid.UUID                     `json:"approved_by"`
	StartsAt     time.Time                     `json:"starts_at"`
	ExpiresAt    time.Time                     `json:"expires_at"`
	Status       domain.ControlExceptionStatus `json:"status"`
	RevokedAt    *time.Time                    `json:"revoked_at,omitempty"`
	RevokedBy    *uuid.UUID                    `json:"revoked_by,omitempty"`
	RevokeReason string                        `json:"revoke_reason,omitempty"`
	CreatedAt    time.Time                     `json:"created_at"`
	UpdatedAt    time.Time                     `json:"updated_at"`
}

// ViewOf renders an exception at now. The tenant is deliberately not part of it.
func ViewOf(e domain.ControlException, now time.Time) View {
	return View{
		ID: e.ID, ControlID: e.ControlID, OwnerID: e.OwnerID, Reason: e.Reason,
		ApprovedBy: e.ApprovedBy, StartsAt: e.StartsAt, ExpiresAt: e.ExpiresAt,
		Status:    e.StatusAt(now),
		RevokedAt: e.RevokedAt, RevokedBy: e.RevokedBy, RevokeReason: e.RevokeReason,
		CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt,
	}
}

const auditEntityType = "control_exception"

// snapshot is the audited image of an exception: the decision, who owns it, when
// it ends, how it reads at now.
func snapshot(e domain.ControlException, now time.Time) domain.JSONMap {
	m := domain.JSONMap{
		"id":          e.ID.String(),
		"tenant_id":   e.TenantID.String(),
		"control_id":  e.ControlID.String(),
		"owner_id":    e.OwnerID.String(),
		"reason":      e.Reason,
		"approved_by": e.ApprovedBy.String(),
		"starts_at":   e.StartsAt.UTC().Format(time.RFC3339Nano),
		"expires_at":  e.ExpiresAt.UTC().Format(time.RFC3339Nano),
		"status":      string(e.StatusAt(now)),
	}
	if e.RevokedAt != nil {
		m["revoked_at"] = e.RevokedAt.UTC().Format(time.RFC3339Nano)
	}
	if e.RevokedBy != nil {
		m["revoked_by"] = e.RevokedBy.String()
	}
	if e.RevokeReason != "" {
		m["revoke_reason"] = e.RevokeReason
	}
	return m
}

// auditEvent builds the entry written in the write's transaction. Source is
// explicit: the use case knows what the action meant. Reason text is part of the
// snapshot; no credential is.
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

func isValidation(err error) bool { return errors.Is(err, domain.ErrValidation) }
