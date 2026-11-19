// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The register's toolbar controls (#913): four band pills with counts, the
// status and owner listboxes, the reset link and the Tableau / Matrice switch.
// The search box is the table's own (table mode) or SearchBox below (matrix).

import { useEffect, useMemo, useState } from 'react';
import { LayoutGrid, List, Search, X } from 'lucide-react';
import { useI18n } from '../../hooks/useI18n';
import { Listbox, type ListboxOption } from '../../shared/ds';
import { critColor, type Criticality } from '../../shared/riskColors';
import type { Assignee } from '../../services/ownershipService';
import { memberName } from './riskMembers';
import type { RiskSummary } from './riskRegisterService';
import {
  BANDS,
  RISK_STATUSES,
  type RegisterFilters,
  type RegisterLayout,
  type RiskStatusValue,
} from './riskViews';

const ALL = 'all';

interface FiltersProps {
  filters: RegisterFilters;
  summary: RiskSummary | undefined;
  summaryLoading: boolean;
  members: readonly Assignee[];
  membersLoading: boolean;
  onToggleBand: (band: Criticality) => void;
  onStatus: (status: RiskStatusValue | '') => void;
  onOwner: (ownerId: string) => void;
  hasFilters: boolean;
  onReset: () => void;
}

export function RegisterFilters({
  filters,
  summary,
  summaryLoading,
  members,
  membersLoading,
  onToggleBand,
  onStatus,
  onOwner,
  hasFilters,
  onReset,
}: FiltersProps) {
  const { t } = useI18n();

  const statusOptions: ListboxOption[] = useMemo(
    () => [
      { value: ALL, label: t('riskRegister.filter.allStatuses') },
      ...RISK_STATUSES.map((s) => ({ value: s, label: t(`riskRegister.status.${s}`) })),
    ],
    [t],
  );
  const ownerOptions: ListboxOption[] = useMemo(
    () => [
      { value: ALL, label: t('riskRegister.filter.allOwners') },
      ...members.map((m) => ({ value: m.user_id, label: memberName(m) })),
    ],
    [members, t],
  );

  return (
    <>
      <div role="group" aria-label={t('riskRegister.filter.bands')} className="flex gap-1">
        {BANDS.map((band) => {
          const on = filters.bands.includes(band);
          const n = summary?.by_criticality?.[band];
          return (
            <button
              key={band}
              type="button"
              aria-pressed={on}
              onClick={() => onToggleBand(band)}
              data-testid={`risk-band-${band}`}
              className={`h-[34px] px-2.5 rounded-[10px] border text-[12.5px] font-semibold inline-flex items-center gap-1.5 shrink-0 transition-colors motion-safe:active:translate-y-px ${
                on
                  ? 'bg-surface-2 border-border-strong text-ink'
                  : 'bg-transparent border-border-default text-ink-soft hover:border-border-strong'
              }`}
            >
              <span
                aria-hidden="true"
                className="size-1.5 rounded-full"
                style={{ background: critColor[band] }}
              />
              {t(`risks.band.${band}`)}
              {n !== undefined ? (
                <span className="mono text-[11px] text-ink-muted">{n}</span>
              ) : summaryLoading ? (
                <span aria-hidden="true" className="or-skeleton h-3 w-4 rounded-[4px]" />
              ) : null}
            </button>
          );
        })}
      </div>
      <div className="shrink-0">
        <Listbox
          size="sm"
          value={filters.status || ALL}
          options={statusOptions}
          onChange={(v) => onStatus(v === ALL ? '' : (v as RiskStatusValue))}
          aria-label={t('riskRegister.filter.status')}
          data-testid="risk-filter-status"
          className="min-w-[10.5rem]"
        />
      </div>
      <div className="shrink-0">
        <Listbox
          size="sm"
          value={filters.ownerId || ALL}
          options={ownerOptions}
          onChange={(v) => onOwner(v === ALL ? '' : v)}
          loading={membersLoading}
          aria-label={t('riskRegister.filter.owner')}
          data-testid="risk-filter-owner"
          className="min-w-[11.5rem]"
        />
      </div>
      {hasFilters && (
        <button
          type="button"
          onClick={onReset}
          data-testid="risk-filter-reset"
          className="h-[34px] px-2.5 rounded-lg text-[12.5px] font-semibold text-accent-strong hover:bg-accent-soft"
        >
          {t('riskRegister.filter.reset')}
        </button>
      )}
    </>
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
  const modes = [
    { id: 'table', icon: List },
    { id: 'matrix', icon: LayoutGrid },
  ] as const;
  return (
    <div
      role="group"
      aria-label={t('riskRegister.layout.label')}
      className="ml-auto inline-flex gap-0.5 p-[3px] rounded-[10px] bg-surface-1 border border-border-subtle"
    >
      {modes.map(({ id, icon: Icon }) => {
        const on = layout === id;
        return (
          <button
            key={id}
            type="button"
            aria-pressed={on}
            onClick={() => onChange(id)}
            data-testid={`risk-layout-${id}`}
            className={`h-7 px-2.5 rounded-[7px] text-[12.5px] font-semibold inline-flex items-center gap-1.5 transition-colors ${
              on ? 'bg-surface-3 text-ink' : 'text-ink-soft hover:text-ink'
            }`}
          >
            <Icon size={14} aria-hidden="true" />
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
