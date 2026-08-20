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

// SendEmail logs the recipient and subject. The body is deliberately not
// logged: these messages carry invitation links and reset tokens (RULE #6).
func (s *LogService) SendEmail(_ context.Context, to, subject, _ string) error {
	log.Printf("email[log-transport]: to=%s subject=%q (body withheld — it may carry a credential)", to, subject)
	return nil
}
