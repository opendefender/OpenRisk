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
// preferences allow it.
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
	// background, when set, sends the e-mail off the caller's goroutine and
	// reports its outcome here. Request paths use it: an assignment must not
	// wait on a slow SMTP relay.
	background func(error)
	timeout    time.Duration
}

// NewDispatcher builds a dispatcher with in-app delivery only. E-mail is added
// by WithEmail; a deployment without SMTP leaves it off and in-app remains the
// channel (#757 criterion 4).
func NewDispatcher(inApp *UseCase, directory RecipientDirectory) *Dispatcher {
	return &Dispatcher{inApp: inApp, directory: directory}
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

// InBackground makes the e-mail half asynchronous. report receives every
// failure, already stripped of the address; it may be nil. The in-app record
// stays synchronous, so the bell is up to date when the request returns.
func (d *Dispatcher) InBackground(report func(error)) *Dispatcher {
	if report == nil {
		report = func(error) {}
	}
	d.background = report
	d.timeout = 30 * time.Second
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
		if d.link != nil {
			link = d.link(n.ResourceType, n.ResourceID)
		}
		send := func(ctx context.Context) error {
			if err := d.mailer.SendNotice(ctx, rcpt.Email, rcpt.FullName, locale, msg, link); err != nil {
				return redactedEmailError(err, rcpt.Email)
			}
			return nil
		}
		if d.background != nil {
			report, timeout := d.background, d.timeout
			go func() {
				// A panic in a template must not take the process down.
				defer func() {
					if r := recover(); r != nil {
						report(fmt.Errorf("%w: panic while sending", ErrEmailFailed))
					}
				}()
				bg, cancel := context.WithTimeout(context.Background(), timeout)
				defer cancel()
				if err := send(bg); err != nil {
					report(err)
				}
			}()
		} else if err := send(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
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
