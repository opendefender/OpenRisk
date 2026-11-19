// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package monitoring

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ControlChainReconcileRefused is 1 when this process's boot-time control chain
// reconciliation (#553, D-108) was refused: the counts did not reconcile, the
// transaction rolled back and the boot went on. 0 when it reconciled. It stays
// at its value for the life of the process, because Reconcile only runs at boot.
// It alerts as ControlChainReconcileRefused in deployment/monitoring/alerts.yml.
var ControlChainReconcileRefused = promauto.NewGauge(prometheus.GaugeOpts{
	Namespace: "openrisk",
	Name:      "control_chain_reconcile_refused",
	Help:      "1 when the boot-time control chain reconciliation was refused and rolled back, 0 when it reconciled.",
})

// ControlChainMirrorSkipped counts the legacy writes whose control chain edge
// was not written (#1413, D-108 follow-up R3/R4): the legacy write succeeded and
// the chain is left to the boot reconciler, which refuses the same input. The
// reason label is a closed set: requirement_unsplittable, edge_invalid.
var ControlChainMirrorSkipped = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: "openrisk",
	Name:      "control_chain_mirror_skipped_total",
	Help:      "Legacy compliance writes whose control chain edge was skipped because the chain refuses the input.",
}, []string{"reason"})
