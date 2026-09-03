// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The drawer's summary: identity, score, and whatever fields the entity really
// has (W1-02 §12-§13).

import type { ReactNode } from 'react';
import { Badge, severityIntent, type BadgeIntent, type SeverityValue } from '../../../shared/ds';
import type { Chip, EntityField, EntityScore, EntitySummary } from '../types';

/**
 * Server tone → the design system's badge intent.
 *
 * The server speaks in severity bands (critical/high/medium/low) and outcomes
 * (success/warning/info/neutral). The design system speaks in INTENTS, and it
 * deliberately has no `critical` intent — `severityIntent` in shared/ds is where
 * the domain says which intent each severity deserves, and reusing it is what
 * keeps a critical risk and a critical finding the same colour as everywhere
 * else in the app. A second table here would be a second answer.
 */
export function intentOf(tone?: string): BadgeIntent {
  if (tone && tone in severityIntent) {
    return severityIntent[tone as SeverityValue];
  }
  switch (tone) {
    case 'success':
      return 'success';
    case 'warning':
      return 'warning';
    case 'info':
      return 'info';
    default:
      return 'neutral';
  }
}

export function ChipBadge({ chip }: { chip: Chip }) {
  return <Badge intent={intentOf(chip.tone)}>{chip.label}</Badge>;
}

/**
 * The score block.
 *
 * When the server says the score is unavailable this renders the reason, not a
 * zero. A control has an implementation state and no number; a risk that has
 * never been through the Score Engine has no score. Showing "0" for either
 * reads as a measurement of "no risk", which is the opposite of the truth.
 */
function ScoreBlock({ score }: { score: EntityScore }) {
  if (!score.available) {
    return (
      <div className="rounded-md border border-subtle bg-surface-sunken px-3 py-2.5">
        <div className="text-2xs uppercase tracking-wide text-fg-muted">{score.label}</div>
        <div className="mt-0.5" data-testid="score-unavailable">
          {/* The one intent that says "no data here" — and, per its own
              definition in the design system, must never be mistaken for a
              healthy zero. That is exactly this case. */}
          <Badge intent="unavailable">Not available</Badge>
        </div>
        {score.unavailable && (
          <div className="mt-1 text-2xs text-fg-muted">{score.unavailable}</div>
        )}
      </div>
    );
  }

  const pct = score.max > 0 ? Math.min(100, Math.max(0, (score.value / score.max) * 100)) : 0;
  return (
    <div className="rounded-md border border-subtle bg-surface-sunken px-3 py-2.5">
      <div className="flex items-baseline justify-between gap-2">
        <span className="text-2xs uppercase tracking-wide text-fg-muted">{score.label}</span>
        <span className="font-mono text-md font-semibold tabular-nums text-fg-primary">
          {formatNumber(score.value)}
          <span className="ml-0.5 text-2xs font-normal text-fg-muted">
            / {formatNumber(score.max)}
          </span>
        </span>
      </div>
      <div
        className="mt-2 h-1.5 overflow-hidden rounded-full bg-surface-3"
        role="meter"
        aria-valuenow={score.value}
        aria-valuemin={0}
        aria-valuemax={score.max}
        aria-label={score.label}
      >
        <div
          className="h-full rounded-full transition-[width]"
          style={{ width: `${pct}%`, background: `var(--${score.tone ?? 'accent'})` }}
        />
      </div>
      {/* Naming the engine is what lets a reader tell a CVSS from a P×I×AC
          product without inferring it from the magnitude. */}
      {score.basis && <div className="mt-1.5 text-2xs text-fg-muted">{score.basis}</div>}
    </div>
  );
}

function formatNumber(n: number): string {
  return Number.isInteger(n) ? String(n) : n.toFixed(2).replace(/\.?0+$/, '');
}

function FieldValue({ field }: { field: EntityField }): ReactNode {
  switch (field.kind) {
    case 'tags':
      return (
        <div className="flex flex-wrap gap-1">
          {(field.values ?? []).map((v) => (
            <Badge key={v} intent="neutral" size="sm">
              {v}
            </Badge>
          ))}
        </div>
      );
    case 'badge':
      return (
        <Badge intent={intentOf(field.tone)} size="sm">
          {field.value}
        </Badge>
      );
    case 'link':
      return field.href ? (
        <a
          href={field.href}
          target="_blank"
          rel="noopener noreferrer"
          className="text-sm text-accent underline underline-offset-2 hover:opacity-80"
        >
          {field.value}
        </a>
      ) : (
        <span className="text-sm text-fg-primary">{field.value}</span>
      );
    case 'date':
      return (
        <time dateTime={field.value} className="font-mono text-sm tabular-nums text-fg-primary">
          {formatDate(field.value)}
        </time>
      );
    case 'number':
    case 'money':
      return (
        <span
          className="font-mono text-sm tabular-nums text-fg-primary"
          style={field.tone ? { color: `var(--${field.tone})` } : undefined}
        >
          {field.value}
        </span>
      );
    case 'multiline':
      return (
        <p className="whitespace-pre-wrap text-sm leading-relaxed text-fg-primary">{field.value}</p>
      );
    default:
      return <span className="text-sm text-fg-primary">{field.value}</span>;
  }
}

export function formatDate(iso?: string): string {
  if (!iso) return '—';
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleString();
}

export function SummarySection({ summary }: { summary: EntitySummary }) {
  // Long-form fields get their own full-width row; short ones pair up. A
  // description squeezed into half a drawer is unreadable.
  const wide = summary.fields.filter((f) => f.kind === 'multiline' || f.kind === 'tags');
  const compact = summary.fields.filter((f) => f.kind !== 'multiline' && f.kind !== 'tags');

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center gap-2">
        {summary.status && <ChipBadge chip={summary.status} />}
        {summary.severity && <ChipBadge chip={summary.severity} />}
        <Badge intent="neutral" size="sm">
          {summary.type_label}
        </Badge>
      </div>

      <ScoreBlock score={summary.score} />

      {compact.length > 0 && (
        <dl className="grid grid-cols-1 gap-x-4 gap-y-3 sm:grid-cols-2">
          {compact.map((f) => (
            <div key={f.key} className="min-w-0">
              <dt className="text-2xs uppercase tracking-wide text-fg-muted">{f.label}</dt>
              <dd className="mt-0.5 wrap-break-word">
                <FieldValue field={f} />
              </dd>
            </div>
          ))}
        </dl>
      )}

      {wide.map((f) => (
        <div key={f.key} className="min-w-0">
          <div className="text-2xs uppercase tracking-wide text-fg-muted">{f.label}</div>
          <div className="mt-1">
            <FieldValue field={f} />
          </div>
        </div>
      ))}

      <dl className="grid grid-cols-2 gap-x-4 gap-y-3 border-t border-subtle pt-3">
        {summary.owner && (
          <div className="col-span-2">
            <dt className="text-2xs uppercase tracking-wide text-fg-muted">Owner</dt>
            <dd className="mt-0.5 text-sm text-fg-primary">
              {summary.owner.email || summary.owner.label || shortId(summary.owner.id)}
            </dd>
          </div>
        )}
        <div>
          <dt className="text-2xs uppercase tracking-wide text-fg-muted">Created</dt>
          <dd className="mt-0.5 font-mono text-2xs tabular-nums text-fg-secondary">
            {formatDate(summary.created_at)}
          </dd>
        </div>
        <div>
          <dt className="text-2xs uppercase tracking-wide text-fg-muted">Updated</dt>
          <dd className="mt-0.5 font-mono text-2xs tabular-nums text-fg-secondary">
            {formatDate(summary.updated_at)}
          </dd>
        </div>
      </dl>
    </div>
  );
}

/** A bare uuid is unreadable; the first segment is enough to tell two apart. */
export function shortId(id?: string): string {
  if (!id) return '—';
  return id.length > 8 ? `${id.slice(0, 8)}…` : id;
}
