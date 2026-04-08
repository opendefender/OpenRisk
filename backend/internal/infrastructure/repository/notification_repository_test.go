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
	require.NoError(t, db.AutoMigrate(&domain.Notification{}, &domain.NotificationPreference{}))
	return NewNotificationRepository(db)
}

func TestNotificationRepositoryTenantIsolationReadAndDelete(t *testing.T) {
	repo := setupNotificationRepo(t)
	userID := uuid.New()
	tenantAllowed := uuid.New()
	tenantOther := uuid.New()
	allowedID := uuid.New()
	otherID := uuid.New()

	require.NoError(t, repo.db.Create(&domain.Notification{
		ID:       allowedID,
		UserID:   userID,
		TenantID: tenantAllowed,
		Channel:  domain.NotificationChannelInApp,
		Status:   domain.NotificationStatusPending,
		Subject:  "ok",
	}).Error)
	require.NoError(t, repo.db.Create(&domain.Notification{
		ID:       otherID,
		UserID:   userID,
		TenantID: tenantOther,
		Channel:  domain.NotificationChannelInApp,
		Status:   domain.NotificationStatusPending,
		Subject:  "other",
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

	for _, n := range []domain.Notification{
		{ID: idA, UserID: userID, TenantID: tenantA, Channel: domain.NotificationChannelInApp, Status: domain.NotificationStatusPending, Subject: "a"},
		{ID: idB, UserID: userID, TenantID: tenantB, Channel: domain.NotificationChannelInApp, Status: domain.NotificationStatusPending, Subject: "b"},
	} {
		require.NoError(t, repo.db.Create(&n).Error)
	}

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
