// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// One row of the Action Center, shared by the dashboard panel and the full page
// (#430, #433).
//
// It lives in its own file for one reason, and it is the reason acceptance
// criterion 6 of #433 exists: the panel and the page must not each own a copy of
// "how an action item becomes a link". Two copies drift, and the way they drift
// is that one of them forgets to run `safeDeepLink` and starts rendering rows
// that go nowhere. There is one row component, it takes an href that has already
// been proved to resolve, and neither surface can render an item any other way.

import type { KeyboardEvent } from 'react';
import { Link, useNavigate } from 'react-router';
import { ArrowRight } from 'lucide-react';

import { useI18n, interpolate } from '../../hooks/useI18n';
import { presentationFor } from './actionLinks';
import { formatDue, isOverdue } from './dueDate';
import type { ActionItem } from './actionCenterService';

export interface ActionItemRowProps {
  item: ActionItem;
  /** An href that has already passed `safeDeepLink`. Never a raw `deep_link`. */
  href: string;
}

export function ActionItemRow({ item, href }: ActionItemRowProps) {
  const { t, locale } = useI18n();
  const navigate = useNavigate();
  const { icon: Icon, labelKey, tone } = presentationFor(item.type);
  const due = formatDue(item.due_at, locale);
  const overdue = isOverdue(item.due_at);
  const typeLabel = t(`actionCenter.types.${labelKey}`);

  // An anchor already activates on Enter. Space does not, and #430's acceptance
  // criterion names both, so it is handled explicitly rather than left to be
  // discovered as a dead key by whoever navigates this list without a mouse.
  const onKeyDown = (event: KeyboardEvent<HTMLAnchorElement>) => {
    if (event.key === ' ' || event.key === 'Spacebar') {
      event.preventDefault();
      navigate(href);
    }
  };

  return (
    <li>
      <Link
        to={href}
        onKeyDown={onKeyDown}
        data-testid="action-center-item"
        data-action-type={item.type}
        className="group flex items-center gap-3 rounded-md px-3 py-2.5 no-underline transition-colors hover:bg-hover focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus"
      >
        <Icon size={16} strokeWidth={1.75} className={`shrink-0 ${tone}`} aria-hidden="true" />

        <span className="min-w-0 flex-1">
          <span className="block truncate text-sm font-medium text-fg-primary">{item.title}</span>
          <span className="mt-0.5 block text-xs text-fg-muted">
            {typeLabel}
            {due && (
              <>
                {' · '}
                <span className={overdue ? 'text-danger-text font-medium' : undefined}>
                  {overdue
                    ? `${t('actionCenter.overdue')} — ${due}`
                    : interpolate(t('actionCenter.due'), { date: due })}
                </span>
              </>
            )}
            {!due && ` · ${t('actionCenter.noDueDate')}`}
          </span>
        </span>

        {/* The arrow is decoration, so it is hidden from the accessibility tree
            and the link carries an explicit label instead of relying on it. */}
        <ArrowRight
          size={15}
          strokeWidth={1.75}
          aria-hidden="true"
          className="shrink-0 text-fg-muted opacity-0 transition-opacity group-hover:opacity-100 group-focus-visible:opacity-100"
        />
        <span className="sr-only">
          {interpolate(t('actionCenter.openItem'), { title: item.title })}
        </span>
      </Link>
    </li>
  );
}
