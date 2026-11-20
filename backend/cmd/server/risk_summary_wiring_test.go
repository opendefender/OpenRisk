// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package main

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ent "github.com/opendefender/openrisk/pkg/entitlements"
)

type fixedLevel struct {
	level ent.Level
	err   error
}

func (f fixedLevel) Level(context.Context, uuid.UUID, ent.Feature) (ent.Level, ent.Plan, error) {
	return f.level, ent.PlanFree, f.err
}

func TestRiskFinancialEntitlement(t *testing.T) {
	cases := []struct {
		name    string
		level   ent.Level
		err     error
		want    bool
		wantErr bool
	}{
		{"off", ent.LevelOff, nil, false, false},
		{"basic admits the closed-form ALE", ent.LevelBasic, nil, true, false},
		{"on", ent.LevelOn, nil, true, false},
		{"resolver error is reported, not guessed", ent.LevelOn, errors.New("down"), false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := riskFinancialEntitlement{levels: fixedLevel{tc.level, tc.err}}.
				HasFinancialQuantification(context.Background(), uuid.New())
			assert.Equal(t, tc.want, got)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
