// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Investment simulator (spec §5): pick a risk, a budget and an effectiveness,
// get one plain-language decision sentence with its ROSI and payback. Nothing
// is saved. The redesign has no slot for it; it sits below the design's blocks
// on the financial page (#904) so the capability is not lost.

import { useState } from 'react';
import { Coins, FlaskConical, Info, ShieldCheck } from 'lucide-react';

import { useI18n } from '../../hooks/useI18n';
import { Button, Select } from '../../shared/ds';
import { Panel } from '../dashboard/console/Panel';
import type { FinancialAssessment, FinancialSummary, Methodology } from './financialService';
import { currencyLabel, millions, toDisplay } from './money';
import { useSimulateFinancial } from './useFinancial';

const C_LOSS = 'var(--chart-5)';
const C_RESID = 'var(--chart-3)';

function useMoney(summary: FinancialSummary) {
  const { locale } = useI18n();
  const cur = currencyLabel(summary.currency);
  return (xaf: number) => `${millions(toDisplay(xaf, summary.fx_rate_xaf), locale)} M ${cur}`;
}

function pct(ratio: number): string {
  return `${ratio >= 0 ? '+' : '−'}${Math.abs(Math.round(ratio * 100))} %`;
}

export function SimulatorCard({
  summary,
  onExplain,
}: {
  summary: FinancialSummary;
  onExplain: (m: Methodology) => void;
}) {
  const { t } = useI18n();
  const rows = summary.top_risks;
  const [riskId, setRiskId] = useState<string>(rows[0]?.id ?? '');
  const [action, setAction] = useState('');
  const [cost, setCost] = useState(5_000_000);
  const [eff, setEff] = useState(0.7);
  const sim = useSimulateFinancial(riskId);
  const money = useMoney(summary);

  // The budget slider scales with the chosen risk's worst case.
  const selected = rows.find((r) => r.id === riskId);
  const costMax = Math.max(20_000_000, Math.round(selected?.ale_worst.xaf ?? 50_000_000));

  if (rows.length === 0) return null;

  return (
    <Panel testId="fin-simulator" className="px-5 py-[18px]">
      <div className="flex items-center gap-2 mb-1">
        <FlaskConical size={16} className="text-accent" aria-hidden="true" />
        <h2 className="m-0 text-[13.5px] font-semibold text-ink">{t('financial.sim.title')}</h2>
      </div>
      <p className="m-0 mb-4 text-[12px] text-ink-muted">{t('financial.sim.sub')}</p>

      <div className="grid gap-4 md:grid-cols-[1fr_1fr] items-start">
        <div className="grid gap-3">
          <label className="grid gap-1.5">
            <span className="text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted">
              {t('financial.sim.risk')}
            </span>
            <Select value={riskId} onChange={(e) => setRiskId(e.target.value)}>
              {rows.map((r) => (
                <option key={r.id} value={r.id}>
                  {r.title}
                </option>
              ))}
            </Select>
          </label>
          <label className="grid gap-1.5">
            <span className="text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted">
              {t('financial.sim.measure')}
            </span>
            <input
              value={action}
              onChange={(e) => setAction(e.target.value)}
              placeholder={t('financial.sim.measurePh')}
              className="h-(--control-h-md) rounded-[10px] px-3 text-[13px] text-ink bg-surface-0 border border-border-subtle"
            />
          </label>
          <label className="grid gap-1.5">
            <span className="flex items-center justify-between text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted">
              {t('financial.sim.budget')}
              <span className="mono text-ink normal-case">{money(cost)}</span>
            </span>
            <input
              type="range"
              value={cost}
              onChange={(e) => setCost(Number(e.target.value))}
              min={0}
              max={costMax}
              step={Math.max(500_000, Math.round(costMax / 40))}
              className="w-full accent-(--accent)"
            />
          </label>
          <label className="grid gap-1.5">
            <span className="flex items-center justify-between text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted">
              {t('financial.sim.effectiveness')}
              <span className="mono text-ink">{Math.round(eff * 100)} %</span>
            </span>
            <input
              type="range"
              value={eff}
              onChange={(e) => setEff(Number(e.target.value))}
              min={0}
              max={1}
              step={0.05}
              className="w-full accent-(--accent)"
            />
          </label>
          <Button
            variant="primary"
            icon={Coins}
            loading={sim.isPending}
            disabled={!riskId}
            onClick={() =>
              sim.mutate({ remediation_cost_xaf: cost, mitigation_effectiveness: eff })
            }
          >
            {t('financial.sim.run')}
          </Button>
          {sim.isError && (
            <p role="alert" className="m-0 text-[12px] text-danger-text">
              {t('financial.sim.failed')}
            </p>
          )}
        </div>
        {sim.data && (
          <SimResult
            a={sim.data}
            riskTitle={selected?.title ?? ''}
            action={action}
            summary={summary}
            onExplain={onExplain}
          />
        )}
      </div>
    </Panel>
  );
}

function SimResult({
  a,
  riskTitle,
  action,
  summary,
  onExplain,
}: {
  a: FinancialAssessment;
  riskTitle: string;
  action: string;
  summary: FinancialSummary;
  onExplain: (m: Methodology) => void;
}) {
  const { t } = useI18n();
  const money = useMoney(summary);
  const before = a.ale.xaf;
  const after = a.ale_after.xaf;
  const cost = a.remediation_cost.xaf;
  const reduction = a.risk_reduction.xaf;
  const months = reduction > 0 ? Math.round((cost * 12) / reduction) : 0;
  const rosi = a.rosi_computable ? pct(a.rosi) : '—';

  const sentence = t('financial.sim.sentence', {
    cost: money(cost),
    measure: action.trim() || t('financial.sim.thisMeasure'),
    forRisk: riskTitle ? t('financial.sim.forRisk', { title: riskTitle }) : '',
    before: money(before),
    after: money(after),
    rosi: a.rosi_computable ? t('financial.sim.rosiPart', { rosi }) : '',
    payback: months > 0 && cost > 0 ? t('financial.sim.paybackPart', { months }) : '',
  });

  return (
    <div data-testid="fin-sim-result">
      <div
        className="rounded-[10px] p-3 mb-3 text-[13px] leading-[1.5] text-ink"
        style={{
          background: 'color-mix(in srgb, var(--accent) 8%, transparent)',
          border: '1px solid color-mix(in srgb, var(--accent) 22%, transparent)',
        }}
      >
        {sentence}
      </div>
      <div className="flex items-center justify-between mb-3">
        <span className="inline-flex items-center gap-1.5 text-[12px] text-ink-soft">
          <ShieldCheck size={14} className="text-success-text" aria-hidden="true" />
          ROSI
        </span>
        <span
          className="mono text-[22px] font-semibold"
          style={{
            color: a.rosi_computable
              ? a.rosi >= 0
                ? 'var(--success-text)'
                : 'var(--danger-text)'
              : 'var(--fg-muted)',
          }}
        >
          {rosi}
        </span>
      </div>
      <div className="grid gap-3">
        <SimBar
          label={t('financial.sim.currentAle')}
          v={before}
          max={before}
          text={money(before)}
          color={C_LOSS}
        />
        <SimBar
          label={t('financial.sim.residualAle')}
          v={after}
          max={before}
          text={money(after)}
          color={C_RESID}
        />
      </div>
      <div className="mt-3 flex items-center justify-between text-[11.5px] text-ink-muted">
        <span>
          {t('financial.sim.avoided')}{' '}
          <span className="text-ink font-semibold">{money(reduction)}</span>
        </span>
        {a.methodology && (
          <button
            type="button"
            onClick={() => a.methodology && onExplain(a.methodology)}
            className="inline-flex items-center gap-1 text-accent hover:underline"
          >
            <Info size={12} aria-hidden="true" /> {t('financial.methodology.title')}
          </button>
        )}
      </div>
    </div>
  );
}

function SimBar({
  label,
  v,
  max,
  text,
  color,
}: {
  label: string;
  v: number;
  max: number;
  text: string;
  color: string;
}) {
  const w = Math.min(100, (v / Math.max(1, max)) * 100);
  return (
    <div>
      <div className="flex items-center justify-between mb-1">
        <span className="text-[11.5px] text-ink-soft">{label}</span>
        <span className="mono text-[12px] font-semibold text-ink">{text}</span>
      </div>
      <div
        className="h-2 rounded-full overflow-hidden"
        style={{ background: 'var(--chart-track)' }}
      >
        <div className="h-full rounded-full" style={{ width: `${w}%`, background: color }} />
      </div>
    </div>
  );
}
