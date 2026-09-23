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
	// The state label exists because the two populations migrate differently. An
	// active account upgrades itself the next time somebody signs in. A
	// soft-deleted one never signs in, so its hash sits in the table until the
	// row is erased for good — invisible to a gauge that filtered it out, and
	// still in any dump of that table.
	//
	// The migration is done when sha256_legacy reads zero on active and stays
	// there; the deleted series is what tells you whether purge, not sign-in, is
	// the remaining work.
	PasswordHashAccounts = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "openrisk",
		Name:      "password_hash_accounts",
		Help:      "Accounts holding a password hash, by hashing algorithm and account state.",
	}, []string{"algorithm", "state"})

	// PasswordHashUpgradesTotal counts passwords rewritten to current Argon2id
	// parameters during a successful sign-in. Its rate is the migration's speed.
	PasswordHashUpgradesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "openrisk",
		Name:      "password_hash_upgrades_total",
		Help:      "Passwords rehashed to current Argon2id parameters on sign-in, by the algorithm replaced.",
	}, []string{"from"})

	// PasswordHashExpiredLoginsTotal counts sign-ins refused because the account
	// still held a legacy hash after the cutoff. Any value above zero means real
	// people are being sent to password reset — expected after the deadline,
	// worth an alert before it.
	PasswordHashExpiredLoginsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "openrisk",
		Name:      "password_hash_expired_logins_total",
		Help:      "Sign-ins refused because the stored hash was legacy and the migration cutoff had passed.",
	})

	// PasswordHashLegacyCutoff is the cutoff as a Unix timestamp, so a dashboard
	// can plot "days left" next to the legacy count instead of hard-coding a
	// date that drifts from the deployment's own configuration.
	PasswordHashLegacyCutoff = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "openrisk",
		Name:      "password_hash_legacy_cutoff_timestamp_seconds",
		Help:      "Instant after which a legacy password hash is refused at sign-in.",
	})
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

// RecordPasswordHashExpiredLogin counts one sign-in refused past the cutoff.
func RecordPasswordHashExpiredLogin() { PasswordHashExpiredLoginsTotal.Inc() }

// SetPasswordHashLegacyCutoff publishes the configured cutoff.
func SetPasswordHashLegacyCutoff(at time.Time) {
	PasswordHashLegacyCutoff.Set(float64(at.Unix()))
}
