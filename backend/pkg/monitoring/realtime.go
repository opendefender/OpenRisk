// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package monitoring

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Realtime event hub metrics (W0-07).
//
// Registered on the default registry at import time, for the same reason the
// activation metrics are: a metric that only exists once someone remembers to
// construct a collector is a metric that is not there when the page fires. This
// package is already served by GET /metrics.
//
// CARDINALITY. Not one of these is labelled by tenant, user, connection or
// event id. A tenant label on a per-event counter is an unbounded series set,
// and the question it would answer ("which tenant is noisy?") is answered by the
// structured logs, which carry tenant and connection ids and are not indexed by
// a time-series database. Labels here are drawn from finite sets only: the event
// catalog, the aggregate list, and a handful of fixed outcome strings.
var (
	// RealtimeEventsPublishedTotal counts events accepted by the publisher —
	// validated, sanitised, and durably appended.
	RealtimeEventsPublishedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "openrisk",
		Subsystem: "realtime",
		Name:      "events_published_total",
		Help:      "Domain events accepted and durably appended, by event type and origin.",
	}, []string{"event_type", "origin"})

	// RealtimePublishFailuresTotal counts events that never reached the log.
	// Split by reason because the three cases need different responses: an
	// invalid event is a code defect, a log failure is a database problem, and a
	// fanout failure means Redis is down but replay still works.
	RealtimePublishFailuresTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "openrisk",
		Subsystem: "realtime",
		Name:      "publish_failures_total",
		Help:      "Publications that failed, by reason (invalid, log, fanout).",
	}, []string{"reason"})

	// RealtimeEventsDeliveredTotal counts envelopes written to a client stream,
	// separated by whether they arrived live or through a reconnect replay.
	RealtimeEventsDeliveredTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "openrisk",
		Subsystem: "realtime",
		Name:      "events_delivered_total",
		Help:      "Envelopes written to a subscriber, by aggregate and delivery mode (live, replay).",
	}, []string{"aggregate", "mode"})

	// RealtimeEventsDroppedTotal counts envelopes discarded because a
	// subscriber's bounded buffer was full. A non-zero value is the signal that
	// slow consumers are being protected against, and each drop is followed by a
	// resync instruction — it is never a silent loss.
	RealtimeEventsDroppedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "openrisk",
		Subsystem: "realtime",
		Name:      "events_dropped_total",
		Help:      "Envelopes dropped because a subscriber could not keep up.",
	})

	// RealtimeDuplicatesSuppressedTotal counts envelopes the hub recognised as
	// already dispatched. Cross-instance fanout plus a local publish can present
	// the same event twice; this is where that is absorbed.
	RealtimeDuplicatesSuppressedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "openrisk",
		Subsystem: "realtime",
		Name:      "duplicates_suppressed_total",
		Help:      "Envelopes recognised as already dispatched and suppressed by event id.",
	})

	// RealtimeSubscriptionsTotal counts stream subscription attempts and their
	// outcome. "unauthenticated" and "forbidden" are the two that matter for
	// security review; "capacity" is the connection limit refusing a client.
	RealtimeSubscriptionsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "openrisk",
		Subsystem: "realtime",
		Name:      "subscriptions_total",
		Help:      "Stream subscription attempts by outcome (accepted, unauthenticated, forbidden, invalid, capacity).",
	}, []string{"outcome"})

	// RealtimeReplaysTotal counts reconnects that asked to resume from a cursor,
	// by what the server could actually do about it.
	RealtimeReplaysTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "openrisk",
		Subsystem: "realtime",
		Name:      "replays_total",
		Help:      "Reconnects carrying a cursor, by outcome (served, empty, expired, error).",
	}, []string{"outcome"})

	// RealtimeResyncsTotal counts times a client was told its view is no longer
	// trustworthy and it must refetch.
	RealtimeResyncsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "openrisk",
		Subsystem: "realtime",
		Name:      "resyncs_total",
		Help:      "Resync instructions sent to clients, by reason (cursor_expired, buffer_overflow).",
	}, []string{"reason"})

	// RealtimeActiveConnections is the live count of open streams. A gauge, not
	// a counter: the operational question is "how many are open now", and a
	// leaked connection shows up here as a number that never comes down.
	RealtimeActiveConnections = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "openrisk",
		Subsystem: "realtime",
		Name:      "active_connections",
		Help:      "Currently open realtime stream connections.",
	})

	// RealtimeActiveTenants is how many distinct tenants have at least one open
	// stream.
	RealtimeActiveTenants = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "openrisk",
		Subsystem: "realtime",
		Name:      "active_tenants",
		Help:      "Distinct tenants with at least one open realtime stream.",
	})

	// RealtimeBufferedEvents is the total number of envelopes sitting in
	// subscriber buffers. Rising and staying high means consumers are slower
	// than publication, which is the condition that precedes drops.
	RealtimeBufferedEvents = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "openrisk",
		Subsystem: "realtime",
		Name:      "buffered_events",
		Help:      "Envelopes currently buffered across all subscribers.",
	})

	// RealtimePublishLatencySeconds measures validate → sanitise → append →
	// fanout. It is the "why is the UI lagging" measurement on the write side.
	RealtimePublishLatencySeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "openrisk",
		Subsystem: "realtime",
		Name:      "publish_latency_seconds",
		Help:      "Seconds from publish call to durable append plus fanout.",
		Buckets:   []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
	})

	// RealtimeDeliveryLagSeconds measures occurredAt → written to a client
	// socket: the end-to-end number a user would recognise as "how stale is my
	// screen".
	RealtimeDeliveryLagSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "openrisk",
		Subsystem: "realtime",
		Name:      "delivery_lag_seconds",
		Help:      "Seconds between a business change occurring and the event reaching a client socket.",
		Buckets:   []float64{.01, .05, .1, .25, .5, 1, 2.5, 5, 10, 30, 60},
	})

	// RealtimeConnectionDurationSeconds records how long streams stay up. A
	// distribution clustered at the low end is a reconnect storm.
	RealtimeConnectionDurationSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "openrisk",
		Subsystem: "realtime",
		Name:      "connection_duration_seconds",
		Help:      "How long a realtime stream connection stayed open.",
		Buckets:   []float64{1, 5, 15, 30, 60, 300, 900, 1800, 3600, 7200},
	})
)
