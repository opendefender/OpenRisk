// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Notification categories (UX-6 / directive §5). Separating notifications by
// context lets a user tune preferences finely — mute Compliance email but keep
// Security in-app, etc. The taxonomy maps the backend NotificationType values
// (domain/notification.go) onto five human contexts. New types default to Security.

import { ShieldAlert, ClipboardCheck, CheckSquare, Users, CreditCard, type LucideIcon } from 'lucide-react';

export type NotifCategory = 'security' | 'compliance' | 'tasks' | 'collaboration' | 'billing';

export interface NotifCategoryMeta {
  key: NotifCategory;
  label: { fr: string; en: string };
  desc: { fr: string; en: string };
  icon: LucideIcon;
  color: string;
}

export const NOTIF_CATEGORIES: NotifCategoryMeta[] = [
  {
    key: 'security',
    label: { fr: 'Sécurité', en: 'Security' },
    desc: { fr: 'Risques critiques, incidents, scans, SLA.', en: 'Critical risks, incidents, scans, SLA.' },
    icon: ShieldAlert,
    color: 'var(--critical)',
  },
  {
    key: 'compliance',
    label: { fr: 'Conformité', en: 'Compliance' },
    desc: { fr: 'Revues, écarts, audits, échéances réglementaires.', en: 'Reviews, gaps, audits, regulatory deadlines.' },
    icon: ClipboardCheck,
    color: 'var(--info)',
  },
  {
    key: 'tasks',
    label: { fr: 'Tâches', en: 'Tasks' },
    desc: { fr: 'Ce qui vous est assigné et vos échéances.', en: 'What is assigned to you and your deadlines.' },
    icon: CheckSquare,
    color: 'var(--high)',
  },
  {
    key: 'collaboration',
    label: { fr: 'Collaboration', en: 'Collaboration' },
    desc: { fr: 'Mentions, commentaires, War Room.', en: 'Mentions, comments, War Room.' },
    icon: Users,
    color: 'var(--accent)',
  },
  {
    key: 'billing',
    label: { fr: 'Facturation', en: 'Billing' },
    desc: { fr: 'Abonnement, limites de plan, factures.', en: 'Subscription, plan limits, invoices.' },
    icon: CreditCard,
    color: 'var(--low)',
  },
];

/** Map a backend NotificationType string onto a category. */
export function categoryForType(type: string): NotifCategory {
  switch (type) {
    case 'action_assigned':
    case 'mitigation_deadline':
      return 'tasks';
    case 'risk_review':
      return 'compliance';
    case 'critical_risk':
    case 'risk_update':
    case 'risk_resolved':
    case 'scan_complete':
    case 'sla_breach':
    case 'automation':
    default:
      return 'security';
  }
}

export function categoryMeta(key: NotifCategory): NotifCategoryMeta {
  return NOTIF_CATEGORIES.find((c) => c.key === key) ?? NOTIF_CATEGORIES[0];
}

/* ---------------- per-category channel preferences (persisted) ---------------- */

export type NotifChannelPrefs = Record<NotifCategory, { inApp: boolean; email: boolean }>;
const PREFS_KEY = 'openrisk_notif_prefs';

export function defaultNotifPrefs(): NotifChannelPrefs {
  return {
    security: { inApp: true, email: true },
    compliance: { inApp: true, email: true },
    tasks: { inApp: true, email: false },
    collaboration: { inApp: true, email: false },
    billing: { inApp: true, email: true },
  };
}

export function loadNotifPrefs(): NotifChannelPrefs {
  try {
    const raw = localStorage.getItem(PREFS_KEY);
    if (raw) return { ...defaultNotifPrefs(), ...JSON.parse(raw) };
  } catch { /* ignore */ }
  return defaultNotifPrefs();
}

export function saveNotifPrefs(prefs: NotifChannelPrefs) {
  try { localStorage.setItem(PREFS_KEY, JSON.stringify(prefs)); } catch { /* ignore */ }
}
