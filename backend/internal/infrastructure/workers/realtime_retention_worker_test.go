// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package workers

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

type fakePurger struct {
	mu      sync.Mutex
	cutoffs []time.Time
	err     error
	calls   chan struct{}
}

func newFakePurger() *fakePurger { return &fakePurger{calls: make(chan struct{}, 16)} }

func (f *fakePurger) PurgeBefore(_ context.Context, cutoff time.Time) (int64, error) {
	f.mu.Lock()
	f.cutoffs = append(f.cutoffs, cutoff)
	err := f.err
	f.mu.Unlock()
	select {
	case f.calls <- struct{}{}:
	default:
	}
	if err != nil {
		return 0, err
	}
	return 3, nil
}

func (f *fakePurger) seen() []time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]time.Time(nil), f.cutoffs...)
}

// A deployment that was down over a weekend must not serve an oversized replay
// window for a whole interval after coming back.
func TestRealtimeRetentionWorker_SweepsOnceAtBoot(t *testing.T) {
	p := newFakePurger()
	w := NewRealtimeRetentionWorker(p, 24*time.Hour, zerolog.New(io.Discard)).WithInterval(time.Hour)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Start(ctx)

	select {
	case <-p.calls:
	case <-time.After(2 * time.Second):
		t.Fatal("the worker did not sweep at boot")
	}

	cutoffs := p.seen()
	if len(cutoffs) == 0 {
		t.Fatal("no cutoff recorded")
	}
	age := time.Since(cutoffs[0])
	if age < 23*time.Hour || age > 25*time.Hour {
		t.Fatalf("the cutoff must be one retention window back, got %s", age)
	}
}

func TestRealtimeRetentionWorker_KeepsSweepingOnSchedule(t *testing.T) {
	p := newFakePurger()
	w := NewRealtimeRetentionWorker(p, time.Hour, zerolog.New(io.Discard)).WithInterval(30 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Start(ctx)

	for i := 0; i < 3; i++ {
		select {
		case <-p.calls:
		case <-time.After(2 * time.Second):
			t.Fatalf("only %d sweeps happened", i)
		}
	}
}

// A transient database error costs disk. A worker that exits on one costs the
// replay window forever, which is the worse failure.
func TestRealtimeRetentionWorker_SurvivesAFailedSweep(t *testing.T) {
	p := newFakePurger()
	p.err = errors.New("database unavailable")
	w := NewRealtimeRetentionWorker(p, time.Hour, zerolog.New(io.Discard)).WithInterval(20 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Start(ctx)

	for i := 0; i < 3; i++ {
		select {
		case <-p.calls:
		case <-time.After(2 * time.Second):
			t.Fatal("the worker stopped after an error instead of retrying")
		}
	}
}

func TestRealtimeRetentionWorker_StopsWithItsContext(t *testing.T) {
	p := newFakePurger()
	w := NewRealtimeRetentionWorker(p, time.Hour, zerolog.New(io.Discard)).WithInterval(20 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { w.Start(ctx); close(done) }()

	<-p.calls
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the worker ignored its cancelled context")
	}
}

// Retention that is not configured must delete nothing, rather than default to
// something nobody asked for.
func TestRealtimeRetentionWorker_DoesNothingWithoutARetentionWindow(t *testing.T) {
	p := newFakePurger()
	w := NewRealtimeRetentionWorker(p, 0, zerolog.New(io.Discard)).WithInterval(10 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { w.Start(ctx); close(done) }()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("the worker should have returned immediately")
	}
	if len(p.seen()) != 0 {
		t.Fatal("something was purged without a configured window")
	}
}
