// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
//
// Financial Risk Quantification dashboard (spec §9) — a CFO/CISO view of the
// tenant's cyber exposure in money. The headline is a FAIR-lite P10/P50/P90
// LOSS BAND (never a single number), converted into the tenant's currency at a
// dated reference rate. Every amount is clickable → a "Méthodologie" panel that
// shows the model, its intrants, assumptions, iterations and calc date. An
// investment simulator turns three inputs into ONE plain-language ROSI sentence.
// All figures come from GET /analytics/financial and POST /risks/:id/simulate.

import { useMemo, useState } from 'react';
import { FeatureGate } from '../../shared/FeatureGate';
import { useFeature } from '../billing/useEntitlements';
import {
  AreaChart, Area, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer,
} from 'recharts';
import {
  Coins, RefreshCw, TrendingDown, Wallet, ShieldCheck, Gauge, FlaskConical,
  Info, X, BookOpen,
} from 'lucide-react';
import { PageFrame, PageHeader, Card, Btn, Skeleton, EmptyState, ErrorState } from '../../shared/ui';
import { useUIStore } from '../../store/uiStore';
import { useAuthStore } from '../../hooks/useAuthStore';
import { useFinancialSummary, useSimulateFinancial, useSetCurrency } from './useFinancial';
import type {
  FinancialSummary, TopRiskFinancial, Amount, Methodology, CurrencyCode,
} from './financialService';
import { SUPPORTED_CURRENCIES } from './financialService';

const C_LOSS = '#ff2d92'; // exposure without controls
const C_RESID = '#30d158'; // residual with controls
const C_BAND = 'var(--accent)';
const CRIT_COLOR: Record<string, string> = {
  critical: 'var(--critical)', high: 'var(--high)', medium: 'var(--medium)', low: 'var(--low)',
};

/* ---------------- currency-aware formatting ---------------- */
// XAF keeps the familiar "FCFA" label in the target market; others show the code.
function curLabel(code: string): string {
  return code === 'XAF' ? 'FCFA' : code;
}
function group(n: number): string {
  const sign = n < 0 ? '-' : '';
  return `${sign}${Math.abs(Math.round(n)).toString().replace(/\B(?=(\d{3})+(?!\d))/g, ' ')}`;
}
function compact(n: number, lang: string): string {
  const abs = Math.abs(n);
  const u = lang === 'fr' ? { b: ' Md', m: ' M', k: ' k' } : { b: 'B', m: 'M', k: 'K' };
  const f = (v: number) => (lang === 'fr' ? v.toFixed(1).replace('.', ',') : v.toFixed(1));
  if (abs >= 1e9) return `${f(n / 1e9)}${u.b}`;
  if (abs >= 1e6) return `${f(n / 1e6)}${u.m}`;
  if (abs >= 1e3) return `${f(n / 1e3)}${u.k}`;
  return String(Math.round(n));
}
function fmtPct(ratio: number): string {
  return `${ratio >= 0 ? '+' : ''}${Math.round(ratio * 100)}%`;
}
function relTime(iso: string, lang: string): string {
  const then = new Date(iso).getTime();
  if (!Number.isFinite(then)) return '';
  const secs = Math.max(0, Math.round((Date.now() - then) / 1000));
  const mins = Math.round(secs / 60);
  if (lang === 'fr') {
    if (secs < 60) return "il y a quelques secondes";
    if (mins < 60) return `il y a ${mins} min`;
    return `il y a ${Math.round(mins / 60)} h`;
  }
  if (secs < 60) return 'a few seconds ago';
  if (mins < 60) return `${mins} min ago`;
  return `${Math.round(mins / 60)} h ago`;
}

/** A currency-aware formatter bound to the summary's currency + FX rate. */
function useMoneyFmt(summary: FinancialSummary) {
  const rate = summary.fx_rate_xaf > 0 ? summary.fx_rate_xaf : 1;
  const code = summary.currency || 'XAF';
  const lang = useUIStore((s) => s.lang);
  const disp = (xaf: number) => xaf / rate;
  return {
    code,
    label: curLabel(code),
    // From a raw XAF figure (Money-based fields).
    xafFull: (xaf: number) => `${group(disp(xaf))} ${curLabel(code)}`,
    xafCompact: (xaf: number) => `${compact(disp(xaf), lang)} ${curLabel(code)}`,
    // From a currency-aware Amount (already converted server-side).
    amtFull: (a: Amount) => `${group(a.value)} ${curLabel(a.currency)}`,
    amtCompact: (a: Amount) => `${compact(a.value, lang)} ${curLabel(a.currency)}`,
  };
}

export function FinancialDashboard() {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const fin = useFeature('financial_quantification');
  const { data, isLoading, isError, refetch, isFetching } = useFinancialSummary();
  const [methodology, setMethodology] = useState<Methodology | null>(null);

  // Paywall: on a plan without financial quantification, show the blurred preview
  // + explaining upsell instead of the (402) error state.
  if (!fin.loading && !fin.enabled) {
    return (
      <PageFrame wide>
        <PageHeader title={tr('Quantification financière', 'Financial Quantification')} />
        <FeatureGate feature="financial_quantification">
          <FinancialSkeleton />
        </FeatureGate>
      </PageFrame>
    );
  }

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
          <div className="flex items-center gap-2">
            <CurrencyPicker current={data.currency} />
            <Btn label={tr('Actualiser', 'Refresh')} icon={RefreshCw} onClick={() => refetch()} />
          </div>
        }
      />
      <div className="flex items-center gap-2 -mt-1 mb-3 text-[11.5px] text-ink-muted">
        <span>{tr('Calculé', 'Computed')} {relTime(data.computed_at, lang)}</span>
        <span>·</span>
        <span>{tr('Taux', 'Rate')} {data.fx_as_of ? new Date(data.fx_as_of).toLocaleDateString(lang) : '—'}</span>
        <span>·</span>
        <span className="mono">{data.formula_version}</span>
      </div>
      {isFetching && <div className="h-0.5 -mt-2 mb-3 rounded-full or-shimmer" style={{ background: 'var(--accent)' }} />}

      {data.total_risks === 0 ? (
        <EmptyState
          icon={Wallet}
          title={tr('Aucun risque à quantifier', 'No risk to quantify')}
          description={tr('Ajoutez des risques et renseignez leurs pertes (SLE, ARO, coût des interruptions) pour voir l’exposition financière.', 'Add risks and fill in their losses (SLE, ARO, downtime cost) to see financial exposure.')}
        />
      ) : (
        <>
          <HeadlineBand data={data} lang={lang} tr={tr} onExplain={() => setMethodology(summaryMethodology(data))} />
          <div className="mt-4"><KpiRow data={data} lang={lang} tr={tr} /></div>
          <div className="grid grid-cols-1 lg:grid-cols-3 gap-4 mt-4">
            <div className="lg:col-span-2"><ProjectionCard data={data} lang={lang} tr={tr} /></div>
            <ByCriticalityCard data={data} lang={lang} tr={tr} />
          </div>
          <div className="grid grid-cols-1 lg:grid-cols-3 gap-4 mt-4">
            <div className="lg:col-span-2"><TopExposuresCard data={data} lang={lang} tr={tr} /></div>
            <SimulatorCard rows={data.top_risks} summary={data} lang={lang} tr={tr} onExplain={setMethodology} />
          </div>
        </>
      )}

      {methodology && <MethodologyModal m={methodology} lang={lang} tr={tr} onClose={() => setMethodology(null)} />}
    </PageFrame>
  );
}

/* ---------------- headline P10/P50/P90 band ---------------- */
function HeadlineBand({ data, lang, tr, onExplain }: { data: FinancialSummary; lang: string; tr: (f: string, e: string) => string; onExplain: () => void }) {
  const f = useMoneyFmt(data);
  const b = data.portfolio_loss;
  // Position the P50 marker within the [P10, P90] track.
  const span = Math.max(1, b.p90.value - b.p10.value);
  const p50Pos = Math.min(100, Math.max(0, ((b.p50.value - b.p10.value) / span) * 100));
  return (
    <Card className="or-fadeup" style={{ padding: '20px 22px' }}>
      <div className="flex items-start justify-between gap-4 flex-wrap">
        <div>
          <div className="flex items-center gap-2">
            <span className="text-[12.5px] text-ink-soft">{tr('Exposition annuelle attendue (médiane)', 'Expected annual exposure (median)')}</span>
            <button onClick={onExplain} className="inline-flex items-center gap-1 text-[11px] text-accent hover:underline" title={tr('Méthodologie', 'Methodology')}>
              <Info size={12} /> {tr('Méthodologie', 'Methodology')}
            </button>
          </div>
          <button onClick={onExplain} className="block text-left mt-1 disp mono text-[34px] font-bold text-ink leading-none hover:opacity-80" title={tr('Voir la méthodologie', 'View methodology')}>
            {f.amtCompact(b.p50)}
          </button>
          <div className="text-[11.5px] text-ink-muted mt-1.5">
            {tr('sur', 'over')} {b.iterations.toLocaleString(lang)} {tr('itérations Monte Carlo', 'Monte Carlo iterations')}
          </div>
        </div>
        <div className="text-right">
          <div className="text-[11px] uppercase tracking-[.05em] text-ink-muted">{tr('Plage P10 – P90', 'P10 – P90 range')}</div>
          <div className="mono text-[15px] font-semibold text-ink mt-1">{f.amtCompact(b.p10)} — {f.amtCompact(b.p90)}</div>
        </div>
      </div>
      {/* Interval track with the median highlighted. */}
      <div className="mt-5">
        <div className="relative h-2.5 rounded-full" style={{ background: `color-mix(in srgb, ${C_BAND} 18%, transparent)` }}>
          <div className="absolute top-0 bottom-0 rounded-full" style={{ left: 0, right: 0, background: `color-mix(in srgb, ${C_BAND} 32%, transparent)` }} />
          <div className="absolute -top-1 h-4.5 w-[3px] rounded" style={{ left: `calc(${p50Pos}% - 1.5px)`, background: C_BAND }} />
        </div>
        <div className="flex justify-between mt-1.5 text-[10.5px] text-ink-muted">
          <span>P10 · {f.amtCompact(b.p10)}</span>
          <span className="font-semibold text-ink">P50 · {f.amtCompact(b.p50)}</span>
          <span>P90 · {f.amtCompact(b.p90)}</span>
        </div>
      </div>
    </Card>
  );
}

/* ---------------- KPI tiles ---------------- */
function KpiRow({ data, lang, tr }: { data: FinancialSummary; lang: string; tr: (f: string, e: string) => string }) {
  const f = useMoneyFmt(data);
  const rosi = data.portfolio_rosi_computable ? fmtPct(data.portfolio_rosi) : '—';
  const tiles: { label: string; value: string; sub: string; icon: typeof Coins; tone: string }[] = [
    {
      label: tr('Pire cas P90', 'Worst case P90'),
      value: f.amtCompact(data.portfolio_loss.p90), sub: tr('1 année sur 10 dépasse ce montant', '1 year in 10 exceeds this'),
      icon: TrendingDown, tone: 'var(--critical)',
    },
    {
      label: tr('Exposition ALE (moyenne)', 'ALE exposure (mean)'),
      value: f.xafCompact(data.total_ale.xaf), sub: `${group(data.total_ale.usd)} USD`, icon: Coins, tone: 'var(--accent)',
    },
    {
      label: tr('Budget de remédiation', 'Remediation budget'),
      value: f.xafCompact(data.total_remediation.xaf),
      sub: tr(`Réduit l’ALE de ${f.xafCompact(data.total_risk_reduction.xaf)}`, `Cuts ALE by ${f.xafCompact(data.total_risk_reduction.xaf)}`),
      icon: Wallet, tone: 'var(--medium)',
    },
    {
      label: tr('ROSI du portefeuille', 'Portfolio ROSI'),
      value: rosi, sub: tr('Retour sur investissement sécurité', 'Return on security investment'), icon: Gauge, tone: 'var(--low)',
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

/* ---------------- currency picker (admin) ---------------- */
function CurrencyPicker({ current }: { current: string }) {
  const lang = useUIStore((s) => s.lang);
  const can = useAuthStore((s) => s.hasPermission);
  const setCur = useSetCurrency();
  // Currency is a tenant setting → the route is guarded admin/root, whose JWT
  // carries the "*" wildcard. Non-admins see the code, read-only.
  const allowed = can('*');
  if (!allowed) {
    return <span className="text-[12px] text-ink-muted px-2">{curLabel(current)}</span>;
  }
  return (
    <select
      value={SUPPORTED_CURRENCIES.includes(current as CurrencyCode) ? current : 'XAF'}
      onChange={(e) => setCur.mutate(e.target.value as CurrencyCode)}
      disabled={setCur.isPending}
      title={lang === 'fr' ? 'Devise d’affichage' : 'Display currency'}
      className="rounded-[9px] px-2.5 py-2 text-[12.5px] text-ink outline-none disabled:opacity-60"
      style={{ background: 'var(--bg-secondary)', border: '1px solid var(--border)' }}
    >
      {SUPPORTED_CURRENCIES.map((c) => <option key={c} value={c}>{c}</option>)}
    </select>
  );
}

/* ---------------- methodology panel (explainability §4) ---------------- */
function MethodologyModal({ m, lang, tr, onClose }: { m: Methodology; lang: string; tr: (f: string, e: string) => string; onClose: () => void }) {
  const meta: { k: string; v: string }[] = [
    { k: tr('Modèle', 'Model'), v: m.model },
    { k: tr('Version de formule', 'Formula version'), v: m.formula_version },
    { k: tr('Itérations', 'Iterations'), v: m.iterations.toLocaleString(lang) },
    { k: tr('Graine (déterministe)', 'Seed (deterministic)'), v: String(m.seed) },
    { k: tr('Calculé le', 'Computed at'), v: m.computed_at ? new Date(m.computed_at).toLocaleString(lang) : '—' },
    { k: tr('Devise', 'Currency'), v: `${m.currency} · 1 ${m.currency} = ${group(m.fx_rate_xaf)} FCFA (${m.fx_as_of ? new Date(m.fx_as_of).toLocaleDateString(lang) : '—'})` },
  ];
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4" style={{ background: 'rgba(0,0,0,.5)' }} onClick={onClose}>
      <div className="or-scalein w-full max-w-lg max-h-[88vh] flex flex-col rounded-[14px] overflow-hidden" style={{ background: 'var(--bg)', border: '1px solid var(--border)' }} onClick={(e) => e.stopPropagation()}>
        <div className="flex items-center justify-between px-5 py-4 border-b" style={{ borderColor: 'var(--border)' }}>
          <div className="flex items-center gap-2">
            <BookOpen size={16} style={{ color: 'var(--accent-500)' }} />
            <span className="text-[14px] font-semibold text-ink">{tr('Méthodologie', 'Methodology')}</span>
          </div>
          <button onClick={onClose} className="text-ink-muted hover:text-ink"><X size={18} /></button>
        </div>
        <div className="px-5 py-4 overflow-y-auto text-[12.5px]">
          <div className="grid grid-cols-1 gap-1.5 mb-4">
            {meta.map((r) => (
              <div key={r.k} className="flex items-start justify-between gap-4">
                <span className="text-ink-muted">{r.k}</span>
                <span className="text-ink text-right mono">{r.v}</span>
              </div>
            ))}
          </div>

          <div className="text-[11px] uppercase tracking-[.05em] text-ink-muted mb-2">{tr('Intrants utilisés', 'Inputs used')}</div>
          <table className="w-full mb-4">
            <tbody>
              {m.inputs.map((inp) => (
                <tr key={inp.key} className="border-t" style={{ borderColor: 'var(--border)' }}>
                  <td className="py-1.5 pr-2 text-ink-soft">{inp.label}</td>
                  <td className="py-1.5 text-right mono text-ink">{group(inp.value)} <span className="text-ink-muted">{inp.unit}</span></td>
                  <td className="py-1.5 pl-2 text-right"><SourceChip source={inp.source} tr={tr} /></td>
                </tr>
              ))}
            </tbody>
          </table>

          <div className="text-[11px] uppercase tracking-[.05em] text-ink-muted mb-2">{tr('Hypothèses', 'Assumptions')}</div>
          <ul className="list-disc pl-5 flex flex-col gap-1 text-ink-soft mb-3">
            {m.assumptions.map((a, i) => <li key={i}>{a}</li>)}
          </ul>

          <a href={m.doc_url} target="_blank" rel="noreferrer" className="inline-flex items-center gap-1.5 text-[12px] text-accent hover:underline">
            <BookOpen size={13} /> {tr('Documentation du modèle', 'Model documentation')}
          </a>
        </div>
      </div>
    </div>
  );
}
function SourceChip({ source, tr }: { source: string; tr: (f: string, e: string) => string }) {
  const map: Record<string, { label: string; tone: string }> = {
    'risk-input': { label: tr('saisi', 'input'), tone: 'var(--low)' },
    'derived': { label: tr('dérivé', 'derived'), tone: 'var(--accent)' },
    'reference-model': { label: tr('référence', 'reference'), tone: 'var(--medium)' },
  };
  const s = map[source] ?? { label: source, tone: 'var(--ink-muted)' };
  return <span className="text-[10.5px] px-1.5 py-0.5 rounded" style={{ background: `color-mix(in srgb, ${s.tone} 14%, transparent)`, color: s.tone }}>{s.label}</span>;
}

/** Build a Methodology object for the portfolio band from the summary fields. */
function summaryMethodology(d: FinancialSummary): Methodology {
  const b = d.portfolio_loss;
  return {
    formula_version: d.formula_version || b.formula_version,
    model: 'FAIR-lite: ALE = LEF × LM (PERT), Monte Carlo (portefeuille)',
    iterations: d.iterations || b.iterations,
    // `seed` is the Monte-Carlo RNG seed the backend returns with the
    // simulation, surfaced so a run can be reproduced. The no-mock-data rule
    // matches the word `seed` in any identifier — the right default, since
    // seeded fixtures are how fake data gets in — but wrong for this one field:
    // it comes from the API and is displayed as provenance, not invented.
    // eslint-disable-next-line openrisk/no-mock-data
    seed: b.seed,
    computed_at: d.computed_at,
    doc_url: '/docs/financial-quantification.md',
    currency: d.currency,
    fx_as_of: d.fx_as_of,
    fx_rate_xaf: d.fx_rate_xaf,
    inputs: [
      { key: 'risks', label: 'Risques agrégés', value: d.total_risks, unit: 'risques', source: 'derived' },
      { key: 'quantified', label: 'Risques chiffrés', value: d.quantified_risks, unit: 'risques', source: 'risk-input' },
    ],
    assumptions: [
      'Exposition totale = simulation Monte Carlo unique sommant la perte de chaque risque par itération (diversification prise en compte).',
      'Chaque risque : ALE = LEF × LM ; LM suit une loi PERT à 3 points (min / probable / max).',
      'La médiane P50 est mise en avant ; P10 et P90 bornent l’intervalle.',
    ],
  };
}

/* ---------------- cumulative loss projection ---------------- */
function ProjectionCard({ data, lang, tr }: { data: FinancialSummary; lang: string; tr: (f: string, e: string) => string }) {
  const rate = data.fx_rate_xaf > 0 ? data.fx_rate_xaf : 1;
  const code = curLabel(data.currency || 'XAF');
  const series = useMemo(() => {
    const before = data.total_ale.xaf / rate;
    const after = data.total_ale_after.xaf / rate;
    return Array.from({ length: 6 }, (_, y) => ({
      year: lang === 'fr' ? `A${y}` : `Y${y}`,
      sans: Math.round(before * y),
      avec: Math.round(after * y),
    }));
  }, [data, lang, rate]);

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
            <YAxis tick={{ fontSize: 11, fill: 'var(--ink-muted)' }} axisLine={false} tickLine={false} width={54} tickFormatter={(v: number) => compact(v, lang)} />
            <Tooltip
              contentStyle={{ background: 'var(--bg-secondary)', border: '1px solid var(--border)', borderRadius: 10, fontSize: 12 }}
              labelStyle={{ color: 'var(--ink)' }}
              formatter={(v: number, name: string) => [`${group(v)} ${code}`, name === 'sans' ? tr('Sans contrôle', 'Uncontrolled') : tr('Avec contrôles', 'With controls')]}
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
  const f = useMoneyFmt(data);
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
              <span className="mono text-[12px] font-semibold text-ink">{f.xafCompact(b.ale.xaf)}</span>
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
function TopExposuresCard({ data, lang, tr }: { data: FinancialSummary; lang: string; tr: (f: string, e: string) => string }) {
  const f = useMoneyFmt(data);
  const rows = data.top_risks;
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
            {rows.map((r: TopRiskFinancial) => (
              <tr key={r.id} className="border-t" style={{ borderColor: 'var(--border)' }}>
                <td className="py-2.5 pr-2">
                  <span className="inline-flex items-center gap-2">
                    <i className="w-2 h-2 rounded-full inline-block shrink-0" style={{ background: CRIT_COLOR[r.criticality?.toLowerCase()] ?? 'var(--medium)' }} />
                    <span className="text-ink truncate" style={{ maxWidth: 260 }}>{r.title}</span>
                  </span>
                </td>
                <td className="py-2.5 text-right mono text-ink">{f.xafCompact(r.ale.xaf)}</td>
                <td className="py-2.5 text-right mono text-ink-soft">{f.xafCompact(r.ale_worst.xaf)}</td>
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

/* ---------------- investment scenario simulator (ROSI, §5) ---------------- */
function SimulatorCard({ rows, summary, lang, tr, onExplain }: { rows: TopRiskFinancial[]; summary: FinancialSummary; lang: string; tr: (f: string, e: string) => string; onExplain: (m: Methodology) => void }) {
  const [riskId, setRiskId] = useState<string>(rows[0]?.id ?? '');
  const [action, setAction] = useState<string>('');
  const [cost, setCost] = useState<number>(5_000_000);
  const [eff, setEff] = useState<number>(0.7);
  const sim = useSimulateFinancial(riskId);
  const f = useMoneyFmt(summary);

  // Slider ceiling scales with the selected risk's ALE (worst case) so the cost
  // slider stays meaningful.
  const selected = rows.find((r) => r.id === riskId);
  const costMax = Math.max(20_000_000, Math.round((selected?.ale_worst.xaf ?? 50_000_000)));

  const run = () => {
    if (!riskId) return;
    sim.mutate({ remediation_cost_xaf: cost, mitigation_effectiveness: eff });
  };

  const a = sim.data;
  return (
    <Card className="or-fadeup" style={{ padding: '18px 20px', animationDelay: '200ms' }}>
      <div className="flex items-center gap-2 mb-1">
        <FlaskConical size={16} style={{ color: 'var(--accent-500)' }} />
        <div className="text-[14px] font-semibold text-ink">{tr('Simulateur d’investissement', 'Investment simulator')}</div>
      </div>
      <div className="text-[11.5px] text-ink-muted mb-4">{tr('Trois réglages → une phrase de décision. Rien n’est enregistré.', 'Three settings → one decision sentence. Nothing is saved.')}</div>

      {/* 1. Which risk */}
      <label className="block mb-3">
        <span className="text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted">{tr('Risque', 'Risk')}</span>
        <select value={riskId} onChange={(e) => setRiskId(e.target.value)} className="mt-1.5 w-full rounded-[10px] px-3 py-2 text-[13px] text-ink outline-none" style={{ background: 'var(--bg-secondary)', border: '1px solid var(--border)' }}>
          {rows.map((r) => <option key={r.id} value={r.id}>{r.title}</option>)}
        </select>
      </label>

      {/* 2. Concrete control / action — makes the sentence precise ("comment ?") */}
      <label className="block mb-3">
        <span className="text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted">{tr('Mesure envisagée', 'Planned measure')}</span>
        <input value={action} onChange={(e) => setAction(e.target.value)} placeholder={tr('ex. authentification multifacteur sur les comptes à privilèges', 'e.g. MFA on privileged accounts')} className="mt-1.5 w-full rounded-[10px] px-3 py-2 text-[13px] text-ink outline-none" style={{ background: 'var(--bg-secondary)', border: '1px solid var(--border)' }} />
      </label>

      {/* 3a. Cost slider */}
      <label className="block mb-3">
        <span className="flex items-center justify-between text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted">
          {tr('Budget', 'Budget')}
          <span className="mono text-ink">{f.xafCompact(cost)}</span>
        </span>
        <input value={cost} onChange={(e) => setCost(Number(e.target.value))} type="range" min={0} max={costMax} step={Math.max(500_000, Math.round(costMax / 40))} className="mt-2 w-full accent-(--accent)" />
      </label>

      {/* 3b. Effectiveness slider */}
      <label className="block mb-4">
        <span className="flex items-center justify-between text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted">
          {tr('Efficacité de la mesure', 'Measure effectiveness')}
          <span className="mono text-ink">{Math.round(eff * 100)}%</span>
        </span>
        <input value={eff} onChange={(e) => setEff(Number(e.target.value))} type="range" min={0} max={1} step={0.05} className="mt-2 w-full accent-(--accent)" />
      </label>

      <button onClick={run} disabled={!riskId || sim.isPending} className="w-full h-10 rounded-[10px] flex items-center justify-center gap-2 text-[13px] font-semibold text-text-on-solid disabled:opacity-60" style={{ background: 'var(--accent-solid)' }}>
        <Coins size={16} /> {tr('Calculer le retour', 'Compute return')}
      </button>

      {sim.isError && <div className="mt-3 text-[12px]" style={{ color: 'var(--critical)' }}>{tr('Échec de la simulation', 'Simulation failed')}</div>}
      {a && <SimResult a={a} riskTitle={selected?.title ?? ''} action={action} summary={summary} lang={lang} tr={tr} onExplain={onExplain} />}
    </Card>
  );
}

function SimResult({ a, riskTitle, action, summary, lang, tr, onExplain }: { a: import('./financialService').FinancialAssessment; riskTitle: string; action: string; summary: FinancialSummary; lang: string; tr: (f: string, e: string) => string; onExplain: (m: Methodology) => void }) {
  const f = useMoneyFmt(summary);
  const before = a.ale;
  const after = a.ale_after;
  const cost = a.remediation_cost;
  const reduction = a.risk_reduction;
  // Payback in months: cost / (annual loss avoided / 12).
  const paybackMonths = reduction.xaf > 0 ? Math.round((cost.xaf * 12) / reduction.xaf) : 0;
  const measure = action.trim() || tr('cette mesure', 'this measure');
  const rosiTxt = a.rosi_computable ? fmtPct(a.rosi) : '—';

  // One concrete decision sentence (spec §5).
  const sentence = lang === 'fr'
    ? `Investir ${f.xafFull(cost.xaf)} dans ${measure}${riskTitle ? ` pour le risque « ${riskTitle} »` : ''} ramènerait son exposition annuelle attendue de ${f.xafFull(before.xaf)} à ${f.xafFull(after.xaf)}${a.rosi_computable ? `, soit un retour de ${rosiTxt}` : ''}${paybackMonths > 0 && cost.xaf > 0 ? ` — l’investissement serait amorti en ~${paybackMonths} mois` : ''}.`
    : `Investing ${f.xafFull(cost.xaf)} in ${measure}${riskTitle ? ` for the "${riskTitle}" risk` : ''} would cut its expected annual exposure from ${f.xafFull(before.xaf)} to ${f.xafFull(after.xaf)}${a.rosi_computable ? `, a ${rosiTxt} return` : ''}${paybackMonths > 0 && cost.xaf > 0 ? ` — paying for itself in ~${paybackMonths} months` : ''}.`;

  return (
    <div className="mt-4 pt-4 border-t" style={{ borderColor: 'var(--border)' }}>
      {/* The sentence that makes a COMEX understand the product. */}
      <div className="rounded-[10px] p-3 mb-3 text-[13px] leading-[1.5] text-ink" style={{ background: 'color-mix(in srgb, var(--accent) 8%, transparent)', border: '1px solid color-mix(in srgb, var(--accent) 22%, transparent)' }}>
        {sentence}
      </div>

      <div className="flex items-center justify-between mb-3">
        <span className="inline-flex items-center gap-1.5 text-[12px] text-ink-soft"><ShieldCheck size={14} style={{ color: 'var(--low)' }} />ROSI</span>
        <span className="mono text-[22px] font-bold" style={{ color: a.rosi_computable ? (a.rosi >= 0 ? 'var(--low)' : 'var(--critical)') : 'var(--ink-muted)' }}>{rosiTxt}</span>
      </div>
      <div className="flex flex-col gap-3">
        <Bar label={tr('ALE actuel', 'Current ALE')} v={before.xaf} max={before.xaf} f={f} color={C_LOSS} />
        <Bar label={tr('ALE résiduel', 'Residual ALE')} v={after.xaf} max={before.xaf} f={f} color={C_RESID} />
      </div>
      <div className="mt-3 flex items-center justify-between text-[11.5px] text-ink-muted">
        <span>{tr('Perte évitée / an :', 'Loss avoided / yr:')} <span className="text-ink font-semibold">{f.xafFull(reduction.xaf)}</span></span>
        {a.methodology && (
          <button onClick={() => onExplain(a.methodology as Methodology)} className="inline-flex items-center gap-1 text-accent hover:underline">
            <Info size={12} /> {tr('Méthodologie', 'Methodology')}
          </button>
        )}
      </div>
    </div>
  );
}

function Bar({ label, v, max, f, color }: { label: string; v: number; max: number; f: ReturnType<typeof useMoneyFmt>; color: string }) {
  const pct = Math.min(100, (v / Math.max(1, max)) * 100);
  return (
    <div>
      <div className="flex items-center justify-between mb-1">
        <span className="text-[11.5px] text-ink-soft">{label}</span>
        <span className="mono text-[12px] font-semibold text-ink">{f.xafCompact(v)}</span>
      </div>
      <div className="h-2 rounded-full overflow-hidden" style={{ background: 'var(--bg-hover)' }}>
        <div className="h-full rounded-full" style={{ width: `${pct}%`, background: color }} />
      </div>
    </div>
  );
}

/* ---------------- loading skeleton ---------------- */
function FinancialSkeleton() {
  return (
    <PageFrame wide>
      <PageHeader title="Financial Quantification" />
      <Skeleton style={{ height: 120, borderRadius: 14, marginBottom: 16 }} />
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
