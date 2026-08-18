// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package telemetry defines the ANONYMOUS, OPT-IN usage payload a self-hosted
// OpenRisk instance may send, and the rules that keep it irreproachable:
//
//   - Opt-in: nothing is sent unless an admin explicitly enables it (default off).
//   - Kill switch: setting OPENRISK_TELEMETRY=off (or 0/false/disabled) hard-disables
//     telemetry regardless of the stored consent — a deployer can guarantee silence
//     with one env var.
//   - Anonymous: the payload carries a random InstanceID and coarse COUNTS only.
//     Never an org name, user, email, hostname, IP, or any risk/asset content.
//   - Documented: the exact schema below is published in docs/TELEMETRY.md.
//
// In cybersecurity, sneaky telemetry destroys a reputation overnight. This package
// is deliberately small and auditable so anyone can verify what leaves the box.
package telemetry

import (
	"strings"
	"time"
)

// Payload is the COMPLETE set of fields ever transmitted. If a field is not here,
// it is not sent. Keep this struct and docs/TELEMETRY.md in lockstep.
type Payload struct {
	// InstanceID is a random UUID generated on first run. It is the ONLY identifier
	// and maps to no organization, user, or network address.
	InstanceID string `json:"instance_id"`
	Version    string `json:"version"`
	SentAt     string `json:"sent_at"` // RFC3339

	// Coarse platform facts, for compatibility statistics only.
	OS   string `json:"os"`
	Arch string `json:"arch"`
	DB   string `json:"db"` // "postgres"

	// Aggregate, non-identifying counts. Bucketed where a raw count could be
	// re-identifying for a tiny deployment (see Bucket).
	Orgs         int `json:"orgs"`
	UsersBucket  string `json:"users_bucket"`
	RisksBucket  string `json:"risks_bucket"`
	AssetsBucket string `json:"assets_bucket"`

	// Plan distribution (how many orgs on each tier) — product signal, no identity.
	PlanDistribution map[string]int `json:"plan_distribution"`
}

// Disabled reports whether the env kill switch forces telemetry off. Any of
// off/0/false/no/disabled (case-insensitive) disables it hard.
func Disabled(env string) bool {
	switch strings.ToLower(strings.TrimSpace(env)) {
	case "off", "0", "false", "no", "disabled":
		return true
	default:
		return false
	}
}

// Bucket coarsens a raw count into a range, so a small self-hosted instance is not
// re-identifiable by an unusually specific number.
func Bucket(n int) string {
	switch {
	case n <= 0:
		return "0"
	case n <= 10:
		return "1-10"
	case n <= 50:
		return "11-50"
	case n <= 200:
		return "51-200"
	case n <= 1000:
		return "201-1000"
	default:
		return "1000+"
	}
}

// NewPayload assembles a payload from already-collected aggregates, stamping the
// time via the provided clock (kept explicit so it is testable and deterministic).
func NewPayload(instanceID, version, os, arch string, now time.Time, orgs, users, risks, assets int, plans map[string]int) Payload {
	if plans == nil {
		plans = map[string]int{}
	}
	return Payload{
		InstanceID:       instanceID,
		Version:          version,
		SentAt:           now.UTC().Format(time.RFC3339),
		OS:               os,
		Arch:             arch,
		DB:               "postgres",
		Orgs:             orgs,
		UsersBucket:      Bucket(users),
		RisksBucket:      Bucket(risks),
		AssetsBucket:     Bucket(assets),
		PlanDistribution: plans,
	}
}
