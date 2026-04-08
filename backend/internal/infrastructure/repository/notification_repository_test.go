package repository

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupNotificationRepo(t *testing.T) *NotificationRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`
		CREATE TABLE notifications (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			tenant_id TEXT NOT NULL,
			type TEXT,
			channel TEXT,
			status TEXT,
			subject TEXT,
			message TEXT,
			description TEXT,
			resource_id TEXT,
			resource_type TEXT,
			metadata TEXT,
			sent_at DATETIME,
			delivered_at DATETIME,
			read_at DATETIME,
			failure_reason TEXT,
			created_at DATETIME,
			updated_at DATETIME
		);
	`).Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE notification_preferences (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			tenant_id TEXT NOT NULL,
			email_on_mitigation_deadline BOOLEAN,
			email_on_critical_risk BOOLEAN,
			email_on_action_assigned BOOLEAN,
			email_on_risk_update BOOLEAN,
			email_on_risk_resolved BOOLEAN,
			email_deadline_advance_days INTEGER,
			slack_enabled BOOLEAN,
			slack_channel_override TEXT,
			slack_on_mitigation_deadline BOOLEAN,
			slack_on_critical_risk BOOLEAN,
			slack_on_action_assigned BOOLEAN,
			webhook_enabled BOOLEAN,
			webhook_on_mitigation_deadline BOOLEAN,
			webhook_on_critical_risk BOOLEAN,
			webhook_on_action_assigned BOOLEAN,
			disable_all_notifications BOOLEAN,
			enable_sound_notifications BOOLEAN,
			enable_desktop_notifications BOOLEAN,
			created_at DATETIME,
			updated_at DATETIME
		);
	`).Error)
	return NewNotificationRepository(db)
}

func TestNotificationRepositoryTenantIsolationReadAndDelete(t *testing.T) {
	repo := setupNotificationRepo(t)
	userID := uuid.New()
	tenantAllowed := uuid.New()
	tenantOther := uuid.New()
	allowedID := uuid.New()
	otherID := uuid.New()

	require.NoError(t, repo.db.Table("notifications").Create(map[string]interface{}{
		"id":         allowedID.String(),
		"user_id":    userID.String(),
		"tenant_id":  tenantAllowed.String(),
		"channel":    string(domain.NotificationChannelInApp),
		"status":     string(domain.NotificationStatusPending),
		"subject":    "ok",
		"created_at": time.Now(),
		"updated_at": time.Now(),
	}).Error)
	require.NoError(t, repo.db.Table("notifications").Create(map[string]interface{}{
		"id":         otherID.String(),
		"user_id":    userID.String(),
		"tenant_id":  tenantOther.String(),
		"channel":    string(domain.NotificationChannelInApp),
		"status":     string(domain.NotificationStatusPending),
		"subject":    "other",
		"created_at": time.Now(),
		"updated_at": time.Now(),
	}).Error)

	items, err := repo.GetUserNotifications(userID, tenantAllowed, 10, 0)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, allowedID, items[0].ID)

	require.NoError(t, repo.DeleteNotification(otherID, userID, tenantAllowed))
	var stillThere domain.Notification
	err = repo.db.First(&stillThere, "id = ?", otherID).Error
	require.NoError(t, err)
}

func TestNotificationRepositoryMarkReadScopedByTenant(t *testing.T) {
	repo := setupNotificationRepo(t)
	userID := uuid.New()
	tenantA := uuid.New()
	tenantB := uuid.New()
	idA := uuid.New()
	idB := uuid.New()

	require.NoError(t, repo.db.Table("notifications").Create(map[string]interface{}{
		"id":         idA.String(),
		"user_id":    userID.String(),
		"tenant_id":  tenantA.String(),
		"channel":    string(domain.NotificationChannelInApp),
		"status":     string(domain.NotificationStatusPending),
		"subject":    "a",
		"created_at": time.Now(),
		"updated_at": time.Now(),
	}).Error)
	require.NoError(t, repo.db.Table("notifications").Create(map[string]interface{}{
		"id":         idB.String(),
		"user_id":    userID.String(),
		"tenant_id":  tenantB.String(),
		"channel":    string(domain.NotificationChannelInApp),
		"status":     string(domain.NotificationStatusPending),
		"subject":    "b",
		"created_at": time.Now(),
		"updated_at": time.Now(),
	}).Error)

	require.NoError(t, repo.MarkNotificationAsRead(idB, userID, tenantA))

	var targetA, targetB domain.Notification
	require.NoError(t, repo.db.First(&targetA, "id = ?", idA).Error)
	require.NoError(t, repo.db.First(&targetB, "id = ?", idB).Error)
	require.Equal(t, domain.NotificationStatusPending, targetB.Status)
	require.Equal(t, domain.NotificationStatusPending, targetA.Status)
}

func TestNotificationRepositoryPreferencesCreateAndUpdate(t *testing.T) {
	repo := setupNotificationRepo(t)
	userID := uuid.New()
	tenantID := uuid.New()

	prefs, err := repo.GetUserNotificationPreferences(userID, tenantID)
	require.NoError(t, err)
	require.Equal(t, 3, prefs.EmailDeadlineAdvanceDays)

	err = repo.UpdateNotificationPreferences(userID, tenantID, map[string]interface{}{
		"slack_enabled":               true,
		"email_deadline_advance_days": 5,
	})
	require.NoError(t, err)

	updated, err := repo.GetUserNotificationPreferences(userID, tenantID)
	require.NoError(t, err)
	require.True(t, updated.SlackEnabled)
	require.Equal(t, 5, updated.EmailDeadlineAdvanceDays)
	require.WithinDuration(t, time.Now(), updated.UpdatedAt, 2*time.Second)
}
