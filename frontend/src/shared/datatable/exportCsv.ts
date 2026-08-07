// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// CSV export shared by every table. Client-side by design: it exports exactly
// what the user is looking at (their columns, in their order, filtered as shown),
// which is what "export the current view" has to mean to be trustworthy.

import type { Column } from './types';

function escapeCell(value: unknown): string {
  if (value == null) return '';
  const s = String(value);
  // Spreadsheet formula injection: a cell starting with = + - @ is executed by
  // Excel/Sheets on open. A GRC export lands in an analyst's spreadsheet, so
  // neutralise it rather than ship a CSV-injection vector.
  const guarded = /^[=+\-@\t\r]/.test(s) ? `'${s}` : s;
  return /[",\n;]/.test(guarded) ? `"${guarded.replace(/"/g, '""')}"` : guarded;
}

function headerText<T>(col: Column<T>): string {
  if (col.headerLabel) return col.headerLabel;
  return typeof col.header === 'string' ? col.header : col.key;
}

function cellText<T>(col: Column<T>, row: T): string {
  if (col.exportValue) {
    const v = col.exportValue(row);
    return v == null ? '' : String(v);
  }
  if (col.sortValue) return String(col.sortValue(row));
  return '';
}

/** Builds the CSV text for the given rows and (visible, ordered) columns. */
export function buildCsv<T>(rows: T[], columns: Column<T>[]): string {
  const exportable = columns.filter((c) => c.exportValue || c.sortValue);
  const header = exportable.map((c) => escapeCell(headerText(c))).join(',');
  const body = rows.map((row) => exportable.map((c) => escapeCell(cellText(c, row))).join(','));
  return [header, ...body].join('\n');
}

/** Triggers a browser download of `content` as `filename`. */
export function downloadCsv(filename: string, content: string): void {
  // BOM so Excel opens accented French headers correctly.
  const BOM = '\uFEFF';
  const blob = new Blob([`${BOM}${content}`], { type: 'text/csv;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  a.rel = 'noopener';
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

export function exportRowsToCsv<T>(filename: string, rows: T[], columns: Column<T>[]): void {
  downloadCsv(filename, buildCsv(rows, columns));
}
