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

// buildEmailBody builds HTML email body from notification
func (ep *EmailProvider) buildEmailBody(notification *domain.Notification) string {
	html := `
	<html>
	<head>
		<style>
			body { font-family: Arial, sans-serif; }
			.container { max-width: 600px; margin: 0 auto; padding: 20px; }
			.header { background-color: #f5f5f5; padding: 20px; border-radius: 5px; }
			.content { padding: 20px 0; }
			.footer { font-size: 12px; color: #999; margin-top: 20px; }
			.button { background-color: #007bff; color: white; padding: 10px 20px; text-decoration: none; border-radius: 3px; }
		</style>
	</head>
	<body>
		<div class="container">
			<div class="header">
				<h2>%s</h2>
			</div>
			<div class="content">
				<p>%s</p>
				<p>%s</p>
			</div>
			<div class="footer">
				<p>This is an automated notification from OpenRisk.</p>
			</div>
		</div>
	</body>
	</html>
	`

	return fmt.Sprintf(html, notification.Subject, notification.Message, notification.Description)
}
