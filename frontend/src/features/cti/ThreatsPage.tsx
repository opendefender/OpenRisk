// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Threat Intel (#911, on the redesign layout). The intelligence catalogue (NVD,
// CISA KEV, MITRE ATT&CK) read as threats to this tenant: only the entries that
// touch its assets, by CPE or because a scanner found them, ranked by a
// relevance the backend computes and the page explains part by part (D-069 E).
//
// The IOC panel keeps the design's place but says plainly that matches are
// unavailable: OpenRisk ingests no logs yet, and the current sources publish
// no indicators. Nothing in it is simulated. The full CVE feed stays one click
// away at /threat-map.

import { useMemo, useState } from 'react';
import { Link } from 'react-router';
import { toast } from 'sonner';
import { Database, Globe, Info, ShieldAlert } from 'lucide-react';

import { useI18n } from '../../hooks/useI18n';
import { useAuthStore } from '../../hooks/useAuthStore';
import { formatDate, formatNumber } from '../../i18n/format';
import { Button, Popover } from '../../shared/ds';
import { PageFrame, PageHeader } from '../../shared/ui';
import { BlockEmpty, BlockError, BlockSkeleton, HeaderLink } from '../dashboard/console/Panel';
import { CreateRiskModal } from '../risks/CreateRiskModal';
import { useThreatDismissal, useThreats } from './useCTI';
import type { Threat, ThreatFactor } from './ctiService';
import {
  relevanceTone,
  sourceLabels,
  threatTitle,
  visibleThreats,
  type ThreatMode,
} from './threatMeta';
import { FeatureGate } from '../../shared/FeatureGate';

const MODES: readonly ThreatMode[] = ['all', 'high'];

// The plan gate (#783, ADR 0007): the API refuses these routes with 402 on a
// plan without cti; the screen shows the upsell instead of the refusal.
export function ThreatsPage() {
  return (
    <FeatureGate feature="cti">
      <ThreatsPageScreen />
    </FeatureGate>
  );
}

function ThreatsPageScreen() {
  const { t } = useI18n();
  const [mode, setMode] = useState<ThreatMode>('all');
  const threats = useThreats();
  const list = useMemo(
    () => visibleThreats(threats.data?.threats ?? [], mode),
    [threats.data, mode],
  );

  return (
    <PageFrame wide>
      <PageHeader
        className="!mb-[18px]"
        title={t('threats.title')}
        subtitle={t('threats.subtitle')}
        actions={
          <div className="flex items-center gap-3">
            <HeaderLink to="/threat-map">{t('threats.feedLink')}</HeaderLink>
            <div
              role="group"
              aria-label={t('threats.modeLabel')}
              className="flex gap-0.5 p-[3px] rounded-[10px] bg-surface-1 border border-border-subtle"
            >
              {MODES.map((m) => {
                const on = m === mode;
                return (
                  <button
                    key={m}
                    type="button"
                    aria-pressed={on}
                    onClick={() => setMode(m)}
                    data-testid={`threat-mode-${m}`}
                    className={`h-7 px-2.5 rounded-[7px] text-[12.5px] font-semibold transition-colors ${
                      on ? 'bg-surface-3 text-ink' : 'text-ink-soft hover:text-ink'
                    }`}
                  >
                    {t(`threats.mode.${m}`)}
                  </button>
                );
              })}
            </div>
          </div>
        }
      />

      <div className="flex gap-4 flex-wrap items-start">
        <div className="flex-[1.5_1_520px] min-w-0 grid gap-3" data-testid="threat-list">
          {threats.isLoading ? (
            <section className="rounded-[14px] border border-border-subtle bg-surface-1 px-[18px] py-4">
              <BlockSkeleton lines={6} height={18} />
            </section>
          ) : threats.isError ? (
            <section className="rounded-[14px] border border-border-subtle bg-surface-1 px-[18px]">
              <BlockError onRetry={() => void threats.refetch()} />
            </section>
          ) : list.length === 0 ? (
            <section className="rounded-[14px] border border-border-subtle bg-surface-1 px-[18px] py-4 grid gap-3 justify-items-start">
              <BlockEmpty>
                {mode === 'high' && (threats.data?.threats.length ?? 0) > 0
                  ? t('threats.emptyHigh')
                  : threats.data?.assets_with_cpes === 0
                    ? t('threats.emptyNoCpe')
                    : t('threats.empty')}
              </BlockEmpty>
              {mode === 'high' && (threats.data?.threats.length ?? 0) > 0 ? (
                <Button variant="secondary" size="sm" onClick={() => setMode('all')}>
                  {t('threats.showAll')}
                </Button>
              ) : (
                <Link
                  to="/assets"
                  className="text-[12.5px] font-semibold text-accent-strong hover:underline"
                >
                  {t('threats.toAssets')}
                </Link>
              )}
            </section>
          ) : (
            <>
              {list.map((th) => (
                <ThreatCard key={th.cve_id} threat={th} />
              ))}
              {threats.data?.truncated && (
                <p className="m-0 text-[12px] text-ink-muted">{t('threats.truncated')}</p>
              )}
            </>
          )}
        </div>

        <IocPanel />
      </div>
    </PageFrame>
  );
}

/* ------------------------------------------------------------------ card */

function ThreatCard({ threat }: { threat: Threat }) {
  const { t, locale } = useI18n();
  const canDismiss = useAuthStore((s) => s.hasPermission('risks:update'));
  const canCreateRisk = useAuthStore((s) => s.hasPermission('risks:create'));
  const dismissal = useThreatDismissal();
  const [riskOpen, setRiskOpen] = useState(false);
  const dismissed = !!threat.dismissed_at;
  const title = threatTitle(threat);

  const toggleDismissed = () =>
    dismissal.mutate(
      { cve: threat.cve_id, dismissed: !dismissed },
      { onError: () => toast.error(t('threats.toast.dismissFailed')) },
    );

  const meta = [
    // The strongest feed leads: KEV says it is exploited, NVD only that it exists.
    sourceLabels([threat.sources.includes('cisa_kev') ? 'cisa_kev' : 'nvd'])[0],
    threat.published_at
      ? formatDate(locale, threat.published_at, { day: 'numeric', month: 'short' })
      : null,
    threat.cve_id,
  ].filter(Boolean);

  return (
    <article
      className="rounded-[14px] border border-border-subtle bg-surface-1 px-[18px] py-4 transition-opacity"
      style={{ opacity: dismissed ? 0.45 : 1 }}
      aria-label={title}
      data-testid={`threat-${threat.cve_id}`}
      data-dismissed={dismissed || undefined}
    >
      <div className="flex gap-3.5 items-start">
        <div className="flex-1 min-w-0">
          <div className="text-[11.5px] text-ink-muted">{meta.join(' · ')}</div>
          <h3 className="m-0 mt-1 text-[15px] font-semibold leading-[1.35] text-ink">{title}</h3>
        </div>
        <RelevanceScore threat={threat} />
      </div>

      <div className="flex gap-1.5 flex-wrap mt-3">
        {threat.techniques.map((x) => (
          <span
            key={x}
            className="h-[22px] px-2 rounded-md bg-surface-3 mono text-[11px] text-ink-soft inline-flex items-center"
          >
            {x}
          </span>
        ))}
        {threat.assets.map((a) => {
          const how = a.via.map((v) => t(`threats.via.${v}`)).join(', ');
          const chip = (
            <>
              <Database size={12} aria-hidden="true" className="text-ink-muted" />
              {a.name}
              <span className="sr-only"> ({how})</span>
            </>
          );
          const cls =
            'h-[22px] px-2 rounded-md border border-border-default text-[11.5px] text-ink inline-flex items-center gap-[5px]';
          return a.id ? (
            <Link
              key={a.id}
              to={`/assets?focus=${encodeURIComponent(a.id)}`}
              title={how}
              className={`${cls} hover:border-border-strong`}
            >
              {chip}
            </Link>
          ) : (
            <span key={`name:${a.name}`} title={how} className={cls}>
              {chip}
            </span>
          );
        })}
      </div>

      {(canCreateRisk || canDismiss) && (
        <div className="flex gap-2 mt-3.5">
          {canCreateRisk && (
            <Button
              variant="secondary"
              size="sm"
              icon={ShieldAlert}
              onClick={() => setRiskOpen(true)}
              disabled={dismissed}
            >
              {t('threats.createRisk')}
            </Button>
          )}
          {canDismiss && (
            <Button
              variant="ghost"
              size="sm"
              onClick={toggleDismissed}
              data-testid={`threat-dismiss-${threat.cve_id}`}
            >
              {dismissed ? t('threats.restore') : t('threats.dismiss')}
            </Button>
          )}
        </div>
      )}

      {riskOpen && (
        <CreateRiskModal
          isOpen
          onClose={() => setRiskOpen(false)}
          initialAssetIds={threat.assets.flatMap((a) => (a.id ? [a.id] : []))}
          initialTitle={t('threats.riskTitle', { title, cve: threat.cve_id })}
        />
      )}
    </article>
  );
}

/* --------------------------------------------------- relevance, explained */

function RelevanceScore({ threat }: { threat: Threat }) {
  const { t, locale } = useI18n();
  const factorText = (f: ThreatFactor) => {
    switch (f.key) {
      case 'asset':
        return t('threats.factor.asset', {
          crit: t(`assetAnalysis.levels.${f.value || 'MEDIUM'}`, f.value),
        });
      case 'exploitation':
        return f.value ? t(`threats.factor.${f.value}`) : t('threats.factor.noExploitation');
      case 'severity':
        return threat.cvss > 0
          ? t('threats.factor.cvss', {
              value: formatNumber(locale, threat.cvss, {
                minimumFractionDigits: 1,
                maximumFractionDigits: 1,
              }),
            })
          : t('threats.factor.noCvss');
      case 'evidence':
        return t(`threats.factor.${f.value === 'scan' ? 'scan' : 'cpe'}`);
    }
  };
  return (
    <Popover
      placement="left"
      label={t('threats.why', { value: threat.relevance })}
      trigger={
        <Button
          variant="ghost"
          aria-label={t('threats.relevanceSr', { value: threat.relevance })}
          data-testid={`threat-relevance-${threat.cve_id}`}
          className="h-auto flex-col items-end gap-0 px-1.5 py-0.5 -mr-1.5 rounded-lg"
        >
          <span
            className={`mono text-[22px] font-semibold leading-tight ${relevanceTone(threat.relevance)}`}
          >
            {threat.relevance}
          </span>
          <span className="text-[10.5px] tracking-[.06em] uppercase font-semibold text-ink-muted inline-flex items-center gap-1">
            {t('threats.relevance')}
            <Info size={11} aria-hidden="true" />
          </span>
        </Button>
      }
    >
      <div className="w-[300px] grid gap-2 text-[12.5px]" data-testid="threat-why">
        <div className="font-semibold text-ink">
          {t('threats.why', { value: threat.relevance })}
        </div>
        <ul className="m-0 p-0 list-none grid gap-1">
          {threat.factors.map((f) => (
            <li key={f.key} className="flex justify-between gap-3">
              <span className="text-ink-soft">{factorText(f)}</span>
              <span className="mono text-ink whitespace-nowrap">
                {f.points}/{f.max}
              </span>
            </li>
          ))}
        </ul>
        <p className="m-0 text-[11.5px] text-ink-muted">{t('threats.factor.sector')}</p>
        <p className="m-0 text-[11.5px] text-ink-muted">
          {t('threats.sources', { list: sourceLabels(threat.sources).join(', ') })}
        </p>
      </div>
    </Popover>
  );
}

/* -------------------------------------------------------------- IOC panel */

function IocPanel() {
  const { t } = useI18n();
  return (
    <section
      className="flex-[1_1_340px] min-w-0 rounded-[14px] border border-border-subtle bg-surface-1 overflow-hidden"
      aria-labelledby="ioc-title"
      data-testid="ioc-panel"
    >
      <div className="px-[18px] pt-4 pb-2.5">
        <h2 id="ioc-title" className="m-0 text-[13.5px] font-semibold text-ink">
          {t('threats.ioc.title')}
        </h2>
        <div className="text-[12px] text-ink-muted mt-0.5">{t('threats.ioc.subtitle')}</div>
      </div>
      <div className="border-t border-border-subtle px-[18px] py-4 grid gap-2 justify-items-start">
        <Globe size={18} aria-hidden="true" className="text-ink-muted" />
        <p className="m-0 text-[13px] font-semibold text-ink">{t('threats.ioc.unavailable')}</p>
        <p className="m-0 text-[12.5px] text-ink-soft leading-relaxed">{t('threats.ioc.why')}</p>
      </div>
    </section>
  );
}
