package handler

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	notificationapp "github.com/opendefender/openrisk/internal/application/notification"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/stretchr/testify/require"
)

type handlerMockRepo struct {
	notifications []*domain.Notification
	preferences   *domain.NotificationPreference
}

func (m *handlerMockRepo) GetUserNotifications(userID, tenantID uuid.UUID, limit, offset int) ([]*domain.Notification, error) {
	return m.notifications, nil
}
func (m *handlerMockRepo) GetUnreadCount(userID, tenantID uuid.UUID) (int64, error) {
	return int64(len(m.notifications)), nil
}
func (m *handlerMockRepo) MarkNotificationAsRead(notificationID, userID, tenantID uuid.UUID) error {
	return nil
}
func (m *handlerMockRepo) MarkAllNotificationsAsRead(userID, tenantID uuid.UUID) error {
	return nil
}
func (m *handlerMockRepo) DeleteNotification(notificationID, userID, tenantID uuid.UUID) error {
	return nil
}
func (m *handlerMockRepo) GetUserNotificationPreferences(userID, tenantID uuid.UUID) (*domain.NotificationPreference, error) {
	if m.preferences != nil {
		return m.preferences, nil
	}
	return &domain.NotificationPreference{UserID: userID, TenantID: tenantID, EmailDeadlineAdvanceDays: 3}, nil
}
func (m *handlerMockRepo) UpdateNotificationPreferences(userID, tenantID uuid.UUID, updates map[string]interface{}) error {
	return nil
}

func setupNotificationTestApp(repo *handlerMockRepo) *fiber.App {
	app := fiber.New()
	uc := notificationapp.NewUseCase(repo)
	h := NewNotificationHandler(uc)
	userID := uuid.New()
	tenantID := uuid.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", userID)
		c.Locals("tenant_id", tenantID)
		return c.Next()
	})
	app.Get("/notifications", h.GetNotifications)
	app.Get("/notifications/unread-count", h.GetUnreadCount)
	app.Patch("/notifications/preferences", h.UpdateNotificationPreferences)
	return app
}

func TestNotificationHandlerGetNotifications(t *testing.T) {
	app := setupNotificationTestApp(&handlerMockRepo{
		notifications: []*domain.Notification{{ID: uuid.New(), Subject: "n1"}},
	})
	req := httptest.NewRequest("GET", "/notifications?limit=10&offset=0", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestNotificationHandlerGetUnreadCount(t *testing.T) {
	app := setupNotificationTestApp(&handlerMockRepo{
		notifications: []*domain.Notification{{ID: uuid.New()}, {ID: uuid.New()}},
	})
	req := httptest.NewRequest("GET", "/notifications/unread-count", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestNotificationHandlerUpdatePreferencesValidation(t *testing.T) {
	app := setupNotificationTestApp(&handlerMockRepo{})
	req := httptest.NewRequest("PATCH", "/notifications/preferences", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestNotificationHandlerUpdatePreferencesSuccess(t *testing.T) {
	app := setupNotificationTestApp(&handlerMockRepo{})
	req := httptest.NewRequest("PATCH", "/notifications/preferences", strings.NewReader(`{"slack_enabled":true}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}
