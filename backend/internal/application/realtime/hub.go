// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package realtime is the application-side event hub: the in-process fan-out
// that holds every open stream, routes an envelope to exactly the subscriptions
// entitled to see it, and protects the server from clients that cannot keep up.
package realtime

import (
	"errors"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/pkg/monitoring"
	rt "github.com/opendefender/openrisk/pkg/realtime"
)

// Errors a subscription attempt can fail with.
var (
	// ErrNoTenant is returned when a subscription is attempted without a tenant.
	// It fails closed: a subscription with no tenant could only ever be a
	// subscription to everything.
	ErrNoTenant = errors.New("realtime hub: a subscription requires a tenant")
	// ErrHubCapacity is returned when the instance-wide connection limit is
	// reached.
	ErrHubCapacity = errors.New("realtime hub: connection capacity reached")
	// ErrTenantCapacity is returned when one tenant holds too many connections.
	ErrTenantCapacity = errors.New("realtime hub: tenant connection capacity reached")
)

// Defaults for the hub's limits.
//
// These are bounds, not tuning: each exists because the unbounded version is a
// resource a client controls.
const (
	// DefaultBufferSize is how many envelopes may queue for one subscriber
	// before the hub starts dropping. Sized for a burst — a bulk import
	// producing a few hundred events — not for a client that has stopped
	// reading, which no buffer size can rescue.
	DefaultBufferSize = 256
	// DefaultMaxConnections caps open streams on one instance.
	DefaultMaxConnections = 2000
	// DefaultMaxPerTenant stops a single tenant from consuming the whole
	// instance's connection budget. A browser opens one stream per tab, so this
	// is generous for humans and restrictive for a loop.
	DefaultMaxPerTenant = 100
	// dedupWindow is how many recently dispatched event ids the hub remembers.
	// Duplicates arrive from the fanout arriving twice, which happens within
	// milliseconds, so the window only has to outlive that.
	dedupWindow = 4096
)

// Subscription is one client's view of one tenant's stream.
//
// It carries its tenant as a field set at creation and never afterwards: a
// subscription cannot be moved between tenants, which is what makes the
// isolation structural rather than a check somebody has to remember.
type Subscription struct {
	ID       string
	TenantID uuid.UUID
	Filter   rt.Filter

	events chan rt.Envelope
	// resync is a coalescing signal, capacity 1. Repeated overflow leaves one
	// pending instruction rather than a queue of them — the client only needs to
	// be told once that it must refetch.
	resync chan struct{}

	closeOnce sync.Once
	closed    chan struct{}
	dropped   atomic.Int64
}

// Events is the delivery channel. It is closed when the subscription ends.
func (s *Subscription) Events() <-chan rt.Envelope { return s.events }

// Resync fires when the subscriber fell behind and its view can no longer be
// trusted to be complete.
func (s *Subscription) Resync() <-chan struct{} { return s.resync }

// Done closes when the subscription is torn down.
func (s *Subscription) Done() <-chan struct{} { return s.closed }

// Dropped is how many envelopes this subscriber missed.
func (s *Subscription) Dropped() int64 { return s.dropped.Load() }

// Hub fans envelopes out to the subscriptions of their own tenant.
//
// The routing table is keyed by tenant, so dispatch never walks other tenants'
// subscriptions and cannot deliver to them even if a filter were wrong. That is
// deliberate: tenant isolation is a property of the data structure here, not of
// a conditional inside a loop.
type Hub struct {
	mu      sync.RWMutex
	tenants map[uuid.UUID]map[string]*Subscription
	total   int

	bufferSize     int
	maxConnections int
	maxPerTenant   int

	// recent holds ids of already-dispatched events so a duplicate fanout — the
	// same event arriving from the local publisher and from Redis — is absorbed
	// here rather than by every consumer separately.
	dedupMu sync.Mutex
	recent  map[string]struct{}
	order   []string
}

// HubOptions overrides the hub's bounds. A zero field keeps the default.
type HubOptions struct {
	BufferSize     int
	MaxConnections int
	MaxPerTenant   int
}

// NewHub builds the fan-out.
func NewHub(opts HubOptions) *Hub {
	h := &Hub{
		tenants:        map[uuid.UUID]map[string]*Subscription{},
		bufferSize:     DefaultBufferSize,
		maxConnections: DefaultMaxConnections,
		maxPerTenant:   DefaultMaxPerTenant,
		recent:         make(map[string]struct{}, dedupWindow),
	}
	if opts.BufferSize > 0 {
		h.bufferSize = opts.BufferSize
	}
	if opts.MaxConnections > 0 {
		h.maxConnections = opts.MaxConnections
	}
	if opts.MaxPerTenant > 0 {
		h.maxPerTenant = opts.MaxPerTenant
	}
	return h
}

// Subscribe registers a stream for one tenant.
//
// The tenant comes from the caller's resolved session, never from anything the
// client sent; the hub simply refuses a nil one, which is the last line of a
// defence that starts in the handler.
func (h *Hub) Subscribe(tenantID uuid.UUID, filter rt.Filter) (*Subscription, error) {
	if tenantID == uuid.Nil {
		return nil, ErrNoTenant
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.total >= h.maxConnections {
		return nil, ErrHubCapacity
	}
	perTenant := h.tenants[tenantID]
	if len(perTenant) >= h.maxPerTenant {
		return nil, ErrTenantCapacity
	}

	sub := &Subscription{
		ID:       uuid.NewString(),
		TenantID: tenantID,
		Filter:   filter,
		events:   make(chan rt.Envelope, h.bufferSize),
		resync:   make(chan struct{}, 1),
		closed:   make(chan struct{}),
	}
	if perTenant == nil {
		perTenant = map[string]*Subscription{}
		h.tenants[tenantID] = perTenant
	}
	perTenant[sub.ID] = sub
	h.total++

	monitoring.RealtimeActiveConnections.Set(float64(h.total))
	monitoring.RealtimeActiveTenants.Set(float64(len(h.tenants)))
	return sub, nil
}

// Unsubscribe tears a stream down. Safe to call more than once, because the
// handler unsubscribes on every exit path and there is more than one.
//
// The channels are closed while the write lock is held, which is what makes the
// close safe: Dispatch delivers under the read lock, so a send and a close can
// never overlap. Closing a channel another goroutine is sending on is not a
// race the runtime tolerates — it is a panic that would take the process down,
// and it is reachable any time a client disconnects during a burst.
func (h *Hub) Unsubscribe(sub *Subscription) {
	if sub == nil {
		return
	}
	h.mu.Lock()
	if perTenant, ok := h.tenants[sub.TenantID]; ok {
		if _, present := perTenant[sub.ID]; present {
			delete(perTenant, sub.ID)
			h.total--
			if len(perTenant) == 0 {
				delete(h.tenants, sub.TenantID)
			}
		}
	}
	sub.closeOnce.Do(func() {
		close(sub.closed)
		close(sub.events)
	})
	total, tenants := h.total, len(h.tenants)
	h.mu.Unlock()

	monitoring.RealtimeActiveConnections.Set(float64(total))
	monitoring.RealtimeActiveTenants.Set(float64(tenants))
}

// Dispatch routes one envelope to the subscriptions of its tenant.
//
// Returns how many subscribers received it, which is what the publisher logs
// when an event appears to have gone nowhere.
//
// Delivery is non-blocking by construction. A subscriber whose buffer is full
// does NOT stall the dispatch: the envelope is dropped for that subscriber
// alone, counted, and a resync instruction is raised so the client knows its
// view is incomplete and refetches. The alternative — blocking until the slow
// client reads — would let one stalled browser stop every other tenant's
// events, which is how a realtime feature takes down a product.
func (h *Hub) Dispatch(env rt.Envelope) int {
	tenantID, err := uuid.Parse(env.TenantID)
	if err != nil || tenantID == uuid.Nil {
		// An envelope without a resolvable tenant has no correct destination.
		// Dropping it is the only safe answer: guessing would mean delivering
		// somebody's event to somebody else.
		return 0
	}
	if h.seenRecently(env.ID) {
		monitoring.RealtimeDuplicatesSuppressedTotal.Inc()
		return 0
	}

	// The read lock is held across the sends, not just across the lookup. Every
	// send is non-blocking, so the critical section is bounded by the number of
	// this tenant's subscribers and cannot be extended by a slow client — and
	// holding it is what guarantees Unsubscribe cannot close a channel underneath
	// a send in flight.
	h.mu.RLock()
	defer h.mu.RUnlock()

	delivered := 0
	dropped := 0
	buffered := 0
	for _, s := range h.tenants[tenantID] {
		if !s.Filter.Match(env) {
			continue
		}
		select {
		case s.events <- env:
			delivered++
		default:
			s.dropped.Add(1)
			dropped++
			select {
			case s.resync <- struct{}{}:
			default: // an instruction is already pending; one is enough
			}
		}
		buffered += len(s.events)
	}
	if dropped > 0 {
		monitoring.RealtimeEventsDroppedTotal.Add(float64(dropped))
	}
	monitoring.RealtimeBufferedEvents.Set(float64(buffered))
	return delivered
}

// seenRecently reports whether this event id was already dispatched, recording
// it if not.
//
// The window is bounded and evicted in insertion order: an unbounded set here
// would be a memory leak driven by publication volume, which is precisely the
// thing that grows without limit in a busy tenant.
func (h *Hub) seenRecently(id string) bool {
	if id == "" {
		return false
	}
	h.dedupMu.Lock()
	defer h.dedupMu.Unlock()
	if _, ok := h.recent[id]; ok {
		return true
	}
	h.recent[id] = struct{}{}
	h.order = append(h.order, id)
	if len(h.order) > dedupWindow {
		evict := h.order[0]
		h.order = h.order[1:]
		delete(h.recent, evict)
	}
	return false
}

// HubStats is a point-in-time view for diagnostics.
type HubStats struct {
	Connections int `json:"connections"`
	Tenants     int `json:"tenants"`
	Buffered    int `json:"buffered"`
}

// Stats reports the hub's live shape.
func (h *Hub) Stats() HubStats {
	h.mu.RLock()
	defer h.mu.RUnlock()
	s := HubStats{Connections: h.total, Tenants: len(h.tenants)}
	for _, perTenant := range h.tenants {
		for _, sub := range perTenant {
			s.Buffered += len(sub.events)
		}
	}
	return s
}

// SubscriberCount reports how many streams one tenant currently holds.
func (h *Hub) SubscriberCount(tenantID uuid.UUID) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.tenants[tenantID])
}
