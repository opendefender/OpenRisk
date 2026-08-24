// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package realtime

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	rt "github.com/opendefender/openrisk/pkg/realtime"
)

func env(tenant uuid.UUID, typ rt.EventType, aggID string, seq int64) rt.Envelope {
	desc, _ := rt.Lookup(typ)
	return rt.Envelope{
		ID:              uuid.NewString(),
		EnvelopeVersion: rt.EnvelopeVersion,
		Type:            typ,
		Version:         desc.Version,
		OccurredAt:      time.Now().UTC(),
		TenantID:        tenant.String(),
		Aggregate:       rt.Aggregate{Type: desc.Aggregate, ID: aggID},
		Sequence:        seq,
	}
}

func recv(t *testing.T, sub *Subscription) (rt.Envelope, bool) {
	t.Helper()
	select {
	case e, ok := <-sub.Events():
		return e, ok
	case <-time.After(500 * time.Millisecond):
		return rt.Envelope{}, false
	}
}

func assertNothing(t *testing.T, sub *Subscription) {
	t.Helper()
	select {
	case e := <-sub.Events():
		t.Fatalf("subscriber received an event it must never see: %s (tenant %s)", e.Type, e.TenantID)
	case <-time.After(120 * time.Millisecond):
	}
}

// THE mandatory cross-tenant test, at the hub level.
//
//	Tenant A publishes risk.created  → stream A receives it, stream B must not.
//	Tenant B publishes asset.created → stream B receives it, stream A must not.
func TestHub_CrossTenantIsolation(t *testing.T) {
	hub := NewHub(HubOptions{})
	a, b := uuid.New(), uuid.New()

	subA, err := hub.Subscribe(a, rt.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	subB, err := hub.Subscribe(b, rt.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	defer hub.Unsubscribe(subA)
	defer hub.Unsubscribe(subB)

	e1 := env(a, rt.RiskCreated, "risk-A", 1)
	hub.Dispatch(e1)

	got, ok := recv(t, subA)
	if !ok || got.ID != e1.ID {
		t.Fatal("stream A did not receive tenant A's event")
	}
	assertNothing(t, subB)

	e2 := env(b, rt.AssetCreated, "asset-B", 1)
	hub.Dispatch(e2)

	got, ok = recv(t, subB)
	if !ok || got.ID != e2.ID {
		t.Fatal("stream B did not receive tenant B's event")
	}
	assertNothing(t, subA)
}

// Concurrent sessions of the same tenant — several browser tabs — must each get
// a full copy, and still never see another tenant's traffic.
func TestHub_ConcurrentSessionsOfOneTenantEachReceiveTheEvent(t *testing.T) {
	hub := NewHub(HubOptions{})
	tenant, other := uuid.New(), uuid.New()

	subs := make([]*Subscription, 3)
	for i := range subs {
		s, err := hub.Subscribe(tenant, rt.Filter{})
		if err != nil {
			t.Fatal(err)
		}
		subs[i] = s
		defer hub.Unsubscribe(s)
	}
	stranger, _ := hub.Subscribe(other, rt.Filter{})
	defer hub.Unsubscribe(stranger)

	e := env(tenant, rt.RiskUpdated, "r1", 1)
	if n := hub.Dispatch(e); n != 3 {
		t.Fatalf("expected 3 deliveries, got %d", n)
	}
	for i, s := range subs {
		if got, ok := recv(t, s); !ok || got.ID != e.ID {
			t.Fatalf("tab %d did not receive the event", i)
		}
	}
	assertNothing(t, stranger)
}

// A subscription is created with its tenant and can never be moved, so an
// envelope whose tenant is unparseable has no correct destination and is
// dropped rather than guessed at.
func TestHub_EnvelopeWithoutAResolvableTenantIsDropped(t *testing.T) {
	hub := NewHub(HubOptions{})
	tenant := uuid.New()
	sub, _ := hub.Subscribe(tenant, rt.Filter{})
	defer hub.Unsubscribe(sub)

	bad := env(tenant, rt.RiskCreated, "r1", 1)
	bad.TenantID = "not-a-uuid"
	if n := hub.Dispatch(bad); n != 0 {
		t.Fatalf("expected no delivery, got %d", n)
	}
	bad.TenantID = uuid.Nil.String()
	if n := hub.Dispatch(bad); n != 0 {
		t.Fatalf("the nil tenant must not be a wildcard, got %d deliveries", n)
	}
	assertNothing(t, sub)
}

func TestHub_SubscribeRefusesANilTenant(t *testing.T) {
	hub := NewHub(HubOptions{})
	if _, err := hub.Subscribe(uuid.Nil, rt.Filter{}); !errors.Is(err, ErrNoTenant) {
		t.Fatalf("want ErrNoTenant, got %v", err)
	}
}

func TestHub_FilterNarrowsWithinTheTenantStream(t *testing.T) {
	hub := NewHub(HubOptions{})
	tenant := uuid.New()

	f, err := rt.ParseFilter("", "asset")
	if err != nil {
		t.Fatal(err)
	}
	sub, _ := hub.Subscribe(tenant, f)
	defer hub.Unsubscribe(sub)

	hub.Dispatch(env(tenant, rt.RiskCreated, "r1", 1))
	assertNothing(t, sub)

	want := env(tenant, rt.AssetCreated, "a1", 2)
	hub.Dispatch(want)
	if got, ok := recv(t, sub); !ok || got.ID != want.ID {
		t.Fatal("an asset event must pass an asset filter")
	}
}

// Backpressure: a subscriber that stops reading must not stall the hub, must
// not grow without bound, and must be told its view is incomplete.
//
// The healthy subscriber is drained synchronously between dispatches. That is
// deliberate: a background drainer racing a tight publish loop would make the
// assertion about buffer sizing rather than about isolation, and the property
// under test is that a stalled client cannot cost a healthy one its events.
func TestHub_SlowSubscriberIsBoundedDroppedAndToldToResync(t *testing.T) {
	const buffer = 4
	hub := NewHub(HubOptions{BufferSize: buffer})
	tenant := uuid.New()

	slow, _ := hub.Subscribe(tenant, rt.Filter{}) // never read
	defer hub.Unsubscribe(slow)
	healthy, _ := hub.Subscribe(tenant, rt.Filter{})
	defer hub.Unsubscribe(healthy)

	const total = 20
	dispatched := make(chan struct{})
	go func() {
		for i := 1; i <= total; i++ {
			hub.Dispatch(env(tenant, rt.RiskUpdated, "r1", int64(i)))
			// Give the healthy reader its turn; the slow one still never reads.
			if got, ok := recv(t, healthy); !ok {
				t.Errorf("the healthy subscriber missed event %d while another client was stalled", i)
				break
			} else if got.Sequence != int64(i) {
				t.Errorf("healthy subscriber saw sequence %d, expected %d", got.Sequence, i)
				break
			}
		}
		close(dispatched)
	}()

	select {
	case <-dispatched:
	case <-time.After(3 * time.Second):
		t.Fatal("dispatch blocked on a subscriber that stopped reading — one stalled client must never stall the hub")
	}

	if got := len(slow.Events()); got > buffer {
		t.Fatalf("the slow subscriber's buffer grew past its bound: %d > %d", got, buffer)
	}
	if slow.Dropped() == 0 {
		t.Fatal("events were dropped for the slow subscriber but not counted")
	}
	if slow.Dropped() != int64(total-buffer) {
		t.Fatalf("expected %d drops once the buffer filled, got %d", total-buffer, slow.Dropped())
	}
	select {
	case <-slow.Resync():
	default:
		t.Fatal("a subscriber that lost events must be told to resync — a silent gap is the one outcome that is not allowed")
	}
}

// The resync signal coalesces: repeated overflow leaves one pending
// instruction, not a queue of them.
func TestHub_ResyncSignalCoalesces(t *testing.T) {
	hub := NewHub(HubOptions{BufferSize: 1})
	tenant := uuid.New()
	sub, _ := hub.Subscribe(tenant, rt.Filter{})
	defer hub.Unsubscribe(sub)

	for i := 0; i < 50; i++ {
		hub.Dispatch(env(tenant, rt.RiskUpdated, "r1", int64(i)))
	}
	<-sub.Resync()
	select {
	case <-sub.Resync():
		t.Fatal("resync instructions queued up instead of coalescing")
	default:
	}
}

// The same event arriving twice — local publish plus cross-instance fanout —
// must be dispatched once.
func TestHub_DuplicateDispatchIsSuppressedByEventID(t *testing.T) {
	hub := NewHub(HubOptions{})
	tenant := uuid.New()
	sub, _ := hub.Subscribe(tenant, rt.Filter{})
	defer hub.Unsubscribe(sub)

	e := env(tenant, rt.RiskCreated, "r1", 1)
	if n := hub.Dispatch(e); n != 1 {
		t.Fatalf("first dispatch delivered %d", n)
	}
	if n := hub.Dispatch(e); n != 0 {
		t.Fatalf("the same event was dispatched twice (%d deliveries)", n)
	}
	if n := hub.Dispatch(e); n != 0 {
		t.Fatalf("a third delivery of the same event got through (%d)", n)
	}
	if _, ok := recv(t, sub); !ok {
		t.Fatal("the first delivery never arrived")
	}
	assertNothing(t, sub)
}

func TestHub_ConnectionLimitsRefuseRatherThanGrow(t *testing.T) {
	hub := NewHub(HubOptions{MaxConnections: 3, MaxPerTenant: 2})
	a, b := uuid.New(), uuid.New()

	s1, err := hub.Subscribe(a, rt.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := hub.Subscribe(a, rt.Filter{}); err != nil {
		t.Fatal(err)
	}
	if _, err := hub.Subscribe(a, rt.Filter{}); !errors.Is(err, ErrTenantCapacity) {
		t.Fatalf("want ErrTenantCapacity, got %v", err)
	}
	// One tenant exhausting its own budget must not exhaust the instance's.
	if _, err := hub.Subscribe(b, rt.Filter{}); err != nil {
		t.Fatalf("a second tenant must still be able to connect: %v", err)
	}
	if _, err := hub.Subscribe(b, rt.Filter{}); !errors.Is(err, ErrHubCapacity) {
		t.Fatalf("want ErrHubCapacity, got %v", err)
	}

	// Freeing a slot must free it for real.
	hub.Unsubscribe(s1)
	if _, err := hub.Subscribe(b, rt.Filter{}); err != nil {
		t.Fatalf("a released connection slot was not reusable: %v", err)
	}
}

func TestHub_UnsubscribeIsIdempotentAndClosesTheChannel(t *testing.T) {
	hub := NewHub(HubOptions{})
	tenant := uuid.New()
	sub, _ := hub.Subscribe(tenant, rt.Filter{})

	hub.Unsubscribe(sub)
	hub.Unsubscribe(sub) // the handler unsubscribes on every exit path

	if _, ok := <-sub.Events(); ok {
		t.Fatal("the events channel must be closed after unsubscribe")
	}
	select {
	case <-sub.Done():
	default:
		t.Fatal("Done must close on teardown")
	}
	if hub.Stats().Connections != 0 {
		t.Fatalf("connection accounting leaked: %+v", hub.Stats())
	}
	// Dispatch after teardown must not panic on a closed channel.
	hub.Dispatch(env(tenant, rt.RiskCreated, "r1", 1))
}

func TestHub_StatsReportTheLiveShape(t *testing.T) {
	hub := NewHub(HubOptions{})
	a, b := uuid.New(), uuid.New()
	s1, _ := hub.Subscribe(a, rt.Filter{})
	s2, _ := hub.Subscribe(a, rt.Filter{})
	s3, _ := hub.Subscribe(b, rt.Filter{})

	if got := hub.Stats(); got.Connections != 3 || got.Tenants != 2 {
		t.Fatalf("unexpected stats: %+v", got)
	}
	if got := hub.SubscriberCount(a); got != 2 {
		t.Fatalf("tenant a has %d subscribers", got)
	}
	hub.Unsubscribe(s1)
	hub.Unsubscribe(s2)
	if got := hub.Stats(); got.Tenants != 1 {
		t.Fatalf("an emptied tenant must be forgotten: %+v", got)
	}
	hub.Unsubscribe(s3)
}

// Subscribing and dispatching from many goroutines must not race. Run with
// -race, this is the test that catches a missing lock.
func TestHub_ConcurrentSubscribeDispatchUnsubscribe(t *testing.T) {
	hub := NewHub(HubOptions{BufferSize: 8})
	tenants := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}

	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			tenant := tenants[i%len(tenants)]
			sub, err := hub.Subscribe(tenant, rt.Filter{})
			if err != nil {
				return
			}
			go func() {
				for range sub.Events() {
				}
			}()
			for j := 0; j < 20; j++ {
				hub.Dispatch(env(tenant, rt.RiskUpdated, "r1", int64(j)))
			}
			hub.Unsubscribe(sub)
		}(i)
	}
	wg.Wait()

	if got := hub.Stats(); got.Connections != 0 {
		t.Fatalf("connections leaked under concurrency: %+v", got)
	}
}
