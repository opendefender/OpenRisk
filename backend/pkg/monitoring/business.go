// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package monitoring

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Package-level business metrics (task §4). These sit alongside the activation
// metrics and the request-scoped MetricsCollector, and are incremented directly
// from the use cases that produce the event — no collector threading required.
var (
	// ReportsGeneratedTotal counts generated reports, labelled by kind
	// (board / compliance / …). The kind label is bounded; never label by tenant.
	ReportsGeneratedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "openrisk",
		Name:      "reports_generated_total",
		Help:      "Reports generated, by kind.",
	}, []string{"kind"})
)

// RecordReportGenerated counts one generated report of the given kind.
func RecordReportGenerated(kind string) {
	if kind == "" {
		kind = "unknown"
	}
	ReportsGeneratedTotal.WithLabelValues(kind).Inc()
}
