// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRiskView(t *testing.T) {
	for _, v := range AllRiskViews {
		got, err := ParseRiskView(string(v))
		require.NoError(t, err)
		assert.Equal(t, v, got)
	}
	got, err := ParseRiskView("")
	require.NoError(t, err)
	assert.Equal(t, RiskViewAll, got, "the parameter is optional")

	got, err = ParseRiskView(" Hot ")
	require.NoError(t, err)
	assert.Equal(t, RiskViewHot, got)

	for _, bad := range []string{"everything", "mine;drop", "hot,mine", "ALL RISKS"} {
		_, err := ParseRiskView(bad)
		require.Error(t, err, bad)
		assert.True(t, errors.Is(err, ErrValidation), "%q must be a validation error", bad)
	}
}

func TestRiskViewPredicate_Definitions(t *testing.T) {
	now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)

	all := RiskViewAll.Predicate(now)
	assert.True(t, all.IsZero(), "all constrains nothing")

	mine := RiskViewMine.Predicate(now)
	assert.True(t, mine.InvolvedOnly)
	assert.Empty(t, mine.CriticalityIn)
	assert.Empty(t, mine.StatusNot)

	hot := RiskViewHot.Predicate(now)
	assert.Equal(t, []CriticalityLevel{CriticalityHighNew, CriticalityCriticalNew}, hot.CriticalityIn)
	assert.Equal(t, RiskClosed, hot.StatusNot)
	assert.Nil(t, hot.NextReviewBy)

	due := RiskViewReviewDue.Predicate(now)
	require.NotNil(t, due.NextReviewBy)
	assert.Equal(t, time.Date(2026, 12, 19, 12, 0, 0, 0, time.UTC), *due.NextReviewBy, "now + 30 days, no lower bound: overdue is included")
	assert.Equal(t, RiskClosed, due.StatusNot)

	acc := RiskViewAccepted.Predicate(now)
	assert.Equal(t, RiskAccepted, acc.StatusIs)
	assert.Empty(t, acc.StatusNot)
	assert.False(t, acc.IsZero())

	assert.True(t, RiskView("unknown").Predicate(now).IsZero(), "an unparsed view must not invent a constraint")
}
