// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// One action of the redesigned Action Center (#902): the type's icon on a
// tile, the record's own title, the facts line, the due pill, one verb that
// opens the record, and — when the work can be finished from here — a tick
// that finishes it for real (see actionKinds.ts).

import { createElement } from 'react';
import { Check } from 'lucide-react';
import { Link } from 'react-router';
import { useI18n } from '../../hooks/useI18n';
import type { ActionItem } from './actionCenterService';
import { actionFacts } from './actionFacts';
import { iconFor, kindOf } from './actionKinds';
import { DuePill } from './DuePill';

export interface ActionItemRowProps {
  item: ActionItem;
  /** An href that has already passed `safeDeepLink`. Never a raw `deep_link`. */
  href: string;
  /** Present when the item can be completed from the list. */
  onDone?: () => void;
}

export function ActionItemRow({ item, href, onDone }: ActionItemRowProps) {
  const { t } = useI18n();
  const cta = kindOf(item.type) ? t(`actionCenter.cta.${item.type}`) : t('actionCenter.cta.unknown');

  return (
    <li
      data-testid="action-center-item"
      data-action-type={item.type}
      className="flex items-center gap-3.5 px-4 py-3.5 border-b border-border-subtle last:border-b-0"
    >
      <span
        aria-hidden="true"
        className="w-[34px] h-[34px] rounded-[9px] bg-surface-3 flex items-center justify-center shrink-0 text-fg-secondary"
      >
        {createElement(iconFor(item.type), { size: 16, strokeWidth: 1.75 })}
      </span>

      <span className="flex-1 min-w-0">
        <span className="block text-[13.5px] font-semibold text-ink truncate">{item.title}</span>
        <span className="block text-[12px] text-ink-muted mt-0.5 truncate">{actionFacts(item, t)}</span>
      </span>

      {item.due_at && <DuePill iso={item.due_at} />}

      <Link
        to={href}
        data-testid="action-center-open"
        aria-label={`${cta} — ${item.title}`}
        className="h-8 px-3 rounded-[9px] border border-border-control bg-surface-2 text-ink text-[12.5px] font-semibold flex items-center whitespace-nowrap hover:bg-surface-3 transition-colors"
      >
        {cta}
      </Link>

      {onDone ? (
        <button
          type="button"
          onClick={onDone}
          data-testid="action-center-done"
          aria-label={`${t('actionCenter.done')} — ${item.title}`}
          title={t('actionCenter.done')}
          className="w-8 h-8 rounded-[9px] flex items-center justify-center text-ink-muted hover:bg-success-surface hover:text-success-text transition-colors shrink-0"
        >
          <Check size={16} strokeWidth={2} />
        </button>
      ) : (
        // Keeps the verbs aligned in a column whether or not a row has a tick.
        <span aria-hidden="true" className="w-8 shrink-0" />
      )}
    </li>
  );
}
