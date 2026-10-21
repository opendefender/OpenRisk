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
	"sync"
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

// ErrEmailDropped reports a background e-mail that never started: the pool of
// concurrent sends was full, or the dispatcher was draining for shutdown. The
// in-app record stands.
var ErrEmailDropped = fmt.Errorf("%w: too many e-mails in flight", ErrEmailFailed)

// ErrThrottled reports a notice e-mail held back because one already went out
// for the same recipient, type and resource within the window (#997). Like
// ErrSuppressed it is not a failure and is never returned or reported.
var ErrThrottled = errors.New("notice e-mail throttled")

// EmailThrottle holds one claim per (tenant, user, type, resource) so a
// colleague who flips an assignment back and forth does not mail someone every
// time. It must be shared by every replica: the Postgres implementation is
// repository.GormNoticeEmailThrottle.
type EmailThrottle interface {
	// Claim takes the slot unless it was taken less than window ago. It returns
	// nil, nil when the slot is held, and the claim (SentAt set) when taken.
	Claim(ctx context.Context, key domain.NoticeEmailClaim, window time.Duration) (*domain.NoticeEmailClaim, error)
	// Release gives back a claim whose e-mail failed, so the retry is not held
	// for the whole window. It removes that claim only, matched on SentAt.
	Release(ctx context.Context, claim domain.NoticeEmailClaim) error
}

const (
	// defaultSendTimeout bounds one e-mail, on either path.
	defaultSendTimeout = 30 * time.Second
	// defaultMaxInFlight bounds the background sends. A bulk status change
	// starts up to 100 notices at once (MaxBulkActionItems).
	defaultMaxInFlight = 16
	// drainGrace is how long Drain waits for cancelled sends to return.
	drainGrace = 2 * time.Second
	// releaseTimeout bounds the release of a claim after a failed send, whose
	// own context may already be spent.
	releaseTimeout = 5 * time.Second
)

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

	throttle EmailThrottle
	window   time.Duration

	// The background pool: slots bounds the sends in flight, inflight lets
	// Drain wait for them, and base is cancelled when the drain deadline
	// passes. mu orders inflight.Add against Drain.
	mu       sync.Mutex
	draining bool
	slots    chan struct{}
	inflight sync.WaitGroup
	base     context.Context
	stop     context.CancelFunc
}

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

// WithThrottle holds back a notice e-mail when one already went out for the
// same recipient, type and resource within window. The in-app record is not
// throttled. A window of zero, or a nil throttle, turns it off.
func (d *Dispatcher) WithThrottle(t EmailThrottle, window time.Duration) *Dispatcher {
	d.throttle = t
	d.window = window
	return d
}

// InBackground makes the e-mail half asynchronous. report receives every
// failure, already stripped of the address; it may be nil. The in-app record
// stays synchronous, so the bell is up to date when the request returns.
//
// At most maxInFlight e-mails are sent at once (a default when it is not
// positive). When the pool is full the e-mail is dropped and reported as
// ErrEmailDropped: the caller never waits for a slot.
func (d *Dispatcher) InBackground(report func(error), maxInFlight int) *Dispatcher {
	if report == nil {
		report = func(error) {}
	}
	if maxInFlight <= 0 {
		maxInFlight = defaultMaxInFlight
	}
	d.background = report
	d.slots = make(chan struct{}, maxInFlight)
	d.base, d.stop = context.WithCancel(context.Background())
	return d
}

// Drain stops new background sends and waits for the ones in flight until ctx
// is done. Past that deadline it cancels them, waits briefly for them to
// return, and reports ctx's error. Call it once the HTTP server has stopped.
func (d *Dispatcher) Drain(ctx context.Context) error {
	if d == nil || d.background == nil {
		return nil
	}
	d.mu.Lock()
	d.draining = true
	d.mu.Unlock()

	done := make(chan struct{})
	go func() {
		d.inflight.Wait()
		close(done)
	}()
	defer d.stop()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
	}
	d.stop()
	select {
	case <-done:
	case <-time.After(drainGrace):
	}
	return ctx.Err()
}

// acquire takes a pool slot without waiting. false means the e-mail is dropped.
func (d *Dispatcher) acquire() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.draining {
		return false
	}
	select {
	case d.slots <- struct{}{}:
		d.inflight.Add(1)
		return true
	default:
		return false
	}
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
// ErrSuppressed and ErrThrottled are not failures and are never returned.
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
		// warn takes what goes wrong around the send without stopping it: the
		// throttle store being down. In the background it is reported; inline
		// it is returned with the rest.
		warn := d.background
		if warn == nil {
			warn = func(err error) { errs = append(errs, err) }
		}
		send := func(ctx context.Context) error {
			ctx, cancel := context.WithTimeout(ctx, d.timeout)
			defer cancel()
			claim, err := d.claim(ctx, n, warn)
			if err != nil {
				return err
			}
			mail := msg
			if claim != nil {
				mail = withThrottleNote(msg, locale, d.window)
			}
			if err := d.mailer.SendNotice(ctx, rcpt.Email, rcpt.FullName, locale, mail, link); err != nil {
				d.release(ctx, claim, warn)
				return redactedEmailError(err, rcpt.Email)
			}
			return nil
		}
		if d.background != nil {
			report := d.background
			if !d.acquire() {
				report(ErrEmailDropped)
				return errors.Join(errs...)
			}
			go func() {
				defer d.inflight.Done()
				defer func() { <-d.slots }()
				// A panic in a template must not take the process down.
				defer func() {
					if r := recover(); r != nil {
						report(fmt.Errorf("%w: panic while sending", ErrEmailFailed))
					}
				}()
				if err := send(d.base); err != nil && !errors.Is(err, ErrThrottled) {
					report(err)
				}
			}()
		} else if err := send(ctx); err != nil && !errors.Is(err, ErrThrottled) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// claim takes the throttle slot for n. It returns nil, nil when the notice is
// not throttled at all: no throttle, no window, a producer that deduplicates,
// or no resource to key on. When the throttle store fails, the e-mail goes out
// unthrottled and the failure is reported.
func (d *Dispatcher) claim(ctx context.Context, n domain.Notice, warn func(error)) (*domain.NoticeEmailClaim, error) {
	if d.throttle == nil || d.window <= 0 || n.Deduplicated || n.ResourceID == nil || *n.ResourceID == uuid.Nil {
		return nil, nil
	}
	key := domain.NoticeEmailClaim{
		TenantID: n.TenantID, UserID: n.UserID, Type: n.Type,
		ResourceType: n.ResourceType, ResourceID: *n.ResourceID,
	}
	claim, err := d.throttle.Claim(ctx, key, d.window)
	if err != nil {
		warn(fmt.Errorf("notice e-mail throttle unavailable, sending unthrottled: %w", err))
		return nil, nil
	}
	if claim == nil {
		return nil, ErrThrottled
	}
	return claim, nil
}

// release gives the claim back after a failed send, on a context of its own:
// the send's may be the one that just expired.
func (d *Dispatcher) release(ctx context.Context, claim *domain.NoticeEmailClaim, warn func(error)) {
	if claim == nil {
		return
	}
	rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), releaseTimeout)
	defer cancel()
	if err := d.throttle.Release(rctx, *claim); err != nil {
		warn(fmt.Errorf("notice e-mail throttle: claim not released: %w", err))
	}
}

// withThrottleNote tells the reader that later changes to the same item will
// not be e-mailed for a while. Only the first e-mail in a window goes out, so
// it must not pass for the current state (#997).
func withThrottleNote(msg domain.NoticeMessage, locale string, window time.Duration) domain.NoticeMessage {
	span := throttleSpan(window, locale)
	note := fmt.Sprintf("Les autres changements sur cet élément dans %s ne seront pas envoyés par e-mail. L'application affiche toujours l'état à jour.", span)
	if locale == "en" {
		note = fmt.Sprintf("Further changes to this item within %s will not be e-mailed. The app always shows the current state.", span)
	}
	out := msg
	out.Lines = append(append([]string{}, msg.Lines...), note)
	return out
}

func throttleSpan(window time.Duration, locale string) string {
	if window >= 2*time.Hour && window%time.Hour == 0 {
		if locale == "en" {
			return fmt.Sprintf("the next %d hours", int(window.Hours()))
		}
		return fmt.Sprintf("les %d prochaines heures", int(window.Hours()))
	}
	minutes := int(window.Round(time.Minute).Minutes())
	if minutes < 1 {
		minutes = 1
	}
	if locale == "en" {
		return fmt.Sprintf("the next %d minutes", minutes)
	}
	return fmt.Sprintf("les %d prochaines minutes", minutes)
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
