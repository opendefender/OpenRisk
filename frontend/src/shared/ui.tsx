// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Reusable design-system primitives for the dc.html reskin — the React
// equivalents of the prototype's card()/pageHeader()/btn()/chip()/badge()/
// statusPill()/avatar() helpers. Every screen composes from these so spacing,
// radii and motion stay identical across the app.

import { useEffect, useRef, useState } from 'react';
import type { LucideIcon } from 'lucide-react';
import { Clock, AlertTriangle } from 'lucide-react';
import { critColor, scoreColor, softFill, type Criticality } from './riskColors';
import { useUIStrings } from './uiStrings';

/* ---------------- math + motion ---------------- */

/** Numeric count-up over ~1.1s, ease-out cubic. Re-runs when target changes. */
export function useCountUp(target: number, duration = 1100): number {
  const [value, setValue] = useState(0);
  const raf = useRef<number>(0);
  useEffect(() => {
    const reduce = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches;
    if (reduce) { setValue(target); return; }
    const t0 = performance.now();
    const tick = (now: number) => {
      let p = Math.min(1, (now - t0) / duration);
      p = 1 - Math.pow(1 - p, 3);
      setValue(target * p);
      if (p < 1) raf.current = requestAnimationFrame(tick);
    };
    raf.current = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(raf.current);
  }, [target, duration]);
  return value;
}

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

export const Card = ({ children, className = '', style }: { children: React.ReactNode; className?: string; style?: React.CSSProperties }) => (
  <div className={`or-card ${className}`} style={style}>{children}</div>
);

/** Standard scrollable page frame (fade-up in, max width, padding). */
export const PageFrame = ({ children, wide }: { children: React.ReactNode; wide?: boolean }) => (
  <div className="flex-1 overflow-y-auto">
    <div className="mx-auto px-5 sm:px-7 pt-6 pb-10" style={{ maxWidth: wide ? '1320px' : '1180px', animation: 'or-fadeup .35s ease' }}>
      {children}
    </div>
  </div>
);

export function PageHeader({ title, count, actions, badge }: { title: string; count?: string | null; actions?: React.ReactNode; badge?: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between flex-wrap gap-3 mb-[18px]">
      <div className="flex items-center gap-3">
        <h1 className="disp text-[24px] font-bold tracking-tight text-ink">{title}</h1>
        {count != null && (
          <span className="text-[12.5px] font-semibold text-ink-soft px-2.5 py-1 rounded-full" style={{ background: 'var(--bg-hover)' }}>
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

export function Btn({
  label, icon: Icon, onClick, primary, danger, className = '', type = 'button',
}: {
  label?: string; icon?: LucideIcon; onClick?: () => void; primary?: boolean; danger?: boolean;
  className?: string; type?: 'button' | 'submit';
}) {
  const base = 'h-9 rounded-[10px] text-[13px] font-semibold inline-flex items-center justify-center gap-[7px] transition-all shrink-0';
  const pad = label ? 'px-3.5' : 'w-9';
  const style: React.CSSProperties = primary
    ? { border: 'none', background: 'linear-gradient(135deg,var(--accent),var(--accent-hover))', color: '#fff', boxShadow: '0 3px 12px var(--accent-glow)' }
    : danger
      ? { border: '1px solid color-mix(in srgb,var(--critical) 30%,transparent)', background: softFill('var(--critical)', 12), color: 'var(--critical)' }
      : { border: '1px solid var(--border-strong)', background: 'var(--bg-elevated)', color: 'var(--text-primary)' };
  return (
    <button
      type={type}
      onClick={onClick}
      className={`${base} ${pad} ${primary ? 'hover:brightness-110' : danger ? 'hover:brightness-110' : 'hover:bg-hover'} ${className}`}
      style={style}
    >
      {Icon && <Icon size={16} strokeWidth={1.8} />}
      {label}
    </button>
  );
}

export function Chip({ label, active, onClick, color }: { label: string; active?: boolean; onClick?: () => void; color?: string }) {
  return (
    <button
      onClick={onClick}
      className="h-[30px] px-[13px] rounded-full text-[12.5px] font-semibold inline-flex items-center gap-1.5 transition-all"
      style={{
        border: `1px solid ${active ? 'transparent' : 'var(--border)'}`,
        background: active ? (color ? softFill(color, 16) : 'var(--accent-soft)') : 'transparent',
        color: active ? color ?? 'var(--accent)' : 'var(--text-secondary)',
      }}
    >
      {label}
    </button>
  );
}

export function IconBtn({ icon: Icon, onClick, title, active }: { icon: LucideIcon; onClick?: () => void; title?: string; active?: boolean }) {
  return (
    <button
      onClick={onClick}
      title={title}
      aria-label={title}
      className="w-9 h-9 rounded-[9px] flex items-center justify-center transition-colors"
      style={{
        border: '1px solid var(--border)',
        background: active ? 'var(--accent-soft)' : 'var(--bg-elevated)',
        color: active ? 'var(--accent)' : 'var(--text-secondary)',
      }}
    >
      <Icon size={17} strokeWidth={1.75} />
    </button>
  );
}

/* ---------------- data cells ---------------- */

export function CritBadge({ crit }: { crit: Criticality }) {
  const L = useUIStrings();
  const label = { critical: L.critical, high: L.high, medium: L.medium, low: L.low }[crit];
  const col = critColor[crit];
  return (
    <span className="inline-flex items-center gap-[5px] text-[11.5px] font-semibold px-[9px] py-[3px] rounded-full" style={{ color: col, background: softFill(col, 15) }}>
      <span className="w-1.5 h-1.5 rounded-full" style={{ background: col, animation: crit === 'critical' ? 'or-pulsedot 1.5s infinite' : 'none' }} />
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
    accepted: [L.st_accepted, 'var(--text-muted)'],
  };
  const [lbl, col] = map[status] ?? map.open;
  return (
    <span className="inline-flex items-center gap-[5px] text-[12px] font-medium text-ink-soft">
      <span className="w-[7px] h-[7px] rounded-full" style={{ background: col }} />
      {lbl}
    </span>
  );
}

export function Avatar({ initials, size = 26, title }: { initials: string; size?: number; title?: string }) {
  return (
    <div
      title={title ?? initials}
      className="rounded-full flex items-center justify-center font-bold text-white shrink-0"
      style={{
        width: size, height: size, fontSize: size * 0.4,
        background: 'linear-gradient(135deg,var(--accent),var(--accent-2))',
      }}
    >
      {initials}
    </div>
  );
}

export function FwBadge({ fw }: { fw: string }) {
  const col = { ISO27001: '#7c6cff', COBAC: '#30d158', BCEAO: '#ff9f0a', NIST: '#0a84ff', DORA: '#ff2d92', SOC2: '#64d2ff', ANSSI: '#ff453a' }[fw] ?? 'var(--text-secondary)';
  return (
    <span className="text-[11.5px] font-semibold px-2 py-[3px] rounded-md" style={{ color: col, background: softFill(col, 14) }}>
      {fw}
    </span>
  );
}

export function ScoreText({ score }: { score: number }) {
  return <span className="mono text-[15px] font-bold" style={{ color: scoreColor(score) }}>{score.toFixed(1)}</span>;
}

/** Semicircular gauge with a big centered value (used by score hero + compliance). */
export function RadialGauge({
  value, max = 100, size = 220, label, suffix, color, countUp = true,
}: {
  value: number; max?: number; size?: number; label?: string; suffix?: string; color?: string; countUp?: boolean;
}) {
  const shown = useCountUp(countUp ? value : 0);
  const v = countUp ? shown : value;
  const pct = Math.max(0, Math.min(1, v / max));
  const h = size * 0.68;
  const cx = size / 2, cy = size * 0.51, r = size * 0.345;
  const track = arcPath(cx, cy, r, -115, 115);
  const prog = arcPath(cx, cy, r, -115, -115 + 230 * pct);
  const col = color ?? (pct >= 0.7 ? 'var(--low)' : pct >= 0.45 ? 'var(--high)' : 'var(--critical)');
  const display = max === 100 ? Math.round(v).toString() : v.toFixed(1);
  return (
    <div className="relative flex justify-center" style={{ width: size, height: h }}>
      <svg viewBox={`0 0 ${size} ${h}`} width={size} height={h}>
        <path d={track} fill="none" stroke="var(--bg-hover)" strokeWidth={size * 0.064} strokeLinecap="round" />
        <path d={prog} fill="none" stroke={col} strokeWidth={size * 0.064} strokeLinecap="round" style={{ filter: `drop-shadow(0 0 6px ${col})` }} />
      </svg>
      <div className="absolute left-0 right-0 text-center" style={{ top: h * 0.34 }}>
        <div className="disp mono font-bold text-ink leading-none" style={{ fontSize: size * 0.2 }}>{display}{suffix}</div>
        {label && <div className="text-[12px] text-ink-muted mt-1">{label}</div>}
      </div>
    </div>
  );
}

/** Full-circle progress ring with centered content (compliance / simulation gauges). */
export function RingGauge({ value, size = 128, color, thickness, children }: { value: number; size?: number; color: string; thickness?: number; children?: React.ReactNode }) {
  const stroke = thickness ?? Math.max(6, size * 0.075);
  const r = (size - stroke) / 2;
  const c = 2 * Math.PI * r;
  const pct = Math.max(0, Math.min(1, value / 100));
  return (
    <div className="relative" style={{ width: size, height: size }}>
      <svg width={size} height={size} style={{ transform: 'rotate(-90deg)' }}>
        <circle cx={size / 2} cy={size / 2} r={r} fill="none" stroke="var(--bg-hover)" strokeWidth={stroke} />
        <circle cx={size / 2} cy={size / 2} r={r} fill="none" stroke={color} strokeWidth={stroke} strokeLinecap="round" strokeDasharray={c} strokeDashoffset={c * (1 - pct)} style={{ transition: 'stroke-dashoffset .9s cubic-bezier(.2,.8,.2,1)' }} />
      </svg>
      {children && <div className="absolute inset-0 flex flex-col items-center justify-center">{children}</div>}
    </div>
  );
}

/* ---------------- loading / empty / error states (dc.html §8) ---------------- */

/** Shimmer skeleton block. Never a full-page spinner. */
export function Skeleton({ className = '', style }: { className?: string; style?: React.CSSProperties }) {
  return (
    <div
      className={`rounded-lg ${className}`}
      style={{
        background: 'linear-gradient(90deg,var(--bg-hover) 25%,var(--bg-elevated) 37%,var(--bg-hover) 63%)',
        backgroundSize: '400px 100%',
        animation: 'or-shimmer 1.4s infinite linear',
        ...style,
      }}
    />
  );
}

/** A stack of shimmer rows for table/list loading. */
export function SkeletonRows({ rows = 5, height = 44 }: { rows?: number; height?: number }) {
  return (
    <div className="flex flex-col gap-2 p-2">
      {Array.from({ length: rows }).map((_, i) => <Skeleton key={i} style={{ height }} />)}
    </div>
  );
}

export function EmptyState({
  icon: Icon, title, sub, cta,
}: {
  icon: LucideIcon; title: string; sub?: string; cta?: React.ReactNode;
}) {
  return (
    <div className="flex flex-col items-center justify-center text-center py-16 px-6" style={{ animation: 'or-fadein .3s ease' }}>
      <div className="w-16 h-16 rounded-2xl flex items-center justify-center mb-5" style={{ background: 'var(--bg-hover)', color: 'var(--text-muted)' }}>
        <Icon size={30} strokeWidth={1.6} />
      </div>
      <div className="text-[15px] font-semibold text-ink mb-1.5">{title}</div>
      {sub && <div className="text-[13px] text-ink-soft max-w-sm mb-5">{sub}</div>}
      {cta}
    </div>
  );
}

export function ErrorState({ title, sub, onRetry, retryLabel }: { title: string; sub?: string; onRetry?: () => void; retryLabel?: string }) {
  return (
    <div className="flex flex-col items-center justify-center text-center py-16 px-6">
      <div className="w-16 h-16 rounded-2xl flex items-center justify-center mb-5" style={{ background: softFill('var(--critical)', 12), color: 'var(--critical)' }}>
        <AlertTriangle size={28} strokeWidth={1.7} />
      </div>
      <div className="text-[15px] font-semibold text-ink mb-1.5">{title}</div>
      {sub && <div className="text-[13px] text-ink-soft max-w-sm mb-5">{sub}</div>}
      {onRetry && <Btn label={retryLabel ?? 'Retry'} onClick={onRetry} />}
    </div>
  );
}

/** Small honest badge for design-language screens not yet backed by live data. */
export function PreviewBadge({ label }: { label: string }) {
  return (
    <span className="inline-flex items-center gap-1 text-[10.5px] font-semibold uppercase tracking-[.06em] px-2 py-[3px] rounded-full" style={{ color: 'var(--accent)', background: 'var(--accent-soft)' }}>
      {label}
    </span>
  );
}

export { critColor, scoreColor, softFill };
export { Clock };
