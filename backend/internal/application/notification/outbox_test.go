// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package notification

// #997 — the dispatcher queues notice e-mails, the outbox sender sends them.
// The outbox's own SQL (grouping, claims, replicas) is tested on Postgres in
// repository/gorm_notice_email_outbox_pg_test.go.

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/stretchr/testify/require"
)

type queuedRow struct {
	row    domain.NoticeEmailOutbox
	window time.Duration
}

type memOutbox struct {
	mu     sync.Mutex
	queued []queuedRow
	err    error

	claimable []domain.NoticeEmailOutbox
	sent      []uuid.UUID
	retried   map[uuid.UUID]time.Time
	reasons   map[uuid.UUID]string
	closed    map[uuid.UUID]string
}

func newMemOutbox() *memOutbox {
	return &memOutbox{retried: map[uuid.UUID]time.Time{}, reasons: map[uuid.UUID]string{}, closed: map[uuid.UUID]string{}}
}

func (m *memOutbox) Enqueue(_ context.Context, row domain.NoticeEmailOutbox, window time.Duration) (string, time.Time, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return "", time.Time{}, m.err
	}
	m.queued = append(m.queued, queuedRow{row: row, window: window})
	return domain.NoticeEmailSingle, time.Now(), nil
}

func (m *memOutbox) ClaimDue(_ context.Context, limit int, _ time.Duration) ([]domain.NoticeEmailOutbox, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := min(limit, len(m.claimable))
	out := m.claimable[:n]
	m.claimable = m.claimable[n:]
	return out, nil
}

func (m *memOutbox) MarkSent(_ context.Context, row domain.NoticeEmailOutbox) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, row.ID)
	return nil
}

func (m *memOutbox) Retry(_ context.Context, row domain.NoticeEmailOutbox, at time.Time, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.retried[row.ID] = at
	m.reasons[row.ID] = reason
	return nil
}

func (m *memOutbox) Close(_ context.Context, row domain.NoticeEmailOutbox, status, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed[row.ID] = status
	m.reasons[row.ID] = reason
	return nil
}

type memPolicies struct {
	p   *domain.NoticeEmailPolicy
	err error
}

func (m *memPolicies) GetNoticeEmailPolicy(context.Context, uuid.UUID) (*domain.NoticeEmailPolicy, error) {
	return m.p, m.err
}

func (m *memPolicies) SaveNoticeEmailPolicy(_ context.Context, p *domain.NoticeEmailPolicy) error {
	if m.err != nil {
		return m.err
	}
	m.p = p
	return nil
}

// --- Dispatcher → outbox ---------------------------------------------------

func TestDispatcherOutbox_Success(t *testing.T) {
	f := newDispatcherFixture("en")
	box := newMemOutbox()
	woken := 0
	f.dispatcher.WithOutbox(box, &memPolicies{p: &domain.NoticeEmailPolicy{WindowMinutes: 240}}, func() { woken++ })
	n := testNotice(f.tenantID, f.userID)
	n.ResourceTitle = "Patch VPN"

	require.NoError(t, f.dispatcher.Deliver(context.Background(), n))

	require.Empty(t, f.mailer.sent, "nothing is sent from the request")
	require.Len(t, f.created, 1, "the bell rings at once")
	require.Len(t, box.queued, 1)
	q := box.queued[0]
	require.Equal(t, 4*time.Hour, q.window, "the organization's window")
	require.Equal(t, f.tenantID, q.row.TenantID)
	require.Equal(t, f.userID, q.row.UserID)
	require.Equal(t, *n.ResourceID, *q.row.ResourceID)
	require.Equal(t, "mitigation", q.row.ResourceType)
	require.Equal(t, "Patch VPN", q.row.ResourceTitle)
	require.Equal(t, "en", q.row.Locale)
	require.True(t, q.row.Grouped)
	require.Len(t, q.row.Events, 1)
	require.Equal(t, "New assignment", q.row.Events[0].Subject, "worded in the recipient's language")
	require.Equal(t, 1, woken, "the worker is woken for an e-mail due now")
}

// Not a member, or the e-mail switched off: nothing is queued.
func TestDispatcherOutbox_NotFound(t *testing.T) {
	f := newDispatcherFixture("fr")
	box := newMemOutbox()
	f.dispatcher.WithOutbox(box, nil, nil)

	require.ErrorIs(t, f.dispatcher.Deliver(context.Background(), testNotice(uuid.New(), f.userID)), ErrNotMember)
	f.prefs = &domain.NotificationPreference{EmailOnActionAssigned: false}
	require.NoError(t, f.dispatcher.Deliver(context.Background(), testNotice(f.tenantID, f.userID)))
	require.Empty(t, box.queued)
}

func TestDispatcherOutbox_Unauthorized(t *testing.T) {
	f := newDispatcherFixture("fr")
	box := newMemOutbox()
	f.dispatcher.WithOutbox(box, nil, nil)

	require.ErrorIs(t, f.dispatcher.Deliver(context.Background(), testNotice(uuid.Nil, f.userID)), ErrValidation)
	require.Empty(t, box.queued)
}

// A producer that deduplicates (the deadline sweep) is queued ungrouped.
func TestDispatcherOutbox_DeduplicatedIsNotGrouped(t *testing.T) {
	f := newDispatcherFixture("fr")
	box := newMemOutbox()
	f.dispatcher.WithOutbox(box, nil, nil)
	n := testNotice(f.tenantID, f.userID)
	n.Deduplicated = true

	require.NoError(t, f.dispatcher.Deliver(context.Background(), n))
	require.False(t, box.queued[0].row.Grouped)
	require.Equal(t, time.Hour, box.queued[0].window, "no policy: the default")
}

// An unreadable policy still queues the e-mail, with the default window.
func TestDispatcherOutbox_PolicyErrorUsesDefault(t *testing.T) {
	f := newDispatcherFixture("fr")
	box := newMemOutbox()
	f.dispatcher.WithOutbox(box, &memPolicies{err: errors.New("db down")}, nil)

	require.NoError(t, f.dispatcher.Deliver(context.Background(), testNotice(f.tenantID, f.userID)))
	require.Len(t, box.queued, 1)
	require.Equal(t, time.Hour, box.queued[0].window)
}

// The producers drop Deliver's error, so a queue failure is reported too.
func TestDispatcherOutbox_QueueErrorIsReported(t *testing.T) {
	f := newDispatcherFixture("fr")
	box := newMemOutbox()
	box.err = errors.New("connection refused")
	var reported []error
	f.dispatcher.WithOutbox(box, nil, nil).ReportingTo(func(err error) { reported = append(reported, err) })

	err := f.dispatcher.Deliver(context.Background(), testNotice(f.tenantID, f.userID))
	require.ErrorIs(t, err, ErrEmailFailed)
	require.Len(t, reported, 1)
	require.Len(t, f.created, 1, "the bell does not depend on the outbox")
}

// The request's cancellation does not cancel the outbox write.
func TestDispatcherOutbox_CancelledRequestStillQueues(t *testing.T) {
	f := newDispatcherFixture("fr")
	box := newMemOutbox()
	f.dispatcher.WithOutbox(box, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.NoError(t, f.dispatcher.Deliver(ctx, testNotice(f.tenantID, f.userID)))
	require.Len(t, box.queued, 1)
}

// --- Outbox sender ----------------------------------------------------------

type senderFixture struct {
	tenantID, userID uuid.UUID
	prefs            *domain.NotificationPreference
	box              *memOutbox
	mailer           *stubMailer
	sender           *OutboxSender
	now              time.Time
}

func newSenderFixture() *senderFixture {
	f := &senderFixture{tenantID: uuid.New(), userID: uuid.New(), box: newMemOutbox(), mailer: &stubMailer{}, now: time.Now()}
	repo := &mockRepo{getPrefsFn: func(uuid.UUID, uuid.UUID) (*domain.NotificationPreference, error) { return f.prefs, nil }}
	dir := stubDirectory{tenantID: f.tenantID, userID: f.userID, rcpt: Recipient{Email: "Alice@Example.com", FullName: "Alice", Locale: "fr"}}
	f.sender = NewOutboxSender(f.box, dir, NewUseCase(repo), f.mailer,
		func(rt string, id *uuid.UUID) string { return "/" + rt + "/" + id.String() })
	f.sender.now = func() time.Time { return f.now }
	return f
}

func (f *senderFixture) row(events ...domain.NoticeEmailEvent) domain.NoticeEmailOutbox {
	id := uuid.New()
	return domain.NoticeEmailOutbox{
		ID: uuid.New(), TenantID: f.tenantID, UserID: f.userID,
		ResourceType: "mitigation", ResourceID: &id, ResourceTitle: "Patch VPN",
		Kind: domain.NoticeEmailSingle, Locale: "fr", Grouped: true, Events: events, Attempts: 1,
	}
}

func event(subject string) domain.NoticeEmailEvent {
	return domain.NoticeEmailEvent{Type: domain.NotificationTypeActionAssigned, At: time.Now(), Subject: subject, Lines: []string{subject + "."}}
}

func TestOutboxSender_Success(t *testing.T) {
	f := newSenderFixture()
	row := f.row(event("Nouvelle affectation"))

	require.NoError(t, f.sender.Send(context.Background(), row))
	require.Equal(t, []uuid.UUID{row.ID}, f.box.sent)
	require.Len(t, f.mailer.sent, 1)
	require.Equal(t, "Alice@Example.com", f.mailer.sent[0].to, "the address is resolved at send time")
	require.Equal(t, "Nouvelle affectation", f.mailer.sent[0].msg.Subject)
	require.Equal(t, "/mitigation/"+row.ResourceID.String(), f.mailer.sent[0].link)
}

// Removed from the tenant since the notice was queued: nothing goes out.
func TestOutboxSender_NotFound(t *testing.T) {
	f := newSenderFixture()
	row := f.row(event("Nouvelle affectation"))
	row.TenantID = uuid.New()

	require.NoError(t, f.sender.Send(context.Background(), row))
	require.Empty(t, f.mailer.sent)
	require.Equal(t, domain.NoticeEmailSkipped, f.box.closed[row.ID])
	require.Equal(t, "recipient is no longer a member", f.box.reasons[row.ID])
}

// Turned the e-mail off since: nothing goes out.
func TestOutboxSender_Unauthorized(t *testing.T) {
	f := newSenderFixture()
	f.prefs = &domain.NotificationPreference{EmailOnActionAssigned: false}
	row := f.row(event("Nouvelle affectation"))

	require.NoError(t, f.sender.Send(context.Background(), row))
	require.Empty(t, f.mailer.sent)
	require.Equal(t, domain.NoticeEmailSkipped, f.box.closed[row.ID])
}

func TestOutboxSender_DigestListsEveryChange(t *testing.T) {
	f := newSenderFixture()
	row := f.row(event("Affecté"), event("Désaffecté"), event("Réaffecté"))
	row.Kind = domain.NoticeEmailDigest

	require.NoError(t, f.sender.Send(context.Background(), row))
	msg := f.mailer.sent[0].msg
	require.Equal(t, "3 changements sur « Patch VPN »", msg.Subject)
	require.Equal(t, []string{
		"3 changements ont eu lieu sur « Patch VPN » depuis le dernier e-mail à ce sujet :",
		"1. Affecté", "2. Désaffecté", "3. Réaffecté",
		"Le plus récent : Réaffecté.",
	}, msg.Lines)
}

// An event whose e-mail was turned off since is left out of the digest.
func TestOutboxSender_DigestDropsEventsNowMuted(t *testing.T) {
	f := newSenderFixture()
	f.prefs = &domain.NotificationPreference{EmailOnActionAssigned: false, EmailOnMitigationDeadline: true}
	deadline := event("Échéance demain")
	deadline.Type = domain.NotificationTypeMitigationDeadline
	row := f.row(event("Affecté"), deadline)

	require.NoError(t, f.sender.Send(context.Background(), row))
	require.Equal(t, "Échéance demain", f.mailer.sent[0].msg.Subject, "one event left, sent as written")
}

func TestOutboxSender_FailureRetriesWithBackoff(t *testing.T) {
	f := newSenderFixture()
	f.mailer.err = errors.New("421 4.7.0 <alice@example.com> try again later")
	row := f.row(event("Nouvelle affectation"))
	row.Attempts = 3

	require.NoError(t, f.sender.Send(context.Background(), row), "a retry is not a failure yet")
	require.Equal(t, f.now.Add(4*time.Minute), f.box.retried[row.ID])
	require.NotContains(t, f.box.reasons[row.ID], "alice@example.com", "RULE #6: the stored reason has no address")
	require.Contains(t, f.box.reasons[row.ID], "[recipient]")
	require.Empty(t, f.box.sent)
}

func TestOutboxSender_LastAttemptMarksFailed(t *testing.T) {
	f := newSenderFixture()
	f.mailer.err = errors.New("550 5.1.1 <alice@example.com> unknown")
	row := f.row(event("Nouvelle affectation"))
	row.Attempts = OutboxMaxAttempts

	err := f.sender.Send(context.Background(), row)
	require.ErrorIs(t, err, ErrOutboxFailed)
	require.NotContains(t, strings.ToLower(err.Error()), "alice@example.com")
	require.Equal(t, domain.NoticeEmailFailed, f.box.closed[row.ID], "kept, marked failed")
}

// A send cut by shutdown goes back, due now, whatever its attempt count.
func TestOutboxSender_InterruptedGoesBackNow(t *testing.T) {
	f := newSenderFixture()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	f.mailer.err = context.Canceled
	row := f.row(event("Nouvelle affectation"))
	row.Attempts = OutboxMaxAttempts

	require.NoError(t, f.sender.Send(ctx, row))
	require.Equal(t, f.now, f.box.retried[row.ID])
	require.Contains(t, f.box.reasons[row.ID], "interrupted")
	require.Empty(t, f.box.closed)
}

func TestOutboxBackoff(t *testing.T) {
	require.Equal(t, time.Minute, OutboxBackoff(0))
	require.Equal(t, time.Minute, OutboxBackoff(1))
	require.Equal(t, 2*time.Minute, OutboxBackoff(2))
	require.Equal(t, 32*time.Minute, OutboxBackoff(6))
	require.Equal(t, time.Hour, OutboxBackoff(7))
	require.Equal(t, time.Hour, OutboxBackoff(OutboxMaxAttempts))
}

func TestDigestMessage_English(t *testing.T) {
	msg := DigestMessage([]domain.NoticeEmailEvent{event("Assigned"), event("Status changed")}, "", "risk", "en", nil)
	require.Equal(t, "2 changes to “a risk”", msg.Subject)
	require.Equal(t, "Latest: Status changed.", msg.Lines[len(msg.Lines)-1])
}

type stateReader struct {
	state *domain.NoticeResourceState
	err   error
	asked []uuid.UUID // tenants asked
}

func (r *stateReader) NoticeResourceState(_ context.Context, tenantID uuid.UUID, _ string, _ uuid.UUID) (*domain.NoticeResourceState, error) {
	r.asked = append(r.asked, tenantID)
	return r.state, r.err
}

// The digest ends on the item as it stands when it is sent, not on the last
// change it lists.
func TestOutboxSender_DigestEndsOnCurrentState(t *testing.T) {
	f := newSenderFixture()
	progress := 40
	due := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	reader := &stateReader{state: &domain.NoticeResourceState{
		Exists: true, Title: "Patch VPN (renommé)", MitigationStatus: domain.MitigationInProgress,
		Progress: &progress, DueDate: &due, Owner: "Alice", Assignee: "Bob",
	}}
	f.sender.WithCurrentState(reader)
	row := f.row(event("Affecté"), event("Désaffecté"), event("Réaffecté"))

	require.NoError(t, f.sender.Send(context.Background(), row))
	msg := f.mailer.sent[0].msg
	require.Equal(t, "3 changements sur « Patch VPN (renommé) »", msg.Subject, "the current title")
	require.Equal(t, []string{
		"3 changements ont eu lieu sur « Patch VPN (renommé) » depuis le dernier e-mail à ce sujet :",
		"1. Affecté", "2. Désaffecté", "3. Réaffecté",
		"État actuel :",
		"Statut : En cours",
		"Avancement : 40 %",
		"Échéance : 12/11/2026",
		"Responsable : Alice",
		"Exécutant : Bob",
	}, msg.Lines)
	require.Equal(t, []uuid.UUID{f.tenantID}, reader.asked, "read in the row's own tenant")
}

func TestOutboxSender_DigestSaysWhenTheItemIsGone(t *testing.T) {
	f := newSenderFixture()
	f.sender.WithCurrentState(&stateReader{state: &domain.NoticeResourceState{Exists: false}})
	row := f.row(event("Affecté"), event("Désaffecté"))
	row.Locale = "en"

	require.NoError(t, f.sender.Send(context.Background(), row))
	lines := f.mailer.sent[0].msg.Lines
	require.Equal(t, "This item has since been deleted.", lines[len(lines)-1])
}

// The state cannot be read: the digest still goes out, ending on the latest
// change. A single e-mail does not read the state at all.
func TestOutboxSender_StateErrorStillSends(t *testing.T) {
	f := newSenderFixture()
	reader := &stateReader{err: errors.New("db down")}
	f.sender.WithCurrentState(reader)

	require.NoError(t, f.sender.Send(context.Background(), f.row(event("Affecté"), event("Désaffecté"))))
	lines := f.mailer.sent[0].msg.Lines
	require.Equal(t, "Le plus récent : Désaffecté.", lines[len(lines)-1])

	require.NoError(t, f.sender.Send(context.Background(), f.row(event("Affecté"))))
	require.Len(t, reader.asked, 1, "no state read for a single notice")
}

func TestDigestMessage_RiskStateAndNobodyAssigned(t *testing.T) {
	msg := DigestMessage([]domain.NoticeEmailEvent{event("a"), event("b")}, "R", "risk", "en",
		&domain.NoticeResourceState{Exists: true, Title: "R", RiskState: domain.StateInTreatment})
	require.Equal(t, []string{"Current state:", "Stage: " + domain.StateInTreatment.Label("en"), "Assignee: nobody"}, msg.Lines[3:])
}

// --- Policy use cases ------------------------------------------------------

func TestNoticeEmailPolicy_Success(t *testing.T) {
	store := &memPolicies{}
	tenant, actor := uuid.New(), uuid.New()

	view, err := NewUpdateNoticeEmailPolicyUseCase(store).Execute(context.Background(),
		UpdateNoticeEmailPolicyInput{TenantID: tenant, ActorID: actor, WindowMinutes: 240})
	require.NoError(t, err)
	require.Equal(t, 240, view.WindowMinutes)
	require.True(t, view.Configured)
	require.Equal(t, actor, *view.UpdatedByID)

	view, err = NewGetNoticeEmailPolicyUseCase(store).Execute(context.Background(), tenant)
	require.NoError(t, err)
	require.Equal(t, 240, view.WindowMinutes)
	require.Equal(t, []int{15, 30, 60, 240, 1440}, view.Options)
}

// Never saved: the default, said to be the default.
func TestNoticeEmailPolicy_NotFound(t *testing.T) {
	view, err := NewGetNoticeEmailPolicyUseCase(&memPolicies{}).Execute(context.Background(), uuid.New())
	require.NoError(t, err)
	require.Equal(t, 60, view.WindowMinutes)
	require.False(t, view.Configured)
	require.Equal(t, 60, view.DefaultMinutes)
}

func TestNoticeEmailPolicy_Unauthorized(t *testing.T) {
	_, err := NewGetNoticeEmailPolicyUseCase(&memPolicies{}).Execute(context.Background(), uuid.Nil)
	require.Error(t, err)
	require.Equal(t, 401, domain.HTTPStatusFromError(err))
	_, err = NewUpdateNoticeEmailPolicyUseCase(&memPolicies{}).Execute(context.Background(), UpdateNoticeEmailPolicyInput{WindowMinutes: 60})
	require.Equal(t, 401, domain.HTTPStatusFromError(err))
}

func TestNoticeEmailPolicy_RefusesAWindowOffTheList(t *testing.T) {
	store := &memPolicies{}
	for _, w := range []int{0, 1, 45, 10080, -60} {
		_, err := NewUpdateNoticeEmailPolicyUseCase(store).Execute(context.Background(),
			UpdateNoticeEmailPolicyInput{TenantID: uuid.New(), WindowMinutes: w})
		require.Equal(t, 400, domain.HTTPStatusFromError(err), "window %d", w)
	}
	require.Nil(t, store.p)
}
