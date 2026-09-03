// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Typed client for the real notification endpoints. The bell used to render four
// hardcoded notifications ("RDP exposé sur srv-paie-01", "INC-2026-014") with a
// permanently lit unread dot, on every tenant, forever — including tenants that
// had never recorded anything.

import { api } from '../../lib/api';

export type NotificationType =
  | 'mitigation_deadline'
  | 'critical_risk'
  | 'action_assigned'
  | 'risk_update'
  | 'risk_resolved'
  | 'scan_complete'
  | 'risk_review'
  | 'automation'
  | 'sla_breach';

export interface Notification {
  id: string;
  user_id: string;
  tenant_id: string;
  type: NotificationType;
  channel: 'email' | 'slack' | 'webhook' | 'teams' | 'in_app';
  status: 'pending' | 'sent' | 'delivered' | 'failed' | 'read';
  subject: string;
  message: string;
  description?: string;
  resource_id?: string;
  resource_type?: string;
  read_at?: string | null;
  created_at: string;
}

/**
 * The stored preference row, mirroring domain.NotificationPreference.
 *
 * Every field is a real column the backend reads before it sends
 * (domain.NotificationPreference.Allows). Nothing here is decorative — which is
 * the whole point of the change: the screen may only offer switches the server
 * actually honours.
 */
export interface NotificationPreferences {
  // Email, per event.
  email_on_mitigation_deadline: boolean;
  email_on_critical_risk: boolean;
  email_on_action_assigned: boolean;
  email_on_risk_update: boolean;
  email_on_risk_resolved: boolean;
  email_deadline_advance_days: number;

  // Slack. `slack_enabled` gates the per-event switches: an unconfigured
  // channel must never read as "yes, send there".
  slack_enabled: boolean;
  slack_on_mitigation_deadline: boolean;
  slack_on_critical_risk: boolean;
  slack_on_action_assigned: boolean;

  // Outbound webhook, same gating.
  webhook_enabled: boolean;
  webhook_on_mitigation_deadline: boolean;
  webhook_on_critical_risk: boolean;
  webhook_on_action_assigned: boolean;

  // Global.
  disable_all_notifications: boolean;
  enable_sound_notifications: boolean;
  enable_desktop_notifications: boolean;
}

/** A partial update — the server applies only the keys present. */
export type NotificationPreferencePatch = Partial<NotificationPreferences>;

interface ListResponse {
  data: Notification[] | null;
  limit: number;
  offset: number;
  total: number;
}

export const notificationService = {
  async list(limit = 20): Promise<Notification[]> {
    const { data } = await api.get<ListResponse>('/notifications', { params: { limit } });
    return data.data ?? [];
  },

  async unreadCount(): Promise<number> {
    const { data } = await api.get<{ count?: number; unread_count?: number }>(
      '/notifications/unread-count',
    );
    return data.count ?? data.unread_count ?? 0;
  },

  async markRead(id: string): Promise<void> {
    await api.patch(`/notifications/${id}/read`);
  },

  async markAllRead(): Promise<void> {
    await api.patch('/notifications/read-all');
  },

  // --- preferences -------------------------------------------------------
  //
  // Settings › Notifications used to write these to localStorage while this API
  // sat unused (W0-05 / D2). Two consequences: the switches changed nothing,
  // and they carried to the next person to sign in on the same browser, since
  // localStorage has no idea who is logged in. They are per user AND per tenant
  // on the server, keyed `(user_id, tenant_id)`.

  async getPreferences(): Promise<NotificationPreferences> {
    const { data } = await api.get<NotificationPreferences>('/notifications/preferences');
    return data;
  },

  // PATCH, not PUT: the server applies only the fields present, so two switches
  // flipped in quick succession cannot clobber each other.
  async updatePreferences(patch: NotificationPreferencePatch): Promise<NotificationPreferences> {
    const { data } = await api.patch<NotificationPreferences>('/notifications/preferences', patch);
    return data;
  },
};
