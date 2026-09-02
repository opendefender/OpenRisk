// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

/**
 * PieChart — a part-to-whole split of at most three slices.
 *
 * THE THREE-SLICE LIMIT IS A TYPE, NOT A WARNING
 * ----------------------------------------------
 * The design guide bans "a pie chart with more than three slices", and #444's
 * anti-cliché table repeats it: permitted at ≤3 only. A runtime warning would
 * be discovered in review, or not at all. `slices` is a tuple union instead, so
 * a fourth slice does not compile.
 *
 * The reason is readability, not taste: people compare angles poorly, and past
 * three wedges the ranking of the middle ones stops being legible — which is
 * the one thing a distribution chart exists to show. Four or more categories is
 * a bar chart, and `CartesianChart` is next door.
 *
 * COLOUR follows `chart.ts`'s two rules, as elsewhere: a slice that names a
 * `band` takes the risk scale, and anything else takes the categorical palette
 * by index. There is no prop for a raw colour — the dashboards this replaces
 * passed `fill="#ef4444"` per `<Cell>`, which is how a category ends up
 * wearing a verdict.
 */

import { Pie } from '@visx/shape';
import { Group } from '@visx/group';
import { ParentSize } from '@visx/responsive';

import { cn } from '../cn';
import { seriesColor, severity, type SeverityKey } from '../chart';

export interface PieSlice {
  key: string;
  label: string;
  value: number;
  /** Set only when the slice is a severity rather than a category. */
  band?: SeverityKey;
}

/** At most three. See the header — this is the guide's rule, as a type. */
export type PieSlices = readonly [PieSlice] | readonly [PieSlice, PieSlice] | readonly [PieSlice, PieSlice, PieSlice];

export interface PieChartProps {
  slices: PieSlices;
  height?: number;
  ariaLabel: string;
  formatValue?: (v: number) => string;
  className?: string;
}

function colourFor(s: PieSlice, index: number): string {
  return s.band ? severity[s.band] : seriesColor(index);
}

export function PieChart({
  slices,
  height = 300,
  ariaLabel,
  formatValue = (v) => String(v),
  className,
}: PieChartProps) {
  /* visx's Pie takes a mutable array; the readonly tuple is the public
     contract, so the copy is made once here rather than widening the prop. */
  const list: PieSlice[] = [...(slices as readonly PieSlice[])];
  const total = list.reduce((sum, s) => sum + (Number.isFinite(s.value) ? s.value : 0), 0);

  return (
    <div className={cn('w-full', className)}>
      <div style={{ height }}>
        {/* debounceTime={0}: ParentSize defaults to a 300ms debounce before its FIRST
            measurement, so the chart renders nothing at all for that third of a
            second — a visible blank on every dashboard load, and a screenshot
            race that made this gallery flaky in the visual suite. Debouncing is
            worth having on resize, not on mount. */}
        <ParentSize debounceTime={0}>
          {({ width }) => {
            if (width <= 0) return null;
            const radius = Math.max(0, Math.min(width, height) / 2 - 8);
            return (
              <svg width={width} height={height} role="img" aria-label={ariaLabel}>
                <Group top={height / 2} left={width / 2}>
                  <Pie
                    data={list}
                    pieValue={(s) => s.value}
                    outerRadius={radius}
                    innerRadius={0}
                    padAngle={0.004}
                  >
                    {(pie) =>
                      pie.arcs.map((arc, i) => {
                        const path = pie.path(arc);
                        const [cx, cy] = pie.path.centroid(arc);
                        const slice = arc.data;
                        /* Direct labels rather than a legend: a wedge has room,
                           and a legend would make the reader look twice. Hidden
                           when the wedge is too thin to hold the text. */
                        const wide = arc.endAngle - arc.startAngle > 0.35;
                        return (
                          <g key={slice.key}>
                            <path d={path ?? undefined} fill={colourFor(slice, i)} />
                            {wide && (
                              <text
                                x={cx}
                                y={cy}
                                textAnchor="middle"
                                dominantBaseline="middle"
                                fill="var(--fg-on-solid)"
                                fontSize={11}
                                fontWeight={600}
                              >
                                {formatValue(slice.value)}
                              </text>
                            )}
                          </g>
                        );
                      })
                    }
                  </Pie>
                </Group>
              </svg>
            );
          }}
        </ParentSize>
      </div>
      {/* Names beside swatches, and the numbers as text. Colour is never the
          only encoding, and a wedge too thin for its label still has a row. */}
      <ul className="mt-2 flex flex-wrap justify-center gap-x-4 gap-y-1">
        {list.map((s, i) => (
          <li key={s.key} className="flex items-center gap-1.5 text-2xs text-fg-muted">
            <svg width="10" height="10" aria-hidden="true">
              <rect width="10" height="10" rx="2" fill={colourFor(s, i)} />
            </svg>
            {s.label} — {formatValue(s.value)}
            {total > 0 && ` (${Math.round((s.value / total) * 100)}%)`}
          </li>
        ))}
      </ul>
    </div>
  );
}
