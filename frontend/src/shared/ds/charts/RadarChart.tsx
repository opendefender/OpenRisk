// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

/**
 * RadarChart — one shape per series over a shared set of labelled axes.
 *
 * Used for maturity per framework and for the SmartScore's eight weighted
 * factors: cases where the SHAPE is the reading ("strong everywhere except
 * detection") rather than any individual value. Where the individual values are
 * the point, a bar chart beats this and `CartesianChart` is next door.
 *
 * NO visx DEPENDENCY. A radar is a polygon on a polar grid: `Math.cos`/`Math.sin`
 * and a `<polygon>`. Pulling a package for that would add to the charts chunk
 * for arithmetic already in the standard library.
 *
 * SCALE — every axis shares one domain, `[0, max]`, and the rings are drawn at
 * quarters of it. Per-axis domains are what make a radar dishonest: the shape
 * then encodes normalisation rather than data, and two charts of the same
 * subject stop being comparable. `max` is a required prop for the same reason
 * a bar axis starts at zero — the caller states the scale, the chart does not
 * infer a flattering one.
 *
 * COLOUR follows `chart.ts`: `band` takes the risk scale, otherwise the
 * categorical palette by index. No raw-colour prop.
 */

import { cn } from '../cn';
import { chartAxis, chartGrid, seriesColor, severity, type SeverityKey } from '../chart';

export interface RadarAxis {
  key: string;
  label: string;
}

export interface RadarSeries {
  key: string;
  label: string;
  /** One value per axis, in the same order as `axes`. */
  values: number[];
  band?: SeverityKey;
}

export interface RadarChartProps {
  axes: RadarAxis[];
  series: RadarSeries[];
  /** Top of the shared domain. Required — see the header. */
  max: number;
  size?: number;
  ariaLabel: string;
  formatValue?: (v: number) => string;
  className?: string;
}

const RINGS = [0.25, 0.5, 0.75, 1];

function colourFor(s: RadarSeries, index: number): string {
  return s.band ? severity[s.band] : seriesColor(index);
}

export function RadarChart({
  axes,
  series,
  max,
  size = 280,
  ariaLabel,
  formatValue = (v) => String(v),
  className,
}: RadarChartProps) {
  const cx = size / 2;
  const cy = size / 2;
  /* Room for the axis labels outside the outermost ring.
     An SVG clips at its viewport, so this padding is what stops a long label
     like "ISO 27001" or "COBAC" being cut in half at the edge — 34px was not
     enough for real framework names, which is exactly the set this chart is
     for. Sized for roughly ten characters at the axis font size. */
  const LABEL_PAD = 56;
  const radius = Math.max(0, size / 2 - LABEL_PAD);
  const n = axes.length;

  /* Start at twelve o'clock and go clockwise, which is how people read a dial
     and therefore the order they will assume the labels are in. */
  const angleAt = (i: number) => (i / n) * Math.PI * 2 - Math.PI / 2;
  const pointAt = (i: number, r: number) => {
    const a = angleAt(i);
    return [cx + Math.cos(a) * r, cy + Math.sin(a) * r] as const;
  };
  const ratio = (v: number) => (max > 0 ? Math.max(0, Math.min(1, v / max)) : 0);

  const ringPoints = (r: number) =>
    axes.map((_, i) => pointAt(i, r).join(',')).join(' ');

  /* The text alternative is wrapped in an sr-only DIV rather than carrying
     sr-only itself.
     `sr-only` hides by pinning the box to 1px and clipping it — and a <table>
     does not shrink below its min-content width, so the class silently fails on
     one: the table stayed ~450px, escaped its 317px parent, and scrolled the
     PAGE sideways at 390px. A div shrinks as told and clips the table inside it.
     `relative` keeps the absolutely-positioned box anchored here rather than to
     the viewport, the same trap RiskMatrix hit. */
  return (
    <div className={cn('relative w-full', className)}>
      <svg width={size} height={size} role="img" aria-label={ariaLabel} className="mx-auto block">
        {/* 1. Grid — rings and spokes, below everything. */}
        {RINGS.map((f) => (
          <polygon
            key={f}
            points={ringPoints(radius * f)}
            fill="none"
            stroke={chartGrid.stroke}
          />
        ))}
        {axes.map((a, i) => {
          const [x, y] = pointAt(i, radius);
          return <line key={a.key} x1={cx} y1={cy} x2={x} y2={y} stroke={chartAxis.stroke} />;
        })}

        {/* 2. Axis labels. */}
        {axes.map((a, i) => {
          const [x, y] = pointAt(i, radius + 16);
          const anchor = Math.abs(x - cx) < 1 ? 'middle' : x > cx ? 'start' : 'end';
          return (
            <text
              key={a.key}
              x={x}
              y={y}
              textAnchor={anchor}
              dominantBaseline="middle"
              fill={chartAxis.tick.fill}
              fontSize={chartAxis.tick.fontSize}
            >
              {a.label}
            </text>
          );
        })}

        {/* 3. Data — last, so the grid never sits on top of it. */}
        {series.map((s, si) => {
          const colour = colourFor(s, si);
          const points = axes
            .map((_, i) => pointAt(i, radius * ratio(s.values[i] ?? 0)).join(','))
            .join(' ');
          return (
            <polygon
              key={s.key}
              points={points}
              fill={colour}
              fillOpacity={0.16}
              stroke={colour}
              strokeWidth={2}
            />
          );
        })}
      </svg>

      {series.length > 1 && (
        <ul className="mt-2 flex flex-wrap justify-center gap-x-4 gap-y-1">
          {series.map((s, si) => (
            <li key={s.key} className="flex items-center gap-1.5 text-2xs text-fg-muted">
              <svg width="10" height="10" aria-hidden="true">
                <rect width="10" height="10" rx="2" fill={colourFor(s, si)} />
              </svg>
              {s.label}
            </li>
          ))}
        </ul>
      )}

      {/* The numbers. A polygon is unreadable to assistive tech, and the shape
          is the whole point of this chart — so the values must exist as text. */}
      <div className="sr-only">
        <table>
        <caption>{ariaLabel}</caption>
        <thead>
          <tr>
            <th scope="col">Axis</th>
            {series.map((s) => (
              <th key={s.key} scope="col">
                {s.label}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {axes.map((a, i) => (
            <tr key={a.key}>
              <th scope="row">{a.label}</th>
              {series.map((s) => (
                <td key={s.key}>{formatValue(s.values[i] ?? 0)}</td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
      </div>
    </div>
  );
}
