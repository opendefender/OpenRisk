// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package realtime

import (
	"context"
	"sync/atomic"
)

// enlistKey is the context key of the per-request "events were enlisted" holder.
type enlistKey struct{}

// WithEnlistMark installs a request-scoped holder that a use case flips when it
// wrote its realtime events to the transactional outbox (ADR 0010).
//
// The post-commit MutationBridge reads it: a request whose use case already
// enlisted its events must not announce the same change a second time under a
// different id. Same shape as the audit collector, and installed beside it.
func WithEnlistMark(ctx context.Context) context.Context {
	return context.WithValue(ctx, enlistKey{}, &atomic.Bool{})
}

// MarkEnlisted records that the request's events went through the outbox. It is
// a no-op when no holder is installed (a worker, a test, a request outside the
// audited mutation chain).
func MarkEnlisted(ctx context.Context) {
	if h, ok := ctx.Value(enlistKey{}).(*atomic.Bool); ok {
		h.Store(true)
	}
}

// Enlisted reports whether a use case in this request enlisted its events.
func Enlisted(ctx context.Context) bool {
	h, ok := ctx.Value(enlistKey{}).(*atomic.Bool)
	return ok && h.Load()
}
