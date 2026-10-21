// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package notification

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/stretchr/testify/require"
)

type stubDirectory struct {
	tenantID uuid.UUID
	userID   uuid.UUID
	rcpt     Recipient
	err      error
}

func (d stubDirectory) NoticeRecipient(_ context.Context, tenantID, userID uuid.UUID) (*Recipient, error) {
	if d.err != nil {
		return nil, d.err
	}
	if tenantID != d.tenantID || userID != d.userID {
		return nil, nil
	}
	r := d.rcpt
	return &r, nil
}

type sentMail struct {
	to, locale string
	msg        domain.NoticeMessage
	link       string
}

type stubMailer struct {
	sent []sentMail
	err  error
	done chan struct{}
}

func (m *stubMailer) SendNotice(_ context.Context, to, _ string, locale string, msg domain.NoticeMessage, link string) error {
	m.sent = append(m.sent, sentMail{to: to, locale: locale, msg: msg, link: link})
	if m.done != nil {
		defer close(m.done)
	}
	return m.err
}

func testNotice(tenantID, userID uuid.UUID) domain.Notice {
	id := uuid.New()
	return domain.Notice{
		TenantID: tenantID, UserID: userID,
		Type:       domain.NotificationTypeActionAssigned,
		ResourceID: &id, ResourceType: "mitigation",
		Compose: func(locale string) domain.NoticeMessage {
			if locale == "en" {
				return domain.NoticeMessage{Subject: "New assignment", Lines: []string{"Plan assigned."}}
			}
			return domain.NoticeMessage{Subject: "Nouvelle affectation", Lines: []string{"Plan assigné."}}
		},
	}
}

type dispatcherFixture struct {
	tenantID, userID uuid.UUID
	created          []*domain.Notification
	prefs            *domain.NotificationPreference
	mailer           *stubMailer
	dispatcher       *Dispatcher
}

func newDispatcherFixture(locale string) *dispatcherFixture {
	f := &dispatcherFixture{tenantID: uuid.New(), userID: uuid.New(), mailer: &stubMailer{}}
	repo := &mockRepo{
		createFn: func(n *domain.Notification) error { f.created = append(f.created, n); return nil },
		getPrefsFn: func(uuid.UUID, uuid.UUID) (*domain.NotificationPreference, error) {
			return f.prefs, nil
		},
	}
	dir := stubDirectory{tenantID: f.tenantID, userID: f.userID, rcpt: Recipient{Email: "Alice@Example.com", FullName: "Alice", Locale: locale}}
	f.dispatcher = NewDispatcher(NewUseCase(repo), dir).
		WithEmail(f.mailer, true).
		WithLinks(func(rt string, id *uuid.UUID) string { return "/" + rt + "/" + id.String() })
	return f
}

func TestDispatcherDeliver_Success(t *testing.T) {
	f := newDispatcherFixture("en-GB")
	n := testNotice(f.tenantID, f.userID)

	require.NoError(t, f.dispatcher.Deliver(context.Background(), n))

	require.Len(t, f.created, 1)
	require.Equal(t, "New assignment", f.created[0].Subject, "in-app is worded in the recipient's language")
	require.Equal(t, f.tenantID, f.created[0].TenantID)
	require.Len(t, f.mailer.sent, 1)
	require.Equal(t, "en", f.mailer.sent[0].locale)
	require.Equal(t, "/mitigation/"+n.ResourceID.String(), f.mailer.sent[0].link)
}

// Not a member of the notice's tenant: the "not found" of a recipient.
func TestDispatcherDeliver_NotFound(t *testing.T) {
	f := newDispatcherFixture("fr")

	err := f.dispatcher.Deliver(context.Background(), testNotice(uuid.New(), f.userID))
	require.ErrorIs(t, err, ErrNotMember)
	require.Empty(t, f.created)
	require.Empty(t, f.mailer.sent)
}

func TestDispatcherDeliver_Unauthorized(t *testing.T) {
	f := newDispatcherFixture("fr")

	for _, n := range []domain.Notice{
		testNotice(uuid.Nil, f.userID),
		testNotice(f.tenantID, uuid.Nil),
	} {
		require.ErrorIs(t, f.dispatcher.Deliver(context.Background(), n), ErrValidation)
	}
	require.Empty(t, f.created)
	require.Empty(t, f.mailer.sent)

	// No directory, no proof of membership: nothing goes out.
	d := NewDispatcher(NewUseCase(&mockRepo{}), nil).WithEmail(f.mailer, true)
	require.ErrorIs(t, d.Deliver(context.Background(), testNotice(f.tenantID, f.userID)), ErrNotMember)
	require.Empty(t, f.mailer.sent)
}

func TestDispatcherDeliver_RespectsEmailPreference(t *testing.T) {
	f := newDispatcherFixture("fr")
	f.prefs = &domain.NotificationPreference{EmailOnActionAssigned: false}

	require.NoError(t, f.dispatcher.Deliver(context.Background(), testNotice(f.tenantID, f.userID)))
	require.Len(t, f.created, 1, "the bell still rings")
	require.Empty(t, f.mailer.sent, "the e-mail switch is off")

	f.prefs = &domain.NotificationPreference{DisableAllNotifications: true}
	require.NoError(t, f.dispatcher.Deliver(context.Background(), testNotice(f.tenantID, f.userID)),
		"suppression is not a failure")
	require.Len(t, f.created, 1)
	require.Empty(t, f.mailer.sent)
}

// No SMTP (self-host): in-app is the channel, and nothing fails.
func TestDispatcherDeliver_EmailDisabledKeepsInApp(t *testing.T) {
	f := newDispatcherFixture("fr")
	f.dispatcher.WithEmail(f.mailer, false)

	require.False(t, f.dispatcher.EmailEnabled())
	require.NoError(t, f.dispatcher.Deliver(context.Background(), testNotice(f.tenantID, f.userID)))
	require.Len(t, f.created, 1)
	require.Empty(t, f.mailer.sent)
}

// RULE #6: the error a caller logs never carries the address, whatever case
// the SMTP server echoed it in.
func TestDispatcherDeliver_EmailErrorIsRedacted(t *testing.T) {
	f := newDispatcherFixture("fr")
	f.mailer.err = errors.New("email: rcpt: 550 5.1.1 <alice@example.com>: Recipient address rejected")

	err := f.dispatcher.Deliver(context.Background(), testNotice(f.tenantID, f.userID))
	require.ErrorIs(t, err, ErrEmailFailed)
	require.NotContains(t, err.Error(), "alice@example.com")
	require.Contains(t, err.Error(), "[recipient]")
	require.Len(t, f.created, 1, "a failed e-mail does not undo the bell")
}

func TestDispatcherDeliver_InBackgroundReportsRedactedFailure(t *testing.T) {
	f := newDispatcherFixture("fr")
	f.mailer.err = errors.New("550 <Alice@Example.com> unknown")
	f.mailer.done = make(chan struct{})
	reported := make(chan error, 1)
	f.dispatcher.InBackground(func(err error) { reported <- err }, 0)

	require.NoError(t, f.dispatcher.Deliver(context.Background(), testNotice(f.tenantID, f.userID)),
		"the caller does not wait on the relay")
	select {
	case err := <-reported:
		require.ErrorIs(t, err, ErrEmailFailed)
		require.NotContains(t, err.Error(), "Alice@Example.com")
	case <-time.After(2 * time.Second):
		t.Fatal("background failure was never reported")
	}
}
