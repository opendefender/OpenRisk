package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	notificationapp "github.com/opendefender/openrisk/internal/application/notification"
	"github.com/opendefender/openrisk/internal/domain"
)

func safeGetUUID(c *fiber.Ctx, key string) uuid.UUID {
	val := c.Locals(key)
	if val == nil {
		return uuid.Nil
	}
	if u, ok := val.(uuid.UUID); ok {
		return u
	}
	if s, ok := val.(string); ok {
		parsed, err := uuid.Parse(s)
		if err == nil {
			return parsed
		}
	}
	return uuid.Nil
}

// NotificationHandler handles notification-related requests
type NotificationHandler struct {
	useCase *notificationapp.UseCase
}

// NewNotificationHandler creates a new notification handler
func NewNotificationHandler(useCase *notificationapp.UseCase) *NotificationHandler {
	return &NotificationHandler{
		useCase: useCase,
	}
}

// GetNotifications retrieves user's notifications
// GET /api/v1/notifications
func (h *NotificationHandler) GetNotifications(c *fiber.Ctx) error {
	userID := safeGetUUID(c, "user_id")
	tenantID := safeGetUUID(c, "tenant_id")

	limit := c.QueryInt("limit", 50)
	offset := c.QueryInt("offset", 0)

	if limit > 100 {
		limit = 100 // Max 100 per request
	}
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid offset"})
	}

	notifications, err := h.useCase.GetNotifications(userID, tenantID, limit, offset)
	if err != nil {
		if err == notificationapp.ErrUnauthorized {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to retrieve notifications",
		})
	}

	return c.JSON(fiber.Map{
		"data":   notifications,
		"limit":  limit,
		"offset": offset,
		"total":  len(notifications),
	})
}

// GetUnreadCount retrieves count of unread notifications
// GET /api/v1/notifications/unread-count
func (h *NotificationHandler) GetUnreadCount(c *fiber.Ctx) error {
	userID := safeGetUUID(c, "user_id")
	tenantID := safeGetUUID(c, "tenant_id")

	count, err := h.useCase.GetUnreadCount(userID, tenantID)
	if err != nil {
		if err == notificationapp.ErrUnauthorized {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to retrieve unread count",
		})
	}

	return c.JSON(fiber.Map{
		"unread_count": count,
	})
}

// MarkAsRead marks a notification as read
// PATCH /api/v1/notifications/:notificationId/read
func (h *NotificationHandler) MarkAsRead(c *fiber.Ctx) error {
	userID := safeGetUUID(c, "user_id")
	notificationID, err := uuid.Parse(c.Params("notificationId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid notification ID",
		})
	}

	tenantID := safeGetUUID(c, "tenant_id")
	if err := h.useCase.MarkAsRead(notificationID, userID, tenantID); err != nil {
		if err == notificationapp.ErrValidation {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to mark notification as read",
		})
	}

	return c.JSON(fiber.Map{
		"message": "notification marked as read",
	})
}

// MarkAllAsRead marks all notifications as read
// PATCH /api/v1/notifications/read-all
func (h *NotificationHandler) MarkAllAsRead(c *fiber.Ctx) error {
	userID := safeGetUUID(c, "user_id")
	tenantID := safeGetUUID(c, "tenant_id")

	if err := h.useCase.MarkAllAsRead(userID, tenantID); err != nil {
		if err == notificationapp.ErrUnauthorized {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to mark notifications as read",
		})
	}

	return c.JSON(fiber.Map{
		"message": "all notifications marked as read",
	})
}

// DeleteNotification deletes a notification
// DELETE /api/v1/notifications/:notificationId
func (h *NotificationHandler) DeleteNotification(c *fiber.Ctx) error {
	userID := safeGetUUID(c, "user_id")
	notificationID, err := uuid.Parse(c.Params("notificationId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid notification ID",
		})
	}

	tenantID := safeGetUUID(c, "tenant_id")
	if err := h.useCase.DeleteNotification(notificationID, userID, tenantID); err != nil {
		if err == notificationapp.ErrValidation {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to delete notification",
		})
	}

	return c.JSON(fiber.Map{
		"message": "notification deleted",
	})
}

// GetNotificationPreferences retrieves user's notification preferences
// GET /api/v1/notifications/preferences
func (h *NotificationHandler) GetNotificationPreferences(c *fiber.Ctx) error {
	userID := safeGetUUID(c, "user_id")
	tenantID := safeGetUUID(c, "tenant_id")

	prefs, err := h.useCase.GetPreferences(userID, tenantID)
	if err != nil {
		if err == notificationapp.ErrUnauthorized {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to retrieve preferences",
		})
	}

	return c.JSON(prefs)
}

// UpdateNotificationPreferences updates user's notification preferences
// PATCH /api/v1/notifications/preferences
func (h *NotificationHandler) UpdateNotificationPreferences(c *fiber.Ctx) error {
	userID := safeGetUUID(c, "user_id")
	tenantID := safeGetUUID(c, "tenant_id")

	type UpdateRequest struct {
		EmailOnMitigationDeadline   *bool `json:"email_on_mitigation_deadline"`
		EmailOnCriticalRisk         *bool `json:"email_on_critical_risk"`
		EmailOnActionAssigned       *bool `json:"email_on_action_assigned"`
		EmailDeadlineAdvanceDays    *int  `json:"email_deadline_advance_days"`
		SlackEnabled                *bool `json:"slack_enabled"`
		SlackOnMitigationDeadline   *bool `json:"slack_on_mitigation_deadline"`
		SlackOnCriticalRisk         *bool `json:"slack_on_critical_risk"`
		SlackOnActionAssigned       *bool `json:"slack_on_action_assigned"`
		WebhookEnabled              *bool `json:"webhook_enabled"`
		WebhookOnMitigationDeadline *bool `json:"webhook_on_mitigation_deadline"`
		WebhookOnCriticalRisk       *bool `json:"webhook_on_critical_risk"`
		WebhookOnActionAssigned     *bool `json:"webhook_on_action_assigned"`
		DisableAllNotifications     *bool `json:"disable_all_notifications"`
		EnableSoundNotifications    *bool `json:"enable_sound_notifications"`
		EnableDesktopNotifications  *bool `json:"enable_desktop_notifications"`
	}

	var req UpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	// Build updates map
	updates := make(map[string]interface{})

	if req.EmailOnMitigationDeadline != nil {
		updates["email_on_mitigation_deadline"] = *req.EmailOnMitigationDeadline
	}
	if req.EmailOnCriticalRisk != nil {
		updates["email_on_critical_risk"] = *req.EmailOnCriticalRisk
	}
	if req.EmailOnActionAssigned != nil {
		updates["email_on_action_assigned"] = *req.EmailOnActionAssigned
	}
	if req.EmailDeadlineAdvanceDays != nil {
		updates["email_deadline_advance_days"] = *req.EmailDeadlineAdvanceDays
	}
	if req.SlackEnabled != nil {
		updates["slack_enabled"] = *req.SlackEnabled
	}
	if req.SlackOnMitigationDeadline != nil {
		updates["slack_on_mitigation_deadline"] = *req.SlackOnMitigationDeadline
	}
	if req.SlackOnCriticalRisk != nil {
		updates["slack_on_critical_risk"] = *req.SlackOnCriticalRisk
	}
	if req.SlackOnActionAssigned != nil {
		updates["slack_on_action_assigned"] = *req.SlackOnActionAssigned
	}
	if req.WebhookEnabled != nil {
		updates["webhook_enabled"] = *req.WebhookEnabled
	}
	if req.WebhookOnMitigationDeadline != nil {
		updates["webhook_on_mitigation_deadline"] = *req.WebhookOnMitigationDeadline
	}
	if req.WebhookOnCriticalRisk != nil {
		updates["webhook_on_critical_risk"] = *req.WebhookOnCriticalRisk
	}
	if req.WebhookOnActionAssigned != nil {
		updates["webhook_on_action_assigned"] = *req.WebhookOnActionAssigned
	}
	if req.DisableAllNotifications != nil {
		updates["disable_all_notifications"] = *req.DisableAllNotifications
	}
	if req.EnableSoundNotifications != nil {
		updates["enable_sound_notifications"] = *req.EnableSoundNotifications
	}
	if req.EnableDesktopNotifications != nil {
		updates["enable_desktop_notifications"] = *req.EnableDesktopNotifications
	}

	prefs, err := h.useCase.UpdatePreferences(userID, tenantID, updates)
	if err != nil {
		if err == notificationapp.ErrUnauthorized {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}
		if err == notificationapp.ErrValidation {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid preference payload"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to update preferences",
		})
	}

	return c.JSON(prefs)
}

// TestNotification sends a test notification
// POST /api/v1/notifications/test
func (h *NotificationHandler) TestNotification(c *fiber.Ctx) error {
	userID := safeGetUUID(c, "user_id")
	tenantID := safeGetUUID(c, "tenant_id")

	type TestRequest struct {
		Channel string `json:"channel"` // email, slack, webhook
	}

	var req TestRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request",
		})
	}

	// Create test notification
	testNotification := &domain.Notification{
		ID:          uuid.New(),
		UserID:      userID,
		TenantID:    tenantID,
		Type:        domain.NotificationTypeCriticalRisk,
		Channel:     domain.NotificationChannel(req.Channel),
		Status:      domain.NotificationStatusPending,
		Subject:     "Test Notification from OpenRisk",
		Message:     "This is a test notification to verify your notification channels are working correctly.",
		Description: "You can test each channel separately to ensure they are configured properly.",
		Metadata: map[string]interface{}{
			"test": true,
		},
	}

	// Note: In production, actually send the test notification
	// For now, just return success
	return c.JSON(fiber.Map{
		"message": "test notification would be sent",
		"channel": req.Channel,
	})
}
