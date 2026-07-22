// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package governance holds the use cases for the Governance module (spec §15):
// the immutable audit trail, time-boxed delegations, and the configurable
// Maker-Checker approval engine. Each use case is a small injectable struct;
// every method is tenant-scoped and returns typed domain errors.
package governance

import (
	"context"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/audittrail"
)

// UserLookup resolves user emails for display (implemented by the user repo's
// EmailsByIDs). Optional everywhere — a nil lookup just leaves emails blank.
type UserLookup interface {
	EmailsByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error)
}

// AuditRecorder is the explicit, best-effort way to write a governance audit
// event (approvals, delegations, exports…) — complementing the automatic
// audittrail GORM plugin. Nil-safe: a nil repo silently drops the event so a
// telemetry failure never breaks the business action.
type AuditRecorder struct {
	repo domain.AuditEventRepository
}

func NewAuditRecorder(repo domain.AuditEventRepository) *AuditRecorder {
	return &AuditRecorder{repo: repo}
}

// Record appends an event. IP / user-agent / request-id are lifted from the
// request context (stamped by the audit middleware) when present.
func (r *AuditRecorder) Record(ctx context.Context, ev domain.AuditEvent) {
	if r == nil || r.repo == nil || ev.TenantID == uuid.Nil {
		return
	}
	if actor, ok := audittrail.ActorFromContext(ctx); ok {
		if ev.IPAddress == "" {
			ev.IPAddress = actor.IPAddress
		}
		if ev.UserAgent == "" {
			ev.UserAgent = actor.UserAgent
		}
		if ev.RequestID == "" {
			ev.RequestID = actor.RequestID
		}
	}
	_ = r.repo.Append(ctx, &ev)
}

// -------------------- List --------------------

// ListAuditEventsUseCase queries the audit trail with filters and resolves actor
// emails for display.
type ListAuditEventsUseCase struct {
	repo   domain.AuditEventRepository
	lookup UserLookup
}

func NewListAuditEventsUseCase(repo domain.AuditEventRepository) *ListAuditEventsUseCase {
	return &ListAuditEventsUseCase{repo: repo}
}

// WithUserLookup enriches results with actor emails.
func (uc *ListAuditEventsUseCase) WithUserLookup(l UserLookup) *ListAuditEventsUseCase {
	uc.lookup = l
	return uc
}

// AuditEventsResult is a page of events plus the unfiltered total for paging.
type AuditEventsResult struct {
	Events []domain.AuditEvent `json:"events"`
	Total  int64               `json:"total"`
	Limit  int                 `json:"limit"`
	Offset int                 `json:"offset"`
}

func (uc *ListAuditEventsUseCase) Execute(ctx context.Context, tenantID uuid.UUID, f domain.AuditEventFilter) (*AuditEventsResult, error) {
	events, total, err := uc.repo.List(ctx, tenantID, f)
	if err != nil {
		return nil, err
	}
	uc.resolveEmails(ctx, events)
	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	return &AuditEventsResult{Events: events, Total: total, Limit: limit, Offset: f.Offset}, nil
}

// resolveEmails fills ActorEmail on each event (best-effort — degrades to the
// bare UUID / empty when no lookup or on error).
func (uc *ListAuditEventsUseCase) resolveEmails(ctx context.Context, events []domain.AuditEvent) {
	if uc.lookup == nil || len(events) == 0 {
		return
	}
	idset := map[uuid.UUID]struct{}{}
	for _, e := range events {
		if e.ActorID != nil && *e.ActorID != uuid.Nil {
			idset[*e.ActorID] = struct{}{}
		}
	}
	if len(idset) == 0 {
		return
	}
	ids := make([]uuid.UUID, 0, len(idset))
	for id := range idset {
		ids = append(ids, id)
	}
	emails, err := uc.lookup.EmailsByIDs(ctx, ids)
	if err != nil {
		return
	}
	for i := range events {
		if events[i].ActorID != nil {
			events[i].ActorEmail = emails[*events[i].ActorID]
		}
	}
}
