// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The raw audit trail (W1-02 §23).
//
// Deliberately NOT the timeline. The timeline says "severity changed from
// Medium to High"; this says actor, action, target, before, after, IP, request
// id, sequence number. The first is context and the second is evidence, they do
// not carry the same disclosure risk, and they sit behind different permissions.
// A caller without governance:audit:read never sees this tab at all — the server
// omits it from the entity's section list, so it cannot render a tab that would
// always answer 403.

import { ErrorState, PermissionDenied, SkeletonRows, AuditEntry } from '../../../shared/ds';
import { EmptyState } from '../../../shared/EmptyState';
import { isEntityError } from '../entityService';
import { formatDate, shortId } from './SummarySection';
import type { AuditRecord } from '../types';

interface Props {
  records: AuditRecord[] | undefined;
  total: number;
  isLoading: boolean;
  error: unknown;
  onRetry: () => void;
}

export function AuditSection({ records, total, isLoading, error, onRetry }: Props) {
  if (isLoading) return <SkeletonRows rows={5} />;

  if (error) {
    // 403 here is not a failure to be retried — it is an answer. Offering
    // "Retry" on a permission wall trains people to hammer a button that can
    // never work.
    if (isEntityError(error) && error.status === 403) {
      return <PermissionDenied resource="the audit trail" requiredPermission="governance:audit:read" />;
    }
    return (
      <ErrorState
        title="The audit trail could not be loaded"
        description="The rest of this record is still readable."
        onRetry={onRetry}
      />
    );
  }

  if (!records || records.length === 0) {
    return (
      <EmptyState
        variant="first-use"
        title="No audit records"
        description="Every change made through the application is recorded here, with the actor, the request and the before/after values."
      />
    );
  }

  return (
    <div className="flex flex-col gap-2">
      <p className="text-2xs text-text-muted">
        {total} record{total === 1 ? '' : 's'}
        {records.length < total ? ` · showing the most recent ${records.length}` : ''}
      </p>
      <div>
        {records.map((r) => (
          <AuditEntry
            key={r.id}
            actor={r.actor_email || shortId(r.actor_id) || 'System'}
            action={r.summary || r.action}
            timestamp={formatDate(r.created_at)}
            isoTimestamp={r.created_at}
            detail={<AuditDetail record={r} />}
          />
        ))}
      </div>
    </div>
  );
}

/**
 * The evidence half of an audit row.
 *
 * Before → after is rendered per changed FIELD rather than as two JSON blobs:
 * an auditor asks "what moved", and two pretty-printed objects make them diff by
 * eye. Fields the record does not name as changed are not shown even when they
 * are present in the snapshots, because a snapshot carries the whole row.
 */
function AuditDetail({ record }: { record: AuditRecord }) {
  const changed = record.changed_fields ?? [];
  const meta: string[] = [];
  if (record.method && record.path) meta.push(`${record.method} ${record.path}`);
  if (record.ip_address) meta.push(record.ip_address);
  if (record.request_id) meta.push(`req ${record.request_id}`);
  meta.push(`#${record.sequence}`);

  return (
    <div className="flex flex-col gap-1">
      {changed.length > 0 && (
        <div className="flex flex-col gap-0.5">
          {changed.map((field) => (
            <div key={field} className="flex flex-wrap items-baseline gap-1.5 font-mono text-2xs">
              <span className="text-text-secondary">{field}</span>
              <span style={{ color: 'var(--critical)' }}>{render(record.before?.[field])}</span>
              <span aria-hidden className="text-text-muted">→</span>
              <span style={{ color: 'var(--low)' }}>{render(record.after?.[field])}</span>
            </div>
          ))}
        </div>
      )}
      <div className="font-mono text-2xs text-text-muted">{meta.join(' · ')}</div>
    </div>
  );
}

function render(v: unknown): string {
  if (v === null || v === undefined || v === '') return '∅';
  if (typeof v === 'string') return v;
  if (typeof v === 'number' || typeof v === 'boolean') return String(v);
  try {
    return JSON.stringify(v);
  } catch {
    return '[unreadable]';
  }
}
