// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package notification_test

// #757 criterion 5, end to end: a producer → the dispatcher → the HTML mailer →
// the REAL SMTP transport (email.Transport) → an SMTP server listening on a
// loopback port. Nothing on the delivery path is a fake; only the stores are
// in memory.

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/application/notification"
	"github.com/opendefender/openrisk/internal/application/ownership"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/authmail"
	"github.com/opendefender/openrisk/internal/infrastructure/email"
	"github.com/opendefender/openrisk/internal/infrastructure/workers"
	"github.com/opendefender/openrisk/internal/testsupport/smtpmock"
)

// memRepo is the in-app store: notifications kept in memory, default prefs.
type memRepo struct {
	mu    sync.Mutex
	rows  []*domain.Notification
	prefs map[uuid.UUID]*domain.NotificationPreference
}

func (r *memRepo) CreateNotification(n *domain.Notification) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rows = append(r.rows, n)
	return nil
}
func (r *memRepo) GetUserNotifications(uuid.UUID, uuid.UUID, int, int) ([]*domain.Notification, error) {
	return nil, nil
}
func (r *memRepo) GetUnreadCount(uuid.UUID, uuid.UUID) (int64, error)           { return 0, nil }
func (r *memRepo) MarkNotificationAsRead(uuid.UUID, uuid.UUID, uuid.UUID) error { return nil }
func (r *memRepo) MarkAllNotificationsAsRead(uuid.UUID, uuid.UUID) error        { return nil }
func (r *memRepo) DeleteNotification(uuid.UUID, uuid.UUID, uuid.UUID) error     { return nil }
func (r *memRepo) GetUserNotificationPreferences(userID, _ uuid.UUID) (*domain.NotificationPreference, error) {
	if p, ok := r.prefs[userID]; ok {
		return p, nil
	}
	return nil, nil // nil row: Allows() applies the permissive defaults
}
func (r *memRepo) UpdateNotificationPreferences(uuid.UUID, uuid.UUID, map[string]interface{}) error {
	return nil
}
func (r *memRepo) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.rows)
}

// memDirectory is the membership roster: tenant → user → contact.
type memDirectory map[uuid.UUID]map[uuid.UUID]notification.Recipient

func (d memDirectory) NoticeRecipient(_ context.Context, tenantID, userID uuid.UUID) (*notification.Recipient, error) {
	if r, ok := d[tenantID][userID]; ok {
		return &r, nil
	}
	return nil, nil
}

// memDueStore is the deadline sweep's store, honouring the stamps.
type memDueStore struct {
	mu    sync.Mutex
	plans []domain.Mitigation
}

func (s *memDueStore) ListDueForReminder(context.Context, time.Time) ([]domain.Mitigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]domain.Mitigation, len(s.plans))
	copy(out, s.plans)
	return out, nil
}
func (s *memDueStore) MarkReminderSent(_ context.Context, tenantID, id uuid.UUID, offset int, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.plans {
		if s.plans[i].ID == id && s.plans[i].TenantID == tenantID {
			s.plans[i].MarkReminderSent(offset, at)
		}
	}
	return nil
}

type rig struct {
	smtp       *smtpmock.Server
	inApp      *memRepo
	dispatcher *notification.Dispatcher
	tenantA    uuid.UUID
	tenantB    uuid.UUID
	alice      uuid.UUID // member of A
	bob        uuid.UUID // member of B only
}

func newRig(t *testing.T) *rig {
	t.Helper()
	r := &rig{
		smtp:    smtpmock.Start(t),
		inApp:   &memRepo{},
		tenantA: uuid.New(), tenantB: uuid.New(),
		alice: uuid.New(), bob: uuid.New(),
	}
	dir := memDirectory{
		r.tenantA: {r.alice: {Email: "alice@a.example", FullName: "Alice", Locale: "fr"}},
		r.tenantB: {r.bob: {Email: "bob@b.example", FullName: "Bob", Locale: "en"}},
	}
	transport := email.NewTransport(email.Config{
		Host: r.smtp.Host(), Port: r.smtp.Port(), From: "noreply@openrisk.test", Timeout: 5 * time.Second,
	})
	r.dispatcher = notification.NewDispatcher(notification.NewUseCase(r.inApp), dir).
		WithEmail(authmail.NewNoticeMailer(transport, "https://risk.example"), email.Delivers(transport)).
		WithLinks(func(rt string, id *uuid.UUID) string { return "/risks/mitigations/" + id.String() })
	return r
}

func TestNoticeDelivery_AssignmentSendsExactlyOneEmail(t *testing.T) {
	r := newRig(t)
	svc := ownership.NewService().WithDeliverer(r.dispatcher)
	ctx := context.Background()
	actor := uuid.New()
	planID := uuid.New()
	subject := domain.OwnershipSubject{ResourceType: "mitigation", ResourceID: planID, Title: "Patch <VPN> & rotate keys", Locale: "en"}
	alice := r.alice
	patch := domain.OwnershipPatch{Assignee: domain.NullableUUID{Present: true, Value: &alice}}

	var block domain.Ownership
	changes, err := svc.Apply(ctx, r.tenantA, &block, patch, actor)
	require.NoError(t, err)
	require.Len(t, changes, 1)
	svc.Notify(ctx, r.tenantA, changes, subject)

	msgs := r.smtp.Messages()
	require.Len(t, msgs, 1, "one assignment, one e-mail")
	require.Equal(t, []string{"alice@a.example"}, msgs[0].To)
	// Alice reads French, whatever language the person assigning her used.
	require.Contains(t, msgs[0].Data, "Subject: Nouvelle affectation\r\n")
	// The title a user typed is escaped, never markup.
	require.Contains(t, msgs[0].Data, "Patch &lt;VPN&gt; &amp; rotate keys")
	require.NotContains(t, msgs[0].Data, "<VPN>")
	require.Contains(t, msgs[0].Data, "https://risk.example/risks/mitigations/"+planID.String())
	require.Equal(t, 1, r.inApp.count(), "the bell gets the same notice")

	// Saving the same assignee again is not a new assignment.
	changes, err = svc.Apply(ctx, r.tenantA, &block, patch, actor)
	require.NoError(t, err)
	svc.Notify(ctx, r.tenantA, changes, subject)
	require.Len(t, r.smtp.Messages(), 1, "re-saving the same assignee must not mail again")
}

func TestNoticeDelivery_DeadlineSweepSendsOncePerReminder(t *testing.T) {
	r := newRig(t)
	due := time.Now().AddDate(0, 0, 5)
	store := &memDueStore{plans: []domain.Mitigation{{
		ID: uuid.New(), TenantID: r.tenantA, Title: "Rotate keys",
		Status: domain.MitigationInProgress, DueDate: &due,
		Ownership: domain.Ownership{AssigneeID: &r.alice},
	}}}
	w := workers.NewMitigationDueWorker(store, func(ctx context.Context, n domain.Notice) {
		require.NoError(t, r.dispatcher.Deliver(ctx, n))
	}, zerolog.Nop())

	w.Sweep(context.Background())
	require.Len(t, r.smtp.Messages(), 1, "D-7 window: one reminder")
	require.Contains(t, r.smtp.Messages()[0].Data, "=?UTF-8?q?=C3=89ch=C3=A9ance_dans_5_jours")

	w.Sweep(context.Background())
	require.Len(t, r.smtp.Messages(), 1, "a second tick sends nothing")
	require.Equal(t, 1, r.inApp.count())
}

func TestNoticeDelivery_OverdueReminderSentOnce(t *testing.T) {
	r := newRig(t)
	due := time.Now().AddDate(0, 0, -2)
	sent := time.Now().AddDate(0, 0, -3)
	store := &memDueStore{plans: []domain.Mitigation{{
		ID: uuid.New(), TenantID: r.tenantA, Title: "Rotate keys",
		Status: domain.MitigationInProgress, DueDate: &due,
		ReminderD7SentAt: &sent, ReminderD1SentAt: &sent,
		Ownership: domain.Ownership{AssigneeID: &r.alice},
	}}}
	w := workers.NewMitigationDueWorker(store, func(ctx context.Context, n domain.Notice) {
		require.NoError(t, r.dispatcher.Deliver(ctx, n))
	}, zerolog.Nop())

	w.Sweep(context.Background())
	w.Sweep(context.Background())
	msgs := r.smtp.Messages()
	require.Len(t, msgs, 1, "deadline passed: exactly one reminder")
	require.Contains(t, msgs[0].Data, "Subject: Mitigation en retard : Rotate keys")
}

// A user of tenant B receives nothing about tenant A — even when their id is on
// one of A's plans (stale assignee, forged id).
func TestNoticeDelivery_CrossTenantReceivesNothing(t *testing.T) {
	r := newRig(t)
	due := time.Now().AddDate(0, 0, 3)
	store := &memDueStore{plans: []domain.Mitigation{{
		ID: uuid.New(), TenantID: r.tenantA, Title: "Tenant A secret plan",
		Status: domain.MitigationInProgress, DueDate: &due,
		Ownership: domain.Ownership{AssigneeID: &r.bob},
	}}}
	var deliverErr error
	w := workers.NewMitigationDueWorker(store, func(ctx context.Context, n domain.Notice) {
		deliverErr = r.dispatcher.Deliver(ctx, n)
	}, zerolog.Nop())
	w.Sweep(context.Background())

	require.ErrorIs(t, deliverErr, notification.ErrNotMember)
	require.Empty(t, r.smtp.Messages(), "no e-mail crosses tenants")
	require.Zero(t, r.inApp.count(), "no in-app row crosses tenants")

	// The assignment path refuses the same way.
	svc := ownership.NewService().WithDeliverer(r.dispatcher)
	svc.Notify(context.Background(), r.tenantA, []domain.OwnershipChange{{
		Role: domain.RoleAssignee, To: &r.bob, Actor: uuid.New(),
	}}, domain.OwnershipSubject{ResourceType: "mitigation", Title: "Tenant A secret plan"})
	require.Empty(t, r.smtp.Messages())
	require.Zero(t, r.inApp.count())
	for _, m := range r.smtp.Messages() {
		require.False(t, strings.Contains(m.Data, "secret"))
	}
}

// A title is typed by a user and becomes part of the Subject header (deadline
// and state-change notices put it there). A CR/LF in it must not start a new
// header: the transport Q-encodes any control byte, even in plain ASCII.
func TestNoticeDelivery_TitleCannotInjectHeaders(t *testing.T) {
	r := newRig(t)
	id := uuid.New()
	err := r.dispatcher.Deliver(context.Background(), domain.Notice{
		TenantID: r.tenantA, UserID: r.alice,
		Type:       domain.NotificationTypeMitigationDeadline,
		ResourceID: &id, ResourceType: "mitigation",
		Compose: func(string) domain.NoticeMessage {
			return domain.NoticeMessage{Subject: "Due: x\r\nBcc: spy@evil.example", Lines: []string{"x"}}
		},
	})
	require.NoError(t, err)

	msgs := r.smtp.Messages()
	require.Len(t, msgs, 1)
	require.Equal(t, []string{"alice@a.example"}, msgs[0].To, "no extra recipient")
	headers, _, found := strings.Cut(msgs[0].Data, "\r\n\r\n")
	require.True(t, found)
	require.NotContains(t, headers, "\nBcc:")
	require.Contains(t, headers, "Subject: =?UTF-8?q?Due:_x=0D=0ABcc:")
}
