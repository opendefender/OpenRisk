// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The real "Filtres" panel.
//
// The button labelled "Filtres" used to open a single search input. Search and
// filtering are different intents and now have different affordances: the
// search box is always visible in the toolbar; this panel holds the facets —
// combinable, reflected in the URL, with a live result count, a reset, and
// named saved views.

import { useCallback, useState } from 'react';
import { Check, Filter, RotateCcw, Star, Trash2, X } from 'lucide-react';
import {
  FloatingFocusManager,
  FloatingPortal,
  autoUpdate,
  flip,
  offset,
  shift,
  size,
  useClick,
  useDismiss,
  useFloating,
  useInteractions,
  useRole,
} from '@floating-ui/react';
import type { Facet, SavedView, TableState } from './types';
import type { TableStateApi } from './useTableState';

interface FilterPanelProps<T> {
  facets: Facet<T>[];
  api: TableStateApi;
  /** Rows matching the current filter — shown live on the panel. */
  resultCount: number;
  views: SavedView[];
  onSaveView: (name: string, state: SavedView['state']) => void;
  onDeleteView: (id: string) => void;
  labels: FilterLabels;
}

export interface FilterLabels {
  filters: string;
  results: (n: number) => string;
  reset: string;
  savedViews: string;
  saveCurrent: string;
  viewNamePlaceholder: string;
  save: string;
  close: string;
  apply: string;
  delete: string;
}

export function FilterPanel<T>({ facets, api, resultCount, views, onSaveView, onDeleteView, labels }: FilterPanelProps<T>) {
  const [open, setOpen] = useState(false);
  const [viewName, setViewName] = useState('');

  const { refs: anchor, floatingStyles, context } = useFloating({
    open,
    onOpenChange: setOpen,
    placement: 'bottom-end',
    whileElementsMounted: autoUpdate,
    middleware: [
      offset(6),
      flip({ padding: 8 }),
      shift({ padding: 8 }),
      size({
        padding: 8,
        apply({ availableHeight, elements }) {
          Object.assign(elements.floating.style, { maxHeight: `${Math.max(220, availableHeight)}px` });
        },
      }),
    ],
  });
  const { getReferenceProps, getFloatingProps } = useInteractions([
    useClick(context),
    useDismiss(context, { outsidePress: true, escapeKey: true }),
    useRole(context, { role: 'dialog' }),
  ]);

  const active = api.activeFilterCount;
  const canSaveView = active > 0 || api.state.q.length > 0;


  // floating-ui hands back *callback ref setters*, not ref objects. Wrapping
  // them keeps the JSX free of member access, which the react-hooks/refs rule
  // (rightly) forbids for real refs.
  const setAnchor = useCallback((node: HTMLElement | null) => anchor.setReference(node), [anchor]);
  const setPopover = useCallback((node: HTMLElement | null) => anchor.setFloating(node), [anchor]);

  return (
    <>
      <button
        ref={setAnchor}
        {...getReferenceProps()}
        type="button"
        data-testid="filters-trigger"
        aria-expanded={open}
        className="h-9 px-3.5 rounded-[10px] text-[13px] font-semibold inline-flex items-center gap-[7px] transition-all hover:bg-hover shrink-0"
        style={{
          border: `1px solid ${active ? 'var(--accent)' : 'var(--border-strong)'}`,
          background: active ? 'var(--accent-soft)' : 'var(--bg-elevated)',
          color: active ? 'var(--accent)' : 'var(--fg-primary)',
        }}
      >
        <Filter size={16} strokeWidth={1.8} />
        {labels.filters}
        {active > 0 && (
          <span
            data-testid="filters-active-count"
            className="mono text-[11px] font-bold px-1.5 rounded-full"
            style={{ background: 'var(--accent)', color: 'var(--fg-inverse)' }}
          >
            {active}
          </span>
        )}
      </button>

      {open && (
        <FloatingPortal>
          <FloatingFocusManager context={context} modal={false}>
            <div
              ref={setPopover}
              style={{
                ...floatingStyles,
                zIndex: 120,
                width: 320,
                overflowY: 'auto',
                background: 'var(--bg-elevated)',
                border: '1px solid var(--border)',
                borderRadius: 14,
                boxShadow: 'var(--shadow-lg)',
                animation: 'or-scalein .14s cubic-bezier(.2,.8,.2,1)',
              }}
              data-testid="filters-panel"
              aria-label={labels.filters}
              {...getFloatingProps()}
            >
              <div
                className="flex items-center justify-between px-4 py-3 sticky top-0"
                style={{ borderBottom: '1px solid var(--border)', background: 'var(--bg-elevated)' }}
              >
                <span className="text-[13.5px] font-semibold text-ink">{labels.filters}</span>
                <button type="button" onClick={() => setOpen(false)} aria-label={labels.close} className="text-ink-muted hover:text-ink">
                  <X size={16} />
                </button>
              </div>

              <div className="px-4 py-3 space-y-4">
                {facets.map((facet) => {
                  const selected = api.state.filters[facet.key] ?? [];
                  return (
                    <fieldset key={facet.key} data-testid={`facet-${facet.key}`}>
                      <legend className="text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted mb-1.5">
                        {facet.label}
                      </legend>
                      <div className="flex flex-wrap gap-1.5">
                        {facet.options.map((opt) => {
                          const on = selected.includes(opt.value);
                          return (
                            <button
                              key={opt.value}
                              type="button"
                              role="checkbox"
                              aria-checked={on}
                              data-testid={`facet-${facet.key}-${opt.value}`}
                              onClick={() => api.toggleFilter(facet.key, opt.value, facet.single)}
                              className="h-[28px] px-2.5 rounded-full text-[12px] font-semibold inline-flex items-center gap-1.5 transition-colors"
                              style={{
                                border: `1px solid ${on ? 'transparent' : 'var(--border)'}`,
                                background: on
                                  ? `color-mix(in srgb, ${opt.color ?? 'var(--accent)'} 16%, transparent)`
                                  : 'transparent',
                                color: on ? (opt.color ?? 'var(--accent)') : 'var(--fg-secondary)',
                              }}
                            >
                              {on && <Check size={12} />}
                              {opt.label}
                            </button>
                          );
                        })}
                      </div>
                    </fieldset>
                  );
                })}
              </div>

              {/* Saved views — a named filter combination the user can come back to. */}
              <div className="px-4 py-3" style={{ borderTop: '1px solid var(--border)' }}>
                <div className="text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted mb-2 flex items-center gap-1.5">
                  <Star size={12} /> {labels.savedViews}
                </div>
                {views.length > 0 && (
                  <div className="space-y-1 mb-2">
                    {views.map((v) => (
                      <div key={v.id} className="flex items-center gap-1">
                        <button
                          type="button"
                          data-testid={`saved-view-${v.name}`}
                          onClick={() => { api.apply(v.state); setOpen(false); }}
                          className="flex-1 text-left h-8 px-2.5 rounded-[8px] text-[12.5px] font-medium text-ink hover:bg-hover transition-colors"
                        >
                          {v.name}
                        </button>
                        <button
                          type="button"
                          onClick={() => onDeleteView(v.id)}
                          aria-label={`${labels.delete} ${v.name}`}
                          className="w-8 h-8 rounded-[8px] inline-flex items-center justify-center text-ink-muted hover:bg-hover"
                        >
                          <Trash2 size={14} />
                        </button>
                      </div>
                    ))}
                  </div>
                )}
                <form
                  className="flex gap-1.5"
                  onSubmit={(e) => {
                    e.preventDefault();
                    const name = viewName.trim();
                    if (!name || !canSaveView) return;
                    onSaveView(name, { q: api.state.q, filters: api.state.filters, sort: api.state.sort });
                    setViewName('');
                  }}
                >
                  <input
                    value={viewName}
                    onChange={(e) => setViewName(e.target.value)}
                    placeholder={labels.viewNamePlaceholder}
                    aria-label={labels.viewNamePlaceholder}
                    data-testid="saved-view-name"
                    className="flex-1 h-8 px-2.5 rounded-[8px] text-[12.5px] text-ink outline-none"
                    style={{ background: 'var(--bg-secondary)', border: '1px solid var(--border)' }}
                  />
                  <button
                    type="submit"
                    disabled={!canSaveView || !viewName.trim()}
                    data-testid="saved-view-save"
                    className="h-8 px-2.5 rounded-[8px] text-[12px] font-semibold disabled:opacity-40"
                    style={{ background: 'var(--accent-soft)', color: 'var(--accent-500)' }}
                  >
                    {labels.save}
                  </button>
                </form>
                {!canSaveView && (
                  <p className="text-[11px] text-ink-muted mt-1.5">{labels.saveCurrent}</p>
                )}
              </div>

              <div
                className="flex items-center justify-between px-4 py-3 sticky bottom-0"
                style={{ borderTop: '1px solid var(--border)', background: 'var(--bg-elevated)' }}
              >
                <span className="text-[12.5px] font-semibold text-ink" data-testid="filters-result-count">
                  {labels.results(resultCount)}
                </span>
                <button
                  type="button"
                  onClick={() => api.clearFilters()}
                  disabled={!canSaveView}
                  data-testid="filters-reset"
                  className="h-8 px-2.5 rounded-[8px] text-[12.5px] font-semibold inline-flex items-center gap-1.5 disabled:opacity-40 hover:bg-hover"
                  style={{ color: 'var(--fg-secondary)' }}
                >
                  <RotateCcw size={13} /> {labels.reset}
                </button>
              </div>
            </div>
          </FloatingFocusManager>
        </FloatingPortal>
      )}
    </>
  );
}

/** Removable chips for the active facets, shown under the toolbar. */
export function ActiveFilterChips<T>({
  facets,
  state,
  onRemove,
  onClear,
  clearLabel,
}: {
  facets: Facet<T>[];
  state: TableState;
  onRemove: (facetKey: string, value: string) => void;
  onClear: () => void;
  clearLabel: string;
}) {
  const chips = facets.flatMap((facet) =>
    (state.filters[facet.key] ?? []).map((value) => ({
      facetKey: facet.key,
      value,
      label: `${facet.label}: ${facet.options.find((o) => o.value === value)?.label ?? value}`,
      color: facet.options.find((o) => o.value === value)?.color,
    })),
  );
  if (chips.length === 0) return null;
  return (
    <div className="flex flex-wrap items-center gap-1.5 mb-3" data-testid="active-filter-chips">
      {chips.map((c) => (
        <button
          key={`${c.facetKey}:${c.value}`}
          type="button"
          onClick={() => onRemove(c.facetKey, c.value)}
          className="h-[26px] px-2.5 rounded-full text-[11.5px] font-semibold inline-flex items-center gap-1.5"
          style={{
            background: `color-mix(in srgb, ${c.color ?? 'var(--accent)'} 14%, transparent)`,
            color: c.color ?? 'var(--accent)',
          }}
        >
          {c.label} <X size={12} />
        </button>
      ))}
      <button type="button" onClick={onClear} className="text-[11.5px] font-semibold text-ink-muted hover:text-ink px-1.5">
        {clearLabel}
      </button>
    </div>
  );
}
