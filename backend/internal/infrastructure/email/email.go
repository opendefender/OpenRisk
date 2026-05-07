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