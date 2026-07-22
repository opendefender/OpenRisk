// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package audittrail wires an immutable, append-only audit trail into GORM.
//
// The core idea (spec §15): developers must NOT have to remember to call a
// "log this" helper on every mutation. Instead the Plugin registers GORM
// create/update/delete callbacks that automatically write a domain.AuditEvent
// whenever a model implementing domain.Auditable is mutated. The actor (who) and
// request metadata (ip / user-agent / request id) ride along on the request
// context via WithActor, so the trail records *who* did *what* *when* with the
// before → after snapshot.
package audittrail

import (
	"context"

	"github.com/google/uuid"
)

// Actor is the request-scoped identity + metadata stamped onto every mutation.
// A nil ID marks a system/automatic change (background worker, migration…).
type Actor struct {
	ID        *uuid.UUID
	TenantID  uuid.UUID
	IPAddress string
	UserAgent string
	RequestID string
}

type actorCtxKey struct{}

// WithActor returns a context carrying the acting user + request metadata. Pass
// its result to db.WithContext(...) so the Plugin can attribute the mutation.
func WithActor(ctx context.Context, a Actor) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, actorCtxKey{}, a)
}

// ActorFromContext extracts the acting identity, if any.
func ActorFromContext(ctx context.Context) (Actor, bool) {
	if ctx == nil {
		return Actor{}, false
	}
	a, ok := ctx.Value(actorCtxKey{}).(Actor)
	return a, ok
}
