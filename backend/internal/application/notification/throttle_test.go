// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package notification

// #997 — the notice e-mail throttle, the bounded background pool and the
// shutdown drain. The Postgres claim itself, with replicas racing, is tested in
// repository/gorm_notice_email_throttle_pg_test.go.

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/stretchr/testify/require"
)

// memThrottle has the semantics of the Postgres claim, in memory.
type memThrottle struct {
	mu       sync.Mutex
	rows     map[domain.NoticeEmailClaim]time.Time // key (SentAt zero) → sent_at
	err      error
	released int
}

func newMemThrottle() *memThrottle {
	return &memThrottle{rows: map[domain.NoticeEmailClaim]time.Time{}}
}

func (m *memThrottle) Claim(_ context.Context, key domain.NoticeEmailClaim, window time.Duration) (*domain.NoticeEmailClaim, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return nil, m.err
	}
	key.SentAt = time.Time{}
	now := time.Now()
	if at, ok := m.rows[key]; ok && !at.Before(now.Add(-window)) {
		return nil, nil
	}
	m.rows[key] = now
	claim := key
	claim.SentAt = now
	return &claim, nil
}

func (m *memThrottle) Release(_ context.Context, claim domain.NoticeEmailClaim) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	at := claim.SentAt
	claim.SentAt = time.Time{}
	if m.rows[claim].Equal(at) {
		delete(m.rows, claim)
		m.released++
	}
	return nil
}

// sameResource returns a notice builder that always speaks of one plan.
func sameResource(tenantID, userID uuid.UUID) func() domain.Notice {
	n := testNotice(tenantID, userID)
	return func() domain.Notice { return n }
}

func TestDispatcherThrottle_Success(t *testing.T) {
	f := newDispatcherFixture("en")
	f.dispatcher.WithThrottle(newMemThrottle(), time.Hour)
	notice := sameResource(f.tenantID, f.userID)

	require.NoError(t, f.dispatcher.Deliver(context.Background(), notice()))
	require.NoError(t, f.dispatcher.Deliver(context.Background(), notice()),
		"a throttled e-mail is not a failure")
	require.NoError(t, f.dispatcher.Deliver(context.Background(), notice()))

	require.Len(t, f.mailer.sent, 1, "one e-mail per recipient, type and resource in the window")
	require.Len(t, f.created, 3, "the bell is not throttled")
	// The one e-mail that went out says later changes will not be mailed.
	lines := f.mailer.sent[0].msg.Lines
	require.Equal(t, "Further changes to this item within the next 60 minutes will not be e-mailed. The app always shows the current state.", lines[len(lines)-1])
	require.Equal(t, "Plan assigned.", f.created[0].Message, "the in-app text carries no throttle note")

	// Another resource, another type, or another tenant has its own slot.
	other := testNotice(f.tenantID, f.userID)
	require.NoError(t, f.dispatcher.Deliver(context.Background(), other))
	other = notice()
	other.Type = domain.NotificationTypeMitigationValidated
	require.NoError(t, f.dispatcher.Deliver(context.Background(), other))
	require.Len(t, f.mailer.sent, 3)
}

// A notice with no resource has nothing to deduplicate against.
func TestDispatcherThrottle_NotFound(t *testing.T) {
	f := newDispatcherFixture("fr")
	f.dispatcher.WithThrottle(newMemThrottle(), time.Hour).WithLinks(nil)
	n := testNotice(f.tenantID, f.userID)
	n.ResourceID = nil

	require.NoError(t, f.dispatcher.Deliver(context.Background(), n))
	require.NoError(t, f.dispatcher.Deliver(context.Background(), n))
	require.Len(t, f.mailer.sent, 2)
	require.Equal(t, []string{"Plan assigné."}, f.mailer.sent[0].msg.Lines, "no throttle note when nothing is throttled")
}

// The throttle comes after the membership and preference gates: a non-member
// takes no slot, and a muted e-mail takes none either.
func TestDispatcherThrottle_Unauthorized(t *testing.T) {
	f := newDispatcherFixture("fr")
	th := newMemThrottle()
	f.dispatcher.WithThrottle(th, time.Hour)

	require.ErrorIs(t, f.dispatcher.Deliver(context.Background(), testNotice(uuid.New(), f.userID)), ErrNotMember)
	f.prefs = &domain.NotificationPreference{EmailOnActionAssigned: false}
	require.NoError(t, f.dispatcher.Deliver(context.Background(), testNotice(f.tenantID, f.userID)))
	require.Empty(t, th.rows, "no claim is taken for an e-mail that is not sent")
	require.Empty(t, f.mailer.sent)
}

func TestDispatcherThrottle_FailedSendReleasesClaim(t *testing.T) {
	f := newDispatcherFixture("fr")
	th := newMemThrottle()
	f.dispatcher.WithThrottle(th, time.Hour)
	notice := sameResource(f.tenantID, f.userID)

	f.mailer.err = errors.New("421 try again later")
	require.ErrorIs(t, f.dispatcher.Deliver(context.Background(), notice()), ErrEmailFailed)
	require.Equal(t, 1, th.released, "a failed send gives its claim back")

	f.mailer.err = nil
	require.NoError(t, f.dispatcher.Deliver(context.Background(), notice()))
	require.Len(t, f.mailer.sent, 2, "the retry is not held for the window")
}

// D-7, D-1 and "passed" are one type on one plan; the sweep's own stamps
// already send each once, so the throttle must not swallow the second.
func TestDispatcherThrottle_DeduplicatedNoticesBypass(t *testing.T) {
	f := newDispatcherFixture("fr")
	th := newMemThrottle()
	f.dispatcher.WithThrottle(th, 24*time.Hour)
	n := testNotice(f.tenantID, f.userID)
	n.Type = domain.NotificationTypeMitigationDeadline
	n.Deduplicated = true

	for range 3 {
		require.NoError(t, f.dispatcher.Deliver(context.Background(), n))
	}
	require.Len(t, f.mailer.sent, 3)
	require.Empty(t, th.rows)
}

// The throttle store down must not silence notices: the e-mail goes out and
// the failure is returned for the log.
func TestDispatcherThrottle_StoreErrorSendsUnthrottled(t *testing.T) {
	f := newDispatcherFixture("fr")
	th := newMemThrottle()
	th.err = errors.New("connection refused")
	f.dispatcher.WithThrottle(th, time.Hour)

	err := f.dispatcher.Deliver(context.Background(), testNotice(f.tenantID, f.userID))
	require.ErrorContains(t, err, "throttle unavailable")
	require.NotErrorIs(t, err, ErrEmailFailed)
	require.Len(t, f.mailer.sent, 1)
}

func TestDispatcherThrottle_WindowZeroIsOff(t *testing.T) {
	f := newDispatcherFixture("fr")
	f.dispatcher.WithThrottle(newMemThrottle(), 0)
	notice := sameResource(f.tenantID, f.userID)

	require.NoError(t, f.dispatcher.Deliver(context.Background(), notice()))
	require.NoError(t, f.dispatcher.Deliver(context.Background(), notice()))
	require.Len(t, f.mailer.sent, 2)
}

func TestThrottleSpan(t *testing.T) {
	require.Equal(t, "les 30 prochaines minutes", throttleSpan(30*time.Minute, "fr"))
	require.Equal(t, "the next 60 minutes", throttleSpan(time.Hour, "en"))
	require.Equal(t, "les 24 prochaines heures", throttleSpan(24*time.Hour, "fr"))
	require.Equal(t, "the next 1 minutes", throttleSpan(time.Second, "en"))
}

// gateMailer blocks every send until released or until its context ends.
type gateMailer struct {
	mu      sync.Mutex
	started chan struct{}
	release chan struct{}
	sent    int
	ctxErrs []error
}

func newGateMailer() *gateMailer {
	return &gateMailer{started: make(chan struct{}, 100), release: make(chan struct{})}
}

func (m *gateMailer) SendNotice(ctx context.Context, _, _, _ string, _ domain.NoticeMessage, _ string) error {
	m.started <- struct{}{}
	select {
	case <-m.release:
		m.mu.Lock()
		m.sent++
		m.mu.Unlock()
		return nil
	case <-ctx.Done():
		m.mu.Lock()
		m.ctxErrs = append(m.ctxErrs, ctx.Err())
		m.mu.Unlock()
		return ctx.Err()
	}
}

func backgroundFixture(t *testing.T, maxInFlight int) (*dispatcherFixture, *gateMailer, chan error) {
	t.Helper()
	f := newDispatcherFixture("fr")
	mailer := newGateMailer()
	reported := make(chan error, 100)
	f.dispatcher.WithEmail(mailer, true).InBackground(func(err error) { reported <- err }, maxInFlight)
	return f, mailer, reported
}

func waitStarted(t *testing.T, m *gateMailer, n int) {
	t.Helper()
	for range n {
		select {
		case <-m.started:
		case <-time.After(2 * time.Second):
			t.Fatal("background send never started")
		}
	}
}

// A full pool never makes the request wait: the e-mail is dropped, reported,
// and the in-app row stands.
func TestDispatcherPool_FullPoolDropsWithoutBlocking(t *testing.T) {
	f, mailer, reported := backgroundFixture(t, 2)
	t.Cleanup(func() { close(mailer.release) })

	for range 2 {
		require.NoError(t, f.dispatcher.Deliver(context.Background(), testNotice(f.tenantID, f.userID)))
	}
	waitStarted(t, mailer, 2)

	done := make(chan error, 1)
	go func() { done <- f.dispatcher.Deliver(context.Background(), testNotice(f.tenantID, f.userID)) }()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("Deliver blocked on a full pool")
	}
	select {
	case err := <-reported:
		require.ErrorIs(t, err, ErrEmailDropped)
		require.ErrorIs(t, err, ErrEmailFailed)
	case <-time.After(time.Second):
		t.Fatal("the dropped e-mail was not reported")
	}
	require.Len(t, f.created, 3, "every notice still reached the bell")
}

func TestDispatcherPool_ThrottledIsNotReported(t *testing.T) {
	f := newDispatcherFixture("fr")
	reported := make(chan error, 10)
	f.mailer.done = make(chan struct{})
	f.dispatcher.WithThrottle(newMemThrottle(), time.Hour).InBackground(func(err error) { reported <- err }, 0)
	notice := sameResource(f.tenantID, f.userID)

	require.NoError(t, f.dispatcher.Deliver(context.Background(), notice()))
	<-f.mailer.done
	require.NoError(t, f.dispatcher.Deliver(context.Background(), notice()))
	require.NoError(t, f.dispatcher.Drain(context.Background()))
	require.Len(t, f.mailer.sent, 1)
	require.Empty(t, reported, "a throttled e-mail is not a failure")
}

func TestDispatcherDrain_WaitsForInFlight(t *testing.T) {
	f, mailer, reported := backgroundFixture(t, 4)
	require.NoError(t, f.dispatcher.Deliver(context.Background(), testNotice(f.tenantID, f.userID)))
	waitStarted(t, mailer, 1)

	drained := make(chan error, 1)
	go func() { drained <- f.dispatcher.Drain(context.Background()) }()
	select {
	case <-drained:
		t.Fatal("Drain returned while a send was still in flight")
	case <-time.After(50 * time.Millisecond):
	}
	close(mailer.release)
	select {
	case err := <-drained:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Drain never returned")
	}
	require.Equal(t, 1, mailer.sent)
	require.Empty(t, reported)

	// Once drained, nothing new starts.
	require.NoError(t, f.dispatcher.Deliver(context.Background(), testNotice(f.tenantID, f.userID)))
	require.ErrorIs(t, <-reported, ErrEmailDropped)
}

func TestDispatcherDrain_DeadlineCancelsSends(t *testing.T) {
	f, mailer, reported := backgroundFixture(t, 4)
	for range 2 {
		require.NoError(t, f.dispatcher.Deliver(context.Background(), testNotice(f.tenantID, f.userID)))
	}
	waitStarted(t, mailer, 2)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	require.ErrorIs(t, f.dispatcher.Drain(ctx), context.DeadlineExceeded)
	require.Less(t, time.Since(start), drainGrace, "cancelled sends return promptly")

	mailer.mu.Lock()
	defer mailer.mu.Unlock()
	require.Len(t, mailer.ctxErrs, 2, "both sends saw their context cancelled")
	require.Zero(t, mailer.sent)
	for range 2 {
		require.ErrorIs(t, <-reported, ErrEmailFailed)
	}
}

func TestDispatcherDrain_NoBackgroundIsNoop(t *testing.T) {
	f := newDispatcherFixture("fr")
	require.NoError(t, f.dispatcher.Drain(context.Background()))
	var nilDispatcher *Dispatcher
	require.NoError(t, nilDispatcher.Drain(context.Background()))
}
