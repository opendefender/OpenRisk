// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package workers

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
)

// queueSender hands out queued rows and records how many sends ran at once.
type queueSender struct {
	mu       sync.Mutex
	queue    []domain.NoticeEmailOutbox
	sent     []uuid.UUID
	back     []uuid.UUID // put back by an interrupted send
	running  atomic.Int32
	peak     atomic.Int32
	hold     chan struct{} // when set, each send waits on it or on ctx
	started  chan struct{}
	pruneRun atomic.Int32
}

func (s *queueSender) Claim(_ context.Context, limit int) ([]domain.NoticeEmailOutbox, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := min(limit, len(s.queue))
	out := append([]domain.NoticeEmailOutbox(nil), s.queue[:n]...)
	s.queue = s.queue[n:]
	return out, nil
}

func (s *queueSender) Send(ctx context.Context, row domain.NoticeEmailOutbox) error {
	now := s.running.Add(1)
	defer s.running.Add(-1)
	for {
		peak := s.peak.Load()
		if now <= peak || s.peak.CompareAndSwap(peak, now) {
			break
		}
	}
	if s.started != nil {
		s.started <- struct{}{}
	}
	if s.hold != nil {
		select {
		case <-s.hold:
		case <-ctx.Done():
			s.mu.Lock()
			s.back = append(s.back, row.ID)
			s.mu.Unlock()
			return nil
		}
	}
	s.mu.Lock()
	s.sent = append(s.sent, row.ID)
	s.mu.Unlock()
	return nil
}

func (s *queueSender) Prune(context.Context) (int64, error) {
	s.pruneRun.Add(1)
	return 0, nil
}

func rows(n int) []domain.NoticeEmailOutbox {
	out := make([]domain.NoticeEmailOutbox, n)
	for i := range out {
		out[i] = domain.NoticeEmailOutbox{ID: uuid.New(), TenantID: uuid.New()}
	}
	return out
}

// A burst far above the concurrency is sent in full, a few at a time: nothing
// is dropped (#997, owner decision).
func TestNoticeEmailWorker_Success(t *testing.T) {
	s := &queueSender{queue: rows(100)}
	w := NewNoticeEmailWorker(s, s, 4, zerolog.Nop())

	w.Pass(context.Background())
	require.Len(t, s.sent, 100, "every queued e-mail goes out")
	require.LessOrEqual(t, s.peak.Load(), int32(4), "never more than the concurrency at once")
}

// An empty outbox is a quiet pass.
func TestNoticeEmailWorker_NotFound(t *testing.T) {
	s := &queueSender{}
	NewNoticeEmailWorker(s, s, 4, zerolog.Nop()).Pass(context.Background())
	require.Empty(t, s.sent)
}

// A cancelled loop claims nothing more.
func TestNoticeEmailWorker_Unauthorized(t *testing.T) {
	s := &queueSender{queue: rows(3)}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	NewNoticeEmailWorker(s, s, 4, zerolog.Nop()).Pass(ctx)
	require.Empty(t, s.sent)
	require.Len(t, s.queue, 3, "left in the outbox")
}

// Wake runs a pass at once, without waiting for the poll.
func TestNoticeEmailWorker_WakeSendsWithoutWaitingForThePoll(t *testing.T) {
	s := &queueSender{}
	w := NewNoticeEmailWorker(s, s, 4, zerolog.Nop()).WithInterval(time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	go w.Start(ctx)
	t.Cleanup(func() { cancel(); _ = w.Drain(context.Background()) })

	s.mu.Lock()
	s.queue = rows(2)
	s.mu.Unlock()
	w.Wake()
	require.Eventually(t, func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		return len(s.sent) == 2
	}, 2*time.Second, 10*time.Millisecond)
}

// Shutdown waits for the batch in flight.
func TestNoticeEmailWorker_DrainWaitsForTheBatch(t *testing.T) {
	s := &queueSender{queue: rows(2), hold: make(chan struct{}), started: make(chan struct{}, 10)}
	w := NewNoticeEmailWorker(s, s, 4, zerolog.Nop()).WithInterval(time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	go w.Start(ctx)
	<-s.started
	<-s.started

	cancel()
	drained := make(chan error, 1)
	go func() { drained <- w.Drain(context.Background()) }()
	select {
	case <-drained:
		t.Fatal("drained while two sends were still running")
	case <-time.After(50 * time.Millisecond):
	}
	close(s.hold)
	require.NoError(t, <-drained)
	require.Len(t, s.sent, 2)
}

// Past the deadline the sends are cancelled and put their rows back.
func TestNoticeEmailWorker_DrainDeadlinePutsRowsBack(t *testing.T) {
	s := &queueSender{queue: rows(2), hold: make(chan struct{}), started: make(chan struct{}, 10)}
	w := NewNoticeEmailWorker(s, s, 4, zerolog.Nop()).WithInterval(time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	go w.Start(ctx)
	<-s.started
	<-s.started

	cancel()
	dctx, dcancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer dcancel()
	require.ErrorIs(t, w.Drain(dctx), context.DeadlineExceeded)
	s.mu.Lock()
	defer s.mu.Unlock()
	require.Len(t, s.back, 2, "both rows go back to the outbox")
	require.Empty(t, s.sent)
}
