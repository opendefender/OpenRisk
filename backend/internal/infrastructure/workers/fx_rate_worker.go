// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package workers

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/opendefender/openrisk/pkg/crq"
)

// FXRateSource fetches a dated rate table from somewhere (a provider API, a
// config file, …). It is the seam a live FX feed plugs into. The table it returns
// MUST carry an AsOf date — a converted amount without its reference date is not
// defensible (spec §3).
type FXRateSource interface {
	// Fetch returns the current rates. Name identifies the source for logs.
	Fetch(ctx context.Context) (*crq.RateTable, error)
	Name() string
}

// StaticFXSource returns the built-in reference table. It is the honest default
// when no live feed is configured: figures still convert, and the fixed AsOf date
// is shown next to them so nobody mistakes them for live quotes.
type StaticFXSource struct{}

func (StaticFXSource) Name() string { return "static-reference" }
func (StaticFXSource) Fetch(context.Context) (*crq.RateTable, error) {
	return crq.DefaultRateTable(), nil
}

// FXRateWorker holds the current rate table and refreshes it on a daily cadence
// (spec §3: "taux de change via job quotidien"). It satisfies
// risk.RatesProvider via Current(), so handlers and the summary use case read a
// single, dated source of truth. Reads are concurrency-safe.
type FXRateWorker struct {
	source   FXRateSource
	logger   zerolog.Logger
	interval time.Duration

	mu      sync.RWMutex
	current *crq.RateTable
}

// NewFXRateWorker builds the worker and seeds it immediately (falling back to the
// static table if the first fetch fails), so Current() is valid before Start.
func NewFXRateWorker(source FXRateSource, logger zerolog.Logger) *FXRateWorker {
	if source == nil {
		source = StaticFXSource{}
	}
	w := &FXRateWorker{source: source, logger: logger, interval: 24 * time.Hour}
	w.current = crq.DefaultRateTable()
	w.refresh(context.Background())
	return w
}

// Current returns the latest rate table (never nil).
func (w *FXRateWorker) Current() *crq.RateTable {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.current == nil {
		return crq.DefaultRateTable()
	}
	return w.current
}

// Start refreshes the rates once a day until ctx is cancelled.
func (w *FXRateWorker) Start(ctx context.Context) {
	t := time.NewTicker(w.interval)
	defer t.Stop()
	w.logger.Info().Str("source", w.source.Name()).Msg("FX rate worker started (daily refresh)")
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			w.refresh(ctx)
		}
	}
}

// refresh pulls the latest table; a failed fetch keeps the last-good table.
func (w *FXRateWorker) refresh(ctx context.Context) {
	table, err := w.source.Fetch(ctx)
	if err != nil || table == nil {
		w.logger.Warn().Err(err).Msg("FX rate refresh failed — keeping last-known rates")
		return
	}
	w.mu.Lock()
	w.current = table
	w.mu.Unlock()
	w.logger.Debug().Time("as_of", table.AsOf).Str("source", table.Source).Msg("FX rates refreshed")
}
