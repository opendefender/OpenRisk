// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Mirrors frontend features/risks/riskViews.ts toStatusValue, row for row.
func TestNormalizeStatus_MirrorsTheFrontend(t *testing.T) {
	cases := map[string]RegisterStatus{
		"open": RegisterOpen, "OPEN": RegisterOpen, "draft": RegisterOpen, "DRAFT": RegisterOpen,
		"": RegisterOpen, "  ": RegisterOpen, "whatever": RegisterOpen,
		"in_progress": RegisterInProgress, "progress": RegisterInProgress, "active": RegisterInProgress, "ACTIVE": RegisterInProgress,
		"mitigated": RegisterMitigated, "MITIGATED": RegisterMitigated,
		"accepted": RegisterAccepted, "ACCEPTED": RegisterAccepted,
		"closed": RegisterClosed, "done": RegisterClosed, " Closed ": RegisterClosed,
	}
	for raw, want := range cases {
		assert.Equal(t, want, NormalizeStatus(raw), "%q", raw)
	}
}

func TestSelectStatuses(t *testing.T) {
	open := SelectStatuses([]string{"open"})
	assert.True(t, open.IncludeOpen)
	assert.Empty(t, open.In)
	assert.ElementsMatch(t, []string{"in_progress", "progress", "active", "mitigated", "accepted", "closed", "done"}, open.NonOpen,
		"open is the complement of every other status, so DRAFT, NULL and unknown values are open")

	mit := SelectStatuses([]string{"MITIGATED"})
	assert.False(t, mit.IncludeOpen)
	assert.Equal(t, []string{"mitigated"}, mit.In)

	both := SelectStatuses([]string{"open", "in_progress", "draft"})
	assert.True(t, both.IncludeOpen)
	assert.ElementsMatch(t, []string{"in_progress", "progress", "active"}, both.In)

	// A stored alias selects its register status.
	assert.Equal(t, []string{"closed", "done"}, SelectStatuses([]string{"done"}).In)
	assert.True(t, SelectStatuses([]string{"DRAFT"}).IncludeOpen)

	// An unrecognised value is not widened into "open": it keeps matching nothing.
	bogus := SelectStatuses([]string{"bogus"})
	assert.False(t, bogus.IncludeOpen)
	assert.Equal(t, []string{"bogus"}, bogus.In)

	assert.Equal(t, []string{"closed", "done"}, StoredStatusValues(RegisterClosed))
	assert.Empty(t, StoredStatusValues(RegisterOpen))
}
