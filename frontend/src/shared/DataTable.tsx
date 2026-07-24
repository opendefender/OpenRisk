// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Dense "console" table (docs/UI_ELEVATION_PROPOSAL §6): density-aware (via the
// --den-* tokens), sortable headers, frozen first column, optional row selection,
// row = navigation. Zero `any`; fully generic over the row type.

import { useMemo, useState, type ReactNode } from 'react';
import { ChevronUp, ChevronDown } from 'lucide-react';

export interface Column<T> {
  key: string;
  header: ReactNode;
  render: (row: T) => ReactNode;
  /** Provide to make the column sortable. */
  sortValue?: (row: T) => string | number;
  align?: 'left' | 'right';
  /** Sticky first column at horizontal scroll. */
  frozen?: boolean;
  width?: number | string;
}

interface DataTableProps<T> {
  rows: T[];
  columns: Column<T>[];
  rowKey: (row: T) => string;
  onRowClick?: (row: T) => void;
  selectable?: boolean;
  selected?: Set<string>;
  onSelectedChange?: (next: Set<string>) => void;
  minWidth?: number;
  initialSort?: { key: string; dir: 'asc' | 'desc' };
  /** Rendered when there are no rows (pass an <EmptyState/>). */
  empty?: ReactNode;
}

export function DataTable<T>({
  rows,
  columns,
  rowKey,
  onRowClick,
  selectable = false,
  selected,
  onSelectedChange,
  minWidth = 720,
  initialSort,
  empty,
}: DataTableProps<T>) {
  const [sort, setSort] = useState<{ key: string; dir: 'asc' | 'desc' } | null>(initialSort ?? null);

  const sorted = useMemo(() => {
    if (!sort) return rows;
    const col = columns.find((c) => c.key === sort.key);
    if (!col?.sortValue) return rows;
    const get = col.sortValue;
    const factor = sort.dir === 'asc' ? 1 : -1;
    return [...rows].sort((a, b) => {
      const va = get(a);
      const vb = get(b);
      if (va < vb) return -1 * factor;
      if (va > vb) return 1 * factor;
      return 0;
    });
  }, [rows, columns, sort]);

  const toggleSort = (key: string) => {
    setSort((prev) => {
      if (prev?.key !== key) return { key, dir: 'desc' };
      if (prev.dir === 'desc') return { key, dir: 'asc' };
      return null; // third click clears
    });
  };

  const allSelected = selectable && rows.length > 0 && rows.every((r) => selected?.has(rowKey(r)));
  const toggleAll = () => {
    if (!onSelectedChange) return;
    onSelectedChange(allSelected ? new Set() : new Set(rows.map(rowKey)));
  };
  const toggleOne = (id: string) => {
    if (!onSelectedChange) return;
    const next = new Set(selected);
    next.has(id) ? next.delete(id) : next.add(id);
    onSelectedChange(next);
  };

  if (rows.length === 0 && empty) return <>{empty}</>;

  return (
    <div style={{ overflowX: 'auto' }}>
      <table className="or-table" style={{ minWidth }}>
        <thead>
          <tr>
            {selectable && (
              <th style={{ width: 36 }}>
                <input
                  type="checkbox"
                  aria-label="Tout sélectionner"
                  checked={allSelected}
                  onChange={toggleAll}
                  style={{ accentColor: 'var(--accent)', width: 15, height: 15 }}
                />
              </th>
            )}
            {columns.map((c) => {
              const active = sort?.key === c.key;
              return (
                <th
                  key={c.key}
                  className={[c.sortValue ? 'sortable' : '', c.align === 'right' ? 'num' : ''].join(' ').trim()}
                  style={{ width: c.width }}
                  onClick={c.sortValue ? () => toggleSort(c.key) : undefined}
                >
                  <span style={{ display: 'inline-flex', alignItems: 'center', gap: 4, justifyContent: c.align === 'right' ? 'flex-end' : 'flex-start' }}>
                    {c.header}
                    {active && (sort!.dir === 'asc' ? <ChevronUp size={13} /> : <ChevronDown size={13} />)}
                  </span>
                </th>
              );
            })}
          </tr>
        </thead>
        <tbody>
          {sorted.map((row) => {
            const id = rowKey(row);
            const isSel = selected?.has(id) ?? false;
            return (
              <tr
                key={id}
                data-selected={isSel || undefined}
                onClick={onRowClick ? () => onRowClick(row) : undefined}
                style={onRowClick ? undefined : { cursor: 'default' }}
              >
                {selectable && (
                  <td onClick={(e) => { e.stopPropagation(); toggleOne(id); }}>
                    <input
                      type="checkbox"
                      aria-label="Sélectionner la ligne"
                      checked={isSel}
                      onChange={() => toggleOne(id)}
                      onClick={(e) => e.stopPropagation()}
                      style={{ accentColor: 'var(--accent)', width: 15, height: 15 }}
                    />
                  </td>
                )}
                {columns.map((c) => (
                  <td key={c.key} className={[c.frozen ? 'frozen' : '', c.align === 'right' ? 'num' : ''].join(' ').trim()}>
                    {c.render(row)}
                  </td>
                ))}
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
