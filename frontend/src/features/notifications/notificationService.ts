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
    const { data } = await api.get<{ count?: number; unread_count?: number }>('/notifications/unread-count');
    return data.count ?? data.unread_count ?? 0;
  },

  async markRead(id: string): Promise<void> {
    await api.patch(`/notifications/${id}/read`);
  },

  async markAllRead(): Promise<void> {
    await api.patch('/notifications/read-all');
  },
};
