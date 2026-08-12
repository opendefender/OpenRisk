// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package audittrail

import (
	"context"
	"sync"

	"github.com/opendefender/openrisk/internal/domain"
)

// ---------------------------------------------------------------------------
// Request-scoped collection.
//
// Two writers can describe the same user action: the GORM plugin (which knows
// the before → after of each row) and the application layer's explicit Recorder
// (which knows what the action MEANT — "approved step 2 of …"). Writing both
// would give one user action several trail entries, and a trail you have to
// mentally de-duplicate is a trail nobody audits.
//
// So during an HTTP request the middleware installs a Collector on the context.
// Both writers hand their observations to it instead of the database, and the
// middleware appends exactly ONE chained entry per mutating request, merging
// the meaning (explicit) with the evidence (before → after). Outside a request
// — background workers, schedulers — no collector exists and the plugin writes
// straight through, as before.
// ---------------------------------------------------------------------------

// Mutation is one observation about what a request changed.
type Mutation struct {
	EntityType string
	EntityID   string
	Action     domain.AuditAction
	Summary    string
	Before     domain.JSONMap
	After      domain.JSONMap
	Changed    domain.StringList
	// Explicit marks an application-layer observation (it names the intent), as
	// opposed to a row-level one reflected from GORM.
	Explicit bool
}

// Collector accumulates the mutations of a single request. Safe for concurrent
// use: a handler may fan out to goroutines that share the request context.
type Collector struct {
	mu   sync.Mutex
	muts []Mutation
}

// NewCollector returns an empty collector.
func NewCollector() *Collector { return &Collector{} }

// Add records one observation.
func (c *Collector) Add(m Mutation) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.muts = append(c.muts, m)
}

// Mutations returns a copy of everything observed so far.
func (c *Collector) Mutations() []Mutation {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Mutation, len(c.muts))
	copy(out, c.muts)
	return out
}

// Len is the number of observations.
func (c *Collector) Len() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.muts)
}

// Primary picks the observation that best describes the request: an explicit
// one if any (it carries the intent), otherwise the first row mutation.
func (c *Collector) Primary() (Mutation, bool) {
	muts := c.Mutations()
	if len(muts) == 0 {
		return Mutation{}, false
	}
	for _, m := range muts {
		if m.Explicit {
			return m, true
		}
	}
	return muts[0], true
}

type collectorCtxKey struct{}

// WithCollector returns a context carrying c. Installed by the audit middleware
// for mutating requests only.
func WithCollector(ctx context.Context, c *Collector) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, collectorCtxKey{}, c)
}

// CollectorFromContext returns the request collector, if the request is being
// journaled as a single entry.
func CollectorFromContext(ctx context.Context) (*Collector, bool) {
	if ctx == nil {
		return nil, false
	}
	c, ok := ctx.Value(collectorCtxKey{}).(*Collector)
	return c, ok && c != nil
}
