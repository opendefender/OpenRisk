// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package canonical_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain/canonical"
)

// TestCanonical_ControlIDFor is ADR 0008 D7 / #1240 criterion 8: the Control
// split from a compliance_controls row has a stable UUIDv5 id, distinct from
// the row id the Requirement keeps.
func TestCanonical_ControlIDFor(t *testing.T) {
	// Pinned: #553 persists these ids, so the namespace can never change.
	require.Equal(t, "7e575e86-2003-497c-a5f6-c326c0820619", canonical.ControlSplitNamespace.String())

	row := uuid.MustParse("3f1c2b9a-6d4e-4f10-9a7b-1c2d3e4f5a6b")
	first := canonical.ControlIDFor(row)
	second := canonical.ControlIDFor(row)

	assert.Equal(t, first, second, "deterministic: the projection runs on every read")
	assert.NotEqual(t, row, first, "the Control id differs from the Requirement's")
	assert.Equal(t, uuid.Version(5), first.Version())
	assert.Equal(t, uuid.NewSHA1(canonical.ControlSplitNamespace, []byte(row.String())), first,
		"name is the row id's text form, reproducible as uuid_generate_v5(ns, id::text)")
	assert.NotEqual(t, first, canonical.ControlIDFor(uuid.New()), "distinct rows, distinct controls")
	assert.Equal(t, uuid.Nil, canonical.ControlIDFor(uuid.Nil), "a missing row id never becomes a control id")

	ctl := validControl()
	ctl.ID = canonical.ControlIDFor(uuid.Nil)
	requireRefused(t, ctl.Validate(), "id")
}
