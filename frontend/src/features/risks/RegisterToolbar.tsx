// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The register's toolbar controls (#913, V2 layout): the quick views as one
// segmented control with counts and the Table / Matrice switch. Band, status and
// owner are facets of the table's Filters panel, shown as removable chips when
// set. The search box is the table's own (table mode) or SearchBox below (matrix).

import { useEffect, useState } from 'react';
import { Search, X } from 'lucide-react';
import { useI18n } from '../../hooks/useI18n';
import { RISK_VIEWS, type RegisterLayout, type RiskView } from './riskViews';

/**
 * The quick views as one segmented control (V2 mockup): each view with its count
 * from /risks/summary, the active one raised. Counts that are still loading show
 * a short skeleton; a view never shows a number it does not have.
 */
export function QuickViews({
  value,
  counts,
  loading,
  onChange,
}: {
  value: RiskView;
  counts: Partial<Record<RiskView, number>> | undefined;
  loading: boolean;
  onChange: (view: RiskView) => void;
}) {
  const { t } = useI18n();
  return (
    <div
      role="group"
      aria-label={t('riskRegister.view.label')}
      data-testid="risk-views"
      className="inline-flex shrink-0 gap-0.5 p-0.5 rounded-lg bg-surface-0 border border-border-subtle"
    >
      {RISK_VIEWS.map((v) => {
        const on = value === v;
        const n = counts?.[v];
        return (
          <button
            key={v}
            type="button"
            aria-pressed={on}
            onClick={() => onChange(v)}
            data-testid={`risk-tab-${v}`}
            className={`h-8 px-2.5 rounded-md text-[13px] inline-flex items-center gap-1.5 whitespace-nowrap transition-colors ${
              on ? 'bg-surface-2 text-ink font-medium shadow-sm' : 'text-ink-soft hover:text-ink'
            }`}
          >
            {t(`riskRegister.view.${v}`)}
            {typeof n === 'number' ? (
              <span className="mono text-[11px] text-ink-muted">{n}</span>
            ) : loading ? (
              <span aria-hidden="true" className="or-skeleton h-3 w-4 rounded-[4px]" />
            ) : null}
          </button>
        );
      })}
    </div>
  );
}

/** A tab label with its count as plain muted mono text, as designed (not a badge). */
export function TabLabel({ text, count }: { text: string; count?: number }) {
  return (
    <>
      {text}
      {typeof count === 'number' && (
        <span className="mono text-[11px] font-medium text-ink-muted">{count}</span>
      )}
    </>
  );
}

export function LayoutSwitch({
  layout,
  onChange,
}: {
  layout: RegisterLayout;
  onChange: (layout: RegisterLayout) => void;
}) {
  const { t } = useI18n();
  const modes: readonly RegisterLayout[] = ['table', 'matrix'];
  return (
    <div
      role="group"
      aria-label={t('riskRegister.layout.label')}
      className="inline-flex gap-0.5 p-0.5 rounded-lg bg-surface-0 border border-border-subtle"
    >
      {modes.map((id) => {
        const on = layout === id;
        return (
          <button
            key={id}
            type="button"
            aria-pressed={on}
            onClick={() => onChange(id)}
            data-testid={`risk-layout-${id}`}
            className={`h-8 px-3 rounded-md text-[13px] transition-colors ${
              on ? 'bg-surface-2 text-ink font-medium shadow-sm' : 'text-ink-soft hover:text-ink'
            }`}
          >
            {t(`riskRegister.layout.${id}`)}
          </button>
        );
      })}
    </div>
  );
}

/**
 * The matrix has no DataTable around it, so it carries its own search box. It
 * debounces like the table's (220 ms), so a keystroke is not a request.
 */
export function SearchBox({ value, onChange }: { value: string; onChange: (q: string) => void }) {
  const { t } = useI18n();
  const [draft, setDraft] = useState(value);
  // The URL can change under us (reset, back, saved view): follow it.
  const [seen, setSeen] = useState(value);
  if (value !== seen) {
    setSeen(value);
    setDraft(value);
  }
  useEffect(() => {
    if (draft === value) return;
    const timer = window.setTimeout(() => onChange(draft), 220);
    return () => window.clearTimeout(timer);
  }, [draft, value, onChange]);

  return (
    <div className="relative flex-1 min-w-[200px] max-w-[320px]">
      <Search
        size={15}
        aria-hidden="true"
        className="absolute left-3 top-1/2 -translate-y-1/2 text-ink-muted pointer-events-none"
      />
      <input
        type="search"
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
        placeholder={t('riskRegister.search.placeholder')}
        aria-label={t('riskRegister.search.label')}
        data-testid="risk-search"
        className="w-full h-[34px] pl-9 pr-8 rounded-[10px] text-[13px] text-ink bg-surface-1 border border-border-control outline-none focus-visible:ring-2 focus-visible:ring-accent/40"
      />
      {draft && (
        <button
          type="button"
          onClick={() => setDraft('')}
          aria-label={t('riskRegister.search.clear')}
          className="absolute right-2 top-1/2 -translate-y-1/2 text-ink-muted hover:text-ink"
        >
          <X size={14} aria-hidden="true" />
        </button>
      )}
    </div>
  );
}
