// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package email is the mail transport seam. See smtp.go for the two
// implementations and for why there is no third one that quietly succeeds.
package email

import (
	"context"
	"log"
	"strings"
)

// Service defines the email transport interface.
type Service interface {
	SendEmail(ctx context.Context, to, subject, body string) error
}

// LogService writes the message to the log and reports success. It exists for
// local development where seeing the mail is the point, and it is opt-in
// through EMAIL_TRANSPORT=log — never a silent default, because a transport
// that reports success without delivering is how "the invitation was sent"
// becomes a lie the product tells its administrator.
type LogService struct{}

// NewLogService builds the development transport.
func NewLogService() *LogService { return &LogService{} }

// SendEmail logs the recipient's domain and the subject. The body is
// deliberately not logged: these messages carry invitation links and reset
// tokens (RULE #6). Nor is the full address: every work notice goes through
// here when the log transport is on (#757), and a log is not a mailbox.
func (s *LogService) SendEmail(_ context.Context, to, subject, _ string) error {
	log.Printf("email[log-transport]: to=%s subject=%q (body withheld — it may carry a credential)", MaskAddress(to), subject)
	return nil
}

// MaskAddress keeps only the domain of an address: "a@b.example" → "***@b.example".
func MaskAddress(to string) string {
	if i := strings.LastIndex(to, "@"); i >= 0 {
		return "***" + to[i:]
	}
	return "***"
}

// Delivery modes reported by Mode.
const (
	ModeSMTP     = "smtp"     // mail leaves the building
	ModeLog      = "log"      // mail is logged, not delivered (development)
	ModeDisabled = "disabled" // no transport: in-app notifications only
)

// Mode says what a transport does with a message, so the deployment can say
// it out loud (/health) instead of failing quietly on every send (#757).
func Mode(s Service) string {
	switch s.(type) {
	case *Transport:
		return ModeSMTP
	case *LogService:
		return ModeLog
	default:
		return ModeDisabled
	}
}

// Delivers reports whether a message handed to the transport can reach a
// mailbox, or at least the log a developer is watching.
func Delivers(s Service) bool { return Mode(s) != ModeDisabled }
