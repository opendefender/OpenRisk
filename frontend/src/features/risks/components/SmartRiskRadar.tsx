// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).
//
// SmartRiskRadar — the visual for the multifactor smart risk score (spec §8):
// a radar chart of the eight factor risk-contributions plus a ranked breakdown
// bar list. Pure presentation; the caller supplies the computed SmartRiskScore.

import {
  Radar, RadarChart, PolarGrid, PolarAngleAxis, PolarRadiusAxis, ResponsiveContainer,
} from 'recharts';
import { scoreColor } from '../../../shared/riskColors';
import type { FactorKey, SmartRiskScore } from '../smartScoreService';

// Bilingual short labels for the radar spokes, keyed by the stable FactorKey.
const FACTOR_LABELS: Record<FactorKey, [string, string]> = {
  business_criticality: ['Criticité métier', 'Business criticality'],
  internet_exposure: ['Exposition Internet', 'Internet exposure'],
  vulnerabilities: ['Vulnérabilités', 'Vulnerabilities'],
  control_maturity: ['Maturité contrôles', 'Control maturity'],
  incident_history: ['Historique incidents', 'Incident history'],
  exploitability: ['Exploitabilité', 'Exploitability'],
  financial_value: ['Valeur financière', 'Financial value'],
  threat_intel: ['Menaces actives', 'Active threats'],
};

function factorLabel(key: FactorKey, lang: 'fr' | 'en'): string {
  const l = FACTOR_LABELS[key];
  return l ? l[lang === 'fr' ? 0 : 1] : key;
}

// Smart-score criticality band → CSS colour var (matches the 0–100 thresholds).
function smartColor(score: number): string {
  if (score >= 75) return 'var(--critical)';
  if (score >= 50) return 'var(--high)';
  if (score >= 25) return 'var(--medium)';
  return 'var(--low)';
}

export function SmartRiskRadar({ data, lang }: { data: SmartRiskScore; lang: 'fr' | 'en' }) {
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  // Radar plots each factor's normalised risk contribution (0–100%).
  const radarData = data.factors.map((f) => ({
    factor: factorLabel(f.key, lang),
    value: Math.round(f.value * 100),
  }));
  const accent = smartColor(data.score);
  // Ranked breakdown (biggest contributor first).
  const ranked = [...data.factors].sort((a, b) => b.contribution - a.contribution);
  const maxContribution = Math.max(...data.factors.map((f) => f.contribution), 1);

  return (
    <div>
      {/* Headline score */}
      <div className="flex items-center gap-4 mb-4">
        <div
          className="w-[74px] h-[74px] rounded-full flex flex-col items-center justify-center shrink-0"
          style={{ background: 'var(--bg-hover)', border: `3px solid ${accent}` }}
        >
          <span className="mono text-[22px] font-bold leading-none" style={{ color: accent }}>
            {data.score.toFixed(0)}
          </span>
          <span className="text-[9px] text-ink-muted mt-0.5">/ 100</span>
        </div>
        <div className="min-w-0">
          <div className="text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted mb-1">
            {tr('Score de risque intelligent', 'Smart risk score')}
          </div>
          <div className="text-[13px] font-semibold capitalize" style={{ color: accent }}>
            {data.criticality}
          </div>
          <div className="text-[11.5px] text-ink-soft mt-0.5 leading-snug">{data.explanation}</div>
        </div>
      </div>

      {/* Radar */}
      <div style={{ width: '100%', height: 240 }}>
        <ResponsiveContainer>
          <RadarChart data={radarData} outerRadius="72%">
            <PolarGrid stroke="var(--border)" />
            <PolarAngleAxis
              dataKey="factor"
              tick={{ fill: 'var(--text-secondary)', fontSize: 10 }}
            />
            <PolarRadiusAxis domain={[0, 100]} tick={false} axisLine={false} />
            <Radar
              dataKey="value"
              stroke={accent}
              fill={accent}
              fillOpacity={0.28}
              isAnimationActive
            />
          </RadarChart>
        </ResponsiveContainer>
      </div>

      {/* Ranked factor breakdown */}
      <div className="mt-4 space-y-2.5">
        <div className="text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted">
          {tr('Détail par facteur', 'Per-factor breakdown')}
        </div>
        {ranked.map((f) => (
          <div key={f.key}>
            <div className="flex items-center justify-between text-[12.5px] mb-1">
              <span className="text-ink font-medium">{factorLabel(f.key, lang)}</span>
              <span className="mono text-ink-soft">
                +{f.contribution.toFixed(1)}
                <span className="text-ink-muted"> · {Math.round(f.weight * 100)}%</span>
              </span>
            </div>
            <div className="h-1.5 rounded-full overflow-hidden" style={{ background: 'var(--bg-hover)' }}>
              <div
                className="h-full rounded-full"
                style={{
                  width: `${(f.contribution / maxContribution) * 100}%`,
                  background: scoreColor(f.value * 10),
                }}
              />
            </div>
            {f.detail && <div className="text-[10.5px] text-ink-muted mt-0.5">{f.detail}</div>}
          </div>
        ))}
      </div>
    </div>
  );
}

export default SmartRiskRadar;
