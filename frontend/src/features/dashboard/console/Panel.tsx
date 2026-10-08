// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Building blocks of the redesigned dashboard (#901): the panel every block
// sits in, its two header styles, and the loading / error / empty states each
// block renders on its own, so one failing source never blanks the page.

import type { CSSProperties, ReactNode } from 'react';
import { Link } from 'react-router';
import { useI18n } from '../../../hooks/useI18n';

export function Panel({
  children,
  className = '',
  style,
  testId,
}: {
  children: ReactNode;
  className?: string;
  style?: CSSProperties;
  testId?: string;
}) {
  return (
    <section
      data-testid={testId}
      className={`bg-surface-1 border border-border-subtle rounded-[14px] min-w-0 ${className}`}
      style={style}
    >
      {children}
    </section>
  );
}

/** Small-caps label at the top of a figure card ("SCORE D'EXPOSITION"). */
export function Eyebrow({ children }: { children: ReactNode }) {
  return (
    <span className="text-[10.5px] tracking-[0.06em] uppercase font-semibold text-ink-muted">
      {children}
    </span>
  );
}

/** The accent text link in a panel header ("Détail", "Ouvrir", "Tout voir"). */
export function HeaderLink({ to, children }: { to: string; children: ReactNode }) {
  return (
    <Link to={to} className="text-[12px] font-semibold text-accent-strong hover:underline">
      {children}
    </Link>
  );
}

/** A titled list/chart panel header: h2 on the left, a link on the right. */
export function PanelTitle({
  title,
  aside,
  className = '',
}: {
  title: ReactNode;
  aside?: ReactNode;
  className?: string;
}) {
  return (
    <div className={`flex justify-between items-baseline gap-3 ${className}`}>
      <h2 className="m-0 text-[13.5px] font-semibold text-ink">{title}</h2>
      {aside}
    </div>
  );
}

/** Skeleton lines while a block's source loads. Never a spinner. */
export function BlockSkeleton({ lines = 3, height = 14 }: { lines?: number; height?: number }) {
  return (
    <div aria-busy="true" className="grid gap-2.5 py-1">
      {Array.from({ length: lines }, (_, i) => (
        <div
          key={i}
          className="or-skeleton rounded-[6px]"
          style={{ height, width: `${92 - i * 11}%` }}
        />
      ))}
    </div>
  );
}

/** A block whose source failed: says so, offers a retry. */
export function BlockError({ onRetry }: { onRetry: () => void }) {
  const { t } = useI18n();
  return (
    <div role="alert" className="flex items-center gap-3 py-3 text-[12.5px] text-ink-soft">
      <span className="flex-1">{t('dashboard.loadError')}</span>
      <button
        type="button"
        onClick={onRetry}
        className="text-[12px] font-semibold text-accent-strong hover:underline"
      >
        {t('dashboard.retry')}
      </button>
    </div>
  );
}

/** A block with nothing to show yet, in a sentence that says what fills it. */
export function BlockEmpty({ children }: { children: ReactNode }) {
  return <p className="m-0 py-3 text-[12.5px] leading-relaxed text-ink-muted">{children}</p>;
}
