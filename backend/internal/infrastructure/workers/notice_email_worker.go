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

	"github.com/opendefender/openrisk/internal/domain"
)

// NoticeEmailSender claims and sends outbox rows (notification.OutboxSender).
type NoticeEmailSender interface {
	Claim(ctx context.Context, limit int) ([]domain.NoticeEmailOutbox, error)
	Send(ctx context.Context, row domain.NoticeEmailOutbox) error
}

// NoticeEmailPruner deletes outbox rows and throttle stamps past retention.
type NoticeEmailPruner interface {
	Prune(ctx context.Context) (int64, error)
}

// NoticeEmailWorker sends the notice e-mail outbox (#997).
//
// It polls every few seconds and is woken at once when a request queues an
// e-mail, so a first e-mail does not wait for the poll. It sends at most
// concurrency e-mails at a time; what it cannot send yet stays in the outbox,
// so nothing is ever dropped. Every replica runs one: the claim's SKIP LOCKED
// splits the rows between them.
type NoticeEmailWorker struct {
	sender      NoticeEmailSender
	pruner      NoticeEmailPruner
	logger      zerolog.Logger
	interval    time.Duration
	pruneEvery  time.Duration
	concurrency int
	wake        chan struct{}

	// Sends run on base, not on the loop's context: stopping the loop lets
	// the batch in flight finish. Drain cancels base past its deadline.
	base context.Context
	stop context.CancelFunc
	done chan struct{}
}

// drainGrace is how long Drain waits for cancelled sends to put their rows back.
const drainGrace = 3 * time.Second

func NewNoticeEmailWorker(sender NoticeEmailSender, pruner NoticeEmailPruner, concurrency int, logger zerolog.Logger) *NoticeEmailWorker {
	if concurrency <= 0 {
		concurrency = 8
	}
	base, stop := context.WithCancel(context.Background())
	return &NoticeEmailWorker{
		sender: sender, pruner: pruner, logger: logger,
		interval: 5 * time.Second, pruneEvery: time.Hour, concurrency: concurrency,
		wake: make(chan struct{}, 1),
		base: base, stop: stop, done: make(chan struct{}),
	}
}

// WithInterval overrides the poll cadence (tests).
func (w *NoticeEmailWorker) WithInterval(d time.Duration) *NoticeEmailWorker {
	if d > 0 {
		w.interval = d
	}
	return w
}

// Wake asks for a pass now. It never blocks.
func (w *NoticeEmailWorker) Wake() {
	select {
	case w.wake <- struct{}{}:
	default:
	}
}

// Start runs until ctx is done, then returns after the batch in flight.
func (w *NoticeEmailWorker) Start(ctx context.Context) {
	defer close(w.done)
	tick := time.NewTicker(w.interval)
	defer tick.Stop()
	prune := time.NewTicker(w.pruneEvery)
	defer prune.Stop()
	w.logger.Info().Int("concurrency", w.concurrency).Msg("Notice e-mail worker started (outbox, digests, retries)")
	w.Pass(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			w.Pass(ctx)
		case <-w.wake:
			w.Pass(ctx)
		case <-prune.C:
			if n, err := w.pruner.Prune(ctx); err != nil {
				w.logger.Warn().Err(err).Msg("notice e-mail: prune failed")
			} else if n > 0 {
				w.logger.Info().Int64("rows", n).Msg("notice e-mail: pruned sent rows and old throttle stamps")
			}
		}
	}
}

// Pass sends everything due, a batch of concurrency rows at a time, until the
// outbox has nothing due or ctx is done. Exported for tests.
func (w *NoticeEmailWorker) Pass(ctx context.Context) {
	for ctx.Err() == nil {
		rows, err := w.sender.Claim(ctx, w.concurrency)
		if err != nil {
			w.logger.Warn().Err(err).Msg("notice e-mail: could not claim due rows")
			return
		}
		var wg sync.WaitGroup
		for _, row := range rows {
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() {
					// A panic in a template must not take the process down. The
					// row's lease expires and it is tried again.
					if r := recover(); r != nil {
						w.logger.Error().Str("outbox_id", row.ID.String()).Msg("notice e-mail: panic while sending")
					}
				}()
				if err := w.sender.Send(w.base, row); err != nil {
					w.logger.Warn().Err(err).Str("outbox_id", row.ID.String()).
						Str("tenant_id", row.TenantID.String()).Msg("notice e-mail: not delivered")
				}
			}()
		}
		wg.Wait()
		if len(rows) < w.concurrency {
			return
		}
	}
}

// Drain waits for Start to return, which it does after its batch once its
// context is cancelled. Past ctx's deadline it cancels the sends still running;
// they put their rows back, due now, for the next boot or another replica.
func (w *NoticeEmailWorker) Drain(ctx context.Context) error {
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
