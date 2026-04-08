package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
)

// NotificationService handles all notification operations
type NotificationService struct {
	db                   *gorm.DB
	emailProvider        EmailProvider
	slackProvider        SlackProvider
	webhookProvider      WebhookProvider
	mu                   sync.RWMutex
	notificationChannels map[domain.NotificationChannel]NotificationProvider
	templateCache        map[string]*domain.NotificationTemplate
}

// NotificationProvider defines interface for sending notifications
type NotificationProvider interface {
	Send(ctx context.Context, notification *domain.Notification) error
	Validate(config map[string]interface{}) error
}

// EmailProvider handles email notifications
type EmailProvider interface {
	NotificationProvider
	SendBulk(ctx context.Context, emails []string, subject, body string) error
}

// SlackProvider handles Slack notifications
type SlackProvider interface {
	NotificationProvider
	SendToChannel(ctx context.Context, channel, message string) error
	SendDirectMessage(ctx context.Context, userID, message string) error
}

// WebhookProvider handles webhook notifications
type WebhookProvider interface {
	NotificationProvider
	SendWithSignature(ctx context.Context, url, secret string, payload interface{}) error
}

// NewNotificationService creates a new notification service
func NewNotificationService(
	db *gorm.DB,
	emailProvider EmailProvider,
	slackProvider SlackProvider,
	webhookProvider WebhookProvider,
) *NotificationService {
	return &NotificationService{
		db:                   db,
		emailProvider:        emailProvider,
		slackProvider:        slackProvider,
		webhookProvider:      webhookProvider,
		notificationChannels: make(map[domain.NotificationChannel]NotificationProvider),
		templateCache:        make(map[string]*domain.NotificationTemplate),
	}
}

// SendMitigationDeadlineNotification sends notification for approaching mitigation deadline
func (ns *NotificationService) SendMitigationDeadlineNotification(
	ctx context.Context,
	userID uuid.UUID,
	tenantID uuid.UUID,
	payload *domain.MitigationDeadlineNotificationPayload,
) error {
	// Get user preferences
	prefs, err := ns.GetUserNotificationPreferences(userID, tenantID)
	if err != nil || prefs == nil {
		return fmt.Errorf("failed to get notification preferences: %w", err)
	}

	// Check if user has disabled all notifications
	if prefs.DisableAllNotifications {
		return nil
	}

	notification := &domain.Notification{
		ID:           uuid.New(),
		UserID:       userID,
		TenantID:     tenantID,
		Type:         domain.NotificationTypeMitigationDeadline,
		Status:       domain.NotificationStatusPending,
		ResourceID:   &payload.MitigationID,
		ResourceType: "mitigation",
		Subject:      fmt.Sprintf("Mitigation Deadline Approaching: %s", payload.MitigationTitle),
		Message: fmt.Sprintf("The mitigation '%s' for risk '%s' is due in %d days",
			payload.MitigationTitle, payload.RiskTitle, payload.DaysUntilDue),
		Description: fmt.Sprintf("Due date: %s\nAssigned to: %s", payload.DueDate.Format("2006-01-02"), payload.AssignedTo),
		Metadata: map[string]interface{}{
			"risk_title":       payload.RiskTitle,
			"mitigation_title": payload.MitigationTitle,
			"due_date":         payload.DueDate,
			"days_until":       payload.DaysUntilDue,
			"assigned_to":      payload.AssignedTo,
			"risk_link":        payload.RiskLink,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Send through enabled channels
	if prefs.EmailOnMitigationDeadline {
		notification.Channel = domain.NotificationChannelEmail
		if err := ns.sendNotification(ctx, notification, prefs); err != nil {
			ns.logNotificationFailure(ctx, notification, err)
		}
	}

	if prefs.SlackEnabled && prefs.SlackOnMitigationDeadline {
		notification.Channel = domain.NotificationChannelSlack
		if err := ns.sendNotification(ctx, notification, prefs); err != nil {
			ns.logNotificationFailure(ctx, notification, err)
		}
	}

	if prefs.WebhookEnabled && prefs.WebhookOnMitigationDeadline {
		notification.Channel = domain.NotificationChannelWebhook
		if err := ns.sendNotification(ctx, notification, prefs); err != nil {
			ns.logNotificationFailure(ctx, notification, err)
		}
	}

	// Always save in-app notification
	notification.Channel = domain.NotificationChannelInApp
	return ns.db.Create(notification).Error
}

// SendCriticalRiskNotification sends notification for critical risk
func (ns *NotificationService) SendCriticalRiskNotification(
	ctx context.Context,
	userID uuid.UUID,
	tenantID uuid.UUID,
	payload *domain.CriticalRiskNotificationPayload,
) error {
	prefs, err := ns.GetUserNotificationPreferences(userID, tenantID)
	if err != nil || prefs == nil {
		return fmt.Errorf("failed to get notification preferences: %w", err)
	}

	if prefs.DisableAllNotifications {
		return nil
	}

	notification := &domain.Notification{
		ID:           uuid.New(),
		UserID:       userID,
		TenantID:     tenantID,
		Type:         domain.NotificationTypeCriticalRisk,
		Status:       domain.NotificationStatusPending,
		ResourceID:   &payload.RiskID,
		ResourceType: "risk",
		Subject:      fmt.Sprintf("🚨 CRITICAL RISK DETECTED: %s", payload.RiskTitle),
		Message:      fmt.Sprintf("A critical risk '%s' has been created", payload.RiskTitle),
		Description: fmt.Sprintf("Severity: %s\nImpact: %s\nProbability: %s\n\n%s",
			payload.Severity, payload.Impact, payload.Probability, payload.Description),
		Metadata: map[string]interface{}{
			"risk_title":          payload.RiskTitle,
			"severity":            payload.Severity,
			"impact":              payload.Impact,
			"probability":         payload.Probability,
			"created_by":          payload.CreatedBy,
			"recommended_actions": payload.RecommendedActions,
			"risk_link":           payload.RiskLink,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Send through enabled channels
	if prefs.EmailOnCriticalRisk {
		notification.Channel = domain.NotificationChannelEmail
		if err := ns.sendNotification(ctx, notification, prefs); err != nil {
			ns.logNotificationFailure(ctx, notification, err)
		}
	}

	if prefs.SlackEnabled && prefs.SlackOnCriticalRisk {
		notification.Channel = domain.NotificationChannelSlack
		if err := ns.sendNotification(ctx, notification, prefs); err != nil {
			ns.logNotificationFailure(ctx, notification, err)
		}
	}

	if prefs.WebhookEnabled && prefs.WebhookOnCriticalRisk {
		notification.Channel = domain.NotificationChannelWebhook
		if err := ns.sendNotification(ctx, notification, prefs); err != nil {
			ns.logNotificationFailure(ctx, notification, err)
		}
	}

	// Always save in-app notification
	notification.Channel = domain.NotificationChannelInApp
	return ns.db.Create(notification).Error
}

// SendActionAssignedNotification sends notification when action is assigned
func (ns *NotificationService) SendActionAssignedNotification(
	ctx context.Context,
	userID uuid.UUID,
	tenantID uuid.UUID,
	payload *domain.ActionAssignedNotificationPayload,
) error {
	prefs, err := ns.GetUserNotificationPreferences(userID, tenantID)
	if err != nil || prefs == nil {
		return fmt.Errorf("failed to get notification preferences: %w", err)
	}

	if prefs.DisableAllNotifications {
		return nil
	}

	notification := &domain.Notification{
		ID:           uuid.New(),
		UserID:       userID,
		TenantID:     tenantID,
		Type:         domain.NotificationTypeActionAssigned,
		Status:       domain.NotificationStatusPending,
		ResourceID:   &payload.ActionID,
		ResourceType: "action",
		Subject:      fmt.Sprintf("Action Assigned: %s", payload.ActionTitle),
		Message:      fmt.Sprintf("You have been assigned an action: '%s'", payload.ActionTitle),
		Description: fmt.Sprintf("Risk: %s\nAssigned by: %s\nDue date: %s\nPriority: %s",
			payload.RiskTitle, payload.AssignedBy, payload.DueDate.Format("2006-01-02"), payload.Priority),
		Metadata: map[string]interface{}{
			"action_title": payload.ActionTitle,
			"risk_title":   payload.RiskTitle,
			"assigned_by":  payload.AssignedBy,
			"due_date":     payload.DueDate,
			"priority":     payload.Priority,
			"action_link":  payload.ActionLink,
			"risk_link":    payload.RiskLink,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Send through enabled channels
	if prefs.EmailOnActionAssigned {
		notification.Channel = domain.NotificationChannelEmail
		if err := ns.sendNotification(ctx, notification, prefs); err != nil {
			ns.logNotificationFailure(ctx, notification, err)
		}
	}

	if prefs.SlackEnabled && prefs.SlackOnActionAssigned {
		notification.Channel = domain.NotificationChannelSlack
		if err := ns.sendNotification(ctx, notification, prefs); err != nil {
			ns.logNotificationFailure(ctx, notification, err)
		}
	}

	if prefs.WebhookEnabled && prefs.WebhookOnActionAssigned {
		notification.Channel = domain.NotificationChannelWebhook
		if err := ns.sendNotification(ctx, notification, prefs); err != nil {
			ns.logNotificationFailure(ctx, notification, err)
		}
	}

	// Always save in-app notification
	notification.Channel = domain.NotificationChannelInApp
	return ns.db.Create(notification).Error
}

// GetUserNotificationPreferences retrieves user's notification preferences
func (ns *NotificationService) GetUserNotificationPreferences(
	userID uuid.UUID,
	tenantID uuid.UUID,
) (*domain.NotificationPreference, error) {
	var prefs domain.NotificationPreference

	err := ns.db.Where("user_id = ? AND tenant_id = ?", userID, tenantID).
		First(&prefs).Error

	if err == gorm.ErrRecordNotFound {
		// Return default preferences if none exist
		return &domain.NotificationPreference{
			ID:                         uuid.New(),
			UserID:                     userID,
			TenantID:                   tenantID,
			EmailOnMitigationDeadline:  true,
			EmailOnCriticalRisk:        true,
			EmailOnActionAssigned:      true,
			EmailDeadlineAdvanceDays:   3,
			SlackEnabled:               false,
			WebhookEnabled:             false,
			DisableAllNotifications:    false,
			EnableSoundNotifications:   true,
			EnableDesktopNotifications: true,
		}, nil
	}

	if err != nil {
		return nil, err
	}

	return &prefs, nil
}

// UpdateNotificationPreferences updates user's notification preferences
func (ns *NotificationService) UpdateNotificationPreferences(
	userID uuid.UUID,
	tenantID uuid.UUID,
	updates map[string]interface{},
) error {
	return ns.db.Model(&domain.NotificationPreference{}).
		Where("user_id = ? AND tenant_id = ?", userID, tenantID).
		Updates(updates).Error
}

// GetUserNotifications retrieves user's notifications
func (ns *NotificationService) GetUserNotifications(
	userID uuid.UUID,
	tenantID uuid.UUID,
	limit int,
	offset int,
) ([]*domain.Notification, error) {
	var notifications []*domain.Notification

	err := ns.db.Where("user_id = ? AND tenant_id = ?", userID, tenantID).
		Where("channel = ?", domain.NotificationChannelInApp).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&notifications).Error

	return notifications, err
}

// MarkNotificationAsRead marks notification as read
func (ns *NotificationService) MarkNotificationAsRead(
	notificationID uuid.UUID,
	userID uuid.UUID,
) error {
	now := time.Now()
	return ns.db.Model(&domain.Notification{}).
		Where("id = ? AND user_id = ?", notificationID, userID).
		Updates(map[string]interface{}{
			"status":  domain.NotificationStatusRead,
			"read_at": now,
		}).Error
}

// MarkAllNotificationsAsRead marks all user notifications as read
func (ns *NotificationService) MarkAllNotificationsAsRead(userID uuid.UUID, tenantID uuid.UUID) error {
	now := time.Now()
	return ns.db.Model(&domain.Notification{}).
		Where("user_id = ? AND tenant_id = ?", userID, tenantID).
		Where("status != ?", domain.NotificationStatusRead).
		Updates(map[string]interface{}{
			"status":  domain.NotificationStatusRead,
			"read_at": now,
		}).Error
}

// DeleteNotification deletes a notification
func (ns *NotificationService) DeleteNotification(notificationID uuid.UUID, userID uuid.UUID) error {
	return ns.db.Where("id = ? AND user_id = ?", notificationID, userID).
		Delete(&domain.Notification{}).Error
}

// sendNotification is internal method to send notification through appropriate channel
func (ns *NotificationService) sendNotification(
	ctx context.Context,
	notification *domain.Notification,
	prefs *domain.NotificationPreference,
) error {
	switch notification.Channel {
	case domain.NotificationChannelEmail:
		if ns.emailProvider == nil {
			return fmt.Errorf("email provider not configured")
		}
		return ns.emailProvider.Send(ctx, notification)

	case domain.NotificationChannelSlack:
		if ns.slackProvider == nil {
			return fmt.Errorf("slack provider not configured")
		}
		return ns.slackProvider.Send(ctx, notification)

	case domain.NotificationChannelWebhook:
		if ns.webhookProvider == nil {
			return fmt.Errorf("webhook provider not configured")
		}
		return ns.webhookProvider.Send(ctx, notification)

	default:
		return fmt.Errorf("unsupported notification channel: %s", notification.Channel)
	}
}

// logNotificationFailure logs failed notification attempt
func (ns *NotificationService) logNotificationFailure(
	ctx context.Context,
	notification *domain.Notification,
	err error,
) {
	log := &domain.NotificationLog{
		ID:             uuid.New(),
		NotificationID: notification.ID,
		Attempt:        1,
		Status:         domain.NotificationStatusFailed,
		ErrorMessage:   err.Error(),
		SentAt:         time.Now(),
		CreatedAt:      time.Now(),
	}

	ns.db.Create(log)
}

// BroadcastNotificationToTenant sends notification to all users with permission
func (ns *NotificationService) BroadcastNotificationToTenant(
	ctx context.Context,
	tenantID uuid.UUID,
	notificationType domain.NotificationType,
	subject string,
	message string,
	metadata map[string]interface{},
) error {
	// Get all active users in tenant
	var users []*domain.User
	err := ns.db.
		Joins("JOIN user_tenants ON users.id = user_tenants.user_id").
		Where("user_tenants.tenant_id = ? AND users.is_active = true", tenantID).
		Find(&users).Error

	if err != nil {
		return fmt.Errorf("failed to get users: %w", err)
	}

	// Send notification to each user
	var lastErr error
	for _, user := range users {
		notification := &domain.Notification{
			ID:        uuid.New(),
			UserID:    user.ID,
			TenantID:  tenantID,
			Type:      notificationType,
			Channel:   domain.NotificationChannelInApp,
			Status:    domain.NotificationStatusPending,
			Subject:   subject,
			Message:   message,
			Metadata:  metadata,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		if err := ns.db.Create(notification).Error; err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// PruneOldNotifications deletes old notification records (cleanup job)
func (ns *NotificationService) PruneOldNotifications(daysOld int) error {
	cutoffDate := time.Now().AddDate(0, 0, -daysOld)

	return ns.db.Where("created_at < ? AND status IN (?)", cutoffDate, []domain.NotificationStatus{
		domain.NotificationStatusDelivered,
		domain.NotificationStatusRead,
		domain.NotificationStatusFailed,
	}).Delete(&domain.Notification{}).Error
}

// GetUnreadCount returns count of unread notifications for user
func (ns *NotificationService) GetUnreadCount(userID uuid.UUID, tenantID uuid.UUID) (int64, error) {
	var count int64

	err := ns.db.Model(&domain.Notification{}).
		Where("user_id = ? AND tenant_id = ?", userID, tenantID).
		Where("channel = ? AND status != ?", domain.NotificationChannelInApp, domain.NotificationStatusRead).
		Count(&count).Error

	return count, err
}
