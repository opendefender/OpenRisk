// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package workers

import (
	"context"
	"time"

	"github.com/rs/zerolog"
)

// RealtimeOutboxPass runs one relay pass (application/realtime.OutboxRelay).
// It reports how many rows it claimed and whether the batch was full.
type RealtimeOutboxPass interface {
	Pass(ctx context.Context) (claimed int, full bool, err error)
}

// RealtimeOutboxWorker drives the realtime outbox relay (#344, ADR 0010).
//
// It polls every couple of seconds and is woken at once when a transaction that
// enlisted events commits, so an announcement does not wait for the poll.
// Every replica runs one: the claim's SKIP LOCKED splits the tenants between
// them, and what a stopped replica leaves leased is delivered again, with the
// same ids, once its lease expires.
type RealtimeOutboxWorker struct {
	relay    RealtimeOutboxPass
	logger   zerolog.Logger
	interval time.Duration
	wake     chan struct{}

	// Passes run on base, not on the loop's context: stopping the loop lets the
	// pass in flight finish. Drain cancels base past its deadline, and the relay
	// then puts the rows it did not deliver back.
	base context.Context
	stop context.CancelFunc
	done chan struct{}
}

// NewRealtimeOutboxWorker builds the worker.
func NewRealtimeOutboxWorker(relay RealtimeOutboxPass, logger zerolog.Logger) *RealtimeOutboxWorker {
	base, stop := context.WithCancel(context.Background())
	return &RealtimeOutboxWorker{
		relay: relay, logger: logger, interval: 2 * time.Second,
		wake: make(chan struct{}, 1),
		base: base, stop: stop, done: make(chan struct{}),
	}
}

// WithInterval overrides the poll cadence (tests).
func (w *RealtimeOutboxWorker) WithInterval(d time.Duration) *RealtimeOutboxWorker {
	if d > 0 {
		w.interval = d
	}
	return w
}

// Wake asks for a pass now. It never blocks.
func (w *RealtimeOutboxWorker) Wake() {
	select {
	case w.wake <- struct{}{}:
	default:
	}
}

// Start runs one pass at boot, then one per tick or wake, until ctx is done. It
// returns after the pass in flight.
func (w *RealtimeOutboxWorker) Start(ctx context.Context) {
	defer close(w.done)
	tick := time.NewTicker(w.interval)
	defer tick.Stop()
	w.logger.Info().Msg("Realtime outbox worker started (transactional outbox relay)")
	w.Pass(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			w.Pass(ctx)
		case <-w.wake:
			w.Pass(ctx)
		}
	}
}

// Pass runs relay passes while batches come back full and ctx is live.
// Exported for tests.
func (w *RealtimeOutboxWorker) Pass(ctx context.Context) {
	for ctx.Err() == nil {
		_, full, err := w.relay.Pass(w.base)
		if err != nil {
			w.logger.Warn().Err(err).Msg("realtime outbox: pass failed")
			return
		}
		if !full {
			return
		}
	}
}

// Drain waits for Start to return, which it does after its pass once its
// context is cancelled. Past ctx's deadline it cancels the pass still running;
// the relay puts the rows it had not delivered back for the next boot or
// another replica.
func (w *RealtimeOutboxWorker) Drain(ctx context.Context) error {
	defer w.stop()
	select {
	case <-w.done:
		return nil
	case <-ctx.Done():
	}
	w.stop()
	select {
	case <-w.done:
	case <-time.After(drainGrace):
	}
	return ctx.Err()
}
