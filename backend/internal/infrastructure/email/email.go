// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package email

import (
	"context"
	"fmt"
)

// Service defines the email service interface
type Service interface {
	SendEmail(ctx context.Context, to, subject, body string) error
}

// SMTPService implements Service using SMTP
type SMTPService struct {
	host     string
	port     int
	username string
	password string
	from     string
}

// NewSMTPService creates a new SMTP email service
func NewSMTPService(host string, port int, username, password, from string) *SMTPService {
	return &SMTPService{
		host:     host,
		port:     port,
		username: username,
		password: password,
		from:     from,
	}
}

// SendEmail sends an email (placeholder implementation)
func (s *SMTPService) SendEmail(ctx context.Context, to, subject, body string) error {
	// TODO: Implement actual SMTP sending
	fmt.Printf("Sending email to %s: %s\n", to, subject)
	return nil
}

// MockService implements Service for testing
type MockService struct{}

// NewMockService creates a new mock email service
func NewMockService() *MockService {
	return &MockService{}
}

// SendEmail mocks sending an email
func (s *MockService) SendEmail(ctx context.Context, to, subject, body string) error {
	fmt.Printf("MOCK: Sending email to %s: %s\n", to, subject)
	return nil
}