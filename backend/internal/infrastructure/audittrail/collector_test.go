// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package audittrail

import "testing"

func TestCollector_Journaled(t *testing.T) {
	c := NewCollector()
	if c.Journaled() {
		t.Fatal("a new collector is not journaled")
	}
	c.MarkJournaled()
	if !c.Journaled() {
		t.Fatal("MarkJournaled must stick")
	}
	var nilC *Collector
	nilC.MarkJournaled() // must not panic
	if nilC.Journaled() {
		t.Fatal("a nil collector is never journaled")
	}
}
