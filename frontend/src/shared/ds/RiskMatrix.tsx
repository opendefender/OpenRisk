// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

/**
 * RiskMatrix — the 5×5 probability × impact grid.
 *
 * WHY THIS IS A TABLE AND NOT A CHART
 * -----------------------------------
 * A risk matrix is a cross-tabulation: two ordered axes, one cell per pair, a
 * count in each. That is a table, and rendering it as one means the axes are
 * real `<th>` headers, so a screen reader announces "Probability 4, Impact 3"
 * when it reaches a cell instead of reading twenty-five unlabelled divs. It
 * also means no charting library is involved — this is 25 cells, not a plot.
 *
 * D-024 records the reason explicitly: the component this was going to be
 * vendored from (bklit ui's Heatmap) is a *calendar* heatmap — it imports
 * `scaleTime` and ships week-start/week-range helpers — so it renders dates,
 * not a categorical 5×5. The matrix is built; only the charts are vendored.
 *
 * WHY `cellBand` IS A REQUIRED PROP AND NOT A DEFAULT
 * --------------------------------------------------
 * This component holds NO thresholds. `shared/riskColors.ts` records why:
 * four incompatible number→band mappings once shipped at once (≥7/≥4/≥2,
 * ≥15/≥8/≥4, ≥15/≥9/≥5, ≥40), each calibrated for a different scale than the
 * number displayed beside it. They were deleted rather than reconciled, on the
 * principle that as long as a component CAN derive a band it eventually will,
 * with its own thresholds. So the caller supplies the mapping and owns it.
 *
 * The same rule applies to each item: `band` arrives with the value it
 * describes rather than being computed from `score` here.
 *
 * A11Y
 * ----
 * Colour is never the only encoding (WCAG 1.4.1). Every cell carries its band
 * as visually-hidden text, so a cell that is empty — where there is no marker
 * to read — still announces its severity. Markers are real buttons with
 * accessible names, so the grid is keyboard-traversable with Tab.
 *
 * NOT MODELLED: residual position. #444 asks for "inherent vs residual", but a
 * `Risk` carries one probability/impact pair. The domain has residual ALE
 * (CRQ) and a `residual_accepted` state, and neither is a residual coordinate.
 * Rendering a second layer would mean inventing the data, so this component
 * takes one set of positions and a backend field has to exist first.
 */

import type { ReactNode } from 'react';
import { cn } from './cn';
import { severity, type SeverityKey } from './chart';

/** The 1..5 buckets of either axis. */
export type MatrixBucket = 1 | 2 | 3 | 4 | 5;

export interface RiskMatrixItem {
  id: string;
  /** Accessible name for the marker — the risk's title. */
  label: string;
  probability: MatrixBucket;
  impact: MatrixBucket;
  /** Band for the marker. From the server, beside the value it describes. */
  band: SeverityKey;
  /** Short text inside the marker, e.g. a rounded score. Optional. */
  marker?: string;
}

export interface RiskMatrixLabels {
  /** Names the table for assistive tech, e.g. "Risk matrix, probability by impact". */
  caption: string;
  probability: string;
  impact: string;
  /** Renders the per-band text carried by every cell. */
  band: (band: SeverityKey) => string;
  /** Renders the overflow marker when a cell holds more than `maxPerCell`. */
  more: (count: number) => string;
}

export interface RiskMatrixProps {
  items: RiskMatrixItem[];
  /**
   * Band for the CELL at (probability, impact). Required — see the note above.
   * Pure and positional: it must not look at the items.
   */
  cellBand: (probability: MatrixBucket, impact: MatrixBucket) => SeverityKey;
  labels: RiskMatrixLabels;
  /** Called when a marker is activated. Markers are inert when omitted. */
  onSelect?: (id: string) => void;
  /** Markers drawn before collapsing into a "+n". */
  maxPerCell?: number;
  className?: string;
}

const BUCKETS: MatrixBucket[] = [1, 2, 3, 4, 5];
/* Probability runs bottom-to-top, the convention every GRC matrix uses: the
   worst corner is top-right, where the eye lands last and stays. */
const ROWS: MatrixBucket[] = [5, 4, 3, 2, 1];

/**
 * A translucent wash of `color`.
 *
 * Inlined for the same reason `Empty.tsx` inlines it: an Apache-2.0 file may
 * not depend on `shared/riskColors` (AGPL), and the licence-boundary script
 * reads headers rather than imports, so that violation would pass CI silently.
 */
function tint(color: string, pct: number): string {
  return `color-mix(in srgb, ${color} ${pct}%, transparent)`;
}

/** Visually hidden, still announced. */
function SrOnly({ children }: { children: ReactNode }) {
  return <span className="sr-only">{children}</span>;
}

/* `relative min-w-0 overflow-x-auto` on the scroll container, and all three earn
   their place:
   - `overflow-x-auto` so a 640px-floor matrix scrolls in its own box.
   - `min-w-0` because as a grid/flex child its min-width would resolve to auto,
     i.e. min-content, and the box would grow to the table instead of scrolling.
   - `relative` because the sr-only cell labels are `position: absolute`. With no
     positioned ancestor their containing block is the viewport, and overflow
     does NOT clip an absolutely positioned descendant it is not the containing
     block for — so they sat at their static x, past the right edge, and the PAGE
     scrolled sideways on a narrow viewport while the matrix itself did not. */
export function RiskMatrix({
  items,
  cellBand,
  labels,
  onSelect,
  maxPerCell = 6,
  className,
}: RiskMatrixProps) {
  /* One pass into a keyed bucket map, rather than filtering the list 25 times. */
  const grid = new Map<string, RiskMatrixItem[]>();
  for (const item of items) {
    const key = `${item.probability}:${item.impact}`;
    const cell = grid.get(key);
    if (cell) cell.push(item);
    else grid.set(key, [item]);
  }

  return (
    <div className={cn('relative min-w-0 overflow-x-auto', className)}>
      {/* table-fixed: a matrix is a uniform grid. Without it the columns size to
          their contents, so a cell holding five markers renders wider than its
          neighbours and the axis stops being evenly spaced. */}
      <table
        className="w-full table-fixed border-separate border-spacing-1.5"
        style={{ minWidth: 640 }}
      >
        <caption className="sr-only">{labels.caption}</caption>
        <thead>
          <tr>
            {/* Corner: labels the row-header column, which is the probability axis. */}
            <th
              scope="col"
              className="w-8 text-2xs font-semibold uppercase tracking-caps text-fg-muted"
            >
              <SrOnly>{labels.probability}</SrOnly>
            </th>
            {BUCKETS.map((impact) => (
              <th
                key={impact}
                scope="col"
                className="text-center text-2xs font-semibold text-fg-muted"
              >
                {impact}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {ROWS.map((probability) => (
            <tr key={probability}>
              <th scope="row" className="w-8 text-center text-2xs font-semibold text-fg-muted">
                {probability}
              </th>
              {BUCKETS.map((impact) => {
                const band = cellBand(probability, impact);
                const colour = severity[band];
                const cell = grid.get(`${probability}:${impact}`) ?? [];
                const shown = cell.slice(0, maxPerCell);
                const overflow = cell.length - shown.length;
                return (
                  <td
                    key={impact}
                    className="min-h-[84px] h-[84px] rounded-md p-1.5 align-top"
                    style={{
                      background: tint(colour, 10),
                      border: `1px solid ${tint(colour, 22)}`,
                    }}
                  >
                    {/* Carries the band for a cell with no marker to read, so
                        severity is never conveyed by fill alone. */}
                    <SrOnly>{labels.band(band)}</SrOnly>
                    <div className="flex flex-wrap content-start gap-1">
                      {shown.map((item) => {
                        const markerColour = severity[item.band];
                        const name = `${item.label} — ${labels.band(item.band)}`;
                        return onSelect ? (
                          <button
                            key={item.id}
                            type="button"
                            onClick={() => onSelect(item.id)}
                            aria-label={name}
                            className="flex h-5 w-5 items-center justify-center rounded-full text-2xs font-bold text-fg-on-solid transition-transform hover:scale-110 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
                            style={{ background: markerColour }}
                          >
                            <span aria-hidden="true">{item.marker}</span>
                          </button>
                        ) : (
                          <span
                            key={item.id}
                            role="img"
                            aria-label={name}
                            className="flex h-5 w-5 items-center justify-center rounded-full text-2xs font-bold text-fg-on-solid"
                            style={{ background: markerColour }}
                          >
                            <span aria-hidden="true">{item.marker}</span>
                          </span>
                        );
                      })}
                      {overflow > 0 && (
                        <span
                          className="self-center text-2xs font-semibold"
                          style={{ color: colour }}
                        >
                          {labels.more(overflow)}
                        </span>
                      )}
                    </div>
                  </td>
                );
              })}
            </tr>
          ))}
        </tbody>
      </table>
      <p className="mt-1 text-center text-2xs font-semibold uppercase tracking-caps text-fg-muted">
        {labels.impact}
      </p>
    </div>
  );
}
