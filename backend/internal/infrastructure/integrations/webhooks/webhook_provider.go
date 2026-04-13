package providers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/opendefender/openrisk/internal/domain"
)

// WebhookProvider implements webhook notifications
type WebhookProvider struct {
	timeout time.Duration
	retries int
}

// NewWebhookProvider creates a new webhook provider
func NewWebhookProvider() *WebhookProvider {
	return &WebhookProvider{
		timeout: 10 * time.Second,
		retries: 3,
	}
}

// WebhookPayload represents the payload sent to a webhook
type WebhookPayload struct {
	Event          string                 `json:"event"`
	Timestamp      time.Time              `json:"timestamp"`
	NotificationID string                 `json:"notification_id"`
	UserID         string                 `json:"user_id"`
	TenantID       string                 `json:"tenant_id"`
	Type           string                 `json:"type"`
	Subject        string                 `json:"subject"`
	Message        string                 `json:"message"`
	Description    string                 `json:"description"`
	ResourceID     string                 `json:"resource_id,omitempty"`
	ResourceType   string                 `json:"resource_type"`
	Metadata       map[string]interface{} `json:"metadata"`
}

// Send sends a webhook notification
func (wp *WebhookProvider) Send(ctx context.Context, notification *domain.Notification) error {
	// Note: In production, get the webhook URL from notification preferences or user config
	// For now, this is a placeholder
	return fmt.Errorf("webhook URL not configured in notification preferences")
}

// SendWithSignature sends a webhook with HMAC-SHA256 signature
func (wp *WebhookProvider) SendWithSignature(
	ctx context.Context,
	url string,
	secret string,
	payload interface{},
) error {
	if url == "" {
		return fmt.Errorf("webhook URL not provided")
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	// Create HMAC-SHA256 signature
	signature := wp.createSignature(jsonPayload, secret)

	// Create request with context timeout
	ctx, cancel := context.WithTimeout(ctx, wp.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create webhook request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-OpenRisk-Signature", signature)
	req.Header.Set("X-OpenRisk-Event-Timestamp", time.Now().UTC().Format(time.RFC3339))
	req.Header.Set("User-Agent", "OpenRisk-NotificationSystem/1.0")

	// Send with retry logic
	client := &http.Client{}
	for attempt := 0; attempt < wp.retries; attempt++ {
		resp, err := client.Do(req)
		if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
			resp.Body.Close()
			return nil
		}

		if resp != nil {
			resp.Body.Close()
		}

		if attempt < wp.retries-1 {
			// Exponential backoff: 1s, 2s, 4s
			backoff := time.Duration(1<<uint(attempt)) * time.Second
			select {
			case <-time.After(backoff):
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	return fmt.Errorf("webhook delivery failed after %d attempts", wp.retries)
}

// Validate validates webhook provider configuration
func (wp *WebhookProvider) Validate(config map[string]interface{}) error {
	url, ok := config["webhook_url"].(string)
	if !ok || url == "" {
		return fmt.Errorf("webhook URL not provided")
	}

	// Validate URL format
	if len(url) < 10 || (url[:7] != "http://" && url[:8] != "https://") {
		return fmt.Errorf("invalid webhook URL format")
	}

	return nil
}

// createSignature creates HMAC-SHA256 signature
func (wp *WebhookProvider) createSignature(payload []byte, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	return "sha256=" + hex.EncodeToString(h.Sum(nil))
}

// VerifySignature verifies webhook signature (for receiving webhooks)
func VerifySignature(body []byte, signature string, secret string) bool {
	provider := NewWebhookProvider()
	expectedSignature := provider.createSignature(body, secret)
	return hmac.Equal([]byte(signature), []byte(expectedSignature))
}

// SendNotificationWebhook sends a notification as a webhook
func (wp *WebhookProvider) SendNotificationWebhook(
	ctx context.Context,
	url string,
	secret string,
	notification *domain.Notification,
) error {
	payload := WebhookPayload{
		Event:          "notification.sent",
		Timestamp:      time.Now().UTC(),
		NotificationID: notification.ID.String(),
		UserID:         notification.UserID.String(),
		TenantID:       notification.TenantID.String(),
		Type:           string(notification.Type),
		Subject:        notification.Subject,
		Message:        notification.Message,
		Description:    notification.Description,
		ResourceType:   notification.ResourceType,
		Metadata:       notification.Metadata,
	}

	if notification.ResourceID != nil {
		payload.ResourceID = notification.ResourceID.String()
	}

	return wp.SendWithSignature(ctx, url, secret, payload)
}

// SendBulkNotificationWebhook sends multiple notifications to webhook
func (wp *WebhookProvider) SendBulkNotificationWebhook(
	ctx context.Context,
	url string,
	secret string,
	notifications []*domain.Notification,
) error {
	type BulkPayload struct {
		Event         string           `json:"event"`
		Timestamp     time.Time        `json:"timestamp"`
		Count         int              `json:"count"`
		Notifications []WebhookPayload `json:"notifications"`
	}

	webhookPayloads := []WebhookPayload{}
	for _, notif := range notifications {
		payload := WebhookPayload{
			Event:          "notification.sent",
			Timestamp:      time.Now().UTC(),
			NotificationID: notif.ID.String(),
			UserID:         notif.UserID.String(),
			TenantID:       notif.TenantID.String(),
			Type:           string(notif.Type),
			Subject:        notif.Subject,
			Message:        notif.Message,
			Description:    notif.Description,
			ResourceType:   notif.ResourceType,
			Metadata:       notif.Metadata,
		}

		if notif.ResourceID != nil {
			payload.ResourceID = notif.ResourceID.String()
		}

		webhookPayloads = append(webhookPayloads, payload)
	}

	bulkPayload := BulkPayload{
		Event:         "notifications.batch",
		Timestamp:     time.Now().UTC(),
		Count:         len(notifications),
		Notifications: webhookPayloads,
	}

	return wp.SendWithSignature(ctx, url, secret, bulkPayload)
}
