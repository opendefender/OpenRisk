// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package activation turns domain milestones into server-side activation state.
//
// The contract, in one line: activation is DERIVED FROM SERVER EVENTS, never from
// client state. Everything else in this package follows from that.
//
// Emission is best-effort and nil-safe by design (Record returns nothing): a
// failure to note "you created your first risk" must never fail the creation of
// the risk. Activation is telemetry about the product's promise, not part of the
// business transaction.
package activation

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/audittrail"
	"github.com/opendefender/openrisk/pkg/monitoring"
)

// Recorder appends activation events. It is the concrete type behind the narrow
// `ActivationRecorder` ports declared by the consuming use cases (risk, asset,
// compliance, mitigation, board report, rbac), which are satisfied structurally
// so no shared interface has to be threaded through the whole application layer.
type Recorder struct {
	repo domain.ActivationRepository
	now  func() time.Time
}

// NewRecorder builds a recorder. A nil repo is legal: the recorder degrades to a
// no-op, which is what keeps `WithActivation(nil)` safe in tests and in any
// deployment that has not run migration 0043 yet.
func NewRecorder(repo domain.ActivationRepository) *Recorder {
	return &Recorder{repo: repo, now: func() time.Time { return time.Now().UTC() }}
}

// Record appends one activation occurrence for a tenant. Never returns an error:
// see the package doc.
//
// The acting user is read from the request context (the same actor the audit
// trail stamps), so callers do not have to thread a user id through use cases
// that have no other reason to know one.
func (r *Recorder) Record(ctx context.Context, tenantID uuid.UUID, key string, payload map[string]interface{}) {
	if r == nil || r.repo == nil || tenantID == uuid.Nil || key == "" {
		return
	}

	event := &domain.ActivationEvent{
		ID:         uuid.New(),
		TenantID:   tenantID,
		EventKey:   domain.ActivationEventKey(key),
		OccurredAt: r.now(),
		UserID:     actorFromContext(ctx),
	}
	if len(payload) > 0 {
		event.Payload = domain.JSONMap(payload)
	}

	// Errors are swallowed on purpose — a dead activation table must not turn a
	// successful risk creation into a 500.
	if err := r.repo.RecordEvent(ctx, event); err != nil {
		return
	}
	monitoring.RecordActivationEvent(key)
}

// RecordFor is Record with an explicit actor, for the call sites that already
// know who is acting (registration, where there is no request actor on the
// context yet) or that act on someone else's behalf.
func (r *Recorder) RecordFor(ctx context.Context, tenantID, userID uuid.UUID, key string, payload map[string]interface{}) {
	if r == nil || r.repo == nil || tenantID == uuid.Nil || key == "" {
		return
	}

	event := &domain.ActivationEvent{
		ID:         uuid.New(),
		TenantID:   tenantID,
		EventKey:   domain.ActivationEventKey(key),
		OccurredAt: r.now(),
	}
	if userID != uuid.Nil {
		id := userID
		event.UserID = &id
	}
	if len(payload) > 0 {
		event.Payload = domain.JSONMap(payload)
	}

	if err := r.repo.RecordEvent(ctx, event); err != nil {
		return
	}
	monitoring.RecordActivationEvent(key)
}

// RecordOnce appends an event only if that key has never occurred for the tenant.
// Used for the once-in-a-lifetime signals (signup, aha.reached) where a second
// row would be noise at best and a double celebration at worst. Returns true if
// this call is the one that recorded it.
func (r *Recorder) RecordOnce(ctx context.Context, tenantID uuid.UUID, key string, payload map[string]interface{}) bool {
	if r == nil || r.repo == nil || tenantID == uuid.Nil || key == "" {
		return false
	}

	seen, err := r.repo.HasEvent(ctx, tenantID, domain.ActivationEventKey(key))
	if err != nil || seen {
		return false
	}
	r.Record(ctx, tenantID, key, payload)
	return true
}

// actorFromContext extracts the acting user id stamped by the auth middleware,
// or nil for a system/background action.
func actorFromContext(ctx context.Context) *uuid.UUID {
	actor, ok := audittrail.ActorFromContext(ctx)
	if !ok || actor.ID == nil || *actor.ID == uuid.Nil {
		return nil
	}
	id := *actor.ID
	return &id
}
