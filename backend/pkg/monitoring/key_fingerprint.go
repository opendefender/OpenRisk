// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package monitoring

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// KeyFingerprintMismatch is 1 while live replicas disagree on a secret key
// (#1063), by key NAME; never the key or its fingerprint. A rolling key
// rotation reads 1 for its duration; anything longer is a misconfiguration.
var KeyFingerprintMismatch = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "openrisk",
	Name:      "key_fingerprint_mismatch",
	Help:      "1 while live replicas were given different values of a secret key, by key name.",
}, []string{"key"})
