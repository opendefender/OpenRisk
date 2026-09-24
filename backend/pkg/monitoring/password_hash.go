// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package monitoring

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Password hash migration metrics.
//
// The question these answer is "is the Argon2id migration finished, and if not,
// how far off is it" — asked by an operator, from Grafana, without shell access
// to the database. A migration nobody can watch is a migration nobody finishes.
var (
	// PasswordHashAccounts is the number of accounts holding a password hash of
	// each algorithm. Both labels are bounded — algorithm is
	// argon2id / sha256_legacy / unknown, state is active / deleted — and
	// neither is ever a tenant or a user.
	//
	// sha256_legacy counts accounts still holding a digest from the first
	// release. They cannot sign in (D-048) and leave the count by resetting
	// their password. The state label separates soft-deleted rows: they will
	// never reset, so their hash sits in the table, and in any dump of it,
	// until the row is erased for good.
	//
	// Done when sha256_legacy reads zero on both states.
	PasswordHashAccounts = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "openrisk",
		Name:      "password_hash_accounts",
		Help:      "Accounts holding a password hash, by hashing algorithm and account state.",
	}, []string{"algorithm", "state"})

	// PasswordHashUpgradesTotal counts passwords rewritten to current Argon2id
	// parameters during a successful sign-in, after a cost increase.
	PasswordHashUpgradesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "openrisk",
		Name:      "password_hash_upgrades_total",
		Help:      "Passwords rehashed to current Argon2id parameters on sign-in, by the algorithm replaced.",
	}, []string{"from"})
)

// Account states used as the state label of PasswordHashAccounts.
const (
	PasswordHashStateActive  = "active"
	PasswordHashStateDeleted = "deleted"
)

// RecordPasswordHashCensus publishes one census of stored hashes, keyed by
// account state then algorithm.
//
// Pairs absent from the map are set to zero rather than left alone: a series
// that simply stops being reported keeps its last value on the dashboard, which
// is exactly the wrong reading for "no legacy accounts left".
func RecordPasswordHashCensus(counts map[string]map[string]int64) {
	for _, state := range []string{PasswordHashStateActive, PasswordHashStateDeleted} {
		for _, algorithm := range []string{"argon2id", "sha256_legacy", "unknown"} {
			PasswordHashAccounts.WithLabelValues(algorithm, state).Set(float64(counts[state][algorithm]))
		}
	}
}

// RecordPasswordHashUpgrade counts one transparent rehash.
func RecordPasswordHashUpgrade(from string) {
	if from == "" {
		from = "unknown"
	}
	PasswordHashUpgradesTotal.WithLabelValues(from).Inc()
}
