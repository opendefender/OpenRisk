// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Relations, and the navigation between entities (W1-02 §14-§16).
//
// Every row opens the related entity's own drawer, in place, keeping the page
// and its filters behind it. That chaining is what turns the drawer from a
// detail panel into an investigation surface: risk → affected asset → the
// asset's other findings, and Back retraces it.

import { ArrowRight, Lock } from 'lucide-react';
import { Badge, ErrorState, SkeletonRows } from '../../../shared/ds';
import { EmptyState } from '../../../shared/EmptyState';
import { intentOf } from './SummarySection';
import type { EntityType, RelationGroup } from '../types';

interface Props {
  groups: RelationGroup[] | undefined;
  isLoading: boolean;
  error: unknown;
  onRetry: () => void;
  /** Opens a related entity in this same drawer. */
  onOpen: (type: EntityType, id: string) => void;
}

export function RelationsSection({ groups, isLoading, error, onRetry, onOpen }: Props) {
  if (isLoading) return <SkeletonRows rows={4} />;

  if (error) {
    return (
      <ErrorState
        title="Relations could not be loaded"
        description="The rest of this record is still readable. Try again, or contact an administrator if it persists."
        onRetry={onRetry}
      />
    );
  }

  if (!groups || groups.length === 0) {
    return (
      <EmptyState
        variant="first-use"
        title="No relations"
        description="Nothing is linked to this record yet."
      />
    );
  }

  // A group with nothing in it, no denial and no error is not worth a heading —
  // eight "No linked X" headings buries the two groups that do have content.
  // Denied and failed groups ARE shown, because both carry information.
  const visible = groups.filter((g) => g.items.length > 0 || g.denied || g.error);

  if (visible.length === 0) {
    return (
      <EmptyState
        variant="first-use"
        title="Nothing linked yet"
        description="When this record is connected to assets, risks, controls or incidents, they appear here."
      />
    );
  }

  return (
    <div className="flex flex-col gap-5">
      {visible.map((group) => (
        <section key={group.key} aria-labelledby={`rel-${group.key}`}>
          <div className="mb-2 flex items-baseline justify-between gap-2">
            <h3 id={`rel-${group.key}`} className="text-2xs font-semibold uppercase tracking-wide text-fg-muted">
              {group.label}
            </h3>
            {group.items.length > 0 && (
              <span className="font-mono text-2xs tabular-nums text-fg-muted">
                {/* "5 of 312" rather than "5": a preview that looks complete is
                    worse than one that says it is not. */}
                {group.truncated ? `${group.items.length} of ${group.total}` : group.total}
              </span>
            )}
          </div>

          {group.denied ? (
            <div
              className="flex items-center gap-2 rounded-md border border-subtle bg-surface-sunken px-3 py-2.5 text-sm text-fg-secondary"
              data-testid={`relation-denied-${group.key}`}
            >
              <Lock size={14} className="shrink-0 text-fg-muted" aria-hidden />
              <span>
                You do not have permission to see linked {group.label.toLowerCase()}. Ask an
                administrator for access.
              </span>
            </div>
          ) : group.error ? (
            <div className="rounded-md border border-subtle bg-surface-sunken px-3 py-2.5 text-sm text-fg-secondary">
              These links could not be loaded. The rest of this record is unaffected.
            </div>
          ) : (
            <ul className="flex flex-col gap-1.5">
              {group.items.map((rel) => (
                <li key={`${rel.type}:${rel.id}`}>
                  <button
                    type="button"
                    onClick={() => onOpen(rel.type, rel.id)}
                    className="group flex w-full items-center gap-2 rounded-md border border-subtle bg-surface-2 px-3 py-2 text-left transition-colors hover:border-default hover:bg-surface-3 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
                  >
                    <span className="min-w-0 flex-1">
                      <span className="block truncate text-sm font-medium text-fg-primary">
                        {rel.title || rel.id}
                      </span>
                      {(rel.subtitle || rel.relation_label) && (
                        <span className="block truncate text-2xs text-fg-muted">
                          {rel.relation_label && (
                            <span className="font-mono">{rel.relation_label}</span>
                          )}
                          {rel.relation_label && rel.subtitle ? ' · ' : ''}
                          {rel.subtitle}
                        </span>
                      )}
                    </span>
                    {rel.severity && (
                      <Badge intent={intentOf(rel.severity.tone)} size="sm">
                        {rel.severity.label}
                      </Badge>
                    )}
                    {rel.status && (
                      <Badge intent={intentOf(rel.status.tone)} size="sm">
                        {rel.status.label}
                      </Badge>
                    )}
                    <ArrowRight
                      size={14}
                      aria-hidden
                      className="shrink-0 text-fg-muted opacity-0 transition-opacity group-hover:opacity-100 group-focus-visible:opacity-100"
                    />
                  </button>
                </li>
              ))}
            </ul>
          )}
        </section>
      ))}
    </div>
  );
}
