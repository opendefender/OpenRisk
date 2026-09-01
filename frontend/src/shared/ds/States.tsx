// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

/**
 * The states a data surface can be in, as one vocabulary.
 *
 * Loading, empty, error and permission-denied are the four answers to "why is
 * there nothing here", and a user has to be able to tell them apart instantly:
 * "still coming", "nothing to show", "it broke", "not yours to see" lead to
 * four different next actions. When each screen writes its own, they blur —
 * which is how a 403 ends up rendering as an empty table and a user files a bug
 * about missing data.
 *
 * EmptyState already exists (shared/EmptyState) and is not duplicated here.
 * What is here is the rest of the set, plus the audit-record row.
 */

import type { ReactNode } from 'react';
import { AlertTriangle, LockKeyhole, type LucideIcon } from 'lucide-react';
import { cn } from './cn';
import { Spinner } from './Spinner';
import { Button } from './Button';

/* ------------------------------------------------------------------ shell -- */

function StateShell({
  icon: Icon,
  tone,
  title,
  description,
  actions,
  className,
  live,
}: {
  icon: LucideIcon;
  tone: 'danger' | 'muted' | 'accent';
  title: ReactNode;
  description?: ReactNode;
  actions?: ReactNode;
  className?: string;
  live?: 'polite' | 'assertive';
}) {
  const toneClass =
    tone === 'danger'
      ? 'bg-danger-surface text-danger-text'
      : tone === 'accent'
        ? 'bg-accent-soft text-accent'
        : 'bg-surface-3 text-fg-muted';

  return (
    <div
      role={live ? 'status' : undefined}
      aria-live={live}
      className={cn('flex flex-col items-center justify-center px-6 py-14 text-center', className)}
    >
      <div className={cn('mb-4 flex h-12 w-12 items-center justify-center rounded-lg', toneClass)}>
        <Icon size={22} strokeWidth={1.7} aria-hidden="true" />
      </div>
      <p className="text-md font-semibold text-fg-primary">{title}</p>
      {description && (
        <p className="mt-1.5 max-w-[46ch] text-sm text-fg-secondary">{description}</p>
      )}
      {actions && <div className="mt-5 flex flex-wrap items-center justify-center gap-2">{actions}</div>}
    </div>
  );
}

/* ------------------------------------------------------------------ error -- */

export interface ErrorStateProps {
  title: ReactNode;
  /** What went wrong, in the user's terms. */
  description?: ReactNode;
  /** The raw failure, shown collapsed. Support asks for it; users ignore it. */
  detail?: string;
  onRetry?: () => void;
  retryLabel?: string;
  retrying?: boolean;
  secondaryAction?: ReactNode;
  className?: string;
}

export function ErrorState({
  title,
  description,
  detail,
  onRetry,
  retryLabel = 'Retry',
  retrying = false,
  secondaryAction,
  className,
}: ErrorStateProps) {
  return (
    <StateShell
      icon={AlertTriangle}
      tone="danger"
      title={title}
      description={
        <>
          {description}
          {detail && (
            /* The technical text is kept, but demoted. Hiding it entirely
               makes the failure unreportable; leading with it makes the screen
               unreadable. */
            <details className="mt-3 text-left">
              <summary className="cursor-pointer text-2xs text-fg-muted hover:text-fg-secondary">
                Technical detail
              </summary>
              <pre className="mt-1.5 max-h-32 overflow-auto rounded-sm bg-surface-sunken p-2 text-left text-2xs text-fg-muted">
                {detail}
              </pre>
            </details>
          )}
        </>
      }
      /* Assertive: a failure that replaces content the user asked for should
         interrupt, unlike a loading state. */
      live="assertive"
      actions={
        <>
          {onRetry && (
            <Button variant="secondary" onClick={onRetry} loading={retrying}>
              {retryLabel}
            </Button>
          )}
          {secondaryAction}
        </>
      }
      className={className}
    />
  );
}

/* ------------------------------------------------------- permission denied -- */

export interface PermissionDeniedProps {
  /** What was being accessed — named, so the message is not generic. */
  resource?: ReactNode;
  /** The permission or role that would grant it. Naming it turns a dead end
   *  into something the user can actually ask their admin for. */
  requiredPermission?: string;
  description?: ReactNode;
  actions?: ReactNode;
  className?: string;
}

export function PermissionDenied({
  resource,
  requiredPermission,
  description,
  actions,
  className,
}: PermissionDeniedProps) {
  return (
    <StateShell
      icon={LockKeyhole}
      tone="muted"
      title={resource ? <>You do not have access to {resource}</> : 'You do not have access'}
      description={
        description ?? (
          <>
            Your role does not include this permission.
            {requiredPermission && (
              <>
                {' '}
                Ask an administrator for{' '}
                <code className="rounded-xs bg-surface-3 px-1 py-0.5 font-mono text-2xs text-fg-primary">
                  {requiredPermission}
                </code>
                .
              </>
            )}
          </>
        )
      }
      actions={actions}
      className={className}
    />
  );
}

/* ---------------------------------------------------------------- loading -- */

export interface LoadingStateProps {
  label?: string;
  className?: string;
}

/**
 * A spinner, for the case where the shape of what is coming is NOT known.
 * When it is known — a table, a card grid, a detail panel — use a skeleton
 * instead: it reserves the layout, so nothing jumps when the data lands.
 */
export function LoadingState({ label = 'Loading', className }: LoadingStateProps) {
  return (
    <div
      role="status"
      aria-live="polite"
      className={cn('flex items-center justify-center gap-2 px-6 py-14', className)}
    >
      {/* The glyph comes from the Spinner atom; the label below is what a
          screen reader reads, so the spinner itself stays aria-hidden. */}
      <Spinner size="md" className="text-fg-muted" />
      <span className="text-sm text-fg-secondary">{label}</span>
    </div>
  );
}

/** Shimmer block. Sized by the caller so it matches what will replace it. */
export function Skeleton({ className, style }: { className?: string; style?: React.CSSProperties }) {
  return (
    <div
      aria-hidden="true"
      className={cn('or-skeleton', className)}
      style={style}
    />
  );
}

/**
 * A table's worth of skeleton rows.
 *
 * Defaults to the current density's row height, so the real rows land exactly
 * where the placeholders were. `height` overrides it for a list whose rows are
 * not table rows — a card list, a timeline — because a skeleton that does not
 * match what replaces it is just a different kind of layout shift.
 */
export function SkeletonRows({
  rows = 6,
  height,
  className,
}: {
  rows?: number;
  height?: number;
  className?: string;
}) {
  return (
    <div className={cn('flex flex-col gap-1.5 p-2', className)} aria-hidden="true">
      {Array.from({ length: rows }).map((_, index) => (
        <Skeleton key={index} style={{ height: height ?? 'var(--den-row)' }} />
      ))}
    </div>
  );
}

/* ------------------------------------------------------------------ audit -- */

export interface AuditEntryProps {
  /** Who. */
  actor: ReactNode;
  /** What they did, as a verb phrase: "changed the status to Mitigated". */
  action: ReactNode;
  /** What it was done to. */
  target?: ReactNode;
  /** When — an absolute timestamp; pass a formatted string. */
  timestamp: string;
  /** Machine-readable instant for <time datetime>, if available. */
  isoTimestamp?: string;
  /** Before/after, extra context. Rendered quietly beneath. */
  detail?: ReactNode;
  className?: string;
}

/**
 * One auditable change, rendered so it is identifiable at a glance without
 * dominating the screen it sits on.
 *
 * The visual weight is deliberate: an audit trail is scanned far more often
 * than it is read, so the ACTOR and the ACTION are the only things at full
 * contrast, and the timestamp — the thing you filter by but rarely read in
 * sequence — is muted and monospaced so the column aligns.
 */
export function AuditEntry({
  actor,
  action,
  target,
  timestamp,
  isoTimestamp,
  detail,
  className,
}: AuditEntryProps) {
  return (
    <div className={cn('flex gap-3 border-b border-subtle py-2.5 last:border-b-0', className)}>
      <time
        dateTime={isoTimestamp}
        className="shrink-0 pt-px font-mono text-2xs tabular-nums text-fg-muted"
      >
        {timestamp}
      </time>
      <div className="min-w-0 flex-1">
        <p className="text-sm text-fg-secondary">
          <span className="font-semibold text-fg-primary">{actor}</span> {action}
          {target && <span className="font-medium text-fg-primary"> {target}</span>}
        </p>
        {detail && <div className="mt-1 text-2xs text-fg-muted">{detail}</div>}
      </div>
    </div>
  );
}
