// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository_test

// #997 end to end: a producer → the dispatcher → the Postgres outbox → the
// notice e-mail worker → the HTML mailer → the real SMTP transport → an SMTP
// server on a loopback port. Only the in-app store and the member roster are
// in memory.

import (
	"context"
	"mime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/application/notification"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/authmail"
	"github.com/opendefender/openrisk/internal/infrastructure/email"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/internal/infrastructure/workers"
	"github.com/opendefender/openrisk/internal/testsupport/smtpmock"
)

type bellStore struct {
	mu   sync.Mutex
	rows int
}

func (b *bellStore) CreateNotification(*domain.Notification) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rows++
	return nil
}
func (b *bellStore) GetUserNotifications(uuid.UUID, uuid.UUID, int, int) ([]*domain.Notification, error) {
	return nil, nil
}
func (b *bellStore) GetUnreadCount(uuid.UUID, uuid.UUID) (int64, error)           { return 0, nil }
func (b *bellStore) MarkNotificationAsRead(uuid.UUID, uuid.UUID, uuid.UUID) error { return nil }
func (b *bellStore) MarkAllNotificationsAsRead(uuid.UUID, uuid.UUID) error        { return nil }
func (b *bellStore) DeleteNotification(uuid.UUID, uuid.UUID, uuid.UUID) error     { return nil }
func (b *bellStore) GetUserNotificationPreferences(uuid.UUID, uuid.UUID) (*domain.NotificationPreference, error) {
	return nil, nil
}
func (b *bellStore) UpdateNotificationPreferences(uuid.UUID, uuid.UUID, map[string]interface{}) error {
	return nil
}

type roster struct {
	mu      sync.Mutex
	members map[uuid.UUID]map[uuid.UUID]notification.Recipient
}

func (r *roster) NoticeRecipient(_ context.Context, tenantID, userID uuid.UUID) (*notification.Recipient, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if rc, ok := r.members[tenantID][userID]; ok {
		return &rc, nil
	}
	return nil, nil
}

func subjectOf(t *testing.T, data string) string {
	t.Helper()
	for _, line := range strings.Split(data, "\r\n") {
		if strings.HasPrefix(line, "Subject: ") {
			s, err := new(mime.WordDecoder).DecodeHeader(strings.TrimPrefix(line, "Subject: "))
			require.NoError(t, err)
			return s
		}
	}
	t.Fatal("no subject")
	return ""
}

func TestNoticeEmailDelivery_FirstAtOnceThenOneDigest_Postgres(t *testing.T) {
	db, _ := repository.OutboxSchemaForTest(t, false)
	ctx := context.Background()
	smtp := smtpmock.Start(t)
	tenant, alice := uuid.New(), uuid.New()
	members := &roster{members: map[uuid.UUID]map[uuid.UUID]notification.Recipient{
		tenant: {alice: {Email: "alice@a.example", FullName: "Alice", Locale: "fr"}},
	}}
	transport := email.NewTransport(email.Config{Host: smtp.Host(), Port: smtp.Port(), From: "noreply@openrisk.test", Timeout: 5 * time.Second})
	mailer := authmail.NewNoticeMailer(transport, "https://risk.example")
	link := func(rt string, id *uuid.UUID) string { return "/" + rt + "s/" + id.String() }
	inApp := notification.NewUseCase(&bellStore{})
	box := repository.NewGormNoticeEmailOutbox(db)
	policies := repository.NewGormNoticeEmailPolicyRepository(db)
	require.NoError(t, policies.SaveNoticeEmailPolicy(ctx, &domain.NoticeEmailPolicy{TenantID: tenant, WindowMinutes: 15}))

	newWorker := func() *workers.NoticeEmailWorker {
		return workers.NewNoticeEmailWorker(notification.NewOutboxSender(box, members, inApp, mailer, link), box, 4, zerolog.Nop())
	}
	worker := newWorker()
	dispatcher := notification.NewDispatcher(inApp, members).
		WithEmail(mailer, true).WithLinks(link).WithOutbox(box, policies, nil)

	plan := uuid.New()
	notice := func(subject string) domain.Notice {
		return domain.Notice{
			TenantID: tenant, UserID: alice, Type: domain.NotificationTypeActionAssigned,
			ResourceID: &plan, ResourceType: "mitigation", ResourceTitle: "Patch VPN",
			Compose: func(string) domain.NoticeMessage {
				return domain.NoticeMessage{Subject: subject, Lines: []string{subject + "."}}
			},
		}
	}

	// 10:00 assigned → at once.
	require.NoError(t, dispatcher.Deliver(ctx, notice("Affectée à vous")))
	worker.Pass(ctx)
	require.Len(t, smtp.Messages(), 1)
	require.Equal(t, "Affectée à vous", subjectOf(t, smtp.Messages()[0].Data))

	// 10:05 unassigned, 10:12 reassigned → held for the digest.
	require.NoError(t, dispatcher.Deliver(ctx, notice("Désaffectée")))
	require.NoError(t, dispatcher.Deliver(ctx, notice("Réaffectée à vous")))
	worker.Pass(ctx)
	require.Len(t, smtp.Messages(), 1, "nothing more inside the window")

	// The window ends (here: the clock is moved, not waited for). A fresh
	// worker stands for a restart: the outbox is what carries the e-mail.
	require.NoError(t, db.Exec(`UPDATE notice_email_outbox SET due_at = now() - interval '1 second' WHERE tenant_id = ? AND kind = 'digest'`, tenant).Error)
	newWorker().Pass(ctx)
	msgs := smtp.Messages()
	require.Len(t, msgs, 2, "one digest")
	require.Equal(t, "2 changements sur « Patch VPN »", subjectOf(t, msgs[1].Data))
	require.Contains(t, msgs[1].Data, "Le plus r")
	require.Contains(t, msgs[1].Data, "https://risk.example/mitigations/"+plan.String())

	// Deadline reminders skip grouping: D-7, D-1 and passed on the same plan.
	for _, s := range []string{"Échéance dans 7 jours", "Échéance demain", "Mitigation en retard"} {
		n := notice(s)
		n.Type = domain.NotificationTypeMitigationDeadline
		n.Deduplicated = true
		require.NoError(t, dispatcher.Deliver(ctx, n))
	}
	worker.Pass(ctx)
	require.Len(t, smtp.Messages(), 5, "all three reminders")

	// Removed from the tenant after the notice was queued: nothing is sent.
	other := uuid.New()
	n := notice("Affectée à vous")
	n.ResourceID = &other
	require.NoError(t, dispatcher.Deliver(ctx, n))
	members.mu.Lock()
	delete(members.members[tenant], alice)
	members.mu.Unlock()
	worker.Pass(ctx)
	require.Len(t, smtp.Messages(), 5)
	var skipped int64
	require.NoError(t, db.Model(&domain.NoticeEmailOutbox{}).
		Where("tenant_id = ? AND status = ?", tenant, domain.NoticeEmailSkipped).Count(&skipped).Error)
	require.EqualValues(t, 1, skipped)

	var pending int64
	require.NoError(t, db.Model(&domain.NoticeEmailOutbox{}).
		Where("tenant_id = ? AND status IN ?", tenant, []string{domain.NoticeEmailPending, domain.NoticeEmailSending}).Count(&pending).Error)
	require.Zero(t, pending, "nothing left behind")
}
