// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
//
// Financial Risk Quantification dashboard (spec §9) — a CFO/CISO view of the
// tenant's cyber exposure in money: portfolio ALE, worst-case, remediation
// budget and ROSI, a cumulative-loss projection (with vs without controls), the
// annual exposure by criticality, an investment-scenario simulator and the top
// financial exposures. All figures come from GET /analytics/financial and the
// per-risk simulator from POST /risks/:id/simulate — no fixtures.

import { useMemo, useState } from 'react';
import {
  AreaChart, Area, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer,
} from 'recharts';
import { Coins, RefreshCw, TrendingDown, Wallet, ShieldCheck, Gauge, FlaskConical } from 'lucide-react';
import { PageFrame, PageHeader, Card, Btn, Skeleton, EmptyState, ErrorState } from '../../shared/ui';
import { useUIStore } from '../../store/uiStore';
import { useFinancialSummary, useSimulateFinancial } from './useFinancial';
import type { FinancialSummary, TopRiskFinancial, Money } from './financialService';

// Series colours match AnalyticsCiso (theme-agnostic hex so Recharts renders
// consistently in light + dark).
const C_LOSS = '#ff2d92'; // exposure without controls
const C_RESID = '#30d158'; // residual with controls
const CRIT_COLOR: Record<string, string> = {
  critical: 'var(--critical)', high: 'var(--high)', medium: 'var(--medium)', low: 'var(--low)',
};

function fmtFCFA(n: number): string {
  const sign = n < 0 ? '-' : '';
  const grouped = Math.abs(Math.trunc(n)).toString().replace(/\B(?=(\d{3})+(?!\d))/g, ' ');
  return `${sign}${grouped} FCFA`;
}
// Compact form for tiles/axes (e.g. 97,5 M FCFA).
function fmtCompact(n: number, lang: string): string {
  const abs = Math.abs(n);
  const unit = lang === 'fr' ? { b: ' Md', m: ' M', k: ' k' } : { b: 'B', m: 'M', k: 'K' };
  const fmt = (v: number) => (lang === 'fr' ? v.toFixed(1).replace('.', ',') : v.toFixed(1));
  if (abs >= 1e9) return `${fmt(n / 1e9)}${unit.b}`;
  if (abs >= 1e6) return `${fmt(n / 1e6)}${unit.m}`;
  if (abs >= 1e3) return `${fmt(n / 1e3)}${unit.k}`;
  return String(Math.round(n));
}
function fmtUSD(n: number): string {
  return `$${Math.round(n).toLocaleString('en-US')}`;
}
function fmtPct(ratio: number): string {
  return `${ratio >= 0 ? '+' : ''}${Math.round(ratio * 100)}%`;
}

export function FinancialDashboard() {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const { data, isLoading, isError, refetch, isFetching } = useFinancialSummary();

  if (isLoading) return <FinancialSkeleton />;
  if (isError || !data) {
    return (
      <PageFrame>
        <PageHeader title={tr('Quantification financière', 'Financial Quantification')} />
        <ErrorState
          title={tr('Impossible de charger la posture financière', 'Could not load financial posture')}
          onRetry={() => refetch()}
          retryLabel={tr('Réessayer', 'Retry')}
        />
      </PageFrame>
    );
  }

  return (
    <PageFrame wide>
      <PageHeader
        title={tr('Quantification financière', 'Financial Quantification')}
        count={tr(`${data.quantified_risks}/${data.total_risks} risques chiffrés`, `${data.quantified_risks}/${data.total_risks} risks quantified`)}
        actions={
          <Btn label={tr('Actualiser', 'Refresh')} icon={RefreshCw} onClick={() => refetch()} />
        }
      />
      {isFetching && <div className="h-0.5 -mt-2 mb-3 rounded-full or-shimmer" style={{ background: 'var(--accent)' }} />}

      {data.total_risks === 0 ? (
        <EmptyState
          icon={Wallet}
          title={tr('Aucun risque à quantifier', 'No risk to quantify')}
          sub={tr('Ajoutez des risques et renseignez leurs pertes (SLE, ARO, coût des interruptions) pour voir l’exposition financière.', 'Add risks and fill in their losses (SLE, ARO, downtime cost) to see financial exposure.')}
        />
      ) : (
        <>
          <KpiRow data={data} lang={lang} tr={tr} />
          <div className="grid grid-cols-1 lg:grid-cols-3 gap-4 mt-4">
            <div className="lg:col-span-2"><ProjectionCard data={data} lang={lang} tr={tr} /></div>
            <ByCriticalityCard data={data} lang={lang} tr={tr} />
          </div>
          <div className="grid grid-cols-1 lg:grid-cols-3 gap-4 mt-4">
            <div className="lg:col-span-2"><TopExposuresCard rows={data.top_risks} lang={lang} tr={tr} /></div>
            <SimulatorCard rows={data.top_risks} lang={lang} tr={tr} />
          </div>
        </>
      )}
    </PageFrame>
  );
}

/* ---------------- KPI tiles ---------------- */
function KpiRow({ data, lang, tr }: { data: FinancialSummary; lang: string; tr: (f: string, e: string) => string }) {
  const rosi = data.portfolio_rosi_computable ? fmtPct(data.portfolio_rosi) : '—';
  const tiles: { label: string; value: string; sub: string; icon: typeof Coins; tone: string }[] = [
    {
      label: tr('Exposition annuelle (ALE)', 'Annual exposure (ALE)'),
      value: `${fmtCompact(data.total_ale.xaf, lang)} FCFA`,
      sub: fmtUSD(data.total_ale.usd), icon: Coins, tone: 'var(--accent)',
    },
    {
      label: tr('Scénario du pire', 'Worst-case'),
      value: `${fmtCompact(data.total_ale_worst.xaf, lang)} FCFA`,
      sub: fmtUSD(data.total_ale_worst.usd), icon: TrendingDown, tone: 'var(--critical)',
    },
    {
      label: tr('Budget de remédiation', 'Remediation budget'),
      value: `${fmtCompact(data.total_remediation.xaf, lang)} FCFA`,
      sub: tr(`Réduit l’ALE de ${fmtCompact(data.total_risk_reduction.xaf, lang)} FCFA`, `Cuts ALE by ${fmtCompact(data.total_risk_reduction.xaf, lang)} FCFA`),
      icon: Wallet, tone: 'var(--medium)',
    },
    {
      label: tr('ROSI du portefeuille', 'Portfolio ROSI'),
      value: rosi,
      sub: tr('Retour sur investissement sécurité', 'Return on security investment'),
      icon: Gauge, tone: 'var(--low)',
    },
  ];
  return (
    <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
      {tiles.map((t, i) => (
        <Card key={t.label} className="or-fadeup" style={{ padding: '16px 18px', animationDelay: `${i * 60}ms` }}>
          <div className="flex items-center justify-between mb-3">
            <span className="text-[12.5px] text-ink-soft">{t.label}</span>
            <span className="inline-flex items-center justify-center w-7 h-7 rounded-[9px]" style={{ background: `color-mix(in srgb, ${t.tone} 14%, transparent)`, color: t.tone }}>
              <t.icon size={15} />
            </span>
          </div>
          <div className="disp mono text-[24px] font-bold text-ink leading-none">{t.value}</div>
          <div className="text-[11.5px] text-ink-muted mt-1.5">{t.sub}</div>
        </Card>
      ))}
    </div>
  );
}

/* ---------------- cumulative loss projection ---------------- */
function ProjectionCard({ data, lang, tr }: { data: FinancialSummary; lang: string; tr: (f: string, e: string) => string }) {
  // Expected cumulative loss over 5 years: ALE grows linearly with time. The gap
  // between "sans contrôle" and "avec contrôles" is the value of the security
  // program (money not lost thanks to remediation).
  const series = useMemo(() => {
    const before = data.total_ale.xaf;
    const after = data.total_ale_after.xaf;
    return Array.from({ length: 6 }, (_, y) => ({
      year: lang === 'fr' ? `A${y}` : `Y${y}`,
      sans: Math.round(before * y),
      avec: Math.round(after * y),
    }));
  }, [data, lang]);

  return (
    <Card className="or-fadeup" style={{ padding: '18px 20px', animationDelay: '80ms' }}>
      <div className="flex items-center justify-between mb-1">
        <div className="text-[14px] font-semibold text-ink">{tr('Projection des pertes cumulées (5 ans)', 'Cumulative loss projection (5 yrs)')}</div>
        <div className="flex items-center gap-3 text-[11px]">
          <span className="inline-flex items-center gap-1.5 text-ink-soft"><i className="w-2.5 h-2.5 rounded-full inline-block" style={{ background: C_LOSS }} />{tr('Sans contrôle', 'Uncontrolled')}</span>
          <span className="inline-flex items-center gap-1.5 text-ink-soft"><i className="w-2.5 h-2.5 rounded-full inline-block" style={{ background: C_RESID }} />{tr('Avec contrôles', 'With controls')}</span>
        </div>
      </div>
      <div className="text-[11.5px] text-ink-muted mb-3">{tr('L’écart entre les deux courbes est la valeur créée par le programme de sécurité.', 'The gap between the curves is the value created by the security program.')}</div>
      <div style={{ width: '100%', height: 240 }}>
        <ResponsiveContainer>
          <AreaChart data={series} margin={{ top: 6, right: 8, left: 4, bottom: 0 }}>
            <defs>
              <linearGradient id="gLoss" x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor={C_LOSS} stopOpacity={0.35} />
                <stop offset="100%" stopColor={C_LOSS} stopOpacity={0.02} />
              </linearGradient>
              <linearGradient id="gResid" x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor={C_RESID} stopOpacity={0.32} />
                <stop offset="100%" stopColor={C_RESID} stopOpacity={0.02} />
              </linearGradient>
            </defs>
            <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" vertical={false} />
            <XAxis dataKey="year" tick={{ fontSize: 11, fill: 'var(--ink-muted)' }} axisLine={{ stroke: 'var(--border)' }} tickLine={false} />
            <YAxis tick={{ fontSize: 11, fill: 'var(--ink-muted)' }} axisLine={false} tickLine={false} width={54} tickFormatter={(v: number) => fmtCompact(v, lang)} />
            <Tooltip
              contentStyle={{ background: 'var(--bg-secondary)', border: '1px solid var(--border)', borderRadius: 10, fontSize: 12 }}
              labelStyle={{ color: 'var(--ink)' }}
              formatter={(v: number, name: string) => [fmtFCFA(v), name === 'sans' ? tr('Sans contrôle', 'Uncontrolled') : tr('Avec contrôles', 'With controls')]}
            />
            <Area type="monotone" dataKey="sans" stroke={C_LOSS} strokeWidth={2} fill="url(#gLoss)" />
            <Area type="monotone" dataKey="avec" stroke={C_RESID} strokeWidth={2} fill="url(#gResid)" />
          </AreaChart>
        </ResponsiveContainer>
      </div>
    </Card>
  );
}

/* ---------------- ALE by criticality ---------------- */
function ByCriticalityCard({ data, lang, tr }: { data: FinancialSummary; lang: string; tr: (f: string, e: string) => string }) {
  const label: Record<string, string> = {
    critical: tr('Critique', 'Critical'), high: tr('Élevé', 'High'), medium: tr('Moyen', 'Medium'), low: tr('Faible', 'Low'),
  };
  const max = Math.max(1, ...data.by_criticality.map((b) => b.ale.xaf));
  return (
    <Card className="or-fadeup" style={{ padding: '18px 20px', animationDelay: '120ms' }}>
      <div className="text-[14px] font-semibold text-ink mb-4">{tr('Exposition annuelle par criticité', 'Annual exposure by criticality')}</div>
      <div className="flex flex-col gap-4">
        {data.by_criticality.map((b) => (
          <div key={b.criticality}>
            <div className="flex items-center justify-between mb-1.5">
              <span className="inline-flex items-center gap-2 text-[12.5px] text-ink-soft">
                <i className="w-2.5 h-2.5 rounded-full inline-block" style={{ background: CRIT_COLOR[b.criticality] }} />
                {label[b.criticality] ?? b.criticality}
                <span className="text-ink-muted text-[11px]">· {b.count}</span>
              </span>
              <span className="mono text-[12px] font-semibold text-ink">{fmtCompact(b.ale.xaf, lang)}</span>
            </div>
            <div className="h-2 rounded-full overflow-hidden" style={{ background: 'var(--bg-hover)' }}>
              <div className="h-full rounded-full" style={{ width: `${(b.ale.xaf / max) * 100}%`, background: CRIT_COLOR[b.criticality] }} />
            </div>
          </div>
        ))}
      </div>
    </Card>
  );
}

/* ---------------- top exposures table ---------------- */
function TopExposuresCard({ rows, lang, tr }: { rows: TopRiskFinancial[]; lang: string; tr: (f: string, e: string) => string }) {
  return (
    <Card className="or-fadeup" style={{ padding: '18px 20px', animationDelay: '160ms' }}>
      <div className="text-[14px] font-semibold text-ink mb-3">{tr('Principales expositions financières', 'Top financial exposures')}</div>
      <div className="overflow-x-auto">
        <table className="w-full text-[12.5px]" style={{ minWidth: 520 }}>
          <thead>
            <tr className="text-ink-muted text-[11px] uppercase tracking-[.04em]">
              <th className="text-left font-medium pb-2">{tr('Risque', 'Risk')}</th>
              <th className="text-right font-medium pb-2">ALE</th>
              <th className="text-right font-medium pb-2">{tr('Pire cas', 'Worst')}</th>
              <th className="text-right font-medium pb-2">ROSI</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((r) => (
              <tr key={r.id} className="border-t" style={{ borderColor: 'var(--border)' }}>
                <td className="py-2.5 pr-2">
                  <span className="inline-flex items-center gap-2">
                    <i className="w-2 h-2 rounded-full inline-block shrink-0" style={{ background: CRIT_COLOR[r.criticality?.toLowerCase()] ?? 'var(--medium)' }} />
                    <span className="text-ink truncate" style={{ maxWidth: 260 }}>{r.title}</span>
                  </span>
                </td>
                <td className="py-2.5 text-right mono text-ink">{fmtCompact(r.ale.xaf, lang)}</td>
                <td className="py-2.5 text-right mono text-ink-soft">{fmtCompact(r.ale_worst.xaf, lang)}</td>
                <td className="py-2.5 text-right">
                  {r.rosi_computable ? (
                    <span className="mono font-semibold" style={{ color: r.rosi >= 0 ? 'var(--low)' : 'var(--critical)' }}>{fmtPct(r.rosi)}</span>
                  ) : (
                    <span className="text-ink-muted">—</span>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </Card>
  );
}

/* ---------------- investment scenario simulator ---------------- */
function SimulatorCard({ rows, lang, tr }: { rows: TopRiskFinancial[]; lang: string; tr: (f: string, e: string) => string }) {
  const [riskId, setRiskId] = useState<string>(rows[0]?.id ?? '');
  const [cost, setCost] = useState<string>('5000000');
  const [eff, setEff] = useState<number>(0.7);
  const sim = useSimulateFinancial(riskId);

  const run = () => {
    if (!riskId) return;
    sim.mutate({
      remediation_cost_xaf: cost.trim() === '' ? 0 : Number(cost),
      mitigation_effectiveness: eff,
    });
  };

  const a = sim.data;
  return (
    <Card className="or-fadeup" style={{ padding: '18px 20px', animationDelay: '200ms' }}>
      <div className="flex items-center gap-2 mb-1">
        <FlaskConical size={16} style={{ color: 'var(--accent)' }} />
        <div className="text-[14px] font-semibold text-ink">{tr('Simulateur d’investissement', 'Investment simulator')}</div>
      </div>
      <div className="text-[11.5px] text-ink-muted mb-4">{tr('Testez un budget de remédiation et son efficacité — sans rien enregistrer.', 'Test a remediation budget and its effectiveness — nothing is saved.')}</div>

      <label className="block mb-3">
        <span className="text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted">{tr('Risque', 'Risk')}</span>
        <select value={riskId} onChange={(e) => setRiskId(e.target.value)} className="mt-1.5 w-full rounded-[10px] px-3 py-2 text-[13px] text-ink outline-none" style={{ background: 'var(--bg-secondary)', border: '1px solid var(--border)' }}>
          {rows.map((r) => <option key={r.id} value={r.id}>{r.title}</option>)}
        </select>
      </label>

      <label className="block mb-3">
        <span className="text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted">{tr('Coût de remédiation (FCFA)', 'Remediation cost (FCFA)')}</span>
        <input value={cost} onChange={(e) => setCost(e.target.value)} type="number" min={0} className="mt-1.5 w-full rounded-[10px] px-3 py-2 text-[13px] text-ink outline-none" style={{ background: 'var(--bg-secondary)', border: '1px solid var(--border)' }} />
      </label>

      <label className="block mb-4">
        <span className="flex items-center justify-between text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted">
          {tr('Efficacité du contrôle', 'Control effectiveness')}
          <span className="mono text-ink">{Math.round(eff * 100)}%</span>
        </span>
        <input value={eff} onChange={(e) => setEff(Number(e.target.value))} type="range" min={0} max={1} step={0.05} className="mt-2 w-full accent-[var(--accent)]" />
      </label>

      <button onClick={run} disabled={!riskId || sim.isPending} className="w-full h-10 rounded-[10px] flex items-center justify-center gap-2 text-[13px] font-semibold text-white disabled:opacity-60" style={{ background: 'linear-gradient(135deg,var(--accent),var(--accent-hover))' }}>
        <Coins size={16} /> {tr('Calculer le ROSI', 'Compute ROSI')}
      </button>

      {sim.isError && <div className="mt-3 text-[12px]" style={{ color: 'var(--critical)' }}>{tr('Échec de la simulation', 'Simulation failed')}</div>}
      {a && <SimResult a={a} lang={lang} tr={tr} />}
    </Card>
  );
}

function SimResult({ a, lang, tr }: { a: import('./financialService').FinancialAssessment; lang: string; tr: (f: string, e: string) => string }) {
  const beforeXAF = a.ale.xaf;
  const max = Math.max(1, beforeXAF);
  const bar = (label: string, v: MoneyLike, color: string) => (
    <div>
      <div className="flex items-center justify-between mb-1">
        <span className="text-[11.5px] text-ink-soft">{label}</span>
        <span className="mono text-[12px] font-semibold text-ink">{fmtCompact(v.xaf, lang)}</span>
      </div>
      <div className="h-2 rounded-full overflow-hidden" style={{ background: 'var(--bg-hover)' }}>
        <div className="h-full rounded-full" style={{ width: `${(v.xaf / max) * 100}%`, background: color }} />
      </div>
    </div>
  );
  return (
    <div className="mt-4 pt-4 border-t" style={{ borderColor: 'var(--border)' }}>
      <div className="flex items-center justify-between mb-3">
        <span className="inline-flex items-center gap-1.5 text-[12px] text-ink-soft"><ShieldCheck size={14} style={{ color: 'var(--low)' }} />ROSI</span>
        <span className="mono text-[22px] font-bold" style={{ color: a.rosi_computable ? (a.rosi >= 0 ? 'var(--low)' : 'var(--critical)') : 'var(--ink-muted)' }}>
          {a.rosi_computable ? fmtPct(a.rosi) : '—'}
        </span>
      </div>
      <div className="flex flex-col gap-3">
        {bar(tr('ALE actuel', 'Current ALE'), a.ale, C_LOSS)}
        {bar(tr('ALE résiduel', 'Residual ALE'), a.ale_after, C_RESID)}
      </div>
      <div className="mt-3 text-[11.5px] text-ink-muted">
        {tr('Perte évitée / an :', 'Loss avoided / yr:')} <span className="text-ink font-semibold">{fmtFCFA(a.risk_reduction.xaf)}</span>
        {' · '}{fmtUSD(a.risk_reduction.usd)}
      </div>
      <div className="text-[11.5px] text-ink-muted">
        {tr('Coût de remédiation :', 'Remediation cost:')} <span className="text-ink font-semibold">{fmtFCFA(a.remediation_cost.xaf)}</span>
        {beforeXAF > 0 && ` · SLE ${fmtCompact(a.sle.xaf, lang)}`}
      </div>
    </div>
  );
}
type MoneyLike = Money;

/* ---------------- loading skeleton ---------------- */
function FinancialSkeleton() {
  return (
    <PageFrame wide>
      <PageHeader title="Financial Quantification" />
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        {Array.from({ length: 4 }).map((_, i) => <Skeleton key={i} style={{ height: 96, borderRadius: 14 }} />)}
      </div>
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-4 mt-4">
        <Skeleton className="lg:col-span-2" style={{ height: 300, borderRadius: 14 }} />
        <Skeleton style={{ height: 300, borderRadius: 14 }} />
      </div>
    </PageFrame>
  );
}
