// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package monitoring

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Activation metrics (spec §7 "Aha moment + métrique").
//
// These are package-level and registered on the DEFAULT registry at import time,
// deliberately: MetricsCollector is constructed explicitly and, at the time of
// writing, by nobody, so a metric that only exists once someone remembers to
// build the collector is a metric that is not there when the alert fires.
// Importing this package is the only requirement.
//
// The alert lives in deployment/monitoring/alert-rules.yml (SlowTimeToAha,
// P50 > 12 min). The buckets below are chosen around that threshold so the
// quantile is interpolated inside a narrow bucket rather than across a 10-minute
// gap: a histogram whose bucket edges straddle the SLO cannot measure the SLO.
var (
	// TimeToAha measures signup → first Aha, in seconds. "Aha" is defined by the
	// product, not by this package: the first cyber score computed on the
	// tenant's OWN data while at least one compliance gap is identified.
	TimeToAha = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "openrisk",
		Name:      "time_to_aha_seconds",
		Help:      "Seconds between signup and the first Aha moment (first cyber score on the tenant's own data with at least one compliance gap identified).",
		Buckets: []float64{
			60,    // 1 min
			120,   // 2 min
			240,   // 4 min
			360,   // 6 min
			480,   // 8 min — the product target is "under 8 minutes"
			600,   // 10 min
			720,   // 12 min — the alert threshold
			900,   // 15 min
			1800,  // 30 min
			3600,  // 1 h
			21600, // 6 h
			86400, // 1 day: came back another day to find value
		},
	})

	// ActivationEventsTotal counts activation signals by key. Labelled by key
	// only — never by tenant: tenant labels are unbounded cardinality, and the
	// funnel question ("how many tenants import a framework?") is answered by
	// the ratio between keys, not by per-tenant series.
	ActivationEventsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "openrisk",
		Name:      "activation_events_total",
		Help:      "Activation events recorded, by event key.",
	}, []string{"event_key"})

	// AhaReachedTotal counts tenants that reached the Aha moment. The ratio to
	// signups is the activation rate.
	AhaReachedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "openrisk",
		Name:      "aha_reached_total",
		Help:      "Number of tenants that reached the Aha moment for the first time.",
	})
)

// ObserveTimeToAha records one signup → Aha duration. A non-positive duration is
// dropped: a clock skew or a missing signup anchor would otherwise report an
// impossibly fast activation and quietly flatter the P50 the alert watches.
func ObserveTimeToAha(d time.Duration) {
	if d <= 0 {
		return
	}
	TimeToAha.Observe(d.Seconds())
	AhaReachedTotal.Inc()
}

// RecordActivationEvent counts one activation signal.
func RecordActivationEvent(eventKey string) {
	if eventKey == "" {
		return
	}
	ActivationEventsTotal.WithLabelValues(eventKey).Inc()
}
