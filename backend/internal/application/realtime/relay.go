// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package realtime

import (
	"context"
	"encoding/json"
	"log"

	"github.com/redis/go-redis/v9"

	rt "github.com/opendefender/openrisk/pkg/realtime"
)

// PatternSubscriber is the read side of the fanout. Satisfied structurally by
// the project's Redis client.
type PatternSubscriber interface {
	PSubscribe(ctx context.Context, patterns ...string) *redis.PubSub
}

// Relay feeds this instance's hub with events published by the others.
//
// Without it, a browser connected to instance 2 would never see a change made
// on instance 1 — the single most confusing failure a multi-replica deployment
// can produce, because it looks like the feature works until you scale it.
type Relay struct {
	sub PatternSubscriber
	hub *Hub
}

// NewRelay wires the cross-instance relay.
func NewRelay(sub PatternSubscriber, hub *Hub) *Relay {
	return &Relay{sub: sub, hub: hub}
}

// Start consumes the fanout until the context is cancelled. Blocking; run it in
// a goroutine.
func (r *Relay) Start(ctx context.Context) {
	if r == nil || r.sub == nil || r.hub == nil {
		log.Println("Realtime: cross-instance relay not started (no fanout configured) — streams serve this instance only")
		return
	}
	pubsub := r.sub.PSubscribe(ctx, ChannelPattern)
	defer func() { _ = pubsub.Close() }()

	log.Printf("Realtime: relay subscribed to %s", ChannelPattern)
	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			r.handle(msg)
		}
	}
}

// handle turns one fanout message into a dispatch.
//
// The tenant is taken from the ENVELOPE, and the channel it arrived on is used
// only to reject a mismatch. Trusting the channel name would mean trusting a
// string, while trusting the envelope alone would let a malformed publication
// land in the wrong tenant's stream; requiring the two to agree means a single
// bug on either side is caught rather than delivered.
func (r *Relay) handle(msg *redis.Message) {
	if msg == nil {
		return
	}
	var env rt.Envelope
	if err := json.Unmarshal([]byte(msg.Payload), &env); err != nil {
		log.Printf("Realtime relay: dropping unparseable event on %s: %v", msg.Channel, err)
		return
	}
	channelTenant, ok := TenantFromChannel(msg.Channel)
	if !ok {
		log.Printf("Realtime relay: dropping event on unrecognised channel %q", msg.Channel)
		return
	}
	if env.TenantID != channelTenant.String() {
		// This is the alarm, not a warning: an event whose declared tenant does
		// not match the channel it travelled on must never be delivered to
		// either of them.
		log.Printf("Realtime relay: SECURITY — dropping event %s whose tenant (%s) contradicts its channel (%s)",
			env.ID, env.TenantID, channelTenant)
		return
	}
	if !rt.IsRegistered(env.Type) {
		log.Printf("Realtime relay: dropping event %s of unknown type %q", env.ID, env.Type)
		return
	}
	r.hub.Dispatch(env)
}
