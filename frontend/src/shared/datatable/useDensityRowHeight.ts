// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * The table's row height, taken from the density token rather than hardcoded.
 *
 * `.or-table tbody td` has said `height: var(--den-row)` since the table was
 * written, and `--den-row` is 40px in comfort, 32px in compact and 48px in
 * spacious. But DataTable also set `style={{ height: 48 }}` on every `<tr>`, and
 * in table layout the row box is the MAX of the row's height and its cells' —
 * so the inline 48 won at every density. The density control changed the cell
 * padding and the font and left the row height alone, which is the one dimension
 * "compact" is actually about.
 *
 * The virtualiser needs the height as a NUMBER, not a CSS value, which is why
 * this is a hook and not just a class. The number is read back from the computed
 * custom property instead of being duplicated in JS: primitives.css stays the
 * single source of truth, so changing a density there cannot drift from what the
 * virtualiser measures.
 *
 * Recomputed on every density change, because the value only exists after
 * `data-density` has been stamped on <html> by the store. Derived during render
 * rather than in an effect: an effect would render once at the old height and
 * then again at the new one, which is a visible reflow of every row on a table
 * that may be showing ten thousand.
 */

import { useMemo } from 'react';
import { useUIStore, type Density } from '../../store/uiStore';

/*
 * Used only when the DOM cannot be measured — SSR, or a token that has gone
 * missing. NOT the source of truth: primitives.css is, and these exist so a
 * token typo yields a slightly wrong table rather than an invisible one with
 * every row collapsed to zero.
 */
const FALLBACK_ROW_PX: Record<Density, number> = { comfort: 40, compact: 32, spacious: 48 };

function readRowHeight(density: Density): number {
  const fallback = FALLBACK_ROW_PX[density] ?? FALLBACK_ROW_PX.comfort;
  if (typeof document === 'undefined') return fallback;
  const raw = getComputedStyle(document.documentElement).getPropertyValue('--den-row');
  const px = Number.parseFloat(raw);
  return Number.isFinite(px) && px > 0 ? px : fallback;
}

/**
 * @param override Caller-supplied height. Wins, so a table with unusual rows
 *   (two-line cells, thumbnails) can still opt out of the density ramp.
 */
export function useDensityRowHeight(override?: number): number {
  const density = useUIStore((s) => s.density);
  const rowHeight = useMemo(() => readRowHeight(density), [density]);
  return override ?? rowHeight;
}
