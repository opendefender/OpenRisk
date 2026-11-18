// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package workers

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	statusapp "github.com/opendefender/openrisk/internal/application/controlstatus"
)

type fakeSweeper struct {
	mu      sync.Mutex
	results []statusapp.SweepResult
	err     error
	calls   int
	started chan struct{}
	release chan struct{}
}

func (f *fakeSweeper) Sweep(ctx context.Context) (statusapp.SweepResult, error) {
	f.mu.Lock()
	i := f.calls
	f.calls++
	f.mu.Unlock()
	if f.started != nil && i == 0 {
		close(f.started)
		<-f.release
	}
	if f.err != nil {
		return statusapp.SweepResult{}, f.err
	}
	if i < len(f.results) {
		return f.results[i], nil
	}
	return statusapp.SweepResult{}, nil
}

func (f *fakeSweeper) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func TestControlStatusWorker_TickDrainsABacklogInBoundedPasses(t *testing.T) {
	full := statusapp.SweepResult{Claimed: statusapp.DefaultBatch, Recomputed: statusapp.DefaultBatch}
	s := &fakeSweeper{results: []statusapp.SweepResult{full, full, {Claimed: 3, Recomputed: 3}}}
	NewControlStatusWorker(s, zerolog.Nop()).Tick(context.Background())
	assert.Equal(t, 3, s.count(), "two full batches, then a short one: the backlog is drained")

	endless := &fakeSweeper{}
	for i := 0; i < 50; i++ {
		endless.results = append(endless.results, full)
	}
	NewControlStatusWorker(endless, zerolog.Nop()).Tick(context.Background())
	assert.Equal(t, controlStatusMaxPasses, endless.count(), "a tick is bounded")

	idle := &fakeSweeper{}
	NewControlStatusWorker(idle, zerolog.Nop()).Tick(context.Background())
	assert.Equal(t, 1, idle.count(), "nothing claimed: one pass")
}

func TestControlStatusWorker_AFailedPassEndsTheTickAndIsNotFatal(t *testing.T) {
	s := &fakeSweeper{err: errors.New("db down")}
	NewControlStatusWorker(s, zerolog.Nop()).Tick(context.Background())
	assert.Equal(t, 1, s.count())
}

func TestControlStatusWorker_StartSweepsAtBootThenOnTheTickAndStopsGracefully(t *testing.T) {
	s := &fakeSweeper{}
	w := NewControlStatusWorker(s, zerolog.Nop()).WithInterval(10 * time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	go w.Start(ctx)
	require.Eventually(t, func() bool { return s.count() >= 3 }, 2*time.Second, 5*time.Millisecond, "a pass at boot and one per tick")
	cancel()
	drain, stop := context.WithTimeout(context.Background(), 2*time.Second)
	defer stop()
	require.NoError(t, w.Drain(drain), "Start returns once the context is cancelled")
}

func TestControlStatusWorker_ShutdownLetsTheInFlightPassFinish(t *testing.T) {
	s := &fakeSweeper{started: make(chan struct{}), release: make(chan struct{})}
	w := NewControlStatusWorker(s, zerolog.Nop()).WithInterval(time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	go w.Start(ctx)
	<-s.started
	cancel() // shutdown arrives while a pass is running

	short, stop := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer stop()
	assert.Error(t, w.Drain(short), "Drain waits for the pass in flight")

	close(s.release)
	long, stop2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer stop2()
	require.NoError(t, w.Drain(long))
	assert.Equal(t, 1, s.count(), "the pass in flight finished; no new pass started")
}
