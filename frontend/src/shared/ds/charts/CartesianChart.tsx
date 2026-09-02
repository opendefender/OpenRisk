// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

/**
 * CartesianChart — area, bar, line and any mix of them, on one frame.
 *
 * Replaces Recharts' AreaChart / BarChart / LineChart / ComposedChart and the
 * furniture they compose with (CartesianGrid, XAxis, YAxis, Tooltip, Legend,
 * ResponsiveContainer): eleven of the twenty-one symbols the six dashboards
 * imported. Built on visx primitives (D-024, option A).
 *
 * WHY EXPLICIT PROPS RATHER THAN RECHARTS' COMPOSABLE CHILDREN
 * -----------------------------------------------------------
 * Recharts reads its configuration out of `React.Children`, so `<Bar>` is not a
 * component that renders a bar — it is a config object wearing JSX. That is why
 * a chart there can be given a `fill="#ef4444"` per series and nothing stops it,
 * which is exactly what the six dashboards did. A `series` array makes the
 * contract explicit and lets the component OWN the things the design guide says
 * are not the caller's to choose:
 *
 *   - Colour comes from `categorical` by series index. A caller cannot pass a
 *     hex, so "a chart series is a CATEGORY, not a verdict" is enforced rather
 *     than documented.
 *   - The y domain starts at zero. There is no prop to turn that off:
 *     "truncating a bar axis is a lie with a chart around it".
 *   - Every line and area series gets a dash pattern from its index alongside
 *     its colour, because "colour is never the only encoding" (WCAG 1.4.1).
 *
 * PAINT ORDER is grid → axis → data, enforced by the order of the JSX below
 * rather than by z-index, so data is never obscured by its own furniture.
 */

import { useMemo, useState, type ReactNode } from 'react';
import { ParentSize } from '@visx/responsive';
import { scaleBand, scaleLinear } from '@visx/scale';
import { AreaClosed, Bar, LinePath } from '@visx/shape';
import { GridRows } from '@visx/grid';
import { Group } from '@visx/group';
import { curveMonotoneX } from '@visx/curve';

import { cn } from '../cn';
import { chartAxis, chartGrid, seriesColor, severity, type SeverityKey } from '../chart';

export type SeriesType = 'area' | 'bar' | 'line';

/** A series' colour: the risk scale when it is a severity, the categorical
    palette otherwise. Callers cannot pass a colour; see `ChartSeries.band`. */
function colourFor<Row>(s: ChartSeries<Row>, index: number): string {
  return s.band ? severity[s.band] : seriesColor(index);
}

export interface ChartSeries<Row> {
  type: SeriesType;
  /** Field on the row holding this series' value. */
  key: keyof Row & string;
  /** Human label, for the legend and the tooltip. */
  label: string;
  /**
   * Set ONLY when the series is a severity, not a category.
   *
   * `chart.ts` states the two rules this implements. A CATEGORY takes the next
   * colour from `categorical`, because a category carries no verdict — a bar
   * per environment coloured red/amber/green reads as an alert about
   * environments. A SEVERITY takes the risk scale deliberately, so a "critical"
   * bar is the same red as a "critical" badge three rows above it.
   *
   * There is no prop for a raw colour, which is what stopped the six dashboards
   * passing `fill="#ef4444"` for a series that was not a severity at all.
   */
  band?: SeverityKey;
}

export interface CartesianChartProps<Row> {
  data: Row[];
  /** Field on the row holding the category shown on the x axis. */
  x: keyof Row & string;
  series: ChartSeries<Row>[];
  /**
   * Stack the bar series instead of grouping them side by side.
   *
   * Only legitimate when the bars are PARTS OF ONE WHOLE — incidents per month
   * split by severity, where the stack total is itself a number someone reads.
   * Stacking unrelated series makes every segment but the bottom one start at an
   * arbitrary offset, and only the bottom one can then be compared across
   * columns.
   */
  stacked?: boolean;
  height?: number;
  /** Formats a y value for the axis and the tooltip. */
  formatValue?: (v: number) => string;
  /** Formats an x category for the axis and the tooltip. */
  formatCategory?: (v: string) => string;
  /**
   * What the chart says. A chart is an image to a screen reader, so this is
   * required — pass the sentence the reader would otherwise have to infer.
   */
  ariaLabel: string;
  /** Rendered instead of the plot when `data` is empty. */
  empty?: ReactNode;
  className?: string;
}

/* Dash patterns, by series index. Index 0 is solid: the first series is the
   subject of most charts and should not look like an annotation. */
const DASHES = [undefined, '6 3', '2 3', '8 3 2 3', '1 3', '10 4', '4 2 1 2', '3 3'];

const MARGIN = { top: 8, right: 16, bottom: 28, left: 44 };

function toNumber(v: unknown): number {
  return typeof v === 'number' && Number.isFinite(v) ? v : 0;
}

interface PlotProps<Row> extends CartesianChartProps<Row> {
  width: number;
  height: number;
}

function Plot<Row extends object>({
  data,
  x,
  series,
  width,
  height,
  formatValue = (v) => String(v),
  formatCategory = (v) => v,
  stacked = false,
  ariaLabel,
}: PlotProps<Row>) {
  const [hover, setHover] = useState<number | null>(null);

  const innerW = Math.max(0, width - MARGIN.left - MARGIN.right);
  const innerH = Math.max(0, height - MARGIN.top - MARGIN.bottom);

  const categories = useMemo(() => data.map((r) => String(r[x])), [data, x]);

  const xScale = useMemo(
    () => scaleBand<string>({ domain: categories, range: [0, innerW], padding: 0.2 }),
    [categories, innerW],
  );

  /* Zero baseline, always. The domain grows to the data but never lifts off 0,
     so bar length stays proportional to value — see the header. */
  const yScale = useMemo(() => {
    const barKeys = series.filter((s) => s.type === 'bar');
    let max = 0;
    for (const row of data) {
      /* A stack is as tall as its total, so the domain has to clear the sum —
         taking the per-series max would clip every column. */
      if (stacked && barKeys.length > 0) {
        let sum = 0;
        for (const s of barKeys) sum += toNumber(row[s.key]);
        max = Math.max(max, sum);
      }
      for (const s of series) max = Math.max(max, toNumber(row[s.key]));
    }
    return scaleLinear<number>({ domain: [0, max === 0 ? 1 : max], range: [innerH, 0], nice: true });
  }, [data, series, innerH, stacked]);

  const bars = series.filter((s) => s.type === 'bar');
  const bandW = xScale.bandwidth();
  const barW = stacked ? bandW : bars.length > 0 ? bandW / bars.length : bandW;

  return (
    <svg width={width} height={height} role="img" aria-label={ariaLabel}>
      <Group left={MARGIN.left} top={MARGIN.top}>
        {/* 1. Grid — below everything. Horizontal only, per the chart contract. */}
        <GridRows
          scale={yScale}
          width={innerW}
          height={innerH}
          stroke={chartGrid.stroke}
          strokeOpacity={1}
          numTicks={4}
        />

        {/* 2. Axes, hand-rolled.
               NOT @visx/axis: it pulls @visx/text -> reduce-css-calc ->
               math-expression-evaluator, none of which matches a lazy-chunk
               rule, so the subtree landed in the PRELOADED vendor chunk and cost
               ~4 KB of the initial budget (D-024 obligation 1). A tick is a
               <text> and a rule is a <line>; the styling already comes from the
               chartAxis tokens either way. */}
        <line x1={0} y1={innerH} x2={innerW} y2={innerH} stroke={chartAxis.stroke} />
        {yScale.ticks(4).map((t) => (
          <text
            key={`y-${t}`}
            x={-8}
            y={yScale(t)}
            textAnchor="end"
            dominantBaseline="middle"
            fill={chartAxis.tick.fill}
            fontSize={chartAxis.tick.fontSize}
          >
            {formatValue(t)}
          </text>
        ))}
        {categories.map((c, ci) => (
          <text
            key={`x-${c}-${ci}`}
            x={(xScale(c) ?? 0) + bandW / 2}
            y={innerH + 16}
            textAnchor="middle"
            fill={chartAxis.tick.fill}
            fontSize={chartAxis.tick.fontSize}
          >
            {formatCategory(c)}
          </text>
        ))}

        {/* 3. Data — last, so nothing is drawn over it. */}
        {series.map((s, si) => {
          const colour = colourFor(s, si);
          if (s.type === 'bar') {
            const slot = bars.indexOf(s);
            return (
              <Group key={s.key}>
                {data.map((row, ri) => {
                  const v = toNumber(row[s.key]);
                  const left = (xScale(categories[ri]) ?? 0) + (stacked ? 0 : slot * barW);
                  /* Stacked: sit on top of the series already drawn below. */
                  const below = stacked
                    ? bars.slice(0, slot).reduce((sum, b) => sum + toNumber(row[b.key]), 0)
                    : 0;
                  const top = yScale(below + v);
                  const bottom = yScale(below);
                  return (
                    <Bar
                      key={`${s.key}-${ri}`}
                      x={left}
                      y={top}
                      width={Math.max(0, barW - 1)}
                      height={Math.max(0, bottom - top)}
                      fill={colour}
                      opacity={hover === null || hover === ri ? 1 : 0.45}
                      rx={2}
                    />
                  );
                })}
              </Group>
            );
          }
          const centre = (_: unknown, ri: number) =>
            (xScale(categories[ri]) ?? 0) + bandW / 2;
          const value = (row: Row) => yScale(toNumber(row[s.key]));
          if (s.type === 'area') {
            return (
              <Group key={s.key}>
                <AreaClosed
                  data={data}
                  x={centre}
                  y={value}
                  yScale={yScale}
                  curve={curveMonotoneX}
                  fill={colour}
                  fillOpacity={0.18}
                  stroke="none"
                />
                <LinePath
                  data={data}
                  x={centre}
                  y={value}
                  curve={curveMonotoneX}
                  stroke={colour}
                  strokeWidth={2}
                  strokeDasharray={DASHES[si % DASHES.length]}
                />
              </Group>
            );
          }
          return (
            <LinePath
              key={s.key}
              data={data}
              x={centre}
              y={value}
              curve={curveMonotoneX}
              stroke={colour}
              strokeWidth={2}
              strokeDasharray={DASHES[si % DASHES.length]}
            />
          );
        })}

        {/* 4. Hover columns, invisible, one per category. Last so they receive
               the pointer, but they paint nothing. */}
        {data.map((_, ri) => (
          <Bar
            key={`hit-${ri}`}
            x={xScale(categories[ri]) ?? 0}
            y={0}
            width={bandW}
            height={innerH}
            fill="transparent"
            onMouseEnter={() => setHover(ri)}
            onMouseLeave={() => setHover(null)}
          />
        ))}
      </Group>
    </svg>
  );
}

export function CartesianChart<Row extends object>(
  props: CartesianChartProps<Row>,
) {
  const { data, series, height = 300, empty, className, formatValue = (v) => String(v) } = props;

  if (data.length === 0) {
    return <div className={cn('flex items-center justify-center', className)} style={{ height }}>{empty}</div>;
  }

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
      <div style={{ height }}>
        {/* debounceTime={0}: ParentSize defaults to a 300ms debounce before its FIRST
            measurement, so the chart renders nothing at all for that third of a
            second — a visible blank on every dashboard load, and a screenshot
            race that made this gallery flaky in the visual suite. Debouncing is
            worth having on resize, not on mount. */}
        <ParentSize debounceTime={0}>
          {({ width }) =>
            width > 0 ? <Plot {...props} width={width} height={height} /> : null
          }
        </ParentSize>
      </div>
      {series.length > 1 && (
        /* Legend only because there is more than one series: a single series is
           named by the panel heading above it and needs no key. */
        <ul className="mt-2 flex flex-wrap justify-center gap-x-4 gap-y-1">
          {series.map((s, si) => (
            <li key={s.key} className="flex items-center gap-1.5 text-2xs text-fg-muted">
              <svg width="14" height="8" aria-hidden="true">
                {s.type === 'bar' ? (
                  <rect width="14" height="8" rx="2" fill={colourFor(s, si)} />
                ) : (
                  <line
                    x1="0"
                    y1="4"
                    x2="14"
                    y2="4"
                    stroke={colourFor(s, si)}
                    strokeWidth="2"
                    strokeDasharray={DASHES[si % DASHES.length]}
                  />
                )}
              </svg>
              {s.label}
            </li>
          ))}
        </ul>
      )}
      {/* The numbers, for anyone the SVG does not serve. A chart in this product
          usually sits beside its table; where it does not, this is the table. */}
      <div className="sr-only">
        <table>
        <caption>{props.ariaLabel}</caption>
        <thead>
          <tr>
            <th scope="col">{props.x}</th>
            {series.map((s) => (
              <th key={s.key} scope="col">
                {s.label}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {data.map((row, ri) => (
            <tr key={ri}>
              <th scope="row">{String(row[props.x])}</th>
              {series.map((s) => (
                <td key={s.key}>{formatValue(toNumber(row[s.key]))}</td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
      </div>
    </div>
  );
}
