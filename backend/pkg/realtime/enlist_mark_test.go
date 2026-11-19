// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package realtime

import (
	"context"
	"testing"
)

func TestEnlistMark_FlipsOnlyAfterMarked(t *testing.T) {
	ctx := WithEnlistMark(context.Background())
	if Enlisted(ctx) {
		t.Fatal("a fresh mark must read false")
	}
	MarkEnlisted(ctx)
	if !Enlisted(ctx) {
		t.Fatal("MarkEnlisted must flip the holder")
	}
	// A derived context shares the holder: the use case marks the context the
	// handler passed it, and the middleware reads its own.
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	if !Enlisted(child) {
		t.Fatal("a derived context must see the same holder")
	}
}

func TestEnlistMark_NoHolderIsANoOp(t *testing.T) {
	ctx := context.Background()
	MarkEnlisted(ctx) // must not panic
	if Enlisted(ctx) {
		t.Fatal("no holder means not enlisted")
	}
}
