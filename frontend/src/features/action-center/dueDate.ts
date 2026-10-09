// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Due-date formatting for an action item.
//
// Its own module rather than a pair of exports beside the row component: a file
// that exports both a component and a helper breaks Vite's fast refresh, which
// the react-refresh lint rule enforces at error level.

import { localeTag, type LocaleCode } from '../../i18n/locales';

/** Formats a due date in the active locale, or null when the category has none. */
export function formatDue(dueAt: string | null | undefined, locale: LocaleCode): string | null {
  if (!dueAt) return null;
  const date = new Date(dueAt);
  if (Number.isNaN(date.getTime())) return null;
  return date.toLocaleDateString(localeTag(locale), {
    day: '2-digit',
    month: 'short',
    year: 'numeric',
  });
}

/** True when the deadline has passed. False for a category that has no date. */
export function isOverdue(dueAt: string | null | undefined): boolean {
  if (!dueAt) return false;
  const date = new Date(dueAt);
  return !Number.isNaN(date.getTime()) && date.getTime() < Date.now();
}

const DAY = 86_400_000;

/** Whole calendar days from today to `iso`, negative when past. */
export function daysUntil(iso: string, now = new Date()): number {
  const d = new Date(iso);
  const a = Date.UTC(now.getFullYear(), now.getMonth(), now.getDate());
  const b = Date.UTC(d.getFullYear(), d.getMonth(), d.getDate());
  return Math.round((b - a) / DAY);
}
