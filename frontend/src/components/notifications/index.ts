// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Notification System Component Exports
export { default as NotificationBadge } from './NotificationBadge';
export { default as NotificationCenter } from './NotificationCenter';
export { default as NotificationPreferences } from './NotificationPreferences';

// Type exports (if you have a types file)
export type { NotificationBadgeProps } from './NotificationBadge';
export type { NotificationCenterProps } from './NotificationCenter';
export type { NotificationPreferencesProps } from './NotificationPreferences';

// API types
export interface Notification {
  id: string;
  type: 'critical_risk' | 'mitigation_deadline' | 'action_assigned' | 'risk_update' | 'risk_resolved';
  subject: string;
  message: string;
  status: 'pending' | 'sent' | 'delivered' | 'failed' | 'read';
  created_at: string;
  read_at?: string;
  metadata?: Record<string, any>;
}

export interface NotificationPreference {
  email_on_critical_risk: boolean;
  email_on_mitigation_deadline: boolean;
  email_on_action_assigned: boolean;
  email_deadline_advance_days: number;
  slack_enabled: boolean;
  slack_on_critical_risk: boolean;
  slack_on_mitigation_deadline: boolean;
  slack_on_action_assigned: boolean;
  webhook_enabled: boolean;
  webhook_on_critical_risk: boolean;
  webhook_on_mitigation_deadline: boolean;
  webhook_on_action_assigned: boolean;
  disable_all_notifications: boolean;
  enable_sound_notifications: boolean;
  enable_desktop_notifications: boolean;
}

// Hook types
export interface UseNotificationOptions {
  authToken: string;
  onError?: (error: Error) => void;
  onSuccess?: (message: string) => void;
}

export interface UseNotificationWebSocketOptions {
  authToken: string;
  url?: string;
  onMessage?: (notification: Notification) => void;
  onError?: (error: Error) => void;
}
