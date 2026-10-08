// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The due pill of the redesign (#901, #902): "En retard de 2 j" on red,
// "Aujourd'hui" on amber, "Demain" / "Dans 3 j" neutral within the week, the
// date muted beyond it.

import { useI18n } from '../../hooks/useI18n';
import { formatDate } from '../../i18n/format';
import { daysUntil } from './dueDate';

export function DuePill({ iso }: { iso: string }) {
  const { t, locale } = useI18n();
  const n = daysUntil(iso);
  let label: string;
  let style: { background: string; color: string };
  if (n < 0) {
    label = t('dashboard.priorities.overdue', { count: -n });
    style = { background: 'var(--danger-surface)', color: 'var(--danger-text)' };
  } else if (n === 0) {
    label = t('dashboard.priorities.today');
    style = { background: 'var(--warning-surface)', color: 'var(--warning-text)' };
  } else if (n <= 7) {
    label = n === 1 ? t('dashboard.priorities.tomorrow') : t('dashboard.priorities.inDays', { count: n });
    style = { background: 'var(--surface-3)', color: 'var(--fg-secondary)' };
  } else {
    label = formatDate(locale, iso, { day: 'numeric', month: 'short' });
    style = { background: 'var(--surface-3)', color: 'var(--fg-muted)' };
  }
  return (
    <span className="text-[11px] font-semibold px-2 py-[3px] rounded-full whitespace-nowrap" style={style}>
      {label}
    </span>
  );
}
