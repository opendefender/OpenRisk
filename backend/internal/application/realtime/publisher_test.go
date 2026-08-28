// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	rt "github.com/opendefender/openrisk/pkg/realtime"
)

// memLog is an in-memory stand-in for the durable log that assigns per-tenant
// sequences the same way the repository does.
type memLog struct {
	mu     sync.Mutex
	rows   []*domain.RealtimeEvent
	nextBy map[uuid.UUID]int64
	fail   error
}

func newMemLog() *memLog { return &memLog{nextBy: map[uuid.UUID]int64{}} }

func (m *memLog) Append(_ context.Context, e *domain.RealtimeEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail != nil {
		return m.fail
	}
	m.nextBy[e.TenantID]++
	e.Sequence = m.nextBy[e.TenantID]
	e.CreatedAt = time.Now().UTC()
	m.rows = append(m.rows, e)
	return nil
}

func (m *memLog) all() []*domain.RealtimeEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*domain.RealtimeEvent, len(m.rows))
	copy(out, m.rows)
	return out
}

type memFanout struct {
	mu       sync.Mutex
	channels []string
	payloads []string
	fail     error
}

func (f *memFanout) Publish(_ context.Context, channel string, payload interface{}) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail != nil {
		return f.fail
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	f.channels = append(f.channels, channel)
	f.payloads = append(f.payloads, string(raw))
	return nil
}

func (f *memFanout) snapshot() ([]string, []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.channels...), append([]string(nil), f.payloads...)
}

func draft(tenant uuid.UUID, typ rt.EventType, aggID string) rt.Envelope {
	return rt.Envelope{
		Type:      typ,
		TenantID:  tenant.String(),
		Aggregate: rt.Aggregate{ID: aggID},
	}
}

func TestPublisher_PersistsBeforeDelivering(t *testing.T) {
	log := newMemLog()
	hub := NewHub(HubOptions{})
	pub := NewPublisher(log, hub)
	tenant := uuid.New()

	sub, _ := hub.Subscribe(tenant, rt.Filter{})
	defer hub.Unsubscribe(sub)

	out, err := pub.Publish(context.Background(), draft(tenant, rt.RiskCreated, "risk-1"))
	if err != nil {
		t.Fatal(err)
	}

	rows := log.all()
	if len(rows) != 1 {
		t.Fatalf("expected the event to be persisted, got %d rows", len(rows))
	}
	if out.Sequence != 1 || rows[0].Sequence != 1 {
		t.Fatalf("the delivered envelope must carry the sequence the log assigned: %d vs %d", out.Sequence, rows[0].Sequence)
	}

	got, ok := recv(t, sub)
	if !ok {
		t.Fatal("the event was not delivered to the subscriber")
	}
	if got.ID != out.ID || got.Sequence != out.Sequence {
		t.Fatalf("the live envelope differs from the stored one: %+v vs %+v", got, out)
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("a delivered event must satisfy the contract: %v", err)
	}
}

// If the log refuses the event, nothing may be delivered: an event a
// reconnecting client could never replay is worse than no event at all.
func TestPublisher_DeliversNothingWhenTheLogRefuses(t *testing.T) {
	log := newMemLog()
	log.fail = errors.New("database is down")
	hub := NewHub(HubOptions{})
	pub := NewPublisher(log, hub)
	tenant := uuid.New()

	sub, _ := hub.Subscribe(tenant, rt.Filter{})
	defer hub.Unsubscribe(sub)

	if _, err := pub.Publish(context.Background(), draft(tenant, rt.RiskCreated, "r1")); err == nil {
		t.Fatal("a log failure must be reported to the caller")
	}
	assertNothing(t, sub)
}

// A fanout failure costs liveness on other instances, not correctness here: the
// event is durable and local clients have it, so the caller — a business
// mutation that already succeeded — must not be told it failed.
func TestPublisher_FanoutFailureDoesNotFailThePublication(t *testing.T) {
	log := newMemLog()
	hub := NewHub(HubOptions{})
	fan := &memFanout{fail: errors.New("redis is unreachable")}
	pub := NewPublisher(log, hub, WithFanout(fan))
	tenant := uuid.New()

	sub, _ := hub.Subscribe(tenant, rt.Filter{})
	defer hub.Unsubscribe(sub)

	if _, err := pub.Publish(context.Background(), draft(tenant, rt.AssetCreated, "a1")); err != nil {
		t.Fatalf("a fanout failure must not fail publication: %v", err)
	}
	if len(log.all()) != 1 {
		t.Fatal("the event must still be durable")
	}
	if _, ok := recv(t, sub); !ok {
		t.Fatal("local subscribers must still receive the event")
	}
}

func TestPublisher_FansOutOnThePublishingTenantsChannelOnly(t *testing.T) {
	log := newMemLog()
	hub := NewHub(HubOptions{})
	fan := &memFanout{}
	pub := NewPublisher(log, hub, WithFanout(fan))
	a, b := uuid.New(), uuid.New()

	if _, err := pub.Publish(context.Background(), draft(a, rt.RiskCreated, "r1")); err != nil {
		t.Fatal(err)
	}
	if _, err := pub.Publish(context.Background(), draft(b, rt.AssetCreated, "a1")); err != nil {
		t.Fatal(err)
	}

	channels, payloads := fan.snapshot()
	if len(channels) != 2 {
		t.Fatalf("expected two fanout publications, got %d", len(channels))
	}
	if channels[0] != TenantChannel(a) || channels[1] != TenantChannel(b) {
		t.Fatalf("events went to the wrong channels: %v", channels)
	}
	// The wire payload must be the envelope, not some internal shape.
	var decoded rt.Envelope
	if err := json.Unmarshal([]byte(payloads[0]), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.TenantID != a.String() || decoded.Sequence != 1 {
		t.Fatalf("unexpected fanout payload: %+v", decoded)
	}
}

// The publisher owns the id, the version and the sequence. A caller cannot
// choose any of them — an id chosen by a caller could make two different events
// collide in every consumer's dedup.
func TestPublisher_OverridesCallerSuppliedIdentityFields(t *testing.T) {
	log := newMemLog()
	hub := NewHub(HubOptions{})
	pub := NewPublisher(log, hub)
	tenant := uuid.New()

	in := draft(tenant, rt.RiskUpdated, "r1")
	in.ID = "chosen-by-the-caller"
	in.Version = 99
	in.Sequence = 12345
	in.EnvelopeVersion = 7

	out, err := pub.Publish(context.Background(), in)
	if err != nil {
		t.Fatalf("the publisher must correct these rather than refuse: %v", err)
	}
	if out.ID == "chosen-by-the-caller" {
		t.Fatal("a caller must not be able to choose the event id")
	}
	if _, err := uuid.Parse(out.ID); err != nil {
		t.Fatalf("the minted id must be a uuid: %v", out.ID)
	}
	if out.Version != 1 || out.EnvelopeVersion != rt.EnvelopeVersion {
		t.Fatalf("versions must come from the catalog, got version=%d envelope=%d", out.Version, out.EnvelopeVersion)
	}
	if out.Sequence != 1 {
		t.Fatalf("the sequence must come from the log, got %d", out.Sequence)
	}
}

// The publisher is the last gate before a secret could reach the wire and the
// database at the same time.
func TestPublisher_StripsForbiddenPayloadFields(t *testing.T) {
	log := newMemLog()
	hub := NewHub(HubOptions{})
	pub := NewPublisher(log, hub)
	tenant := uuid.New()

	in := draft(tenant, rt.RiskUpdated, "r1")
	in.Payload = map[string]any{
		"changedFields": []string{"status"},
		"passwordHash":  "$argon2id$v=19$...",
		"mfaSecret":     "JBSWY3DPEHPK3PXP",
	}

	out, err := pub.Publish(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	for _, banned := range []string{"passwordHash", "mfaSecret"} {
		if _, ok := out.Payload[banned]; ok {
			t.Fatalf("%q reached the wire", banned)
		}
		if _, ok := log.all()[0].Payload[banned]; ok {
			t.Fatalf("%q reached the database", banned)
		}
	}
	if _, ok := out.Payload["changedFields"]; !ok {
		t.Fatal("the legitimate field was stripped too")
	}
}

func TestPublisher_RefusesAnEventOutsideTheCatalog(t *testing.T) {
	log := newMemLog()
	hub := NewHub(HubOptions{})
	pub := NewPublisher(log, hub)

	in := draft(uuid.New(), "risk.invented", "r1")
	if _, err := pub.Publish(context.Background(), in); !errors.Is(err, rt.ErrUnknownType) {
		t.Fatalf("want ErrUnknownType, got %v", err)
	}
	if len(log.all()) != 0 {
		t.Fatal("a refused event must not be persisted")
	}
}

func TestPublisher_RefusesAnEventWithoutAUsableTenant(t *testing.T) {
	log := newMemLog()
	hub := NewHub(HubOptions{})
	pub := NewPublisher(log, hub)

	in := draft(uuid.Nil, rt.RiskCreated, "r1")
	if _, err := pub.Publish(context.Background(), in); err == nil {
		t.Fatal("the nil tenant must be refused, not stored")
	}
	if len(log.all()) != 0 {
		t.Fatal("a refused event must not be persisted")
	}
}

func TestTenantChannel_RoundTripsAndRejectsGarbage(t *testing.T) {
	tenant := uuid.New()
	got, ok := TenantFromChannel(TenantChannel(tenant))
	if !ok || got != tenant {
		t.Fatalf("round trip failed: %v %v", got, ok)
	}
	for _, bad := range []string{
		"", "some:other:channel", ChannelPrefix, ChannelPrefix + "not-a-uuid",
		ChannelPrefix + uuid.Nil.String(),
	} {
		if _, ok := TenantFromChannel(bad); ok {
			t.Fatalf("%q was accepted as a tenant channel", bad)
		}
	}
}
