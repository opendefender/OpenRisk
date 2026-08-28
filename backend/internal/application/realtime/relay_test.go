// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package realtime

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	rt "github.com/opendefender/openrisk/pkg/realtime"
)

func fanoutMessage(t *testing.T, channel string, env rt.Envelope) *redis.Message {
	t.Helper()
	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	return &redis.Message{Channel: channel, Payload: string(raw)}
}

func TestRelay_DeliversAnEventFromAnotherInstance(t *testing.T) {
	hub := NewHub(HubOptions{})
	relay := NewRelay(nil, hub)
	tenant := uuid.New()

	sub, _ := hub.Subscribe(tenant, rt.Filter{})
	defer hub.Unsubscribe(sub)

	e := env(tenant, rt.RiskCreated, "r1", 4)
	relay.handle(fanoutMessage(t, TenantChannel(tenant), e))

	got, ok := recv(t, sub)
	if !ok || got.ID != e.ID {
		t.Fatal("an event published on another instance never reached this one's subscriber")
	}
}

// The security property of the relay: the channel an event arrived on and the
// tenant it claims must agree. Either one alone could be wrong; requiring both
// means a single bug is caught rather than delivered to the wrong customer.
func TestRelay_DropsAnEventWhoseTenantContradictsItsChannel(t *testing.T) {
	hub := NewHub(HubOptions{})
	relay := NewRelay(nil, hub)
	victim, attacker := uuid.New(), uuid.New()

	sub, _ := hub.Subscribe(victim, rt.Filter{})
	defer hub.Unsubscribe(sub)

	// An event claiming to belong to the victim, published on the attacker's
	// channel — and the mirror case.
	forged := env(victim, rt.RiskCreated, "secret-risk", 1)
	relay.handle(fanoutMessage(t, TenantChannel(attacker), forged))
	assertNothing(t, sub)

	mislabelled := env(attacker, rt.RiskCreated, "other-risk", 1)
	relay.handle(fanoutMessage(t, TenantChannel(victim), mislabelled))
	assertNothing(t, sub)
}

func TestRelay_DropsMalformedAndUnknownInput(t *testing.T) {
	hub := NewHub(HubOptions{})
	relay := NewRelay(nil, hub)
	tenant := uuid.New()

	sub, _ := hub.Subscribe(tenant, rt.Filter{})
	defer hub.Unsubscribe(sub)

	relay.handle(nil)
	relay.handle(&redis.Message{Channel: TenantChannel(tenant), Payload: "{not json"})
	relay.handle(fanoutMessage(t, "openrisk:something:else", env(tenant, rt.RiskCreated, "r1", 1)))

	unknown := env(tenant, rt.RiskCreated, "r1", 1)
	unknown.Type = "risk.invented"
	relay.handle(fanoutMessage(t, TenantChannel(tenant), unknown))

	assertNothing(t, sub)
}

// The same event arriving locally and again through the relay must be delivered
// once. This is the concrete case the hub's dedup window exists for.
func TestRelay_EchoOfALocalPublicationIsSuppressed(t *testing.T) {
	hub := NewHub(HubOptions{})
	relay := NewRelay(nil, hub)
	tenant := uuid.New()

	sub, _ := hub.Subscribe(tenant, rt.Filter{})
	defer hub.Unsubscribe(sub)

	e := env(tenant, rt.RiskUpdated, "r1", 9)
	hub.Dispatch(e)                                          // local publish
	relay.handle(fanoutMessage(t, TenantChannel(tenant), e)) // the echo coming back

	if _, ok := recv(t, sub); !ok {
		t.Fatal("the first delivery never arrived")
	}
	assertNothing(t, sub)
}
