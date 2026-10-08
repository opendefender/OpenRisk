// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

const DAY = 86_400_000;

/** Whole calendar days from today to `iso`, negative when past. */
export function daysUntil(iso: string, now = new Date()): number {
  const d = new Date(iso);
  const a = Date.UTC(now.getFullYear(), now.getMonth(), now.getDate());
  const b = Date.UTC(d.getFullYear(), d.getMonth(), d.getDate());
  return Math.round((b - a) / DAY);
}
