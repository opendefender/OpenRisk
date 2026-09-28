// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Reusable design-system primitives for the dc.html reskin — the React
// equivalents of the prototype's card()/pageHeader()/btn()/chip()/badge()/
// statusPill()/avatar() helpers. Every screen composes from these so spacing,
// radii and motion stay identical across the app.

import type { LucideIcon } from 'lucide-react';
import { Clock } from 'lucide-react';
import { critColor, frameworkColor, softFill, type Criticality } from './riskColors';
import { Button, SlotReel, type ButtonVariant } from './ds';
import { useUIStrings } from './uiStrings';

/* ---------------- math + motion ---------------- */

export function polar(cx: number, cy: number, r: number, deg: number): [number, number] {
  const a = ((deg - 90) * Math.PI) / 180;
  return [cx + r * Math.cos(a), cy + r * Math.sin(a)];
}
export function arcPath(cx: number, cy: number, r: number, a0: number, a1: number): string {
  const [x0, y0] = polar(cx, cy, r, a1);
  const [x1, y1] = polar(cx, cy, r, a0);
  const large = a1 - a0 <= 180 ? 0 : 1;
  return `M ${x0} ${y0} A ${r} ${r} 0 ${large} 0 ${x1} ${y1}`;
}

/* ---------------- surfaces ---------------- */

export const Card = ({
  children,
  className = '',
  style,
}: {
  children: React.ReactNode;
  className?: string;
  style?: React.CSSProperties;
}) => (
  <div className={`or-card ${className}`} style={style}>
    {children}
  </div>
);

/** Standard scrollable page frame (fade-up in, max width, padding). */
export const PageFrame = ({ children, wide }: { children: React.ReactNode; wide?: boolean }) => (
  <div className="flex-1 overflow-y-auto">
    <div
      className="mx-auto px-5 sm:px-7 pt-6 pb-10 motion-safe:animate-or-fadeup"
      style={{ maxWidth: wide ? 'var(--content-max-wide)' : 'var(--content-max)' }}
    >
      {children}
    </div>
  </div>
);

export function PageHeader({
  title,
  count,
  actions,
  badge,
}: {
  title: string;
  count?: string | null;
  actions?: React.ReactNode;
  badge?: React.ReactNode;
}) {
  return (
    <div className="flex items-center justify-between flex-wrap gap-3 mb-[18px]">
      <div className="flex items-center gap-3">
        <h1 className="disp text-2xl font-bold tracking-display text-ink">{title}</h1>
        {count != null && (
          <span className="text-xs font-semibold text-ink-soft px-2.5 py-1 rounded-full bg-surface-3">
            {count}
          </span>
        )}
        {badge}
      </div>
      {actions && <div className="flex items-center gap-2.5 flex-wrap">{actions}</div>}
    </div>
  );
}

/* ---------------- controls ---------------- */

/**
 * The dc.html-era button, now a thin adapter over the design system.
 *
 * Kept because ~100 call sites use it and its shape (label + icon as props
 * rather than children) is genuinely convenient for toolbar rows. What it no
 * longer does is decide anything visual: no gradient, no accent glow, no local
 * height or radius. `primary`/`danger` map onto real variants.
 *
 * New code should use <Button> directly — the boolean props cannot express a
 * tertiary action, which is the reason the design system's own API is
 * variant-based.
 */
export function Btn({
  label,
  icon: Icon,
  onClick,
  primary,
  danger,
  className = '',
  type = 'button',
  disabled,
  loading,
}: {
  label?: string;
  icon?: LucideIcon;
  onClick?: () => void;
  primary?: boolean;
  danger?: boolean;
  className?: string;
  type?: 'button' | 'submit';
  /** Disables the button and dims it — for actions in flight (a mutation's
   *  isPending) so the same request cannot be fired twice. */
  disabled?: boolean;
  loading?: boolean;
}) {
  const variant: ButtonVariant = primary ? 'primary' : danger ? 'destructive' : 'secondary';
  return (
    <Button
      type={type}
      variant={variant}
      icon={Icon}
      onClick={onClick}
      disabled={disabled}
      loading={loading}
      className={className}
      // An icon-only Btn still needs a name; callers pass one through `label`
      // when there is text, and this keeps the contract when there is not.
      {...(label ? {} : { 'aria-label': 'action' })}
    >
      {label}
    </Button>
  );
}

export function Chip({
  label,
  active,
  onClick,
  color,
  count,
  testId,
}: {
  label: string;
  active?: boolean;
  onClick?: () => void;
  color?: string;
  /** How many rows the filter would show. */
  count?: number;
  /** Optional hook for the E2E gates. A filter nobody can address from a test
   *  is a filter whose behaviour nothing protects. */
  testId?: string;
}) {
  return (
    <button
      onClick={onClick}
      data-testid={testId}
      aria-pressed={active}
      className="h-(--control-h-sm) px-3 rounded-full text-xs font-semibold inline-flex items-center gap-1.5 transition-colors duration-fast ease-out"
      style={{
        border: `1px solid ${active ? 'transparent' : 'var(--border)'}`,
        background: active ? (color ? softFill(color, 16) : 'var(--accent-soft)') : 'transparent',
        color: active ? (color ?? 'var(--accent)') : 'var(--fg-secondary)',
      }}
    >
      {label}
      {typeof count === 'number' && (
        <span
          className="text-2xs font-bold px-1.5 rounded-full tabular-nums"
          style={{
            background: active ? softFill(color ?? 'var(--accent)', 20) : 'var(--surface-3)',
          }}
        >
          {count}
        </span>
      )}
    </button>
  );
}

/** Square icon-only control. `title` is both the tooltip and the accessible
 *  name — an icon button without one is unusable with a screen reader. */
export function IconBtn({
  icon: Icon,
  onClick,
  title,
  active,
}: {
  icon: LucideIcon;
  onClick?: () => void;
  title?: string;
  active?: boolean;
}) {
  return (
    <Button
      variant={active ? 'secondary' : 'ghost'}
      icon={Icon}
      onClick={onClick}
      title={title}
      aria-label={title ?? 'action'}
      className={active ? 'text-accent' : undefined}
    />
  );
}

/* ---------------- data cells ---------------- */

export function CritBadge({ crit }: { crit: Criticality }) {
  const L = useUIStrings();
  const label = { critical: L.critical, high: L.high, medium: L.medium, low: L.low }[crit];
  const col = critColor[crit];
  return (
    <span
      className="inline-flex items-center gap-1 text-2xs font-semibold px-2 py-0.5 rounded-full"
      style={{ color: col, background: softFill(col, 15) }}
    >
      <span
        className="w-1.5 h-1.5 rounded-full"
        style={{
          background: col,
          animation: crit === 'critical' ? 'or-pulsedot 1.5s infinite' : 'none',
        }}
      />
      {label}
    </span>
  );
}

export type RiskStatus = 'open' | 'progress' | 'mitigated' | 'accepted';
export function StatusPill({ status }: { status: RiskStatus }) {
  const L = useUIStrings();
  const map: Record<RiskStatus, [string, string]> = {
    open: [L.st_open, 'var(--info)'],
    progress: [L.st_progress, 'var(--high)'],
    mitigated: [L.st_mitigated, 'var(--low)'],
    accepted: [L.st_accepted, 'var(--fg-muted)'],
  };
  const [lbl, col] = map[status] ?? map.open;
  return (
    <span className="inline-flex items-center gap-1.5 text-xs font-medium text-ink-soft">
      <span className="w-1.5 h-1.5 rounded-full" style={{ background: col }} />
      {lbl}
    </span>
  );
}

export function Avatar({
  initials,
  size = 26,
  title,
}: {
  initials: string;
  size?: number;
  title?: string;
}) {
  return (
    <div
      title={title ?? initials}
      className="rounded-full flex items-center justify-center font-bold shrink-0"
      style={{
        width: size,
        height: size,
        fontSize: size * 0.4,
        // A flat accent fill. The 135deg gradient this replaces put a piece of
        // brand decoration on every row of every table that shows an owner.
        background: 'var(--accent-solid)',
        color: 'var(--fg-on-solid)',
      }}
    >
      {initials}
    </div>
  );
}

export function FwBadge({ fw }: { fw: string }) {
  const col = frameworkColor[fw] ?? 'var(--fg-secondary)';
  return (
    <span
      className="text-2xs font-semibold px-2 py-0.5 rounded-sm"
      style={{ color: col, background: softFill(col, 14) }}
    >
      {fw}
    </span>
  );
}

export function ScoreText({ score }: { score: number }) {
  // Neutral ink: a bare number with no criticality alongside it must not be
  // given a band colour here, or this becomes a fifth threshold table.
  return <span className="mono text-base font-bold tabular-nums text-ink">{score.toFixed(1)}</span>;
}

/** Semicircular gauge with a big centered value (used by score hero + compliance). */
export function RadialGauge({
  value,
  max = 100,
  size = 220,
  label,
  suffix,
  color,
  countUp = true,
  fresh,
}: {
  value: number;
  max?: number;
  size?: number;
  label?: string;
  suffix?: string;
  color?: string;
  countUp?: boolean;
  /** True when `value` was genuinely fetched during THIS mount (a query's
   *  `isFetchedAfterMount`) — rolls the number in from 0 once, per D-060.
   *  Cached data renders plain. Read once, at mount. No effect if `countUp`
   *  is false. */
  fresh?: boolean;
}) {
  const pct = Math.max(0, Math.min(1, value / max));
  const h = size * 0.68;
  const cx = size / 2,
    cy = size * 0.51,
    r = size * 0.345;
  const track = arcPath(cx, cy, r, -115, 115);
  const col =
    color ?? (pct >= 0.7 ? 'var(--low)' : pct >= 0.45 ? 'var(--high)' : 'var(--critical)');
  const formatOptions: Intl.NumberFormatOptions =
    max === 100
      ? { maximumFractionDigits: 0 }
      : { minimumFractionDigits: 1, maximumFractionDigits: 1 };
  return (
    <div className="relative flex justify-center" style={{ width: size, height: h }}>
      <svg viewBox={`0 0 ${size} ${h}`} width={size} height={h}>
        <path
          d={track}
          fill="none"
          stroke="var(--bg-hover)"
          strokeWidth={size * 0.064}
          strokeLinecap="round"
        />
        <path
          // Same path as the track, always — `pathLength` normalises it to a
          // 0..1 space so the reveal is one `strokeDashoffset` transition to
          // the final `pct`, never a `d` recomputed from a counted value.
          d={track}
          fill="none"
          stroke={col}
          strokeWidth={size * 0.064}
          strokeLinecap="round"
          pathLength={1}
          strokeDasharray={1}
          strokeDashoffset={1 - pct}
          style={{
            filter: `drop-shadow(0 0 6px ${col})`,
            transition: 'stroke-dashoffset var(--dur-panel) var(--ease-out)',
          }}
        />
      </svg>
      <div className="absolute left-0 right-0 text-center" style={{ top: h * 0.34 }}>
        <div className="disp mono font-bold text-ink leading-none" style={{ fontSize: size * 0.2 }}>
          {countUp ? (
            <SlotReel value={value} rollOnMount={fresh} formatOptions={formatOptions} />
          ) : (
            new Intl.NumberFormat(undefined, formatOptions).format(value)
          )}
          {suffix}
        </div>
        {label && <div className="text-[12px] text-ink-muted mt-1">{label}</div>}
      </div>
    </div>
  );
}

/** Full-circle progress ring with centered content (compliance / simulation gauges). */
export function RingGauge({
  value,
  size = 128,
  color,
  thickness,
  children,
}: {
  value: number;
  size?: number;
  color: string;
  thickness?: number;
  children?: React.ReactNode;
}) {
  const stroke = thickness ?? Math.max(6, size * 0.075);
  const r = (size - stroke) / 2;
  const c = 2 * Math.PI * r;
  const pct = Math.max(0, Math.min(1, value / 100));
  return (
    <div className="relative" style={{ width: size, height: size }}>
      <svg width={size} height={size} style={{ transform: 'rotate(-90deg)' }}>
        <circle
          cx={size / 2}
          cy={size / 2}
          r={r}
          fill="none"
          stroke="var(--bg-hover)"
          strokeWidth={stroke}
        />
        <circle
          cx={size / 2}
          cy={size / 2}
          r={r}
          fill="none"
          stroke={color}
          strokeWidth={stroke}
          strokeLinecap="round"
          strokeDasharray={c}
          strokeDashoffset={c * (1 - pct)}
          style={{ transition: 'stroke-dashoffset .9s cubic-bezier(.2,.8,.2,1)' }}
        />
      </svg>
      {children && (
        <div className="absolute inset-0 flex flex-col items-center justify-center">{children}</div>
      )}
    </div>
  );
}

/* ---------------- loading / empty / error states (dc.html §8) ---------------- */

// Skeleton, SkeletonRows and ErrorState live in ./ds/States — one
// implementation each. Re-exported here so the screens that import their
// primitives from shared/ui keep a single import.
export { Skeleton, SkeletonRows, ErrorState } from './ds';

// EmptyState lives in ./EmptyState — one component, four variants. Re-exported
// here so the many screens that import their primitives from shared/ui keep a
// single import, but there is no second implementation behind it.
export { EmptyState, type EmptyStateProps, type EmptyStateVariant } from './EmptyState';

/** Small honest badge for design-language screens not yet backed by live data. */
export function PreviewBadge({ label }: { label: string }) {
  return (
    <span className="inline-flex items-center gap-1 text-2xs font-semibold uppercase tracking-caps px-2 py-0.5 rounded-full text-accent bg-accent-soft">
      {label}
    </span>
  );
}

export { critColor, softFill };
export { Clock };
