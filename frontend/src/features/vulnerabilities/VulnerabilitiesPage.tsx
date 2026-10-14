// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Vulnerability queue (#909, on the redesign layout). A tenant-scoped register
// of findings ingested from scanners and EDRs, ordered by the risk-based
// priority (pkg/vulnprio: CVSS, exploitation, asset criticality, blast radius;
// a KEV flaw is floored to P1). Four figures on top, a toolbar of severity
// pills and two switches, and rows that open in place on the details and the
// actions that used to live in a side drawer.

import { useCallback, useMemo, useState, type ReactNode } from 'react';
import { Link, useNavigate } from 'react-router';
import { toast } from 'sonner';
import { useQuery } from '@tanstack/react-query';
import {
  CheckCircle2,
  ExternalLink,
  GitBranch,
  MoreHorizontal,
  Plug,
  ShieldAlert,
  Ticket,
  Trash2,
  Unlink,
  Upload,
  Wrench,
  X,
} from 'lucide-react';

import { useI18n } from '../../hooks/useI18n';
import { useAuthStore } from '../../hooks/useAuthStore';
import { formatDate, formatNumber } from '../../i18n/format';
import { Badge, Button, Menu, Switch, type MenuItem } from '../../shared/ds';
import { PageFrame, PageHeader, EmptyState } from '../../shared/ui';
import {
  DataTable,
  useTableState,
  type BulkAction,
  type Column,
  type Facet,
  type RowAction,
} from '../../shared/datatable';
import { useFocusParam } from '../../shared/useFocusParam';
import { Term } from '../../shared/Term';
import { useSoftDelete } from '../../shared/useSoftDelete';
import { safeExternalUrl } from '../../shared/safeUrl';
import { BulkPreviewDialog, useGovernedBulk, type BulkChangeInput } from '../../shared/bulk';
import { BlockError } from '../dashboard/console/Panel';
import { DiscoverButton } from '../assets/DiscoverButton';
import { CreateRiskModal } from '../risks/CreateRiskModal';
import { useVulnerabilities, useVulnStats, useVulnMutations } from './useVulnerabilities';
import {
  vulnerabilityService,
  type Vulnerability,
  type VulnListResponse,
  type VulnQueryParams,
  type VulnSeverity,
  type VulnStats,
  type VulnStatus,
} from './vulnerabilityService';
import { SEVERITY_META, SOURCE_LABEL, STATUS_ORDER, TIER_META } from './vulnMeta';
import {
  QUEUE_STATUSES,
  epssPercent,
  nextStatuses,
  prioritySignals,
  slaState,
  statusIntent,
  type PrioritySignal,
} from './vulnQueue';
import { IngestModal } from './IngestModal';
import { IntegrationsPanel } from './IntegrationsPanel';

const PILLS: readonly VulnSeverity[] = ['critical', 'high', 'medium', 'low'];

/**
 * The pending change, applied to a cached page of the register (ABSOLUTE RULE
 * 10). Restored verbatim by the hook if the server refuses.
 *
 * `cached` arrives untyped because the ['vulnerabilities'] prefix also holds the
 * KPI stats and the connector list. Anything that is not a page of the register
 * is handed back untouched rather than guessed at.
 */
function patchVulnPage(
  cached: unknown,
  ids: ReadonlySet<string>,
  change: BulkChangeInput,
): unknown {
  if (!isVulnPage(cached)) return cached;
  if (change.action === 'delete') {
    const items = cached.items.filter((v) => !ids.has(v.id));
    return {
      ...cached,
      items,
      total: Math.max(0, cached.total - (cached.items.length - items.length)),
    };
  }
  const status = change.status;
  if (status === undefined || !STATUS_ORDER.includes(status as VulnStatus)) return cached;
  return {
    ...cached,
    items: cached.items.map((v) => (ids.has(v.id) ? { ...v, status: status as VulnStatus } : v)),
  };
}

function isVulnPage(cached: unknown): cached is VulnListResponse {
  if (typeof cached !== 'object' || cached === null) return false;
  const candidate = cached as { items?: unknown; total?: unknown };
  return Array.isArray(candidate.items) && typeof candidate.total === 'number';
}

// CVSS, NOT the OpenRisk score. CVSS is an external 0–10 scale defined by FIRST
// and its severity cuts (9/7/4) are part of that standard, not ours.
const cvssColor = (s: number) =>
  s >= 9 ? 'var(--critical)' : s >= 7 ? 'var(--high)' : s >= 4 ? 'var(--medium)' : 'var(--low)';

export function VulnerabilitiesPage() {
  const { t, locale } = useI18n();
  const navigate = useNavigate();
  const canWrite = useAuthStore((s) => s.hasPermission('vulnerabilities:update'));
  const canDelete = useAuthStore((s) => s.hasPermission('vulnerabilities:delete'));

  const [ingestOpen, setIngestOpen] = useState(false);
  const [connectorsOpen, setConnectorsOpen] = useState(false);
  const [openIds, setOpenIds] = useState<ReadonlySet<string>>(() => new Set());

  // URL-backed table state; the API does the sorting, filtering and paging.
  const table = useTableState({
    defaultSort: { key: 'priority_score', dir: 'desc' },
    defaultPageSize: 50,
  });
  const { state } = table;
  const severity = state.filters.severity?.[0] as VulnSeverity | undefined;
  const kevOnly = !!state.filters.kev?.includes('true');
  const includeFixed = !!state.filters.fixed?.includes('1');

  const params: VulnQueryParams = useMemo(() => {
    const p: VulnQueryParams = { page: state.page, limit: state.pageSize };
    if (state.sort) {
      p.sort_by = state.sort.key;
      p.sort_dir = state.sort.dir;
    }
    if (state.q.trim()) p.q = state.q.trim();
    const tier = state.filters.tier?.[0];
    const status = state.filters.status?.[0];
    const source = state.filters.source?.[0];
    if (tier) p.tier = tier;
    if (severity) p.severity = severity;
    // An explicit status wins; otherwise the queue hides what is done unless
    // "Inclure corrigées" asks for it.
    if (status) p.status = status;
    else if (!includeFixed) p.status = QUEUE_STATUSES.join(',');
    if (source) p.source = source;
    if (kevOnly) p.kev = true;
    return p;
  }, [state, severity, includeFixed, kevOnly]);

  const { data, isLoading, isError, refetch, dataUpdatedAt } = useVulnerabilities(params);
  const stats = useVulnStats();
  const { remove, updateStatus } = useVulnMutations();
  // Soft delete: hide the row immediately + Undo toast; the API delete defers.
  const { pending, remove: softDeleteVuln } = useSoftDelete<Vulnerability>({
    onCommit: (id) => remove.mutateAsync(id),
    message: (v) => t('vulns.toast.deleted', { title: v.title }),
  });
  const items = useMemo(
    () => (data?.items ?? []).filter((v) => !pending.has(v.id)),
    [data, pending],
  );

  // Deep link (/vulnerabilities?focus=<id>, from universal search and the risk
  // rule page): the row opens when it is on this page; when it is not — another
  // page, or filtered out — it is fetched and shown above the table.
  const { focusId, clearFocus } = useFocusParam();
  const focusOnPage = !!focusId && items.some((v) => v.id === focusId);
  const focused = useQuery({
    queryKey: ['vulnerabilities', 'detail', focusId],
    queryFn: () => vulnerabilityService.get(focusId as string),
    enabled: !!focusId && !focusOnPage && !isLoading,
  });
  const isOpen = (v: Vulnerability) => openIds.has(v.id) || v.id === focusId;
  const toggle = (v: Vulnerability) => {
    if (v.id === focusId) {
      clearFocus();
      return;
    }
    setOpenIds((prev) => {
      const next = new Set(prev);
      if (next.has(v.id)) next.delete(v.id);
      else next.add(v.id);
      return next;
    });
  };

  const changeStatus = updateStatus.mutateAsync;
  const setStatus = useCallback(
    async (v: Vulnerability, status: VulnStatus) => {
      try {
        await changeStatus({ id: v.id, status });
        toast.success(t('vulns.toast.status'));
      } catch {
        toast.error(t('vulns.toast.statusFailed'));
      }
    },
    [changeStatus, t],
  );

  /* --------------------------------------------------------------- facets */
  // Severity and KEV live in the toolbar; the panel keeps the other axes. The
  // backend takes one value per facet, so they are single-choice.
  const facets: Facet<Vulnerability>[] = useMemo(
    () => [
      {
        key: 'tier',
        label: t('vulns.col.priority'),
        single: true,
        options: (['P1', 'P2', 'P3', 'P4'] as const).map((tier) => ({
          value: tier,
          label: tier,
          color: TIER_META[tier].color,
        })),
      },
      {
        key: 'status',
        label: t('vulns.col.status'),
        single: true,
        options: STATUS_ORDER.map((st) => ({ value: st, label: t(`vulns.status.${st}`) })),
      },
      {
        key: 'source',
        label: t('vulns.col.source'),
        single: true,
        options: Object.entries(SOURCE_LABEL).map(([value, label]) => ({ value, label })),
      },
    ],
    [t],
  );

  /* -------------------------------------------------------------- columns */
  // Days left are counted from when the page was fetched, so a row and the
  // figures around it agree. Before the first fetch there is no row to count.
  const now = useMemo(() => new Date(dataUpdatedAt), [dataUpdatedAt]);
  const columns: Column<Vulnerability>[] = useMemo(
    () => [
      {
        key: 'cve',
        header: t('vulns.col.cve'),
        sortKey: 'cve_id',
        width: 150,
        exportValue: (v) => v.cve_id || '',
        render: (v) => (
          <span className="mono text-[12.5px] text-ink whitespace-nowrap">
            {v.cve_id || v.external_id || '—'}
          </span>
        ),
      },
      {
        key: 'title',
        header: t('vulns.col.vuln'),
        hideable: false,
        exportValue: (v) => v.title,
        render: (v) => (
          <span className="inline-flex items-center gap-2 min-w-0 max-w-[420px]">
            <span
              aria-hidden="true"
              className="size-1.5 rounded-full shrink-0"
              style={{ background: (SEVERITY_META[v.severity] ?? SEVERITY_META.info).color }}
            />
            <span className="sr-only">{t(`vulns.sev.${v.severity}`)}</span>
            <span className="truncate font-semibold text-ink">{v.title}</span>
            {v.kev && (
              <span className="shrink-0 px-1.5 rounded text-[10.5px] font-bold leading-[18px] bg-danger-surface text-danger-text">
                <Term term="KEV">{t('vulns.kev')}</Term>
              </span>
            )}
          </span>
        ),
      },
      {
        key: 'cvss',
        header: <Term term="CVSS">{t('vulns.col.cvss')}</Term>,
        headerLabel: t('vulns.col.cvss'),
        align: 'right',
        sortKey: 'cvss_score',
        width: 72,
        exportValue: (v) => (v.cvss_score ? v.cvss_score.toFixed(1) : ''),
        render: (v) => (
          <span className="font-semibold" style={{ color: cvssColor(v.cvss_score) }}>
            {v.cvss_score
              ? formatNumber(locale, v.cvss_score, {
                  minimumFractionDigits: 1,
                  maximumFractionDigits: 1,
                })
              : '—'}
          </span>
        ),
      },
      {
        key: 'epss',
        header: <Term term="EPSS">{t('vulns.col.epss')}</Term>,
        headerLabel: t('vulns.col.epss'),
        align: 'right',
        width: 72,
        exportValue: (v) => (v.epss ? v.epss.toFixed(4) : ''),
        render: (v) => {
          const pct = epssPercent(v.epss);
          return <span className="text-ink-soft">{pct ? `${pct} %` : '—'}</span>;
        },
      },
      {
        key: 'asset',
        header: t('vulns.col.asset'),
        exportValue: (v) => v.asset_name || '',
        render: (v) => (
          <span className="text-[12.5px] text-ink-soft truncate max-w-[220px] inline-block align-middle">
            {v.asset_name || '—'}
          </span>
        ),
      },
      {
        key: 'sla',
        header: t('vulns.col.sla'),
        exportValue: (v) => v.sla_due_at ?? '',
        render: (v) => <SlaCell v={v} now={now} />,
      },
      {
        key: 'status',
        header: t('vulns.col.status'),
        exportValue: (v) => v.status,
        render: (v) => (
          <Badge intent={statusIntent[v.status] ?? 'neutral'}>
            {t(`vulns.status.${v.status}`)}
          </Badge>
        ),
      },
      {
        key: 'priority',
        header: t('vulns.col.priority'),
        sortKey: 'priority_score',
        defaultHidden: true,
        exportValue: (v) => `${v.priority_tier} (${v.priority_score.toFixed(0)})`,
        render: (v) => (
          <span className="mono text-[12px] text-ink">
            {v.priority_tier} · {v.priority_score.toFixed(0)}
          </span>
        ),
      },
    ],
    [t, locale, now],
  );

  /* ---------------------------------------------------------- row actions */
  const rowActions: RowAction<Vulnerability>[] = useMemo(
    () => [
      {
        key: 'take',
        label: t('vulns.act.take'),
        icon: Wrench,
        hidden: (v) => !canWrite || !nextStatuses(v.status).includes('in_remediation'),
        onSelect: (v) => void setStatus(v, 'in_remediation'),
      },
      {
        key: 'fixed',
        label: t('vulns.act.fixed'),
        icon: CheckCircle2,
        hidden: (v) => !canWrite || !nextStatuses(v.status).includes('remediated'),
        onSelect: (v) => void setStatus(v, 'remediated'),
      },
      {
        key: 'delete',
        label: t('vulns.act.delete'),
        icon: Trash2,
        danger: true,
        separatorBefore: true,
        hidden: () => !canDelete,
        onSelect: (v) => softDeleteVuln(v),
      },
    ],
    [canWrite, canDelete, softDeleteVuln, setStatus, t],
  );

  /* --------------------------------------------------------- bulk actions */
  // Governed (#582): every bulk action goes preview → confirm → transactional,
  // audited apply on the server.
  const bulk = useGovernedBulk({
    register: 'vulnerabilities',
    optimistic: {
      // Narrow: the ['vulnerabilities'] prefix also caches stats and connectors,
      // which this patch has no business rewriting.
      queryKey: ['vulnerabilities', 'list'],
      apply: patchVulnPage,
      // Wide: a status change moves the KPI counts too.
      invalidateKey: ['vulnerabilities'],
    },
    onApplied: (result, change) => {
      const n = result.applied ?? 0;
      toast.success(
        change.action === 'delete'
          ? t('vulns.toast.bulkDeleted', { count: n })
          : t('vulns.toast.bulkUpdated', { count: n }),
      );
    },
  });

  const bulkActions: BulkAction<Vulnerability>[] = useMemo(
    () => [
      {
        key: 'remediating',
        label: t('vulns.act.take'),
        icon: Wrench,
        // Two gates: the permission the user holds, and what the server says
        // this register can do. Neither is the enforcement — the route is.
        hidden: !canWrite || !bulk.supports('change_status'),
        selectionOnly: true,
        run: ({ ids }) => bulk.request({ action: 'change_status', status: 'in_remediation' }, ids),
      },
      {
        key: 'delete',
        label: t('vulns.act.delete'),
        icon: Trash2,
        danger: true,
        hidden: !canDelete || !bulk.supports('delete'),
        selectionOnly: true,
        run: ({ ids }) => bulk.request({ action: 'delete' }, ids),
      },
    ],
    [canWrite, canDelete, bulk, t],
  );

  const moreItems: MenuItem[] = [
    {
      label: t('vulns.unassigned'),
      icon: Unlink,
      onSelect: () => navigate('/vulnerabilities/unassigned'),
    },
    {
      label: t('vulns.riskRule'),
      icon: GitBranch,
      onSelect: () => navigate('/vulnerabilities/risk-rule'),
    },
    { label: t('vulns.integrations'), icon: Plug, onSelect: () => setConnectorsOpen(true) },
  ];

  const counts = data?.severity_counts;
  const toolbar = (
    <div className="flex items-center gap-2 flex-wrap">
      <div role="group" aria-label={t('vulns.severityGroup')} className="flex gap-1.5 flex-wrap">
        {PILLS.map((sev) => {
          const on = severity === sev;
          const n = counts?.[sev];
          return (
            <button
              key={sev}
              type="button"
              aria-pressed={on}
              onClick={() => table.toggleFilter('severity', sev, true)}
              data-testid={`vuln-sev-${sev}`}
              className={`h-[34px] px-2.5 rounded-[10px] border text-[12.5px] font-semibold inline-flex items-center gap-1.5 shrink-0 transition-colors motion-safe:active:translate-y-px ${
                on
                  ? 'bg-surface-2 border-border-strong text-ink'
                  : 'bg-transparent border-border-default text-ink-soft hover:border-border-strong'
              }`}
            >
              <span
                aria-hidden="true"
                className="size-1.5 rounded-full"
                style={{ background: SEVERITY_META[sev].color }}
              />
              {t(`vulns.sev.${sev}`)}
              {n !== undefined && <span className="mono text-[11px] text-ink-muted">{n}</span>}
            </button>
          );
        })}
      </div>
      <Switch
        size="sm"
        controlFirst
        checked={kevOnly}
        onChange={(e) => table.setFilter('kev', e.target.checked ? ['true'] : [])}
        label={<span className="text-[12.5px] font-semibold">{t('vulns.kevOnly')}</span>}
        data-testid="vuln-kev-only"
      />
      <Switch
        size="sm"
        controlFirst
        checked={includeFixed}
        onChange={(e) => table.setFilter('fixed', e.target.checked ? ['1'] : [])}
        label={
          <span className="text-[12.5px] font-semibold" title={t('vulns.includeFixedHint')}>
            {t('vulns.includeFixed')}
          </span>
        }
        data-testid="vuln-include-fixed"
      />
    </div>
  );

  return (
    <PageFrame wide>
      <PageHeader
        className="!mb-[18px]"
        title={t('vulns.title')}
        subtitle={t('vulns.subtitle')}
        actions={
          <>
            <Menu
              label={t('vulns.more')}
              trigger={
                <Button variant="ghost" icon={MoreHorizontal} aria-label={t('vulns.more')} />
              }
              items={moreItems}
            />
            {/* Import writes findings (POST /vulnerabilities/ingest needs
                vulnerabilities:update), so a read-only member is not offered it (#739). */}
            {canWrite && (
              <Button variant="secondary" icon={Upload} onClick={() => setIngestOpen(true)}>
                {t('vulns.import')}
              </Button>
            )}
            <DiscoverButton strings="vulns.scan" testId="vuln-scan" />
          </>
        }
      />

      <KpiStrip
        stats={stats.data}
        loading={stats.isLoading}
        error={stats.isError}
        onRetry={() => void stats.refetch()}
      />

      {focusId && !focusOnPage && focused.data && (
        <section
          aria-label={focused.data.cve_id || focused.data.title}
          className="mb-3 rounded-[14px] border border-border-default bg-surface-1"
          data-testid="vuln-focused"
        >
          <div className="flex items-center gap-3 px-5 pt-3">
            <span className="mono text-[12.5px] text-ink-muted">{focused.data.cve_id}</span>
            <span className="flex-1 font-semibold text-ink truncate">{focused.data.title}</span>
            <Button
              variant="ghost"
              size="sm"
              icon={X}
              aria-label={t('topology.close')}
              onClick={clearFocus}
            />
          </div>
          <div className="px-5 pb-4 pt-2">
            <VulnDetail
              v={focused.data}
              onStatus={(s) => void setStatus(focused.data, s)}
              onDelete={() => {
                const d = focused.data;
                clearFocus();
                softDeleteVuln(d);
              }}
            />
          </div>
        </section>
      )}

      <DataTable
        id="vulnerabilities"
        ariaLabel={t('vulns.title')}
        rows={items}
        total={data?.total ?? items.length}
        columns={columns}
        rowKey={(v) => v.id}
        api={table}
        mode="server"
        loading={isLoading}
        error={isError}
        onRetry={() => void refetch()}
        facets={facets}
        searchPlaceholder={t('vulns.search')}
        toolbarExtra={toolbar}
        selectable
        rowActions={rowActions}
        bulkActions={bulkActions}
        onRowClick={toggle}
        expansion={{
          isOpen,
          onToggle: toggle,
          label: (v) => t('vulns.expand', { name: v.cve_id || v.title }),
          render: (v) => (
            <VulnDetail
              v={v}
              onStatus={(s) => void setStatus(v, s)}
              onDelete={() => softDeleteVuln(v)}
            />
          ),
        }}
        exportFilename="vulnerabilites"
        minWidth={1040}
        empty={
          <EmptyState
            icon={ShieldAlert}
            title={t('vulns.empty.title')}
            description={canWrite ? t('vulns.empty.body') : t('vulns.empty.readOnly')}
            primaryAction={
              canWrite ? (
                <Button variant="primary" icon={Upload} onClick={() => setIngestOpen(true)}>
                  {t('vulns.import')}
                </Button>
              ) : undefined
            }
          />
        }
      />

      <BulkPreviewDialog bulk={bulk} entityLabel={t('vulns.entity')} />
      <IngestModal isOpen={ingestOpen} onClose={() => setIngestOpen(false)} />
      <IntegrationsPanel
        isOpen={connectorsOpen}
        onClose={() => setConnectorsOpen(false)}
        onImport={() => {
          setConnectorsOpen(false);
          setIngestOpen(true);
        }}
      />
    </PageFrame>
  );
}

/* ------------------------------------------------------------------ KPIs */

function KpiStrip({
  stats,
  loading,
  error,
  onRetry,
}: {
  stats: VulnStats | undefined;
  loading: boolean;
  error: boolean;
  onRetry: () => void;
}) {
  const { t, locale } = useI18n();
  if (error) {
    return (
      <div className="mb-4 rounded-[14px] border border-border-subtle bg-surface-1 px-5">
        <BlockError onRetry={onRetry} />
      </div>
    );
  }
  const mttr = stats?.mttr_critical_days ?? null;
  const days = stats?.mttr_window_days ?? 90;
  const tiles: {
    key: string;
    label: string;
    hint: string;
    value: ReactNode;
    tone?: string;
    foot?: string;
  }[] = [
    {
      key: 'open',
      label: t('vulns.kpi.open'),
      hint: t('vulns.kpi.openHint'),
      value: stats ? formatNumber(locale, stats.unresolved) : null,
    },
    {
      key: 'kev',
      label: t('vulns.kpi.kevOpen'),
      hint: t('vulns.kpi.kevOpenHint'),
      value: stats ? formatNumber(locale, stats.kev_open) : null,
      tone: stats && stats.kev_open > 0 ? 'text-danger-text' : undefined,
    },
    {
      key: 'sla',
      label: t('vulns.kpi.sla'),
      hint: t('vulns.kpi.slaHint'),
      value: stats ? formatNumber(locale, stats.sla_breached) : null,
      tone: stats && stats.sla_breached > 0 ? 'text-warning-text' : undefined,
    },
    {
      key: 'mttr',
      label: t('vulns.kpi.mttr'),
      hint: t('vulns.kpi.mttrHint'),
      value: stats
        ? mttr === null
          ? '—'
          : t('vulns.kpi.mttrValue', {
              value: formatNumber(locale, mttr, { maximumFractionDigits: 1 }),
            })
        : null,
      foot: stats
        ? mttr === null
          ? t('vulns.kpi.mttrNone', { days })
          : t('vulns.kpi.mttrSample', { count: stats.mttr_critical_sample, days })
        : undefined,
    },
  ];
  return (
    <div
      className="mb-4 grid grid-cols-2 lg:grid-cols-4 rounded-[14px] border border-border-subtle bg-surface-1 overflow-hidden"
      data-testid="vuln-kpis"
      aria-busy={loading || undefined}
    >
      {tiles.map((tile, i) => (
        <div
          key={tile.key}
          className={`px-5 py-4 border-border-subtle ${i > 0 ? 'lg:border-l' : ''} ${
            i % 2 === 1 ? 'border-l lg:border-l' : ''
          } ${i >= 2 ? 'border-t lg:border-t-0' : ''}`}
          data-testid={`vuln-kpi-${tile.key}`}
        >
          <div
            className="text-[10.5px] tracking-[.06em] uppercase font-semibold text-ink-muted"
            title={tile.hint}
          >
            {tile.label}
          </div>
          {tile.value === null ? (
            <div className="or-skeleton rounded-[6px] h-7 w-16 mt-2" />
          ) : (
            <div
              className={`mono text-[28px] font-semibold mt-1.5 leading-tight ${tile.tone ?? 'text-ink'}`}
            >
              {tile.value}
            </div>
          )}
          {tile.foot && <div className="text-[11.5px] text-ink-muted mt-0.5">{tile.foot}</div>}
        </div>
      ))}
    </div>
  );
}

/* ------------------------------------------------------------- SLA cell */

function SlaCell({ v, now }: { v: Vulnerability; now: Date }) {
  const { t } = useI18n();
  const s = slaState(v, now);
  switch (s.kind) {
    case 'none':
      return <span className="text-ink-muted">—</span>;
    case 'today':
      return <span className="font-semibold text-warning-text">{t('vulns.sla.today')}</span>;
    case 'late':
      return (
        <span className="font-semibold text-danger-text">
          {t('vulns.sla.late', { count: s.days })}
        </span>
      );
    case 'left':
      return (
        <span className={`font-semibold ${s.days <= 3 ? 'text-warning-text' : 'text-ink-soft'}`}>
          {t('vulns.sla.left', { count: s.days })}
        </span>
      );
  }
}

/* --------------------------------------------------------- open row */

function VulnDetail({
  v,
  onStatus,
  onDelete,
}: {
  v: Vulnerability;
  onStatus: (s: VulnStatus) => void;
  onDelete: () => void;
}) {
  const { t, locale } = useI18n();
  const canWrite = useAuthStore((s) => s.hasPermission('vulnerabilities:update'));
  const canDelete = useAuthStore((s) => s.hasPermission('vulnerabilities:delete'));
  const canCreateRisk = useAuthStore((s) => s.hasPermission('risks:create'));
  const { createTicket } = useVulnMutations();
  const [riskOpen, setRiskOpen] = useState(false);

  const openTicket = async () => {
    try {
      await createTicket.mutateAsync(v.id);
      toast.success(t('vulns.toast.ticket'));
    } catch {
      toast.error(t('vulns.toast.ticketFailed'));
    }
  };

  const date = (d: string | null | undefined) =>
    d ? formatDate(locale, d, { day: 'numeric', month: 'short', year: 'numeric' }) : '—';
  const signalText = (s: PrioritySignal) => {
    switch (s.key) {
      case 'cvss':
        return t('vulns.signal.cvss', {
          value: formatNumber(locale, s.value, {
            minimumFractionDigits: 1,
            maximumFractionDigits: 1,
          }),
        });
      case 'kev':
        return t('vulns.signal.kev');
      case 'exploit':
        return t('vulns.signal.exploit');
      case 'epss':
        return t('vulns.signal.epss', { value: s.value });
      case 'asset':
        // The enum (CRITICAL…) is the API's; the chip reads it in words.
        return t('vulns.signal.asset', {
          crit: t(`assetAnalysis.levels.${s.crit.toUpperCase()}`, s.crit),
        });
      case 'affected':
        return t('vulns.signal.affected', { count: s.count });
    }
  };
  const label = 'text-[10.5px] tracking-[.06em] uppercase font-semibold text-ink-muted';
  const actionIcon: Partial<Record<VulnStatus, typeof Wrench>> = {
    in_remediation: Wrench,
    remediated: CheckCircle2,
  };
  const actionKey: Record<VulnStatus, string> = {
    open: 'vulns.act.reopen',
    triaged: 'vulns.act.reopen',
    in_remediation: 'vulns.act.take',
    remediated: 'vulns.act.fixed',
    accepted: 'vulns.act.accept',
    false_positive: 'vulns.act.falsePositive',
  };
  const tier = TIER_META[v.priority_tier];

  return (
    <div
      className="grid gap-6 lg:grid-cols-[minmax(0,1.5fr)_minmax(0,1fr)]"
      data-testid="vuln-detail"
      onClick={(e) => e.stopPropagation()}
    >
      <div className="min-w-0 grid gap-4 content-start">
        <div>
          <div className={label}>
            {t('vulns.why', { tier: v.priority_tier, score: v.priority_score.toFixed(0) })}
          </div>
          <ul className="m-0 mt-1.5 p-0 list-none flex flex-wrap gap-1.5">
            {prioritySignals(v).map((s) => (
              <li
                key={s.key}
                className={`text-[12px] px-2 py-0.5 rounded-md ${
                  s.key === 'kev'
                    ? 'bg-danger-surface text-danger-text'
                    : 'bg-surface-2 text-ink-soft'
                }`}
              >
                {signalText(s)}
              </li>
            ))}
          </ul>
          {v.kev && tier && (
            <p className="m-0 mt-1.5 text-[11.5px] text-ink-muted">{t('vulns.signal.kevFloor')}</p>
          )}
        </div>
        {v.description && (
          <div>
            <div className={label}>{t('vulns.field.description')}</div>
            <p className="m-0 mt-1 text-[13px] leading-relaxed text-ink-soft max-w-[75ch]">
              {v.description}
            </p>
          </div>
        )}
        {v.remediation_hint && (
          <div>
            <div className={label}>{t('vulns.field.remediation')}</div>
            <p className="m-0 mt-1 text-[13px] leading-relaxed text-ink max-w-[75ch]">
              {v.remediation_hint}
            </p>
          </div>
        )}
        <dl className="m-0 grid grid-cols-[auto_1fr] sm:grid-cols-[auto_1fr_auto_1fr] gap-x-4 gap-y-1.5 text-[12.5px]">
          <dt className="text-ink-muted">{t('vulns.field.asset')}</dt>
          <dd className="m-0 text-ink">
            {v.asset_id ? (
              <Link
                to={`/assets?focus=${encodeURIComponent(v.asset_id)}`}
                className="hover:underline"
              >
                {v.asset_name || '—'}
              </Link>
            ) : (
              v.asset_name || '—'
            )}
          </dd>
          <dt className="text-ink-muted">{t('vulns.field.source')}</dt>
          <dd className="m-0 text-ink">{SOURCE_LABEL[v.source] ?? v.source}</dd>
          <dt className="text-ink-muted">{t('vulns.field.firstSeen')}</dt>
          <dd className="m-0 text-ink">{date(v.first_seen)}</dd>
          <dt className="text-ink-muted">{t('vulns.field.lastSeen')}</dt>
          <dd className="m-0 text-ink">{date(v.last_seen)}</dd>
          {v.remediated_at ? (
            <>
              <dt className="text-ink-muted">{t('vulns.field.remediatedAt')}</dt>
              <dd className="m-0 text-ink">{date(v.remediated_at)}</dd>
            </>
          ) : (
            <>
              <dt className="text-ink-muted">{t('vulns.field.due')}</dt>
              <dd className="m-0 text-ink">{date(v.sla_due_at)}</dd>
            </>
          )}
          {v.cvss_vector && (
            <>
              <dt className="text-ink-muted">{t('vulns.field.vector')}</dt>
              <dd className="m-0 mono text-[11.5px] text-ink-soft break-all">{v.cvss_vector}</dd>
            </>
          )}
        </dl>
      </div>

      <div className="grid gap-4 content-start">
        {canWrite && (
          <div>
            <div className={label}>{t('vulns.act.label')}</div>
            <div className="mt-2 flex flex-wrap gap-2">
              {nextStatuses(v.status).map((s, i) => (
                <Button
                  key={s}
                  size="sm"
                  variant={i === 0 ? 'primary' : 'secondary'}
                  icon={actionIcon[s]}
                  onClick={() => onStatus(s)}
                  data-testid={`vuln-act-${s}`}
                >
                  {t(actionKey[s])}
                </Button>
              ))}
            </div>
          </div>
        )}
        <div className="flex flex-wrap gap-2">
          {canCreateRisk && (
            <Button
              size="sm"
              variant="secondary"
              icon={ShieldAlert}
              onClick={() => setRiskOpen(true)}
            >
              {t('vulns.act.createRisk')}
            </Button>
          )}
          {v.ticket_key ? (
            // The ITSM base URL comes from tenant-configured integration data,
            // so the href is allowlisted to http(s) rather than trusted.
            <a
              href={safeExternalUrl(v.ticket_url) ?? '#'}
              target="_blank"
              rel="noreferrer"
              className="inline-flex items-center gap-1.5 h-8 px-2 text-[12.5px] font-semibold text-accent hover:underline"
            >
              <Ticket size={13} aria-hidden="true" /> {v.ticket_key}{' '}
              <ExternalLink size={12} aria-hidden="true" />
            </a>
          ) : (
            canWrite && (
              <Button
                size="sm"
                variant="ghost"
                icon={Ticket}
                loading={createTicket.isPending}
                onClick={() => void openTicket()}
              >
                {t('vulns.act.openTicket')}
              </Button>
            )
          )}
          {canDelete && (
            <Button size="sm" variant="ghost" icon={Trash2} onClick={onDelete}>
              {t('vulns.act.delete')}
            </Button>
          )}
        </div>
        {v.risk_id && (
          <Link
            to={`/risks?focus=${encodeURIComponent(v.risk_id)}`}
            className="text-[12.5px] font-semibold text-accent hover:underline"
          >
            {t('vulns.field.linkedRisk')} · {t('vulns.field.openRisk')}
          </Link>
        )}
      </div>

      {riskOpen && (
        <CreateRiskModal
          isOpen
          onClose={() => setRiskOpen(false)}
          initialAssetIds={v.asset_id ? [v.asset_id] : []}
          initialTitle={t('vulns.riskTitle', { title: v.title, cve: v.cve_id || v.external_id })}
        />
      )}
    </div>
  );
}
