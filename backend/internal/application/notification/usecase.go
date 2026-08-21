// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package notification

import (
	"errors"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrValidation   = errors.New("validation error")
)

// Repository defines persistence operations required by notification use cases.
type Repository interface {
	CreateNotification(n *domain.Notification) error
	GetUserNotifications(userID, tenantID uuid.UUID, limit, offset int) ([]*domain.Notification, error)
	GetUnreadCount(userID, tenantID uuid.UUID) (int64, error)
	MarkNotificationAsRead(notificationID, userID, tenantID uuid.UUID) error
	MarkAllNotificationsAsRead(userID, tenantID uuid.UUID) error
	DeleteNotification(notificationID, userID, tenantID uuid.UUID) error
	GetUserNotificationPreferences(userID, tenantID uuid.UUID) (*domain.NotificationPreference, error)
	UpdateNotificationPreferences(userID, tenantID uuid.UUID, updates map[string]interface{}) error
}

type UseCase struct {
	repo Repository
}

func NewUseCase(repo Repository) *UseCase {
	return &UseCase{repo: repo}
}

// ErrSuppressed reports that the recipient's preferences forbid this delivery.
//
// A distinct error, not a silent nil: a producer logging "notification failed"
// for a suppressed one would send whoever reads the logs hunting a bug, and a
// producer treating suppression as success would have no way to tell the two
// apart. Callers that only care about "did it get through" can ignore it.
var ErrSuppressed = errors.New("suppressed by recipient preferences")

// ShouldNotify reports whether this user's preferences allow a notification of
// this type on this channel.
//
// Exported because in-app is not the only channel: the workers in main.go send
// e-mail alongside the in-app record, and they must ask the same question of the
// same stored row, or the Settings screen would govern half a delivery.
//
// Fails OPEN when the preferences cannot be read. A storage error must not
// silence a security alert — the cost of one unwanted e-mail is a nuisance, the
// cost of a swallowed critical-risk alert is the incident nobody saw.
func (uc *UseCase) ShouldNotify(userID, tenantID uuid.UUID, notifType domain.NotificationType, channel domain.NotificationChannel) bool {
	if userID == uuid.Nil || tenantID == uuid.Nil {
		return false
	}
	prefs, err := uc.repo.GetUserNotificationPreferences(userID, tenantID)
	if err != nil {
		return true
	}
	return prefs.Allows(notifType, channel)
}

// NotifyInApp persists an in-app notification for a user. Best-effort creation
// point used by cross-cutting producers (e.g. the scan engine). A Nil user or
// tenant is rejected so we never create an orphan notification.
//
// This is the single choke point every in-app producer goes through — the scan
// sink, the risk-review worker, the mitigation-deadline worker, the evidence
// worker, the ownership service, the automation notifier, the approval notifier
// and the vuln-risk notifier all land here — so the preference check belongs
// here rather than eight times over. Returns ErrSuppressed when the recipient
// has asked not to be notified.
func (uc *UseCase) NotifyInApp(userID, tenantID uuid.UUID, notifType domain.NotificationType, subject, message string, resourceID *uuid.UUID, resourceType string) error {
	if userID == uuid.Nil || tenantID == uuid.Nil {
		return ErrValidation
	}
	if !uc.ShouldNotify(userID, tenantID, notifType, domain.NotificationChannelInApp) {
		return ErrSuppressed
	}
	n := &domain.Notification{
		ID:           uuid.New(),
		UserID:       userID,
		TenantID:     tenantID,
		Type:         notifType,
		Channel:      domain.NotificationChannelInApp,
		Status:       domain.NotificationStatusSent, // unread until read
		Subject:      subject,
		Message:      message,
		ResourceID:   resourceID,
		ResourceType: resourceType,
	}
	return uc.repo.CreateNotification(n)
}

func (uc *UseCase) GetNotifications(userID, tenantID uuid.UUID, limit, offset int) ([]*domain.Notification, error) {
	if userID == uuid.Nil || tenantID == uuid.Nil {
		return nil, ErrUnauthorized
	}
	if limit <= 0 || limit > 100 {
		return nil, ErrValidation
	}
	if offset < 0 {
		return nil, ErrValidation
	}
	return uc.repo.GetUserNotifications(userID, tenantID, limit, offset)
}

func (uc *UseCase) GetUnreadCount(userID, tenantID uuid.UUID) (int64, error) {
	if userID == uuid.Nil || tenantID == uuid.Nil {
		return 0, ErrUnauthorized
	}
	return uc.repo.GetUnreadCount(userID, tenantID)
}

func (uc *UseCase) MarkAsRead(notificationID, userID, tenantID uuid.UUID) error {
	if notificationID == uuid.Nil || userID == uuid.Nil || tenantID == uuid.Nil {
		return ErrValidation
	}
	return uc.repo.MarkNotificationAsRead(notificationID, userID, tenantID)
}

func (uc *UseCase) MarkAllAsRead(userID, tenantID uuid.UUID) error {
	if userID == uuid.Nil || tenantID == uuid.Nil {
		return ErrUnauthorized
	}
	return uc.repo.MarkAllNotificationsAsRead(userID, tenantID)
}

func (uc *UseCase) DeleteNotification(notificationID, userID, tenantID uuid.UUID) error {
	if notificationID == uuid.Nil || userID == uuid.Nil || tenantID == uuid.Nil {
		return ErrValidation
	}
	return uc.repo.DeleteNotification(notificationID, userID, tenantID)
}

func (uc *UseCase) GetPreferences(userID, tenantID uuid.UUID) (*domain.NotificationPreference, error) {
	if userID == uuid.Nil || tenantID == uuid.Nil {
		return nil, ErrUnauthorized
	}
	return uc.repo.GetUserNotificationPreferences(userID, tenantID)
}

func (uc *UseCase) UpdatePreferences(userID, tenantID uuid.UUID, updates map[string]interface{}) (*domain.NotificationPreference, error) {
	if userID == uuid.Nil || tenantID == uuid.Nil {
		return nil, ErrUnauthorized
	}
	if len(updates) == 0 {
		return nil, ErrValidation
	}
	if v, ok := updates["email_deadline_advance_days"]; ok {
		if days, ok := v.(int); !ok || days < 0 || days > 30 {
			return nil, ErrValidation
		}
	}

	if err := uc.repo.UpdateNotificationPreferences(userID, tenantID, updates); err != nil {
		return nil, err
	}
	return uc.repo.GetUserNotificationPreferences(userID, tenantID)
}
