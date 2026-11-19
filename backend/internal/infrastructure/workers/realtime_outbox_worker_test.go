// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package workers

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakePass counts passes; its first `fullFor` passes report a full batch.
type fakePass struct {
	calls   atomic.Int32
	fullFor int32
	err     error
	hold    chan struct{} // when set, a pass waits on it or on ctx
	started chan struct{}
	ctxDone atomic.Bool
}

func (f *fakePass) Pass(ctx context.Context) (int, bool, error) {
	n := f.calls.Add(1)
	if f.started != nil {
		select {
		case f.started <- struct{}{}:
		default:
		}
	}
	if f.hold != nil {
		select {
		case <-f.hold:
		case <-ctx.Done():
			f.ctxDone.Store(true)
		}
	}
	return 0, n <= f.fullFor, f.err
}

func TestRealtimeOutboxWorker_WakeAndDrain(t *testing.T) {
	relay := &fakePass{}
	w := NewRealtimeOutboxWorker(relay, zerolog.Nop()).WithInterval(time.Hour)
	ctx, stop := context.WithCancel(context.Background())
	go w.Start(ctx)

	require.Eventually(t, func() bool { return relay.calls.Load() == 1 }, time.Second, 5*time.Millisecond, "one pass at boot")
	w.Wake()
	require.Eventually(t, func() bool { return relay.calls.Load() == 2 }, time.Second, 5*time.Millisecond, "a wake runs a pass at once")
	w.Wake()
	w.Wake() // never blocks, and two wakes in a row coalesce at most to a couple of passes
	require.Eventually(t, func() bool { return relay.calls.Load() >= 3 }, time.Second, 5*time.Millisecond)

	stop()
	drainCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, w.Drain(drainCtx), "an idle worker drains at once")
}

func TestRealtimeOutboxWorker_LoopsWhileTheBatchIsFull(t *testing.T) {
	relay := &fakePass{fullFor: 3}
	w := NewRealtimeOutboxWorker(relay, zerolog.Nop()).WithInterval(time.Hour)
	w.Pass(context.Background())
	assert.Equal(t, int32(4), relay.calls.Load(), "three full batches, then one that is not")
}

func TestRealtimeOutboxWorker_AFailedPassDoesNotSpin(t *testing.T) {
	relay := &fakePass{fullFor: 100, err: errors.New("db down")}
	w := NewRealtimeOutboxWorker(relay, zerolog.Nop()).WithInterval(time.Hour)
	w.Pass(context.Background())
	assert.Equal(t, int32(1), relay.calls.Load())
}

// Past its deadline Drain cancels the pass in flight, which is how the relay
// learns to put its rows back.
func TestRealtimeOutboxWorker_DrainCancelsAPassStuckPastTheDeadline(t *testing.T) {
	relay := &fakePass{hold: make(chan struct{}), started: make(chan struct{}, 1)}
	w := NewRealtimeOutboxWorker(relay, zerolog.Nop()).WithInterval(time.Hour)
	ctx, stop := context.WithCancel(context.Background())
	go w.Start(ctx)
	<-relay.started
	stop()

	drainCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := w.Drain(drainCtx)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.True(t, relay.ctxDone.Load(), "the pass in flight was cancelled")
}
