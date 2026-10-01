// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Shared dashboard primitives (UX-2). Extracted so every role persona dashboard
// renders from one visual system: the count-up KPI card, the
// card shell and the persona header. Keeps the personas thin — each just wires its
// own real data into these.

import { localeTag, type LocaleCode } from '../../i18n/locales';
import type { ReactNode } from 'react';
import { FileText, type LucideIcon } from 'lucide-react';
import { MFAEnrollmentBanner } from '../auth/MFAEnrollmentBanner';
import { MFAPostAhaPrompt } from '../auth/MFAPostAhaPrompt';
import { ActionCenterPanel } from '../action-center/ActionCenterPanel';
import { SlotReel } from '../../shared/ds/SlotReel';

export const Card = ({
  children,
  className = '',
  style,
}: {
  children: ReactNode;
  className?: string;
  style?: React.CSSProperties;
}) => (
  <div className={`or-card ${className}`} style={style}>
    {children}
  </div>
);

/** Locale number formatter shared by every persona. */
export const numFmt = (lang: LocaleCode) => (n: number) =>
  Math.round(n).toLocaleString(localeTag(lang));

/* ---------------- persona header ---------------- */

export function PersonaHeader({
  title,
  subtitle,
  actionLabel,
  onAction,
}: {
  title: string;
  subtitle: string;
  actionLabel?: string;
  onAction?: () => void;
}) {
  return (
    <div className="flex items-start justify-between flex-wrap gap-3.5 mb-[22px]">
      <div>
        <h1 className="disp text-[27px] font-bold tracking-tight text-ink">{title}</h1>
        <div className="text-[14px] text-ink-soft mt-1">{subtitle}</div>
      </div>
      {actionLabel && onAction && (
        <button
          onClick={onAction}
          className="h-[38px] px-4 rounded-[10px] flex items-center gap-2 text-[13px] font-semibold text-ink hover:bg-hover transition-colors"
          style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border-strong)' }}
        >
          <FileText size={16} strokeWidth={1.75} />
          {actionLabel}
        </button>
      )}
    </div>
  );
}

/* ---------------- KPI card + row ---------------- */

export interface KpiSpec {
  label: string;
  val: number;
  icon: LucideIcon;
  col: string;
  /** Optional value suffix (e.g. "%", "j"). */
  suffix?: string;
  onClick?: () => void;
  /**
   * True when `val` was genuinely fetched during THIS mount, not served from
   * cache — pass the source query's `isFetchedAfterMount`. Rolls the number
   * in from 0 once, per D-060; cached data (a revisit) renders plain. Read
   * once, at mount.
   */
  fresh?: boolean;
}

function softFill(col: string, pct: number): string {
  return `color-mix(in srgb, ${col} ${pct}%, transparent)`;
}

export function KpiCard({ label, val, icon: Icon, col, suffix, onClick, fresh }: KpiSpec) {
  const inner = (
    <>
      <div className="flex items-center mb-3.5">
        <div
          className="w-[34px] h-[34px] rounded-[10px] flex items-center justify-center"
          style={{ color: col, background: softFill(col, 14) }}
        >
          <Icon size={18} strokeWidth={1.75} />
        </div>
      </div>
      <div className="disp mono text-[32px] font-bold text-ink leading-none">
        {/* No `locale` prop: the count-up this replaces called the bare,
            argument-less `.toLocaleString()` (runtime-default locale), and
            this KpiCard has no `lang` prop to do better with — matching the
            prior behaviour exactly rather than widening this component's
            contract for a cosmetic locale match. */}
        <SlotReel value={val} rollOnMount={fresh} formatOptions={{ maximumFractionDigits: 0 }} />
        {suffix && <span className="text-[18px] text-ink-soft ml-0.5">{suffix}</span>}
      </div>
      <div className="text-[12.5px] text-ink-soft mt-[5px]">{label}</div>
    </>
  );
  return onClick ? (
    <button
      onClick={onClick}
      className="or-card text-left p-[18px] hover:bg-hover transition-colors"
    >
      {inner}
    </button>
  ) : (
    <div className="or-card p-[18px]">{inner}</div>
  );
}

/** Like KpiCard but for a pre-formatted string value (e.g. "117 500 000 FCFA"). */
export function StatCard({
  label,
  value,
  col,
  icon: Icon,
  onClick,
}: {
  label: string;
  value: string;
  col: string;
  icon?: LucideIcon;
  onClick?: () => void;
}) {
  const inner = (
    <>
      {Icon && (
        <div
          className="w-[34px] h-[34px] rounded-[10px] flex items-center justify-center mb-3.5"
          style={{ color: col, background: softFill(col, 14) }}
        >
          <Icon size={18} strokeWidth={1.75} />
        </div>
      )}
      <div className="disp mono text-[24px] font-bold text-ink leading-tight wrap-break-word">
        {value}
      </div>
      <div className="text-[12.5px] text-ink-soft mt-[5px]">{label}</div>
    </>
  );
  return onClick ? (
    <button
      onClick={onClick}
      className="or-card text-left p-[18px] hover:bg-hover transition-colors w-full"
    >
      {inner}
    </button>
  ) : (
    <div className="or-card p-[18px]">{inner}</div>
  );
}

export function KpiRow({ items }: { items: KpiSpec[] }) {
  return (
    <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
      {items.map((d) => (
        <KpiCard key={d.label} {...d} />
      ))}
    </div>
  );
}

/* A score is drawn by shared/ScoreGauge only. The ScoreHero that lived here
   took a bare number and picked its own colour thresholds, which is how the
   executive board came to disagree with the sidebar (#287). */

/** Page scroll frame shared by all personas. */
export function DashboardShell({ children }: { children: ReactNode }) {
  return (
    <div className="flex-1 overflow-y-auto">
      <div
        className="mx-auto px-5 sm:px-7 pt-6 pb-10 max-w-[1320px]"
        style={{ animation: 'or-fadeup var(--dur-slow) var(--ease-out)' }}
      >
        {/* OR26-03 — every persona dashboard frames itself with this shell
            except the posture one, which mounts the same pair itself. Putting
            the prompt here means an analyst, an executive or a viewer sees it
            too: the deadline applies to the account, not to the layout they
            happen to land on. Both render nothing when MFA is configured. */}
        <MFAEnrollmentBanner />
        <MFAPostAhaPrompt />
        {/* #430 — the Action Center. Mounted in the shell for the same reason the
            MFA pair is: the question it answers ("what is mine to act on now")
            belongs to the account, not to the persona layout it happens to land
            on. The posture dashboard mounts it itself, as it does not use this
            shell. The server decides which categories a role can see, so this
            renders the caller's own queue on every persona. */}
        <ActionCenterPanel />
        {children}
      </div>
    </div>
  );
}
