package notify

import (
	"context"
	"fmt"
	"html/template"
	"strings"

	"github.com/opendefender/openrisk/internal/infrastructure/email"
)

// Service defines the notification service interface
type Service interface {
	SendWelcomeEmail(ctx context.Context, email, fullName string) error
}

// EmailService implements Service using email
type EmailService struct {
	emailService email.Service
	fromEmail    string
	baseURL      string
}

// NewEmailService creates a new email notification service
func NewEmailService(emailService email.Service, fromEmail, baseURL string) *EmailService {
	return &EmailService{
		emailService: emailService,
		fromEmail:    fromEmail,
		baseURL:      baseURL,
	}
}

// SendWelcomeEmail sends a welcome email to new users
func (s *EmailService) SendWelcomeEmail(ctx context.Context, userEmail, fullName string) error {
	subject := "Welcome to OpenRisk!"

	// Load email template
	templateData := struct {
		FullName string
		BaseURL  string
	}{
		FullName: fullName,
		BaseURL:  s.baseURL,
	}

	htmlBody, err := s.renderTemplate("welcome_email.html", templateData)
	if err != nil {
		// Fallback to plain text if template fails
		htmlBody = s.getPlainWelcomeEmail(fullName)
	}

	// Send email using the service interface
	return s.emailService.SendEmail(ctx, userEmail, subject, htmlBody)
}

func (s *EmailService) renderTemplate(templateName string, data interface{}) (string, error) {
	// In production, templates would be loaded from files
	// For now, we'll use embedded templates
	tmpl, err := template.New(templateName).Parse(s.getWelcomeEmailTemplate())
	if err != nil {
		return "", err
	}

	var result strings.Builder
	if err := tmpl.Execute(&result, data); err != nil {
		return "", err
	}

	return result.String(), nil
}

func (s *EmailService) getWelcomeEmailTemplate() string {
	return `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Welcome to OpenRisk</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background-color: #4F46E5; color: white; padding: 20px; text-align: center; }
        .content { padding: 20px; background-color: #f9f9f9; }
        .button { display: inline-block; background-color: #4F46E5; color: white; padding: 10px 20px; text-decoration: none; border-radius: 5px; }
        .footer { text-align: center; padding: 20px; font-size: 12px; color: #666; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Welcome to OpenRisk!</h1>
        </div>
        <div class="content">
            <p>Hi {{.FullName}},</p>
            <p>Welcome to OpenRisk! We're excited to have you join our community.</p>
            <p>You can now:</p>
            <ul>
                <li>Manage your organization's risk assessments</li>
                <li>Track compliance across frameworks</li>
                <li>Generate executive reports</li>
                <li>Collaborate with your team</li>
            </ul>
            <p>Get started by logging into your account:</p>
            <p style="text-align: center;">
                <a href="{{.BaseURL}}/login" class="button">Login to OpenRisk</a>
            </p>
            <p>If you have any questions, feel free to reach out to our support team.</p>
            <p>Best regards,<br>The OpenRisk Team</p>
        </div>
        <div class="footer">
            <p>This email was sent to you because you registered for an OpenRisk account.</p>
            <p>If you didn't register, please ignore this email.</p>
        </div>
    </div>
</body>
</html>
`
}

func (s *EmailService) getPlainWelcomeEmail(fullName string) string {
	return fmt.Sprintf(`Hi %s,

Welcome to OpenRisk! We're excited to have you join our community.

You can now:
- Manage your organization's risk assessments
- Track compliance across frameworks
- Generate executive reports
- Collaborate with your team

Get started by logging into your account at: %s/login

If you have any questions, feel free to reach out to our support team.

Best regards,
The OpenRisk Team

---
This email was sent to you because you registered for an OpenRisk account.
If you didn't register, please ignore this email.`, fullName, s.baseURL)
}
