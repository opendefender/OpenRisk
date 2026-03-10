package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"backend/internal/core/domain"
)

// SlackProvider implements Slack notifications
type SlackProvider struct {
	webhookURL string
	botName    string
	botIcon    string
}

// NewSlackProvider creates a new Slack provider
func NewSlackProvider(webhookURL string) *SlackProvider {
	return &SlackProvider{
		webhookURL: webhookURL,
		botName:    "OpenRisk",
		botIcon:    "🔒",
	}
}

// SlackMessage represents a Slack message payload
type SlackMessage struct {
	Username    string            `json:"username"`
	IconEmoji   string            `json:"icon_emoji"`
	Text        string            `json:"text"`
	Attachments []SlackAttachment `json:"attachments,omitempty"`
}

// SlackAttachment represents a Slack message attachment
type SlackAttachment struct {
	Color     string       `json:"color"`
	Title     string       `json:"title"`
	TitleLink string       `json:"title_link,omitempty"`
	Text      string       `json:"text"`
	Fields    []SlackField `json:"fields,omitempty"`
	MrkdwnIn  []string     `json:"mrkdwn_in"`
	ImageURL  string       `json:"image_url,omitempty"`
}

// SlackField represents a Slack field in an attachment
type SlackField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

// Send sends a Slack notification
func (sp *SlackProvider) Send(ctx context.Context, notification *domain.Notification) error {
	if sp.webhookURL == "" {
		return fmt.Errorf("slack webhook URL not configured")
	}

	message := sp.buildSlackMessage(notification)

	payload, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal Slack payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", sp.webhookURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create Slack request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send Slack notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack returned status code %d", resp.StatusCode)
	}

	return nil
}

// SendToChannel sends a message to a specific Slack channel
func (sp *SlackProvider) SendToChannel(ctx context.Context, channel, message string) error {
	payload := SlackMessage{
		Username:  sp.botName,
		IconEmoji: sp.botIcon,
		Text:      fmt.Sprintf("<#%s> %s", channel, message),
	}

	return sp.sendPayload(ctx, payload)
}

// SendDirectMessage sends a direct message to a Slack user
func (sp *SlackProvider) SendDirectMessage(ctx context.Context, userID, message string) error {
	payload := SlackMessage{
		Username:  sp.botName,
		IconEmoji: sp.botIcon,
		Text:      fmt.Sprintf("<@%s> %s", userID, message),
	}

	return sp.sendPayload(ctx, payload)
}

// Validate validates Slack provider configuration
func (sp *SlackProvider) Validate(config map[string]interface{}) error {
	if sp.webhookURL == "" {
		return fmt.Errorf("slack webhook URL not configured")
	}
	return nil
}

// buildSlackMessage converts notification to Slack message
func (sp *SlackProvider) buildSlackMessage(notification *domain.Notification) SlackMessage {
	color := "#36a64f" // Green by default

	switch notification.Type {
	case domain.NotificationTypeCriticalRisk:
		color = "#ff0000" // Red for critical
	case domain.NotificationTypeMitigationDeadline:
		color = "#ff9900" // Orange for deadline
	case domain.NotificationTypeActionAssigned:
		color = "#0099ff" // Blue for action
	}

	fields := []SlackField{}

	// Add metadata as fields
	if desc, ok := notification.Metadata["risk_title"].(string); ok {
		fields = append(fields, SlackField{
			Title: "Risk",
			Value: desc,
			Short: true,
		})
	}

	if severity, ok := notification.Metadata["severity"].(string); ok {
		fields = append(fields, SlackField{
			Title: "Severity",
			Value: severity,
			Short: true,
		})
	}

	if daysUntil, ok := notification.Metadata["days_until"].(int); ok {
		fields = append(fields, SlackField{
			Title: "Days Until Due",
			Value: fmt.Sprintf("%d", daysUntil),
			Short: true,
		})
	}

	attachment := SlackAttachment{
		Color:    color,
		Title:    notification.Subject,
		Text:     notification.Message,
		Fields:   fields,
		MrkdwnIn: []string{"text", "pretext"},
	}

	return SlackMessage{
		Username:    sp.botName,
		IconEmoji:   sp.botIcon,
		Text:        notification.Subject,
		Attachments: []SlackAttachment{attachment},
	}
}

// sendPayload sends a payload to Slack webhook
func (sp *SlackProvider) sendPayload(ctx context.Context, message SlackMessage) error {
	payload, err := json.Marshal(message)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", sp.webhookURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack webhook returned status %d", resp.StatusCode)
	}

	return nil
}
