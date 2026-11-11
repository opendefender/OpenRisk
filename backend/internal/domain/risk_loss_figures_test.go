// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRiskHasLossFigures(t *testing.T) {
	v := func(f float64) *float64 { return &f }
	cases := []struct {
		name string
		r    Risk
		want bool
	}{
		{"nothing entered", Risk{}, false},
		{"ARO alone", Risk{ARO: v(0.5)}, false},
		{"remediation cost alone", Risk{RemediationCostXAF: v(1_000_000)}, false},
		{"SLE", Risk{SLEXAF: v(20_000_000)}, true},
		{"fine without ARO", Risk{FinesXAF: v(8_000_000)}, true},
		{"data loss", Risk{DataLossCostXAF: v(1)}, true},
		{"other direct cost", Risk{OtherDirectCostXAF: v(1)}, true},
		{"downtime hours only", Risk{DowntimeHours: v(4)}, false},
		{"downtime pair", Risk{DowntimeHours: v(4), HourlyDowntimeCostXAF: v(100_000)}, true},
		{"zero SLE entered", Risk{SLEXAF: v(0)}, true},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, c.r.HasLossFigures(), c.name)
	}
}
