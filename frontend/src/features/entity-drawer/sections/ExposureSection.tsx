// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// "Exposition" in the asset drawer (#937): what the asset tells a security
// reader. A reading made of facts the server computed (exploited flaws, missed
// deadlines, exposure, business weight, linked risks), the one finding to treat
// first and why, then the open findings by urgency and the linked risks.
// Nothing here is generated, and no score is derived in the browser: risk
// bands are the engine's.

import { AlertTriangle, CircleCheck, Clock, Globe, Info, Link2, ShieldAlert } from 'lucide-react';
import { Link } from 'react-router';

import { useI18n } from '../../../hooks/useI18n';
import { formatDate, formatNumber } from '../../../i18n/format';
import { Button } from '../../../shared/ds';
import { CritBadge } from '../../../shared/ui';
import type { Criticality } from '../../../shared/riskColors';
import { BlockError, BlockSkeleton } from '../../dashboard/console/Panel';
import type { EntityType } from '../types';
import type { AssetAnalysis } from '../useEntityDrawer';

type Finding = AssetAnalysis['findings'][number];
type Vuln = AssetAnalysis['vulnerabilities'][number];

const SHOWN = 10;

const SEV_COLOR: Record<string, string> = {
  critical: 'var(--risk-critical)',
  high: 'var(--risk-high)',
  medium: 'var(--risk-moderate)',
  low: 'var(--risk-low)',
  info: 'var(--fg-muted)',
};

const TONE: Record<string, { icon: typeof Info; color: string }> = {
  kev_open: { icon: ShieldAlert, color: 'var(--danger-text)' },
  overdue: { icon: Clock, color: 'var(--danger-text)' },
  severe_open: { icon: AlertTriangle, color: 'var(--warning-text)' },
  internet_facing: { icon: Globe, color: 'var(--warning-text)' },
  business_critical: { icon: Info, color: 'var(--fg-secondary)' },
  linked_risks: { icon: Link2, color: 'var(--fg-secondary)' },
  clean: { icon: CircleCheck, color: 'var(--success-text)' },
  never_scanned: { icon: Info, color: 'var(--fg-muted)' },
};

export function ExposureSection({
  data,
  isLoading,
  isError,
  onRetry,
  onOpen,
}: {
  data: AssetAnalysis | undefined;
  isLoading: boolean;
  isError: boolean;
  onRetry: () => void;
  onOpen: (type: EntityType, id: string) => void;
}) {
  const { t, locale } = useI18n();
  if (isLoading) return <BlockSkeleton lines={6} height={20} />;
  if (isError || !data) return <BlockError onRetry={onRetry} />;

  const sentence = (f: Finding): string => {
    switch (f.code) {
      case 'clean':
        return (
          t('assetAnalysis.f.clean', { count: f.count ?? 0 }) +
          (data.last_detected_at
            ? ` ${t('assetAnalysis.lastDetected', { date: formatDate(locale, data.last_detected_at) })}.`
            : '')
        );
      case 'internet_facing':
        return f.value
          ? t('assetAnalysis.f.internet_facing', {
              zone: t(`assetAnalysis.zones.${f.value}`, { defaultValue: f.value }),
            })
          : t('assetAnalysis.f.internet_facing_flag');
      case 'business_critical':
        return t('assetAnalysis.f.business_critical', {
          level: t(`assetAnalysis.levels.${f.value ?? 'HIGH'}`),
        });
      case 'linked_risks':
        return t('assetAnalysis.f.linked_risks', {
          count: f.count ?? 0,
          score: formatNumber(locale, Number(f.value ?? 0), { maximumFractionDigits: 1 }),
        });
      default:
        return t(`assetAnalysis.f.${f.code}`, { count: f.count ?? 0, days: f.days ?? 0 });
    }
  };

  const next = data.next_action
    ? data.vulnerabilities.find((v) => v.id === data.next_action?.vulnerability_id)
    : undefined;
  const why = (r: string) =>
    r.startsWith('severity:')
      ? t('assetAnalysis.why.severity', {
          level: t(`assetAnalysis.sev.${r.slice(9)}`).toLowerCase(),
        })
      : t(`assetAnalysis.why.${r}`);

  return (
    <div className="grid gap-5" data-testid="exposure-section">
      <section>
        <h3 className="m-0 mb-2 text-[11px] uppercase tracking-[.06em] font-semibold text-ink-muted">
          {t('assetAnalysis.reading')}
        </h3>
        <ul className="m-0 p-0 list-none grid gap-1.5" data-testid="exposure-reading">
          {data.findings.map((f) => {
            const tone = TONE[f.code] ?? TONE.never_scanned;
            const Icon = tone.icon;
            return (
              <li key={f.code} className="flex gap-2 items-start text-[13px] leading-snug text-ink">
                <Icon
                  size={15}
                  className="shrink-0 mt-0.5"
                  style={{ color: tone.color }}
                  aria-hidden="true"
                />
                <span>{sentence(f)}</span>
              </li>
            );
          })}
        </ul>
        {data.open > 0 && (
          <div className="flex flex-wrap gap-1.5 mt-3" data-testid="exposure-counts">
            {(['critical', 'high', 'medium', 'low'] as const).map((s) =>
              data.by_severity[s] ? (
                <span
                  key={s}
                  className="inline-flex items-center gap-1.5 h-6 px-2 rounded-full text-[11.5px] font-semibold bg-surface-2 text-ink"
                >
                  <span className="w-1.5 h-1.5 rounded-full" style={{ background: SEV_COLOR[s] }} />
                  {t(`assetAnalysis.sev.${s}`)} <span className="mono">{data.by_severity[s]}</span>
                </span>
              ) : null,
            )}
          </div>
        )}
      </section>

      {next && (
        <section
          className="rounded-[12px] border p-3.5"
          style={{
            borderColor: 'color-mix(in srgb, var(--danger-text) 35%, transparent)',
            background: 'color-mix(in srgb, var(--danger-text) 6%, transparent)',
          }}
          data-testid="exposure-next"
        >
          <div className="text-[11px] uppercase tracking-[.06em] font-semibold text-ink-muted mb-1.5">
            {t('assetAnalysis.next')}
          </div>
          <div className="flex items-start gap-3">
            <div className="flex-1 min-w-0">
              <div className="text-[13.5px] font-semibold text-ink">
                {next.cve_id && <span className="mono mr-2">{next.cve_id}</span>}
                {next.title}
              </div>
              <div className="text-[12px] text-ink-soft mt-1">
                {data.next_action?.reasons.map(why).join(' · ')}
              </div>
              {next.remediation_hint && (
                <p className="m-0 mt-1.5 text-[12.5px] text-ink-soft">{next.remediation_hint}</p>
              )}
            </div>
            <Button size="sm" variant="secondary" onClick={() => onOpen('vulnerability', next.id)}>
              {t('assetAnalysis.open')}
            </Button>
          </div>
        </section>
      )}

      {data.vulnerabilities.length > 0 && (
        <section>
          <h3 className="m-0 mb-1 text-[11px] uppercase tracking-[.06em] font-semibold text-ink-muted">
            {t('assetAnalysis.openVulns', { count: data.open })}
          </h3>
          <ul className="m-0 p-0 list-none" data-testid="exposure-vulns">
            {data.vulnerabilities.slice(0, SHOWN).map((v) => (
              <VulnRow key={v.id} v={v} onOpen={() => onOpen('vulnerability', v.id)} />
            ))}
          </ul>
          {(data.vulnerabilities.length > SHOWN || data.truncated) && (
            <div className="mt-2 text-[12px] text-ink-muted flex flex-wrap gap-3 items-center">
              {data.truncated && <span>{t('assetAnalysis.truncated')}</span>}
              <Link to="/vulnerabilities" className="text-accent hover:underline">
                {t('assetAnalysis.more')}
              </Link>
            </div>
          )}
        </section>
      )}

      {data.risks.length > 0 && (
        <section>
          <h3 className="m-0 mb-1 text-[11px] uppercase tracking-[.06em] font-semibold text-ink-muted">
            {t('assetAnalysis.risks')}
          </h3>
          <ul className="m-0 p-0 list-none" data-testid="exposure-risks">
            {data.risks.map((r) => (
              <li key={r.id} className="border-t border-border-subtle first:border-t-0">
                <button
                  type="button"
                  onClick={() => onOpen('risk', r.id)}
                  className="w-full flex items-center gap-3 py-2 text-left hover:bg-surface-2 rounded-[8px] px-1"
                >
                  <span className="flex-1 min-w-0 truncate text-[13px] text-ink">{r.title}</span>
                  <span className="mono text-[12.5px] font-semibold text-ink">
                    {formatNumber(locale, r.score, { maximumFractionDigits: 1 })}
                  </span>
                  <CritBadge crit={(r.criticality || 'low') as Criticality} />
                </button>
              </li>
            ))}
          </ul>
        </section>
      )}
    </div>
  );
}

function VulnRow({ v, onOpen }: { v: Vuln; onOpen: () => void }) {
  const { t, locale } = useI18n();
  const due = v.sla_due_at
    ? t(v.overdue ? 'assetAnalysis.dueLate' : 'assetAnalysis.due', {
        date: formatDate(locale, v.sla_due_at, { day: 'numeric', month: 'short' }),
      })
    : t('assetAnalysis.noDue');
  return (
    <li className="border-t border-border-subtle first:border-t-0" data-testid="exposure-vuln">
      <button
        type="button"
        onClick={onOpen}
        className="w-full grid grid-cols-[auto_1fr_auto] gap-x-3 gap-y-0.5 items-baseline py-2 text-left hover:bg-surface-2 rounded-[8px] px-1"
      >
        <span
          className="w-2 h-2 rounded-full self-center"
          style={{ background: SEV_COLOR[v.severity] ?? SEV_COLOR.info }}
          title={t(`assetAnalysis.sev.${v.severity}`)}
        />
        <span className="min-w-0 truncate text-[13px] text-ink">
          {v.cve_id && <span className="mono text-[12px] text-ink-soft mr-2">{v.cve_id}</span>}
          {v.title}
        </span>
        <span className="flex items-center gap-1.5 justify-end">
          {v.kev && (
            <span className="text-[10.5px] font-bold tracking-[.04em] px-[5px] py-0.5 rounded bg-danger-surface text-danger-text">
              {t('assetAnalysis.kev')}
            </span>
          )}
          {v.overdue && (
            <span className="text-[10.5px] font-semibold px-[5px] py-0.5 rounded bg-danger-surface text-danger-text">
              {t('assetAnalysis.overdueChip')}
            </span>
          )}
        </span>
        <span />
        <span className="text-[11.5px] text-ink-muted col-span-2">
          {[
            t(`assetAnalysis.sev.${v.severity}`),
            v.cvss > 0
              ? t('assetAnalysis.cvss', {
                  score: formatNumber(locale, v.cvss, { maximumFractionDigits: 1 }),
                })
              : '',
            v.epss > 0
              ? t('assetAnalysis.epss', {
                  pct: formatNumber(locale, v.epss, { style: 'percent', maximumFractionDigits: 1 }),
                })
              : '',
            due,
          ]
            .filter(Boolean)
            .join(' · ')}
        </span>
      </button>
    </li>
  );
}

/**
 * The exposure in brief, at the top of an asset's Aperçu: the most pressing
 * lines of the reading, the finding to treat first, and the way to the full
 * analysis. Reading an asset starts with what threatens it.
 */
export function ExposureDigest({
  data,
  isLoading,
  onMore,
}: {
  data: AssetAnalysis | undefined;
  isLoading: boolean;
  onMore: () => void;
}) {
  const { t } = useI18n();
  if (isLoading) return <BlockSkeleton lines={2} height={16} />;
  if (!data) return null;
  const urgent = data.findings.filter((f) =>
    ['kev_open', 'overdue', 'severe_open', 'clean', 'never_scanned'].includes(f.code),
  );
  const next = data.next_action
    ? data.vulnerabilities.find((v) => v.id === data.next_action?.vulnerability_id)
    : undefined;
  const bad = data.kev_open > 0 || data.overdue > 0;
  return (
    <section
      className="rounded-[12px] border p-3.5 mb-4"
      style={{
        borderColor: bad
          ? 'color-mix(in srgb, var(--danger-text) 35%, transparent)'
          : 'var(--border-subtle)',
      }}
      data-testid="exposure-digest"
    >
      <div className="flex items-center justify-between gap-3 mb-1.5">
        <span className="text-[11px] uppercase tracking-[.06em] font-semibold text-ink-muted">
          {t('assetAnalysis.digest')}
        </span>
        <button
          type="button"
          onClick={onMore}
          className="text-[12px] font-semibold text-accent hover:underline"
        >
          {t('assetAnalysis.seeAnalysis')}
        </button>
      </div>
      <ul className="m-0 p-0 list-none grid gap-1">
        {urgent.map((f) => {
          const tone = TONE[f.code] ?? TONE.never_scanned;
          const Icon = tone.icon;
          return (
            <li key={f.code} className="flex gap-2 items-start text-[13px] text-ink">
              <Icon
                size={14}
                className="shrink-0 mt-0.5"
                style={{ color: tone.color }}
                aria-hidden="true"
              />
              <span>
                {f.code === 'clean'
                  ? t('assetAnalysis.f.clean', { count: f.count ?? 0 })
                  : t(`assetAnalysis.f.${f.code}`, { count: f.count ?? 0, days: f.days ?? 0 })}
              </span>
            </li>
          );
        })}
      </ul>
      {next && (
        <p className="m-0 mt-2 text-[12.5px] text-ink-soft">
          {t('assetAnalysis.next')} :{' '}
          <span className="text-ink font-medium">
            {next.cve_id ? `${next.cve_id} — ` : ''}
            {next.title}
          </span>
        </p>
      )}
    </section>
  );
}
