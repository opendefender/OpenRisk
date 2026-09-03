// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// <ScoreGauge> — the ONE way a score is drawn.
//
// It takes a Score object, not a number: the value, the band and the explanation
// arrive together and are rendered together. There is no prop through which a
// caller could supply a number and let this component pick a label, because that
// is the shape that produced "the badge says low while the number says 63".
//
// It replaces two near-identical ScoreHero implementations (one local to
// DashboardPage, one exported from features/dashboard/shared), each with its own
// colour thresholds.

import { useCountUp } from '../features/dashboard/shared';
import { useUIStore } from '../store/uiStore';
import { bandColor, bandLabel, type Score } from '../services/scoreService';
import { ScoreExplainerButton } from './ScoreExplainer';

function polar(cx: number, cy: number, r: number, deg: number): [number, number] {
  const a = ((deg - 90) * Math.PI) / 180;
  return [cx + r * Math.cos(a), cy + r * Math.sin(a)];
}

function arcPath(cx: number, cy: number, r: number, a0: number, a1: number): string {
  const [x0, y0] = polar(cx, cy, r, a1);
  const [x1, y1] = polar(cx, cy, r, a0);
  const large = a1 - a0 <= 180 ? 0 : 1;
  return `M ${x0} ${y0} A ${r} ${r} 0 ${large} 0 ${x1} ${y1}`;
}

export function ScoreGauge({
  score,
  title,
  ctaLabel,
  onDetails,
  loading,
}: {
  /** Undefined while loading, or when the endpoint failed. */
  score: Score | undefined;
  title: string;
  ctaLabel?: string;
  onDetails?: () => void;
  loading?: boolean;
}) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);

  const value = score?.value ?? 0;
  const animated = Math.round(useCountUp(value));
  const cx = 110,
    cy = 112,
    r = 76;
  const track = arcPath(cx, cy, r, -115, 115);
  const pct = Math.max(0, Math.min(1, animated / 100));
  const prog = arcPath(cx, cy, r, -115, -115 + 230 * pct);

  // The colour follows the SERVER's band. No thresholds here.
  const color = bandColor(score?.band);

  // "Not measured" is a distinct state from "zero". A fresh tenant with no data
  // must not read as a flawless posture — that reassurance would be a lie.
  const measured = !!score;

  return (
    <div
      className="rounded-[16px]"
      style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border-strong)' }}
      data-testid="score-gauge"
    >
      <div className="px-[22px] pt-5 pb-2 text-[13px] font-semibold text-ink-soft flex items-center gap-1.5">
        {title}
        <ScoreExplainerButton score={score} />
      </div>

      <div className="relative flex justify-center">
        <svg viewBox="0 0 220 150" width="220" height="150" role="img" aria-label={title}>
          <path
            d={track}
            fill="none"
            stroke="var(--bg-hover)"
            strokeWidth={14}
            strokeLinecap="round"
          />
          {measured && (
            <path
              d={prog}
              fill="none"
              stroke={color}
              strokeWidth={14}
              strokeLinecap="round"
              style={{ filter: `drop-shadow(0 0 6px ${color})` }}
            />
          )}
        </svg>
        <div className="absolute left-0 right-0 text-center" style={{ top: '48px' }}>
          <div
            className="disp mono text-[44px] font-bold text-ink leading-none"
            data-testid="score-value"
          >
            {loading ? '…' : measured ? animated : '—'}
          </div>
          <div className="text-[12px] text-ink-muted mt-0.5">
            {measured ? '/ 100' : tr('non mesuré', 'not measured')}
          </div>
          {/* The band, printed exactly as the server sent it. */}
          {measured && (
            <div
              className="text-[12.5px] font-semibold mt-1"
              style={{ color }}
              data-testid="score-band"
            >
              {bandLabel(score.band, lang)}
            </div>
          )}
        </div>
      </div>

      {/* Inherent → residual, so treatment progress is visible rather than
          appearing as an unexplained drop in the headline number. */}
      {measured && score.inherent !== score.residual && (
        <div className="px-[22px] pt-2 text-[11.5px] text-ink-muted text-center">
          {tr('Inhérent ', 'Inherent ')}
          <span className="mono text-ink-soft">{score.inherent.toFixed(0)}</span>
          {' → '}
          {tr('résiduel ', 'residual ')}
          <span className="mono font-semibold" style={{ color }}>
            {score.residual.toFixed(0)}
          </span>
        </div>
      )}

      {ctaLabel && onDetails && (
        <button
          onClick={onDetails}
          className="mx-[22px] mb-5 mt-3 h-[34px] rounded-[9px] text-[12.5px] font-semibold text-ink hover:bg-hover transition-colors"
          style={{
            width: 'calc(100% - 44px)',
            border: '1px solid var(--border-strong)',
            background: 'transparent',
          }}
        >
          {ctaLabel}
        </button>
      )}
    </div>
  );
}
