// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The executive view of the October 2026 redesign (#903): what a risk
// committee reads in one page.
//
// Four headline figures with their movement since last quarter, the five
// largest exposures (inherent and targeted residual), compliance by
// framework, and the decisions the committee owes, with Approve and Defer.
//
// Deltas compare today with the last daily snapshot taken before the current
// quarter began (/score/history quarter_baseline). With no such snapshot the
// tile says so instead of printing a movement nobody measured.

import { useMemo, useState } from 'react';
import { Link, useNavigate } from 'react-router';
import { Download, Link2 } from 'lucide-react';
import { toast } from 'sonner';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { isAxiosError } from 'axios';

import { useI18n } from '../../hooks/useI18n';
import { useScore } from '../../hooks/useScore';
import { formatDate, formatNumber } from '../../i18n/format';
import { PageFrame, PageHeader } from '../../shared/ui';
import { Button } from '../../shared/ds';
import { bandTextColor } from '../../services/scoreService';
import { complianceService } from '../../services/complianceService';
import { boardService } from '../../services/boardService';
import { useFeature } from '../billing/useEntitlements';
import { useFinancialSummary } from '../financial/useFinancial';
import { currencyLabel, millions, toDisplay } from '../financial/money';
import { useDashboardStats } from '../dashboard/useCommandCenter';
import { useScoreHistory } from '../dashboard/useScoreHistory';
import { frameworkColorFor } from '../compliance/complianceOverview';
import { governanceService, type ApprovalRequest } from '../governance/governanceService';
import {
  BlockEmpty,
  BlockError,
  BlockSkeleton,
  Panel,
  PanelTitle,
} from '../dashboard/console/Panel';

const ALL_TIME = { kind: 'preset', preset: 'all' } as const;
const BAND_FILL: Record<string, string> = {
  critical: 'var(--risk-critical)',
  high: 'var(--risk-high)',
  medium: 'var(--risk-moderate)',
  low: 'var(--risk-low)',
};

function quarterOf(d: Date) {
  return Math.floor(d.getMonth() / 3) + 1;
}

export function ExecutiveDashboard() {
  const { t } = useI18n();
  const navigate = useNavigate();
  const now = new Date();
  const q = quarterOf(now);

  const exportReport = useMutation({
    mutationFn: () =>
      boardService.generate({
        period_label: t('executive.eyebrow', { q, year: now.getFullYear() }),
      }),
    onSuccess: () => {
      toast.success(t('executive.exported'));
      navigate('/reports/board');
    },
    onError: () => toast.error(t('executive.exportFailed')),
  });

  const copyLink = async () => {
    try {
      await navigator.clipboard.writeText(window.location.href);
      toast.success(t('executive.linkCopied'));
    } catch {
      toast.error(t('executive.linkFailed'));
    }
  };

  return (
    <PageFrame>
      <PageHeader
        className="!mb-[22px]"
        eyebrow={t('executive.eyebrow', { q, year: now.getFullYear() })}
        title={t('executive.title')}
        actions={
          <>
            <Button variant="secondary" icon={Link2} onClick={() => void copyLink()}>
              {t('executive.copyLink')}
            </Button>
            <Button
              variant="primary"
              icon={Download}
              loading={exportReport.isPending}
              onClick={() => exportReport.mutate()}
            >
              {exportReport.isPending ? t('executive.exporting') : t('executive.export')}
            </Button>
          </>
        }
      />

      <KpiStrip />

      <div className="flex flex-wrap gap-4 mb-4">
        <TopExposures className="flex-[1.2_1_440px]" />
        <FrameworkBars className="flex-[1_1_360px]" />
      </div>

      <CommitteeDecisions />
    </PageFrame>
  );
}

/* ------------------------------------------------------------- KPI strip */

interface Delta {
  text: string;
  color: string;
}

function KpiStrip() {
  const { t, locale } = useI18n();
  const score = useScore('tenant');
  const history = useScoreHistory(12);
  const stats = useDashboardStats(ALL_TIME);
  const fin = useFeature('financial_quantification');
  const finance = useFinancialSummary();
  const gaps = useQuery({
    queryKey: ['compliance', 'gap-analysis', 'all'],
    queryFn: () => complianceService.getGapAnalysis(),
  });

  const base = history.data?.quarter_baseline ?? null;
  const baseQ = base ? base.quarter.split('-Q')[1] : null;
  const measured = score.data?.measured ? score.data : undefined;
  const fw = gaps.data?.frameworks ?? [];
  const coverage = fw.length
    ? fw.reduce((a, f) => a + (f.percent_complete ?? 0), 0) / fw.length
    : undefined;
  const locked = !fin.loading && !fin.enabled;
  const f = !locked ? finance.data : undefined;
  const ale = f ? toDisplay(f.total_ale.xaf, f.fx_rate_xaf) : undefined;
  const baseAle = f && base?.ale_xaf != null ? toDisplay(base.ale_xaf, f.fx_rate_xaf) : undefined;

  const fmt1 = (n: number) => formatNumber(locale, Math.abs(n), { maximumFractionDigits: 1 });
  const signed = (n: number, text: string) => `${n > 0 ? '+' : n < 0 ? '−' : ''}${text}`;

  // A falling exposure, fewer critical risks and more coverage are good news.
  const delta = (
    cur: number | undefined,
    prev: number | null | undefined,
    lowerIsBetter: boolean,
    unit: (n: number) => string,
  ): Delta => {
    if (cur === undefined || prev === undefined || prev === null || !baseQ) {
      return { text: t('executive.kpi.noBaseline'), color: 'var(--fg-muted)' };
    }
    const d = cur - prev;
    const good = lowerIsBetter ? d <= 0 : d >= 0;
    return {
      text: `${signed(d, unit(d))} ${t('executive.kpi.sinceQuarter', { q: baseQ })}`,
      color: d === 0 ? 'var(--fg-secondary)' : good ? 'var(--success-text)' : 'var(--danger-text)',
    };
  };

  const tiles = [
    {
      key: 'score',
      label: t('executive.kpi.score'),
      value: measured ? String(Math.round(measured.value)) : '—',
      unit: '/100',
      color: measured ? bandTextColor(measured.band) : 'var(--fg-muted)',
      delta: delta(measured?.value, base?.value, true, (d) =>
        t('executive.kpi.pts', { delta: fmt1(d) }),
      ),
      loading: score.isLoading,
    },
    {
      key: 'ale',
      label: t('executive.kpi.ale'),
      value: ale !== undefined ? millions(ale, locale) : '—',
      unit: `M ${currencyLabel(f?.currency)}`,
      color: 'var(--fg-primary)',
      delta: locked
        ? { text: t('executive.kpi.locked', { plan: 'Business' }), color: 'var(--fg-muted)' }
        : delta(ale, baseAle, true, (d) => `${millions(Math.abs(d), locale)} M`),
      loading: !locked && (finance.isLoading || fin.loading),
    },
    {
      key: 'critical',
      label: t('executive.kpi.critical'),
      value: stats.data ? String(stats.data.critical_live) : '—',
      unit: t('executive.kpi.criticalUnit'),
      color: 'var(--risk-critical)',
      delta: delta(stats.data?.critical_live, base?.critical_risks, true, (d) =>
        String(Math.abs(d)),
      ),
      loading: stats.isLoading,
    },
    {
      key: 'compliance',
      label: t('executive.kpi.compliance'),
      value: coverage !== undefined ? String(Math.round(coverage)) : '—',
      unit: '%',
      color: 'var(--fg-primary)',
      delta: delta(coverage, base?.compliance_pct, false, (d) =>
        t('executive.kpi.pts', { delta: fmt1(d) }),
      ),
      loading: gaps.isLoading,
    },
  ];

  return (
    <div className="grid grid-cols-[repeat(auto-fit,minmax(200px,1fr))] gap-px bg-border-subtle border border-border-subtle rounded-[14px] overflow-hidden mb-6">
      {tiles.map((k) => (
        <div key={k.key} className="p-5 bg-surface-0" data-testid={`exec-kpi-${k.key}`}>
          <div className="text-[10.5px] tracking-[0.06em] uppercase font-semibold text-ink-muted">
            {k.label}
          </div>
          {k.loading ? (
            <div className="or-skeleton h-[38px] w-28 rounded-[8px] mt-2" aria-busy="true" />
          ) : (
            <div className="flex items-baseline gap-1.5 mt-2">
              <span
                className="mono text-[38px] font-semibold tracking-[-0.03em] leading-none"
                style={{ color: k.color }}
                data-testid={`exec-kpi-${k.key}-value`}
              >
                {k.value}
              </span>
              <span className="text-[13px] text-ink-muted">{k.unit}</span>
            </div>
          )}
          <div className="text-[12px] mt-2 font-semibold" style={{ color: k.delta.color }}>
            {k.delta.text}
          </div>
        </div>
      ))}
    </div>
  );
}

/* ---------------------------------------------------------- top 5 risks */

function TopExposures({ className = '' }: { className?: string }) {
  const { t, locale } = useI18n();
  const fin = useFeature('financial_quantification');
  const finance = useFinancialSummary();
  const f = finance.data;
  const top = (f?.top_risks ?? []).filter((r) => r.ale.xaf > 0).slice(0, 5);
  const max = Math.max(1, ...top.map((r) => r.ale.xaf));
  const m = (xaf: number) => millions(toDisplay(xaf, f?.fx_rate_xaf), locale);

  return (
    <Panel testId="exec-top" className={`px-5 py-[18px] ${className}`}>
      <PanelTitle className="mb-3.5" title={t('executive.top.title')} />
      {!fin.loading && !fin.enabled ? (
        <BlockEmpty>{t('dashboard.exposure.locked', { plan: 'Business' })}</BlockEmpty>
      ) : finance.isLoading || fin.loading ? (
        <BlockSkeleton lines={5} height={22} />
      ) : finance.isError || !f ? (
        <BlockError onRetry={() => void finance.refetch()} />
      ) : top.length === 0 ? (
        <BlockEmpty>{t('executive.top.empty')}</BlockEmpty>
      ) : (
        <>
          <div className="grid gap-3.5">
            {top.map((r) => {
              const color = BAND_FILL[r.criticality.toLowerCase()] ?? 'var(--fg-muted)';
              const after = r.ale_after?.xaf ?? r.ale.xaf;
              return (
                <Link
                  key={r.id}
                  to={`/risks?focus=${r.id}`}
                  className="grid gap-1.5 hover:opacity-90"
                  data-testid="exec-top-row"
                >
                  <div className="flex gap-2.5 items-baseline text-[13px]">
                    <span className="mono text-[11.5px] text-ink-muted">#{r.id.slice(0, 8)}</span>
                    <span className="flex-1 min-w-0 truncate font-medium text-ink">{r.title}</span>
                    <span className="mono font-semibold text-ink">
                      {t('executive.top.value', { value: m(r.ale.xaf) })}
                    </span>
                  </div>
                  <div
                    className="h-2 rounded-[8px] relative overflow-hidden"
                    style={{ background: 'var(--chart-track)' }}
                    role="img"
                    aria-label={`${r.title}: ${m(r.ale.xaf)} M → ${m(after)} M`}
                  >
                    <span
                      className="absolute inset-y-0 left-0 rounded-[8px]"
                      style={{ width: `${(r.ale.xaf / max) * 100}%`, background: color }}
                    />
                    <span
                      className="absolute inset-y-0 left-0 rounded-[8px] brightness-[0.6]"
                      style={{ width: `${(after / max) * 100}%`, background: color }}
                    />
                  </div>
                </Link>
              );
            })}
          </div>
          <div className="flex gap-4 flex-wrap mt-3.5 text-[11.5px] text-ink-muted">
            <span>{t('executive.top.inherent')}</span>
            <span>{t('executive.top.residual')}</span>
          </div>
        </>
      )}
    </Panel>
  );
}

/* ------------------------------------------------------ compliance bars */

function FrameworkBars({ className = '' }: { className?: string }) {
  const { t } = useI18n();
  const gaps = useQuery({
    queryKey: ['compliance', 'gap-analysis', 'all'],
    queryFn: () => complianceService.getGapAnalysis(),
  });
  const fw = gaps.data?.frameworks ?? [];
  return (
    <Panel testId="exec-frameworks" className={`px-5 py-[18px] ${className}`}>
      <PanelTitle className="mb-3.5" title={t('executive.fw.title')} />
      {gaps.isLoading ? (
        <BlockSkeleton lines={4} />
      ) : gaps.isError ? (
        <BlockError onRetry={() => void gaps.refetch()} />
      ) : fw.length === 0 ? (
        <BlockEmpty>{t('executive.fw.empty')}</BlockEmpty>
      ) : (
        <>
          <div className="grid gap-3.5">
            {fw.map((f, i) => {
              const pct = Math.round(f.percent_complete ?? 0);
              return (
                <Link
                  key={f.framework_id}
                  to={`/compliance/${f.framework_id}`}
                  className="grid grid-cols-[90px_1fr_44px] gap-3 items-center text-[13px]"
                >
                  <span className="font-medium text-ink truncate" title={f.framework_name}>
                    {shortName(f.framework_name)}
                  </span>
                  <span
                    className="h-2 rounded-[8px] overflow-hidden"
                    style={{ background: 'var(--chart-track)' }}
                  >
                    <span
                      className="block h-full rounded-[8px]"
                      style={{
                        width: `${pct}%`,
                        background: frameworkColorFor(f.framework_name, i),
                      }}
                    />
                  </span>
                  <span className="mono font-semibold text-right text-ink">{pct} %</span>
                </Link>
              );
            })}
          </div>
          <div className="text-[11.5px] text-ink-muted mt-3.5">{t('executive.fw.note')}</div>
        </>
      )}
    </Panel>
  );
}

/**
 * "ISO/IEC 27001:2022" → "ISO 27001", "NIST CSF 2.0" → "NIST CSF",
 * "COBAC R-2016/04" → "COBAC", "BCEAO — Sécurité des SI" → "BCEAO".
 */
function shortName(name: string): string {
  const iso = name.match(/ISO(?:\/IEC)?\s*(\d{4,5})/i);
  if (iso) return `ISO ${iso[1]}`;
  const head = name.split(/\s[—–-]\s|:/)[0].trim();
  // Drop a trailing version or regulation number, never the whole name.
  return head.replace(/\s+\S*\d\S*$/, '').trim() || head;
}

/* ------------------------------------------------- committee decisions */

const APPROVALS_KEY = ['governance', 'approvals', 'pending'];

function CommitteeDecisions() {
  const { t, locale } = useI18n();
  const qc = useQueryClient();
  const approvals = useQuery({
    queryKey: APPROVALS_KEY,
    queryFn: () => governanceService.listApprovals({ status: 'pending' }),
  });
  const [approved, setApproved] = useState<Set<string>>(new Set());

  const onError = (e: unknown) => {
    const forbidden = isAxiosError(e) && e.response?.status === 403;
    toast.error(forbidden ? t('executive.decisions.forbidden') : t('executive.decisions.failed'));
  };
  // Both are optimistic: the row changes on click and rolls back if the
  // server refuses. retry:false — a refused signature is a final answer, and
  // the app-wide mutation retry would send it four times.
  const approve = useMutation({
    mutationFn: (id: string) => governanceService.decideApproval(id, { decision: 'approve' }),
    retry: false,
    onMutate: (id) => setApproved((s) => new Set(s).add(id)),
    onSuccess: () => {
      toast.success(t('executive.decisions.done'));
      void qc.invalidateQueries({ queryKey: ['action-center'] });
    },
    onError: (e, id) => {
      setApproved((s) => {
        const next = new Set(s);
        next.delete(id);
        return next;
      });
      onError(e);
    },
  });
  const defer = useMutation({
    mutationFn: (id: string) => governanceService.deferApproval(id),
    retry: false,
    onMutate: async (id) => {
      await qc.cancelQueries({ queryKey: APPROVALS_KEY });
      const prev = qc.getQueryData<ApprovalRequest[]>(APPROVALS_KEY);
      qc.setQueryData<ApprovalRequest[]>(APPROVALS_KEY, (rows) =>
        rows?.map((r) =>
          r.id === id
            ? {
                ...r,
                deferrals: [
                  ...(r.deferrals ?? []),
                  { deferred_by: '', deferred_at: new Date().toISOString() },
                ],
              }
            : r,
        ),
      );
      return { prev };
    },
    onSuccess: () => toast.success(t('executive.decisions.deferred')),
    onError: (e, _id, ctx) => {
      if (ctx?.prev) qc.setQueryData(APPROVALS_KEY, ctx.prev);
      onError(e);
    },
    onSettled: () => void qc.invalidateQueries({ queryKey: APPROVALS_KEY }),
  });

  // Deferred requests stay pending but go to the bottom, as the design shows.
  const list = useMemo(() => {
    const rows = Array.isArray(approvals.data) ? approvals.data : [];
    const deferred = (r: ApprovalRequest) => (r.deferrals?.length ?? 0) > 0;
    return [...rows].sort((a, b) => Number(deferred(a)) - Number(deferred(b)));
  }, [approvals.data]);
  const waiting = list.filter((r) => !approved.has(r.id) && !(r.deferrals?.length ?? 0)).length;

  return (
    <Panel testId="exec-decisions" className="overflow-hidden">
      <PanelTitle
        className="px-5 pt-4 pb-2.5"
        title={t('executive.decisions.title')}
        aside={
          Array.isArray(approvals.data) ? (
            <span className="text-[12px] text-ink-muted">
              {t('executive.decisions.pending', { count: waiting })}
            </span>
          ) : undefined
        }
      />
      {approvals.isLoading ? (
        <div className="px-5 pb-4">
          <BlockSkeleton lines={3} height={28} />
        </div>
      ) : approvals.isError ? (
        <div className="px-5">
          <BlockError onRetry={() => void approvals.refetch()} />
        </div>
      ) : list.length === 0 ? (
        <div className="px-5 pb-3 border-t border-border-subtle">
          <BlockEmpty>{t('executive.decisions.empty')}</BlockEmpty>
        </div>
      ) : (
        list.map((r) => {
          const last = r.deferrals?.[r.deferrals.length - 1];
          const done = approved.has(r.id);
          return (
            <div
              key={r.id}
              data-testid="exec-decision"
              className="flex items-center gap-4 px-5 py-3.5 border-t border-border-subtle flex-wrap"
            >
              <span className="flex-1 min-w-[260px]">
                <span className="block text-[13.5px] font-semibold text-ink">{r.title}</span>
                <span className="block text-[12px] text-ink-muted mt-0.5">
                  {/* The requester's own summary reads like the design ("Exposition
                      2,1 M FCFA/an…"); without one, say what it is and who asked. */}
                  {r.description?.trim() ||
                    t('executive.decisions.sub', {
                      type: r.workflow_name || r.request_type || r.entity_type,
                      who: r.requested_by_email || '—',
                    })}
                  {r.expires_at &&
                    ` · ${t('executive.decisions.expires', {
                      date: formatDate(locale, r.expires_at, { day: 'numeric', month: 'short' }),
                    })}`}
                </span>
              </span>
              {done ? (
                <span
                  className="text-[12px] font-semibold px-2.5 py-1 rounded-full"
                  style={{ background: 'var(--success-surface)', color: 'var(--success-text)' }}
                >
                  {t('executive.decisions.approved')}
                </span>
              ) : (
                <>
                  {last ? (
                    <span className="text-[12px] font-semibold px-2.5 py-1 rounded-full bg-surface-3 text-ink-soft">
                      {t('executive.decisions.deferredOn', {
                        date: formatDate(locale, last.deferred_at, {
                          day: 'numeric',
                          month: 'short',
                        }),
                      })}
                    </span>
                  ) : (
                    <Button
                      variant="secondary"
                      size="sm"
                      loading={defer.isPending && defer.variables === r.id}
                      onClick={() => defer.mutate(r.id)}
                    >
                      {t('executive.decisions.defer')}
                    </Button>
                  )}
                  <Button
                    variant="primary"
                    size="sm"
                    loading={approve.isPending && approve.variables === r.id}
                    onClick={() => approve.mutate(r.id)}
                  >
                    {t('executive.decisions.approve')}
                  </Button>
                </>
              )}
            </div>
          );
        })
      )}
    </Panel>
  );
}
