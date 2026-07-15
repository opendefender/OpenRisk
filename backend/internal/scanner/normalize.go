// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package scanner

import (
	"sort"
	"strings"

	"github.com/opendefender/openrisk/internal/domain"
)

// Normalize brings a raw AssetDiscovery into a canonical shape:
//   - CPEs lower-cased, trimmed, de-duplicated, sorted;
//   - Criticality inferred from environment + tags when the scanner left it 0;
//   - Environment lower-cased; unknown Type coerced to Unknown.
//
// It is a pure function (no I/O) so it is trivially unit-testable.
func normalizeAsset(a AssetDiscovery) AssetDiscovery {
	a.CPE = normalizeCPEList(a.CPE)
	a.Environment = strings.ToLower(strings.TrimSpace(a.Environment))
	a.Tags = dedupeStrings(a.Tags)
	if a.Type == "" {
		a.Type = domain.AssetTypeUnknown
	}
	if a.Criticality <= 0 {
		a.Criticality = inferCriticality(a.Environment, a.Tags)
	}
	// Clamp into the Score Engine's declared factor range [0.1, 3.0].
	if a.Criticality < 0.1 {
		a.Criticality = 0.1
	}
	if a.Criticality > 3.0 {
		a.Criticality = 3.0
	}
	return a
}

func normalizeFinding(f FindingDiscovery) FindingDiscovery {
	f.Severity = normalizeSeverity(f.Severity)
	f.AffectedCPE = normalizeCPEList(f.AffectedCPE)
	if f.CVE != nil {
		cve := strings.ToUpper(strings.TrimSpace(*f.CVE))
		if cve == "" {
			f.CVE = nil
		} else {
			f.CVE = &cve
		}
	}
	return f
}

// inferCriticality maps environment + tags to a [0.1, 3.0] multiplier
// compatible with domain.AssetCriticality.ScoreFactor(). Production and
// sensitive tags push it up; dev/test pull it down. Default is MEDIUM (1.5).
func inferCriticality(environment string, tags []string) float64 {
	base := 1.5 // MEDIUM
	switch environment {
	case "prod", "production", "prd":
		base = 2.5 // HIGH
	case "staging", "stage", "preprod", "uat":
		base = 1.5
	case "dev", "development", "test", "qa", "sandbox":
		base = 0.5 // LOW
	}

	for _, t := range tags {
		switch strings.ToLower(strings.TrimSpace(t)) {
		case "critical", "crown-jewel", "pci", "pii", "phi", "domain-controller", "database", "kms", "secrets":
			base += 0.75
		case "internet-facing", "public", "external":
			base += 0.5
		case "internal", "isolated", "ephemeral":
			base -= 0.25
		}
	}
	if base < 0.1 {
		base = 0.1
	}
	if base > 3.0 {
		base = 3.0
	}
	return base
}

// CriticalityLabel maps a numeric multiplier to the domain enum used when the
// user promotes a discovery to an Asset. Mirrors CLAUDE.md's Score Engine bands
// applied to the [0.1, 3.0] factor scale.
func CriticalityLabel(factor float64) domain.AssetCriticality {
	switch {
	case factor >= 2.75:
		return domain.CriticalityCritical
	case factor >= 2.0:
		return domain.CriticalityHigh
	case factor >= 1.0:
		return domain.CriticalityMedium
	default:
		return domain.CriticalityLow
	}
}

func normalizeSeverity(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "critical", "crit":
		return SeverityCritical
	case "high", "important":
		return SeverityHigh
	case "medium", "moderate", "med":
		return SeverityMedium
	case "low", "minor":
		return SeverityLow
	case "informational", "info", "none", "":
		return SeverityInfo
	default:
		return SeverityMedium // unknown → medium so it isn't silently dropped
	}
}

// normalizeCPEList lower-cases, trims, de-dupes and sorts a CPE slice. CPEs are
// the join key for CTI matching (pkg/cti stores AffectedCPE lower-case), so
// normalising here keeps that lookup exact.
func normalizeCPEList(cpes []string) []string {
	seen := make(map[string]struct{}, len(cpes))
	out := make([]string, 0, len(cpes))
	for _, c := range cpes {
		c = strings.ToLower(strings.TrimSpace(c))
		if c == "" {
			continue
		}
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

func dedupeStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
