// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The exposure score card of the redesigned dashboard (#901).
//
// The design draws a "security score" where higher is better. The tenant score
// is an exposure score where 0 is no exposure and 100 the worst, and the owner
// chose to keep that direction (2026-10-08). So the card has the design's
// layout, but it is labelled for what it measures: a falling score is good
// news and shows green, a rising one shows red, and each factor bar is a
// pressure (100 = worst).
//
// Colours come from the server's band for the headline and from the score
// model's own band table for each factor. No threshold lives in this file.

import { ArrowDownRight, ArrowUpRight } from 'lucide-react';
import { useI18n } from '../../../hooks/useI18n';
import { useScore, useScoreModel } from '../../../hooks/useScore';
import {
  bandColor,
  bandLabel,
  bandTextColor,
  unmeasuredReason,
  type ScoreFactor,
  type ScoreModel,
} from '../../../services/scoreService';
import { bandOfReading } from './scoreBands';
import { formatNumber } from '../../../i18n/format';
import { useScoreHistory } from '../useScoreHistory';
import { BlockError, BlockSkeleton, Eyebrow, HeaderLink, Panel } from './Panel';

// The design's reading order.
const FACTOR_ORDER = [
  'vulnerability_pressure',
  'risk_exposure',
  'control_gaps',
  'incident_pressure',
] as const;

export function ScoreCard({ className = '' }: { className?: string }) {
  const { t, locale } = useI18n();
  const score = useScore('tenant');
  const model = useScoreModel();
  const history = useScoreHistory(12);

  const data = score.data;
  const measured = data?.measured ? data : undefined;
  const value = measured ? Math.round(measured.value) : undefined;
  const delta = history.data?.delta_30d ?? null;

  const factors: ScoreFactor[] = FACTOR_ORDER.map((key) =>
    data?.breakdown.find((f) => f.factor === key),
  ).filter((f): f is ScoreFactor => !!f);
  const measuredCount = factors.filter((f) => f.available).length;

  return (
    <Panel testId="dash-score" className={`p-5 ${className}`}>
      <div className="flex justify-between items-center">
        <Eyebrow>{t('dashboard.score.label')}</Eyebrow>
        <HeaderLink to="/score">{t('dashboard.score.detail')}</HeaderLink>
      </div>

      {score.isLoading ? (
        <div className="mt-4">
          <BlockSkeleton lines={5} />
        </div>
      ) : score.isError ? (
        <BlockError onRetry={() => void score.refetch()} />
      ) : (
        <>
          <div className="flex items-baseline gap-2.5 mt-2.5 flex-wrap">
            <span
              className="mono text-[56px] font-semibold leading-none tracking-[-0.04em]"
              style={{ color: measured ? bandTextColor(measured.band) : 'var(--fg-muted)' }}
              data-testid="dash-score-value"
            >
              {value ?? '—'}
            </span>
            <span className="mono text-[15px] text-ink-muted">{t('dashboard.score.of100')}</span>
            {measured && (
              <span
                className="text-[11px] font-semibold px-[9px] py-[3px] rounded-full ml-1"
                style={{
                  color: bandTextColor(measured.band),
                  background: `color-mix(in srgb, ${bandColor(measured.band)} 14%, transparent)`,
                }}
              >
                {bandLabel(measured.band, locale)}
              </span>
            )}
            {measured && <ScoreDelta delta={delta} />}
          </div>

          {measured ? (
            <div className="grid gap-2.5 mt-5">
              {factors.map((f) => (
                <FactorRow key={f.factor} factor={f} model={model.data} />
              ))}
            </div>
          ) : (
            <div className="mt-5" data-testid="dash-score-unmeasured">
              <p className="m-0 text-[13px] font-semibold text-ink">{bandLabel(undefined, locale)}</p>
              <p className="m-0 mt-1 text-[12.5px] text-ink-muted leading-relaxed">
                {unmeasuredReason(data && !data.measured ? data.reason_i18n_key : undefined, locale)}
              </p>
            </div>
          )}

          {measured && (
            <div className="text-[11.5px] text-ink-muted mt-3.5">
              {t('dashboard.score.weights', { count: measuredCount })}
            </div>
          )}
        </>
      )}
    </Panel>
  );
}

/** "↘ −4 pts en 30 j": a falling exposure is the good direction. */
function ScoreDelta({ delta }: { delta: number | null }) {
  const { t, locale } = useI18n();
  if (delta === null) {
    return (
      <span className="text-[12px] text-ink-muted ml-auto whitespace-nowrap">
        {t('dashboard.score.noDelta')}
      </span>
    );
  }
  const better = delta <= 0;
  const Icon = better ? ArrowDownRight : ArrowUpRight;
  const signed = `${delta > 0 ? '+' : delta < 0 ? '−' : ''}${formatNumber(locale, Math.abs(delta), {
    maximumFractionDigits: 1,
  })}`;
  return (
    <span
      className="text-[12.5px] font-semibold ml-auto flex items-center gap-1 whitespace-nowrap"
      style={{ color: better ? 'var(--success-text)' : 'var(--danger-text)' }}
      data-testid="dash-score-delta"
    >
      <Icon size={14} strokeWidth={2} aria-hidden="true" />
      {t('dashboard.score.delta', { delta: signed })}
    </span>
  );
}

function FactorRow({ factor, model }: { factor: ScoreFactor; model?: ScoreModel }) {
  const { t, locale } = useI18n();
  const band = factor.available ? bandOfReading(model, factor.raw) : undefined;
  const weight = formatNumber(locale, factor.weight, {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  });
  const label = t(`dashboard.score.factor.${factor.factor}`);
  return (
    <div
      className="grid grid-cols-[110px_1fr_34px_40px] items-center gap-2.5 text-[12.5px]"
      title={t('dashboard.score.factorHint', { weight })}
    >
      <span className="text-ink-soft truncate">{label}</span>
      <span
        className="h-1.5 rounded-[6px] overflow-hidden"
        style={{ background: 'var(--chart-track)' }}
        role="img"
        aria-label={
          factor.available ? `${label} ${Math.round(factor.raw)}/100` : `${label} — ${t('dashboard.score.notMeasured')}`
        }
      >
        <span
          className="block h-full rounded-[6px]"
          style={{
            width: factor.available ? `${Math.max(0, Math.min(100, factor.raw))}%` : 0,
            background: bandColor(band),
          }}
        />
      </span>
      <span className="mono font-semibold text-right text-ink">
        {factor.available ? Math.round(factor.raw) : '—'}
      </span>
      <span className="mono text-ink-muted text-[11.5px] text-right">
        {factor.available ? `×${weight}` : ''}
      </span>
    </div>
  );
}
