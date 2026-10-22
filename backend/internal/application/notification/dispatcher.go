// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package notification

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// ErrNotMember reports that the addressee is not an active member of the tenant
// the notice is about. Nothing is delivered, on any channel.
//
// This is the cross-tenant guard (#757): a notice about tenant A's plan reaches
// only people who belong to tenant A today. A stale assignee id left on a plan
// after its holder was removed, or an id that never belonged here, gets nothing.
var ErrNotMember = errors.New("recipient is not an active member of this tenant")

// ErrEmailFailed wraps a transport failure. Its text never carries the address:
// SMTP servers echo the recipient in their refusals ("550 <a@b>: unknown user"),
// and the callers log this error (RULE #6).
var ErrEmailFailed = errors.New("notice e-mail not delivered")

// Recipient is what the dispatcher needs to know about one member.
type Recipient struct {
	Email    string
	FullName string
	Locale   string
}

// RecipientDirectory resolves a user inside ONE tenant. It returns nil, nil
// when the user is not an active member of that tenant.
type RecipientDirectory interface {
	NoticeRecipient(ctx context.Context, tenantID, userID uuid.UUID) (*Recipient, error)
}

// NoticeMailer renders and sends one notice by e-mail.
type NoticeMailer interface {
	SendNotice(ctx context.Context, to, fullName, locale string, msg domain.NoticeMessage, link string) error
}

// LinkFunc turns the notice's resource into a path inside the app ("" when the
// resource has no page). Injected so this package does not pick routes.
type LinkFunc func(resourceType string, resourceID *uuid.UUID) string

// Dispatcher delivers a notice to one member: in-app always (preferences
// permitting), e-mail when a transport is configured and the member's
// preferences allow it. The e-mail is queued in the outbox; OutboxSender sends
// it.
//
// It is the one place that answers "may this person hear about this, and
// how" — the producers (ownership, deadline sweep, lifecycle transitions,
// plan validation) only word the event.
type Dispatcher struct {
	inApp     *UseCase
	directory RecipientDirectory
	mailer    NoticeMailer
	link      LinkFunc
	emailOn   bool
	timeout   time.Duration

	// outbox, when set, takes the e-mail instead of sending it inline: a worker
	// sends it, groups what follows within the organization's window, and
	// retries what fails (#997). Every production path uses it.
	outbox   EmailOutbox
	policies NoticeEmailPolicyReader
	// wake nudges the outbox worker after an enqueue, so an e-mail due now
	// does not wait for the next poll. Optional.
	wake func()
	// report receives e-mail failures the caller would otherwise lose: the
	// producers do not check Deliver's error. Optional.
	report func(error)
}

// EmailOutbox queues notice e-mails (repository.GormNoticeEmailOutbox). It
// returns the row's kind (single or digest) and when it is due.
type EmailOutbox interface {
	Enqueue(ctx context.Context, row domain.NoticeEmailOutbox, window time.Duration) (string, time.Time, error)
}

// NoticeEmailPolicyReader gives a tenant's grouping window.
type NoticeEmailPolicyReader interface {
	GetNoticeEmailPolicy(ctx context.Context, tenantID uuid.UUID) (*domain.NoticeEmailPolicy, error)
}

const (
	// defaultSendTimeout bounds one inline e-mail.
	defaultSendTimeout = 30 * time.Second
	// enqueueTimeout bounds the outbox write on the request path.
	enqueueTimeout = 5 * time.Second
)

// NewDispatcher builds a dispatcher with in-app delivery only. E-mail is added
// by WithEmail; a deployment without SMTP leaves it off and in-app remains the
// channel (#757 criterion 4).
func NewDispatcher(inApp *UseCase, directory RecipientDirectory) *Dispatcher {
	return &Dispatcher{inApp: inApp, directory: directory, timeout: defaultSendTimeout}
}

// WithEmail turns on e-mail delivery through mailer. enabled is false when the
// deployment has no mail transport: the dispatcher then does not try, rather
// than failing on every notice.
func (d *Dispatcher) WithEmail(mailer NoticeMailer, enabled bool) *Dispatcher {
	d.mailer = mailer
	d.emailOn = enabled && mailer != nil
	return d
}

// WithLinks sets how a resource becomes a link in the e-mail.
func (d *Dispatcher) WithLinks(f LinkFunc) *Dispatcher {
	d.link = f
	return d
}

// WithOutbox queues e-mails instead of sending them inline. policies gives
// each tenant's grouping window (the default when nil or unset); wake, when
// set, is called after each enqueue.
func (d *Dispatcher) WithOutbox(outbox EmailOutbox, policies NoticeEmailPolicyReader, wake func()) *Dispatcher {
	d.outbox = outbox
	d.policies = policies
	d.wake = wake
	return d
}

// ReportingTo sends every e-mail failure to report as well as returning it.
// The producers drop Deliver's error, so this is where it gets logged.
func (d *Dispatcher) ReportingTo(report func(error)) *Dispatcher {
	d.report = report
	return d
}

// EmailEnabled reports whether notices can go out by e-mail at all.
func (d *Dispatcher) EmailEnabled() bool { return d != nil && d.emailOn }

// Deliver sends the notice. The in-app record and the e-mail are independent:
// a failed e-mail does not undo the bell, and a suppressed in-app record (the
// global "silence everything" switch) suppresses the e-mail too, through the
// same preference row.
//
// Returns nil when everything that was allowed went out; ErrNotMember when the
// addressee does not belong to the tenant; otherwise the joined failures.
// ErrSuppressed is not a failure and is never returned.
func (d *Dispatcher) Deliver(ctx context.Context, n domain.Notice) error {
	if d == nil || d.inApp == nil {
		return nil
	}
	if n.TenantID == uuid.Nil || n.UserID == uuid.Nil || n.Compose == nil {
		return ErrValidation
	}

	var rcpt *Recipient
	if d.directory != nil {
		r, err := d.directory.NoticeRecipient(ctx, n.TenantID, n.UserID)
		if err != nil {
			return err
		}
		if r == nil {
			return ErrNotMember
		}
		rcpt = r
	} else {
		// No directory, no way to prove membership: fail closed. A notice that
		// cannot be addressed safely is not sent.
		return ErrNotMember
	}

	locale := domain.NoticeLocale(rcpt.Locale)
	msg := n.Compose(locale)

	var errs []error
	if err := d.inApp.NotifyInApp(n.UserID, n.TenantID, n.Type, msg.Subject, msg.InAppText(), n.ResourceID, n.ResourceType); err != nil && !errors.Is(err, ErrSuppressed) {
		errs = append(errs, err)
	}

	if d.emailOn && strings.TrimSpace(rcpt.Email) != "" &&
		d.inApp.ShouldNotify(n.UserID, n.TenantID, n.Type, domain.NotificationChannelEmail) {
		link := ""
		if d.link != nil && d.outbox == nil {
			link = d.link(n.ResourceType, n.ResourceID)
		}
		var err error
		if d.outbox != nil {
			err = d.enqueue(ctx, n, locale, msg)
		} else {
			err = d.sendNow(ctx, rcpt, locale, msg, link)
		}
		if err != nil {
			if d.report != nil {
				d.report(err)
			}
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// enqueue hands the e-mail to the outbox, worded for the recipient. The
// address is not stored: the worker resolves it again at send time.
//
// The write does not inherit the caller's cancellation: a request that ends
// while the row is written must not lose the e-mail.
func (d *Dispatcher) enqueue(ctx context.Context, n domain.Notice, locale string, msg domain.NoticeMessage) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), enqueueTimeout)
	defer cancel()
	window := domain.NoticeEmailWindowDefault * time.Minute
	if d.policies != nil {
		// A policy that cannot be read falls back to the default: the e-mail
		// is still queued.
		if p, err := d.policies.GetNoticeEmailPolicy(ctx, n.TenantID); err == nil {
			window = p.Window()
		}
	}
	row := domain.NoticeEmailOutbox{
		TenantID: n.TenantID, UserID: n.UserID,
		ResourceType: n.ResourceType, ResourceID: n.ResourceID, ResourceTitle: n.ResourceTitle,
		Locale:  locale,
		Grouped: !n.Deduplicated,
		Events: domain.NoticeEmailEvents{{
			Type: n.Type, At: time.Now().UTC(), Subject: msg.Subject, Lines: msg.Lines,
		}},
	}
	if _, _, err := d.outbox.Enqueue(ctx, row, window); err != nil {
		return errors.Join(ErrEmailFailed, fmt.Errorf("queue: %w", err))
	}
	if d.wake != nil {
		d.wake()
	}
	return nil
}

// sendNow is the path without an outbox (tests, tools): one e-mail, inline.
func (d *Dispatcher) sendNow(ctx context.Context, rcpt *Recipient, locale string, msg domain.NoticeMessage, link string) error {
	ctx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()
	if err := d.mailer.SendNotice(ctx, rcpt.Email, rcpt.FullName, locale, msg, link); err != nil {
		return redactedEmailError(err, rcpt.Email)
	}
	return nil
}

// redactedEmailError keeps the transport's diagnosis and drops the address,
// whatever case the server echoed it in.
func redactedEmailError(err error, address string) error {
	text := err.Error()
	if address != "" {
		text = regexp.MustCompile(`(?i)`+regexp.QuoteMeta(address)).ReplaceAllString(text, "[recipient]")
	}
	return errors.Join(ErrEmailFailed, errors.New(text))
}
