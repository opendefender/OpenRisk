// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package authmail

import (
	"context"
	"time"
)

// Async dispatches account-security email off the request goroutine.
//
// This is a security control, not a performance tweak. The password-reset
// endpoint answers identically whether or not the address has an account — but
// only the "account exists" branch sends mail, and an SMTP round trip is tens to
// hundreds of milliseconds. Left inline, that timing difference reinstates
// exactly the account enumeration the uniform response was written to prevent.
// Handing the send to a goroutine makes both branches cost the same.
//
// Delivery is best-effort by design: nothing downstream waits on it, and a mail
// failure must never surface to the caller, since the error itself would leak
// which branch ran.
type Async struct {
	inner   ResetMailerLike
	timeout time.Duration
}

// ResetMailerLike is what Async decorates (satisfied by *Mailer).
type ResetMailerLike interface {
	SendResetLink(ctx context.Context, to, fullName, link, locale string) error
	SendResetConfirmation(ctx context.Context, to, fullName, locale string) error
	SendNewSignInAlert(ctx context.Context, to, fullName, ip, userAgent string, when time.Time, locale string) error
}

// NewAsync wraps a mailer so every send returns immediately.
func NewAsync(inner ResetMailerLike) *Async {
	return &Async{inner: inner, timeout: 30 * time.Second}
}

// SendResetLink queues the reset link.
func (a *Async) SendResetLink(_ context.Context, to, fullName, link, locale string) error {
	a.dispatch(func(ctx context.Context) { _ = a.inner.SendResetLink(ctx, to, fullName, link, locale) })
	return nil
}

// SendResetConfirmation queues the confirmation notice.
func (a *Async) SendResetConfirmation(_ context.Context, to, fullName, locale string) error {
	a.dispatch(func(ctx context.Context) { _ = a.inner.SendResetConfirmation(ctx, to, fullName, locale) })
	return nil
}

// SendNewSignInAlert queues the new-device notice.
func (a *Async) SendNewSignInAlert(_ context.Context, to, fullName, ip, userAgent string, when time.Time, locale string) error {
	a.dispatch(func(ctx context.Context) {
		_ = a.inner.SendNewSignInAlert(ctx, to, fullName, ip, userAgent, when, locale)
	})
	return nil
}

// dispatch runs fn on its own goroutine with a fresh, bounded context.
//
// A fresh context rather than the request's: the handler returns immediately, so
// the request context is cancelled while the send is still in flight and every
// message would be aborted. The timeout keeps a wedged SMTP server from leaking
// goroutines.
func (a *Async) dispatch(fn func(context.Context)) {
	if a.inner == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), a.timeout)
		defer cancel()
		// A panic in a mail template must not take the process down.
		defer func() { _ = recover() }()
		fn(ctx)
	}()
}
