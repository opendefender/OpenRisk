// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func points(fs []ThreatFactor) map[string]int {
	out := map[string]int{}
	for _, f := range fs {
		out[f.Key] = f.Points
	}
	return out
}

func TestThreatRelevance(t *testing.T) {
	// KEV on a critical asset, CVSS 9.8, seen by a scanner: every part at its ceiling.
	rel, fs := ThreatRelevance(ThreatRelevanceInput{
		Criticalities: []AssetCriticality{"HIGH", "CRITICAL"}, KEV: true, CVSS: 9.8, Scanned: true,
	})
	assert.Equal(t, 100, rel)
	assert.Equal(t, map[string]int{"asset": 40, "exploitation": 30, "severity": 20, "evidence": 10}, points(fs))
	assert.Equal(t, "CRITICAL", fs[0].Value, "the most critical asset counts")
	assert.Equal(t, "kev", fs[1].Value)
	assert.Equal(t, "scan", fs[3].Value)

	// Inferred from CPEs on a low asset, no exploitation signal, CVSS 5.4.
	rel, fs = ThreatRelevance(ThreatRelevanceInput{
		Criticalities: []AssetCriticality{"LOW"}, EPSS: 0.04, CVSS: 5.4,
	})
	assert.Equal(t, 10+0+11+5, rel)
	assert.Equal(t, "", fs[1].Value)
	assert.Equal(t, "cpe", fs[3].Value)

	// EPSS bands without KEV.
	_, fs = ThreatRelevance(ThreatRelevanceInput{Criticalities: []AssetCriticality{"MEDIUM"}, EPSS: 0.62})
	assert.Equal(t, 20, points(fs)["exploitation"])
	assert.Equal(t, "epss_high", fs[1].Value)
	_, fs = ThreatRelevance(ThreatRelevanceInput{Criticalities: []AssetCriticality{"MEDIUM"}, EPSS: 0.18})
	assert.Equal(t, 10, points(fs)["exploitation"])

	// Unknown criticality reads as MEDIUM, as in the risk formula.
	_, fs = ThreatRelevance(ThreatRelevanceInput{Criticalities: []AssetCriticality{""}})
	assert.Equal(t, 20, points(fs)["asset"])
	assert.Equal(t, "MEDIUM", fs[0].Value)

	// Out-of-range CVSS is clamped, never above the ceiling.
	_, fs = ThreatRelevance(ThreatRelevanceInput{CVSS: 14})
	assert.Equal(t, 20, points(fs)["severity"])
}
