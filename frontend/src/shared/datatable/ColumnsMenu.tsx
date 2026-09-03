// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Column picker: show / hide / reorder, persisted per user (localStorage, keyed
// by the table id). Reordering is done with explicit up/down buttons rather
// than drag-and-drop: it is operable by keyboard and by screen reader, which a
// pointer-only drag handle is not.

import { useCallback, useState } from 'react';
import { ArrowDown, ArrowUp, Columns3, Eye, EyeOff, RotateCcw } from 'lucide-react';
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
import type { Column } from './types';

interface ColumnsMenuProps<T> {
  columns: Column<T>[];
  order: string[];
  hidden: string[];
  onToggle: (key: string) => void;
  onMove: (key: string, delta: -1 | 1) => void;
  onReset: () => void;
  labels: { columns: string; reset: string; show: string; hide: string; up: string; down: string };
}

export function ColumnsMenu<T>({
  columns,
  order,
  hidden,
  onToggle,
  onMove,
  onReset,
  labels,
}: ColumnsMenuProps<T>) {
  const [open, setOpen] = useState(false);
  const {
    refs: anchor,
    floatingStyles,
    context,
  } = useFloating({
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
          Object.assign(elements.floating.style, {
            maxHeight: `${Math.max(200, availableHeight)}px`,
            overflowY: 'auto',
          });
        },
      }),
    ],
  });
  const { getReferenceProps, getFloatingProps } = useInteractions([
    useClick(context),
    useDismiss(context, { outsidePress: true, escapeKey: true }),
    useRole(context, { role: 'dialog' }),
  ]);

  const ordered = order
    .map((key) => columns.find((c) => c.key === key))
    .filter((c): c is Column<T> => !!c);

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
        data-testid="columns-trigger"
        aria-expanded={open}
        aria-label={labels.columns}
        title={labels.columns}
        className="h-9 w-9 rounded-[10px] inline-flex items-center justify-center transition-colors hover:bg-hover shrink-0"
        style={{
          border: '1px solid var(--border-strong)',
          background: 'var(--bg-elevated)',
          color: 'var(--fg-secondary)',
        }}
      >
        <Columns3 size={16} strokeWidth={1.8} />
      </button>

      {open && (
        <FloatingPortal>
          <FloatingFocusManager context={context} modal={false}>
            <div
              ref={setPopover}
              style={{
                ...floatingStyles,
                zIndex: 120,
                width: 268,
                background: 'var(--bg-elevated)',
                border: '1px solid var(--border)',
                borderRadius: 14,
                boxShadow: 'var(--shadow-lg)',
                padding: 6,
                animation: 'or-scalein .14s cubic-bezier(.2,.8,.2,1)',
              }}
              data-testid="columns-panel"
              aria-label={labels.columns}
              {...getFloatingProps()}
            >
              <div className="px-2 py-1.5 text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted">
                {labels.columns}
              </div>
              {ordered.map((col, i) => {
                const isHidden = hidden.includes(col.key);
                const lockable = col.hideable === false;
                const name =
                  col.headerLabel ?? (typeof col.header === 'string' ? col.header : col.key);
                return (
                  <div
                    key={col.key}
                    className="flex items-center gap-1 px-1 py-0.5"
                    data-testid={`column-row-${col.key}`}
                  >
                    <button
                      type="button"
                      disabled={lockable}
                      onClick={() => onToggle(col.key)}
                      aria-pressed={!isHidden}
                      aria-label={`${isHidden ? labels.show : labels.hide} — ${name}`}
                      data-testid={`column-toggle-${col.key}`}
                      className="flex-1 flex items-center gap-2 h-8 px-2 rounded-[8px] text-[12.5px] font-medium text-left hover:bg-hover disabled:opacity-45 disabled:pointer-events-none"
                      style={{ color: isHidden ? 'var(--fg-muted)' : 'var(--fg-primary)' }}
                    >
                      {isHidden ? <EyeOff size={14} /> : <Eye size={14} />}
                      <span className="truncate">{name}</span>
                    </button>
                    <button
                      type="button"
                      onClick={() => onMove(col.key, -1)}
                      disabled={i === 0}
                      aria-label={`${labels.up} — ${name}`}
                      className="w-7 h-7 rounded-[7px] inline-flex items-center justify-center text-ink-muted hover:bg-hover disabled:opacity-30 disabled:pointer-events-none"
                    >
                      <ArrowUp size={13} />
                    </button>
                    <button
                      type="button"
                      onClick={() => onMove(col.key, 1)}
                      disabled={i === ordered.length - 1}
                      aria-label={`${labels.down} — ${name}`}
                      className="w-7 h-7 rounded-[7px] inline-flex items-center justify-center text-ink-muted hover:bg-hover disabled:opacity-30 disabled:pointer-events-none"
                    >
                      <ArrowDown size={13} />
                    </button>
                  </div>
                );
              })}
              <button
                type="button"
                onClick={onReset}
                className="mt-1 w-full h-8 rounded-[8px] text-[12px] font-semibold inline-flex items-center justify-center gap-1.5 hover:bg-hover"
                style={{ color: 'var(--fg-secondary)' }}
              >
                <RotateCcw size={13} /> {labels.reset}
              </button>
            </div>
          </FloatingFocusManager>
        </FloatingPortal>
      )}
    </>
  );
}
