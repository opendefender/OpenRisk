// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"context"
	"errors"
	"net/http"
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
// TWO WRITE PATHS (ADR 0010).
//
//   - Inline, the default: Append runs after the business transaction commits,
//     stores the row already marked published, and the publisher delivers it.
//     A crash between the commit and the append loses that event. It is
//     survivable because the realtime stream is never the source of truth: a
//     client that misses an event and resyncs reads the same state the API
//     would have given it anyway.
//   - Outbox, for use cases that enlist: RealtimeEventOutbox.Enlist writes the
//     row inside the use case's own transaction, unpublished. It commits or
//     rolls back with the business change, and a relay publishes it after
//     commit, at least once, with the same id on every redelivery.
//
// Which events are enlisted is listed in docs/W0-07_REALTIME_EVENT_HUB.md
// (Outbox / Publishing Strategy, Known Limitation 1). Everything else is on the
// inline path until a use case is enlisted one by one.
//
// ORDERING. Sequence is a per-tenant monotonic counter allocated from a
// one-row-per-tenant table (realtime_event_sequences) inside the writing
// transaction. The counter row lock is held until that transaction ends, so
// two transactions of one tenant cannot interleave their sequences, and the
// relay, which takes the same row, never sees a gap that is still in flight.
// Two events of one tenant therefore have a total order, and a client holding
// sequence N can tell whether it missed anything. The unique (tenant_id,
// sequence) index stays as the backstop.
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
	TenantID uuid.UUID `gorm:"type:uuid;index:idx_rt_tenant_seq,unique,priority:1;index;index:idx_rt_outbox_pending,priority:1,where:published_at IS NULL AND abandoned_at IS NULL;not null" json:"tenant_id"`

	// Sequence is the per-tenant monotonic position, assigned by the repository
	// inside the append transaction. Never by callers.
	Sequence int64 `gorm:"index:idx_rt_tenant_seq,unique,priority:2;index:idx_rt_outbox_pending,priority:2;not null" json:"sequence"`

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

	// Outbox bookkeeping (ADR 0010, migration 0086). Relay state, never part of
	// the envelope: none of it is serialised. The time columns carry no explicit
	// type tag: GORM maps time.Time to timestamptz on Postgres, and an explicit
	// "timestamptz" would stop sqlite (tests) from parsing the value back.
	//
	// PublishedAt is set when the event has been delivered. Rows written by the
	// inline path are born published; enlisted rows are born NULL and the relay
	// sets it. AbandonedAt marks a row the relay gave up on after the attempt
	// cap. Attempts counts claims; LockedUntil is the relay's lease or backoff.
	PublishedAt *time.Time `json:"-"`
	AbandonedAt *time.Time `json:"-"`
	Attempts    int        `gorm:"not null;default:0" json:"-"`
	LockedUntil *time.Time `json:"-"`
	LastError   string     `gorm:"type:text;not null;default:''" json:"-"`
}

// RealtimeEventSequence is one tenant's sequence allocator (ADR 0010). Never
// read by callers: the repository allocates from it inside the writing
// transaction, and the relay locks it to claim a tenant's pending rows.
type RealtimeEventSequence struct {
	TenantID     uuid.UUID `gorm:"type:uuid;primaryKey"`
	LastSequence int64     `gorm:"not null"`
	UpdatedAt    time.Time `gorm:"not null"`
}

// TableName pins the table name.
func (RealtimeEventSequence) TableName() string { return "realtime_event_sequences" }

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

// PrepareRealtimeEvent turns a caller's envelope into a storable row: it mints
// the id (once, here), fills the catalog version and aggregate type, defaults
// OccurredAt to now (UTC), clears Sequence, sanitises the payload, validates,
// and converts. Both the inline publisher and enlisting use cases call it, so
// the two paths cannot diverge. Errors are validation errors.
//
// The id and the payload version are minted here, never taken from a caller:
// the id is the deduplication key every consumer keys on, and letting a caller
// choose it would let one caller make two different events collide. The
// version comes from the catalog so a caller cannot claim a version the
// catalog does not publish.
func PrepareRealtimeEvent(env realtime.Envelope, now time.Time) (*RealtimeEvent, error) {
	env.ID = uuid.NewString()
	env.EnvelopeVersion = realtime.EnvelopeVersion
	if desc, ok := realtime.Lookup(env.Type); ok {
		env.Version = desc.Version
		if env.Aggregate.Type == "" {
			env.Aggregate.Type = desc.Aggregate
		}
	}
	if env.OccurredAt.IsZero() {
		env.OccurredAt = now
	}
	env.OccurredAt = env.OccurredAt.UTC()
	// Sequence is the log's to assign; a caller-supplied one is discarded.
	env.Sequence = 0
	env.Payload = realtime.SanitizePayload(env.Payload)

	if err := env.Validate(); err != nil {
		// Keeps the cause: errors.Is matches ErrValidation and the envelope's own
		// sentinel (ErrUnknownType, ...), as NewOutboundDeniedError does.
		return nil, &AppError{Err: errors.Join(ErrValidation, err), Message: err.Error(), Code: http.StatusBadRequest}
	}
	return NewRealtimeEvent(env)
}

// RealtimeEventOutbox is the transaction-scoped write side. Rows enlisted here
// commit or roll back with the caller's business transaction; the relay
// publishes them after commit. All events of one call belong to one tenant.
// Enlist must be the LAST write of the transaction: it holds the tenant's
// sequence allocator until commit.
type RealtimeEventOutbox interface {
	Enlist(ctx context.Context, events ...*RealtimeEvent) error
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
	// returns how many rows went. A row still waiting for the relay (neither
	// published nor abandoned) is never purged, whatever its age. Cross-tenant by design: retention is an
	// instance-level property, and a per-tenant purge would leave the table
	// growing for every tenant nobody thought to sweep.
	PurgeBefore(ctx context.Context, cutoff time.Time) (int64, error)
}
