// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The four counters on the right of the dashboard's first row (#901). Which
// four depends on the member's view (dashboardVariant.ts); each one reads a
// tenant-scoped aggregate and links to the screen that lists what it counts.
// A counter whose source is loading shows a skeleton, one whose source failed
// shows "—": a zero nobody read is the one number this page must not print.

import { ChevronRight } from 'lucide-react';
import { Link } from 'react-router';
import type { ReactNode } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useI18n } from '../../../hooks/useI18n';
import { usePermissions } from '../../../hooks/usePermissions';
import { formatDate, formatNumber } from '../../../i18n/format';
import { vulnerabilityService } from '../../vulnerabilities/vulnerabilityService';
import { incidentService } from '../../incidents/incidentService';
import { complianceService } from '../../../services/complianceService';
import { evidenceService } from '../../../services/evidenceService';
import { financialService } from '../../financial/financialService';
import { FINANCIAL_SUMMARY_KEY } from '../../financial/useFinancial';
import { currencyLabel, millions, toDisplay } from '../../financial/money';
import { useDashboardStats } from '../useCommandCenter';
import { deepLink } from '../deepLinks';
import type { DashboardVariant } from '../dashboardVariant';
import { Panel } from './Panel';

interface Counter {
  key: string;
  label: string;
  sub: ReactNode;
  value: ReactNode;
  color?: string;
  to: string;
  loading?: boolean;
}

const ALL_TIME = { kind: 'preset', preset: 'all' } as const;

export function CounterList({
  variant,
  className = '',
}: {
  variant: DashboardVariant;
  className?: string;
}) {
  const counters = useCounters(variant);
  return (
    <Panel testId="dash-counters" className={`py-2 ${className}`}>
      {counters.map((c, i) => (
        <Link
          key={c.key}
          to={c.to}
          data-testid={`dash-counter-${c.key}`}
          className={`flex items-center gap-3 px-[18px] py-3 hover:bg-surface-2 transition-colors ${
            i < counters.length - 1 ? 'border-b border-border-subtle' : ''
          }`}
        >
          <span className="flex-1 min-w-0">
            <span className="block text-[12.5px] text-ink-soft">{c.label}</span>
            <span className="block text-[11.5px] text-ink-muted mt-px">{c.sub}</span>
          </span>
          {c.loading ? (
            <span className="or-skeleton w-9 h-[22px] rounded-[6px]" aria-busy="true" />
          ) : (
            <span
              className="mono text-[22px] font-semibold whitespace-nowrap"
              style={{ color: c.color ?? 'var(--fg-primary)' }}
            >
              {c.value}
            </span>
          )}
          <ChevronRight size={14} className="text-ink-muted shrink-0" aria-hidden="true" />
        </Link>
      ))}
    </Panel>
  );
}

function useCounters(variant: DashboardVariant): Counter[] {
  const { t, locale } = useI18n();
  const { can } = usePermissions();
  const stats = useDashboardStats(ALL_TIME);
  const vulns = useQuery({
    queryKey: ['vulnerabilities', 'stats'],
    queryFn: vulnerabilityService.stats,
    enabled: variant === 'rssi' && can('vulnerabilities:read'),
  });
  const incidents = useQuery({
    queryKey: ['incidents', 'stats'],
    queryFn: () => incidentService.stats(),
    enabled: variant === 'rssi' && can('incidents:read'),
  });
  // Same query keys as the compliance, evidence and financial screens, so a
  // visit to either side reuses the other's cache. Each source is fetched only
  // for the view that shows it and only when the member may read it.
  const gaps = useQuery({
    queryKey: ['compliance', 'gap-analysis', 'all'],
    queryFn: () => complianceService.getGapAnalysis(),
    enabled: (variant === 'rssi' || variant === 'audit') && can('compliance:controls:read'),
  });
  const missing = useQuery({
    queryKey: ['evidence', 'missing', 'all'],
    queryFn: () => evidenceService.missing(),
    enabled: variant === 'audit' && can('compliance:evidences:read'),
  });
  const audits = useQuery({
    queryKey: ['compliance', 'audits'],
    queryFn: () => complianceService.listAudits(),
    enabled: variant === 'audit' && can('compliance:audits:read'),
  });
  const finance = useQuery({
    queryKey: FINANCIAL_SUMMARY_KEY,
    queryFn: financialService.getSummary,
    enabled: variant === 'exec' && can('risks:read'),
  });

  const n = (v: number | undefined) => (v === undefined ? '—' : formatNumber(locale, v));
  const s = stats.data;

  switch (variant) {
    case 'rm':
      return [
        {
          key: 'open-risks',
          label: t('dashboard.counters.openRisks'),
          sub: t('dashboard.counters.openRisksSub'),
          value: n(s?.live_risks),
          to: deepLink('risks'),
          loading: stats.isLoading,
        },
        {
          key: 'without-mitigation',
          label: t('dashboard.counters.withoutMitigation'),
          sub: t('dashboard.counters.withoutMitigationSub'),
          value: n(s?.without_mitigation),
          color: 'var(--warning-text)',
          to: deepLink('risks'),
          loading: stats.isLoading,
        },
        {
          key: 'in-treatment',
          label: t('dashboard.counters.inTreatment'),
          sub: t('dashboard.counters.inTreatmentSub'),
          value: n(s?.in_progress_risks),
          color: 'var(--accent-500)',
          to: '/risks/mitigations',
          loading: stats.isLoading,
        },
        {
          key: 'reviews',
          label: t('dashboard.counters.reviews'),
          sub: t('dashboard.counters.reviewsSub'),
          value: n(s?.reviews_due_30d),
          to: deepLink('risks'),
          loading: stats.isLoading,
        },
      ];
    case 'audit': {
      const fw = gaps.data?.frameworks ?? [];
      const cov = missing.data ?? [];
      const nonCompliant = gaps.data ? fw.reduce((a, f) => a + (f.not_implemented ?? 0), 0) : undefined;
      const expiring = missing.data ? cov.reduce((a, f) => a + f.expiring_soon, 0) : undefined;
      const noEvidence = missing.data ? cov.reduce((a, f) => a + f.no_evidence, 0) : undefined;
      const today = new Date().toISOString().slice(0, 10);
      const next = (audits.data ?? [])
        .filter((a) => a.status === 'planned' && a.scheduled_start && a.scheduled_start.slice(0, 10) >= today)
        .sort((a, b) => (a.scheduled_start ?? '').localeCompare(b.scheduled_start ?? ''))[0];
      return [
        {
          key: 'gaps',
          label: t('dashboard.counters.gaps'),
          sub: t('dashboard.counters.gapsSub'),
          value: n(nonCompliant),
          color: 'var(--danger-text)',
          to: '/compliance/gaps',
          loading: gaps.isLoading,
        },
        {
          key: 'expiring',
          label: t('dashboard.counters.expiring'),
          sub: t('dashboard.counters.expiringSub'),
          value: n(expiring),
          color: 'var(--warning-text)',
          to: '/compliance/evidence',
          loading: missing.isLoading,
        },
        {
          key: 'no-evidence',
          label: t('dashboard.counters.noEvidence'),
          sub: t('dashboard.counters.noEvidenceSub'),
          value: n(noEvidence),
          to: '/compliance/evidence/missing',
          loading: missing.isLoading,
        },
        {
          key: 'next-audit',
          label: t('dashboard.counters.nextAudit'),
          sub: next ? next.title : t('dashboard.counters.nextAuditNone'),
          value: next?.scheduled_start
            ? formatDate(locale, next.scheduled_start, { day: '2-digit', month: '2-digit' })
            : '—',
          to: next ? `/compliance/audits/${next.id}` : '/compliance/audits',
          loading: audits.isLoading,
        },
      ];
    }
    case 'exec': {
      const f = finance.data;
      const cur = currencyLabel(f?.currency);
      const m = (xaf: number | undefined) =>
        xaf === undefined || !f ? '—' : millions(toDisplay(xaf, f.fx_rate_xaf), locale);
      return [
        {
          key: 'ale',
          label: t('dashboard.counters.ale'),
          sub: t('dashboard.counters.aleSub', { currency: cur }),
          value: m(f?.total_ale.xaf),
          to: '/analytics/financial',
          loading: finance.isLoading,
        },
        {
          key: 'reduction',
          label: t('dashboard.counters.reduction'),
          sub: t('dashboard.counters.reductionSub', { currency: cur }),
          value: m(f?.total_risk_reduction.xaf),
          color: 'var(--success-text)',
          to: '/analytics/financial',
          loading: finance.isLoading,
        },
        {
          key: 'plan-cost',
          label: t('dashboard.counters.planCost'),
          sub: t('dashboard.counters.planCostSub', { currency: cur }),
          value: m(f?.total_remediation.xaf),
          to: '/analytics/financial',
          loading: finance.isLoading,
        },
        {
          // The risk appetite becomes a tenant setting with the financial
          // screen (#904). Until then the counter says where to set it.
          key: 'appetite',
          label: t('dashboard.counters.appetite'),
          sub: t('dashboard.counters.appetiteUnset'),
          value: t('dashboard.counters.unavailable'),
          to: '/analytics/financial',
        },
      ];
    }
    default: {
      const fw = gaps.data?.frameworks ?? [];
      const coverage =
        gaps.data && fw.length
          ? Math.round(fw.reduce((a, f) => a + (f.percent_complete ?? 0), 0) / fw.length)
          : undefined;
      return [
        {
          key: 'critical-risks',
          label: t('dashboard.counters.criticalRisks'),
          sub: t('dashboard.counters.criticalRisksSub'),
          value: n(s?.critical_live),
          color: 'var(--risk-critical)',
          to: deepLink('risks', {
            filters: { criticality: 'critical' },
            sort: { key: 'score', dir: 'desc' },
          }),
          loading: stats.isLoading,
        },
        {
          key: 'kev',
          label: t('dashboard.counters.kev'),
          sub: t('dashboard.counters.kevSub'),
          value: n(vulns.data?.kev_open),
          color: 'var(--danger-text)',
          to: deepLink('vulnerabilities', { filters: { kev: 'true' } }),
          loading: vulns.isLoading,
        },
        {
          key: 'incidents',
          label: t('dashboard.counters.incidents'),
          sub: t('dashboard.counters.incidentsSub', {
            count: incidents.data?.critical_incidents ?? 0,
          }),
          value: n(incidents.data?.open_incidents),
          to: '/incidents',
          loading: incidents.isLoading,
        },
        {
          key: 'coverage',
          label: t('dashboard.counters.coverage'),
          sub: t('dashboard.counters.coverageSub', { count: fw.length }),
          value: coverage === undefined ? '—' : `${coverage} %`,
          to: '/compliance',
          loading: gaps.isLoading,
        },
      ];
    }
  }
}
