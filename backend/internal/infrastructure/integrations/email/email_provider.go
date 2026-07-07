// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package providers

import (
	"context"
	"fmt"

	"github.com/opendefender/openrisk/internal/domain"
)

// EmailProvider implements email notifications
type EmailProvider struct {
	smtpHost       string
	smtpPort       int
	senderEmail    string
	senderName     string
	senderPassword string
}

// NewEmailProvider creates a new email provider
func NewEmailProvider(host string, port int, email, name, password string) *EmailProvider {
	return &EmailProvider{
		smtpHost:       host,
		smtpPort:       port,
		senderEmail:    email,
		senderName:     name,
		senderPassword: password,
	}
}

// Send sends an email notification
func (ep *EmailProvider) Send(ctx context.Context, notification *domain.Notification) error {
	// This is a placeholder implementation
	// In production, use a proper email library like sendgrid, mailgun, or aws-ses

	if notification.UserID == (notification.UserID) { // dummy check
		return fmt.Errorf("email provider not fully implemented - use SendGrid or similar service")
	}

	return nil
}

// SendBulk sends emails to multiple recipients
func (ep *EmailProvider) SendBulk(ctx context.Context, emails []string, subject, body string) error {
	// Placeholder
	return fmt.Errorf("email provider not fully implemented")
}

// Validate validates email provider configuration
func (ep *EmailProvider) Validate(config map[string]interface{}) error {
	if ep.smtpHost == "" {
		return fmt.Errorf("SMTP host not configured")
	}
	if ep.senderEmail == "" {
		return fmt.Errorf("sender email not configured")
	}
	return nil
}

