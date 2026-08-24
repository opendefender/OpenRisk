// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/pkg/realtime"
)

// ---------------------------------------------------------------------------
// RealtimeEvent — the durable event log behind the realtime hub.
//
// WHY A TABLE AT ALL. Redis pub/sub is fire-and-forget: an event published while
// a browser is reconnecting is gone, and no amount of client retry brings it
// back. Every reconnect would therefore have to be treated as a full resync,
// which on a dashboard means refetching everything on every laptop lid-close.
// Persisting the event first turns a reconnect into a cheap replay from a
// cursor, and gives the ordering guarantee somewhere to live.
//
// WHAT THIS IS NOT. It is not a transactional outbox: the row is appended after
// the business transaction commits, not inside it, so a crash in the window
// between the two loses the event. That window is documented rather than
// papered over, and it is survivable because the realtime stream is never the
// source of truth — a client that misses an event and later resyncs reads the
// same state the API would have given it anyway. Making it a true outbox means
// threading a transaction handle through every use case in the codebase; the
// design note in docs/W0-07_REALTIME_EVENT_HUB.md records that trade and what
// it would take to close it.
//
// ORDERING. Sequence is a per-tenant monotonic counter assigned inside the
// append transaction, exactly as the audit chain assigns its own — same
// technique, same per-tenant mutex plus SELECT … FOR UPDATE plus unique index
// backstop. Two events of one tenant therefore have a total order, and a client
// holding sequence N can tell whether it missed anything.
// ---------------------------------------------------------------------------

// RealtimeEvent is one published domain event, durably stored so it can be
// replayed to a client that reconnects inside the retention window.
type RealtimeEvent struct {
	// No database-side default on ID: the publisher mints it before the row is
	// written because the id is the deduplication key clients hold, and it must
	// be the same value in the log and on the wire. Keeping the model free of
	// Postgres-only DDL also lets the same schema stand up under sqlite in
	// tests, which is how this table's DDL stops drifting from the model.
	ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`

	// TenantID is the isolation boundary. Part of the unique (tenant_id,
	// sequence) index, which is what makes two writers unable to claim the same
	// position.
	TenantID uuid.UUID `gorm:"type:uuid;index:idx_rt_tenant_seq,unique,priority:1;index;not null" json:"tenant_id"`

	// Sequence is the per-tenant monotonic position, assigned by the repository
	// inside the append transaction. Never by callers.
	Sequence int64 `gorm:"index:idx_rt_tenant_seq,unique,priority:2;not null" json:"sequence"`

	Type            string `gorm:"type:varchar(64);not null;index" json:"type"`
	Version         int    `gorm:"not null;default:1" json:"version"`
	EnvelopeVersion int    `gorm:"not null;default:1" json:"envelope_version"`

	AggregateType string `gorm:"type:varchar(64);not null;index" json:"aggregate_type"`
	AggregateID   string `gorm:"type:varchar(128);index" json:"aggregate_id"`

	ActorID       *uuid.UUID `gorm:"type:uuid;index" json:"actor_id,omitempty"`
	CorrelationID string     `gorm:"type:varchar(64);index" json:"correlation_id,omitempty"`
	CausationID   string     `gorm:"type:varchar(64);index" json:"causation_id,omitempty"`

	// Payload is the minimised event body. Sanitised before it is ever written,
	// so a secret cannot reach this table even if a publisher passes one.
	Payload JSONMap `gorm:"type:jsonb" json:"payload,omitempty"`

	// OccurredAt is when the business change happened; CreatedAt is when the log
	// accepted it. They differ for relayed domain events, and the difference is
	// exactly the publication latency an operator wants to see.
	OccurredAt time.Time `gorm:"not null;index" json:"occurred_at"`
	CreatedAt  time.Time `gorm:"index" json:"created_at"`
}

// TableName pins the table name so a future model rename cannot silently orphan
// the data.
func (RealtimeEvent) TableName() string { return "realtime_events" }

// DefaultReplayRetention is how far back a reconnecting client may replay.
//
// Twenty-four hours is chosen against the failure it exists for: a laptop lid
// closed overnight, a train tunnel, a deploy that rolls the API. A client gone
// longer than a working day has almost certainly changed what it is looking at
// anyway, and a full resync — which is always available — is the honest answer.
// Beyond that the table would grow without bound for no reader.
const DefaultReplayRetention = 24 * time.Hour

// ToEnvelope rebuilds the wire envelope from a stored row, so replayed events
// are byte-for-byte the same contract as live ones. A client must not be able to
// tell a replay from a live delivery by looking at the event.
func (e RealtimeEvent) ToEnvelope() realtime.Envelope {
	actor := ""
	if e.ActorID != nil && *e.ActorID != uuid.Nil {
		actor = e.ActorID.String()
	}
	env := realtime.Envelope{
		ID:              e.ID.String(),
		EnvelopeVersion: e.EnvelopeVersion,
		Type:            realtime.EventType(e.Type),
		Version:         e.Version,
		OccurredAt:      e.OccurredAt.UTC(),
		TenantID:        e.TenantID.String(),
		ActorID:         actor,
		Aggregate:       realtime.Aggregate{Type: e.AggregateType, ID: e.AggregateID},
		Sequence:        e.Sequence,
		CorrelationID:   e.CorrelationID,
		CausationID:     e.CausationID,
	}
	if len(e.Payload) > 0 {
		env.Payload = map[string]any(e.Payload)
	}
	return env
}

// NewRealtimeEvent converts a wire envelope into a storable row. Sequence is
// deliberately left at zero: only the repository may assign it.
//
// An envelope whose tenant or id is not a UUID is refused rather than stored
// with a zero value — a row keyed on uuid.Nil would join every other malformed
// row and, worse, would sit in some tenant's stream.
func NewRealtimeEvent(env realtime.Envelope) (*RealtimeEvent, error) {
	id, err := uuid.Parse(env.ID)
	if err != nil {
		return nil, NewValidationError("realtime event id must be a uuid")
	}
	tenant, err := uuid.Parse(env.TenantID)
	if err != nil || tenant == uuid.Nil {
		return nil, NewValidationError("realtime event tenant id must be a non-nil uuid")
	}
	row := &RealtimeEvent{
		ID:              id,
		TenantID:        tenant,
		Type:            string(env.Type),
		Version:         env.Version,
		EnvelopeVersion: env.EnvelopeVersion,
		AggregateType:   env.Aggregate.Type,
		AggregateID:     env.Aggregate.ID,
		CorrelationID:   env.CorrelationID,
		CausationID:     env.CausationID,
		OccurredAt:      env.OccurredAt.UTC(),
	}
	if env.ActorID != "" {
		if actor, err := uuid.Parse(env.ActorID); err == nil && actor != uuid.Nil {
			row.ActorID = &actor
		}
	}
	if len(env.Payload) > 0 {
		row.Payload = JSONMap(env.Payload)
	}
	return row, nil
}

// RealtimeEventRepository is the durable log's port.
//
// Every method takes a tenant id and every implementation must filter on it —
// including Replay, because a replay cursor is a client-supplied value and is
// therefore exactly the input an attacker would use to reach another tenant's
// history.
type RealtimeEventRepository interface {
	// Append assigns the next per-tenant sequence and stores the event. The
	// caller's Sequence is ignored.
	Append(ctx context.Context, e *RealtimeEvent) error

	// Replay returns up to limit events for the tenant with sequence strictly
	// greater than after, oldest first. Oldest first is not a detail: a client
	// applying a replay must see the changes in the order they happened.
	Replay(ctx context.Context, tenantID uuid.UUID, after int64, limit int) ([]RealtimeEvent, error)

	// Bounds returns the oldest and newest sequence still stored for a tenant.
	// A cursor below oldest cannot be honoured and the client must resync; that
	// judgement needs both numbers, so they are read together rather than in two
	// racing queries.
	Bounds(ctx context.Context, tenantID uuid.UUID) (oldest, newest int64, err error)

	// PurgeBefore deletes events older than cutoff across all tenants and
	// returns how many rows went. Cross-tenant by design: retention is an
	// instance-level property, and a per-tenant purge would leave the table
	// growing for every tenant nobody thought to sweep.
	PurgeBefore(ctx context.Context, cutoff time.Time) (int64, error)
}
