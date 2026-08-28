// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package realtime

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/monitoring"
	rt "github.com/opendefender/openrisk/pkg/realtime"
)

// EventLog is the durable side of publication. Satisfied structurally by the
// GORM repository, so nothing here depends on the database package.
type EventLog interface {
	Append(ctx context.Context, e *domain.RealtimeEvent) error
}

// Fanout carries an event to the other API instances. Optional: with none, the
// hub still serves the clients connected to THIS instance, and the durable log
// still lets a client on any instance replay what it missed. That degradation
// is the point — a Redis outage must cost liveness, not correctness.
type Fanout interface {
	Publish(ctx context.Context, channel string, payload interface{}) error
}

// ChannelPrefix is the Redis key space for cross-instance event fanout.
const ChannelPrefix = "openrisk:realtime:events:"

// ChannelPattern is what a relay subscribes to in order to see every tenant.
const ChannelPattern = ChannelPrefix + "*"

// TenantChannel is the fanout channel for one tenant.
//
// Per-tenant rather than one global channel: it is defence in depth (a bug in
// the relay cannot mix tenants that never share a channel) and it makes
// `redis-cli psubscribe` a usable diagnostic when someone asks why a specific
// customer sees nothing.
func TenantChannel(tenantID uuid.UUID) string { return ChannelPrefix + tenantID.String() }

// TenantFromChannel recovers the tenant from a fanout channel name, reporting
// failure rather than a zero value — a relay that silently treated an
// unparseable channel as the nil tenant would be routing events by accident.
func TenantFromChannel(channel string) (uuid.UUID, bool) {
	if !strings.HasPrefix(channel, ChannelPrefix) {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(strings.TrimPrefix(channel, ChannelPrefix))
	if err != nil || id == uuid.Nil {
		return uuid.Nil, false
	}
	return id, true
}

// Publisher turns a domain change into a published event.
//
// The order is deliberate: validate, then persist, then deliver. Delivering
// before persisting would produce an event a reconnecting client could never
// replay — it would have been seen by whoever was online at that instant and by
// nobody else, which is the failure mode that makes a realtime feature
// untrustworthy rather than merely incomplete.
type Publisher struct {
	log    EventLog
	hub    *Hub
	fanout Fanout
	now    func() time.Time
}

// PublisherOption configures optional collaborators.
type PublisherOption func(*Publisher)

// WithFanout attaches cross-instance delivery.
func WithFanout(f Fanout) PublisherOption {
	return func(p *Publisher) { p.fanout = f }
}

// WithClock overrides the clock, for tests.
func WithClock(now func() time.Time) PublisherOption {
	return func(p *Publisher) {
		if now != nil {
			p.now = now
		}
	}
}

// NewPublisher builds the publisher. The log and hub are required; publishing
// without a log would mean publishing something that cannot be replayed.
func NewPublisher(log EventLog, hub *Hub, opts ...PublisherOption) *Publisher {
	p := &Publisher{log: log, hub: hub, now: func() time.Time { return time.Now().UTC() }}
	for _, o := range opts {
		o(p)
	}
	return p
}

// Publish validates, sanitises, persists and delivers one event.
//
// It returns an error only for problems the caller could act on — an event that
// is not publishable, or a log that refused it. A fanout failure is NOT an
// error: the event is durable and every client on this instance has it, so
// failing the caller (which is a business mutation that already succeeded)
// would trade a real success for a cosmetic one.
func (p *Publisher) Publish(ctx context.Context, env rt.Envelope) (rt.Envelope, error) {
	start := p.now()

	env = p.applyDefaults(env)
	env.Payload = rt.SanitizePayload(env.Payload)

	if err := env.Validate(); err != nil {
		monitoring.RealtimePublishFailuresTotal.WithLabelValues("invalid").Inc()
		return env, fmt.Errorf("realtime publish refused: %w", err)
	}

	row, err := domain.NewRealtimeEvent(env)
	if err != nil {
		monitoring.RealtimePublishFailuresTotal.WithLabelValues("invalid").Inc()
		return env, err
	}
	if err := p.log.Append(ctx, row); err != nil {
		monitoring.RealtimePublishFailuresTotal.WithLabelValues("log").Inc()
		return env, fmt.Errorf("realtime publish could not be persisted: %w", err)
	}
	// The sequence only exists once the log has assigned it, so the envelope
	// that goes on the wire is rebuilt from the stored row. A client must never
	// receive a live event whose sequence differs from the one it would see on
	// replay.
	env = row.ToEnvelope()

	origin := "unknown"
	if desc, ok := rt.Lookup(env.Type); ok {
		origin = string(desc.Origin)
	}
	monitoring.RealtimeEventsPublishedTotal.WithLabelValues(string(env.Type), origin).Inc()

	// Deliver locally first: clients on this instance should not wait on a
	// network round trip through Redis and back. The fanout then reaches the
	// other instances, and the echo returning here is absorbed by the hub's
	// dedup — which is exactly what that window is for.
	p.hub.Dispatch(env)

	if p.fanout != nil {
		tenant, err := uuid.Parse(env.TenantID)
		if err == nil {
			if err := p.fanout.Publish(ctx, TenantChannel(tenant), env); err != nil {
				monitoring.RealtimePublishFailuresTotal.WithLabelValues("fanout").Inc()
			}
		}
	}

	monitoring.RealtimePublishLatencySeconds.Observe(p.now().Sub(start).Seconds())
	return env, nil
}

// applyDefaults fills the fields the publisher owns.
//
// The event id and the payload version are minted here, never taken from a
// caller: the id is the deduplication key every consumer keys on, and letting a
// caller choose it would let one caller make two different events collide. The
// version comes from the catalog so a publisher cannot claim a version the
// catalog does not publish.
func (p *Publisher) applyDefaults(env rt.Envelope) rt.Envelope {
	env.ID = uuid.NewString()
	env.EnvelopeVersion = rt.EnvelopeVersion
	if desc, ok := rt.Lookup(env.Type); ok {
		env.Version = desc.Version
		if env.Aggregate.Type == "" {
			env.Aggregate.Type = desc.Aggregate
		}
	}
	if env.OccurredAt.IsZero() {
		env.OccurredAt = p.now()
	}
	env.OccurredAt = env.OccurredAt.UTC()
	// Sequence is the log's to assign; a caller-supplied one is discarded.
	env.Sequence = 0
	return env
}
