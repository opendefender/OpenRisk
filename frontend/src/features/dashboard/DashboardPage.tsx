// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
//
// Signature Dashboard (OpenRisk.dc.html §6.2). Score hero gauge + count-up KPIs,
// 5×5 probability×impact heatmap, risk-trend sparklines, recent activity and the
// War Room widget. Uses the real risk store where data exists and falls back to
// the design's representative fixtures otherwise.

import { useEffect, useMemo, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  ShieldAlert, AlertTriangle, ShieldCheck, CheckCircle2, ArrowUp, ArrowDown, FileText, Zap,
  type LucideIcon,
} from 'lucide-react';
import { useRiskStore } from '../../hooks/useRiskStore';
import { useUIStore } from '../../store/uiStore';
import { useUIStrings } from '../../shared/uiStrings';
import { critColor, frameworkColor, scoreColor, scoreToCriticality, softFill, type Criticality } from '../../shared/riskColors';

/* ---------------- helpers ---------------- */

/** Numeric count-up over ~1.1s, ease-out cubic. Re-runs when target changes. */
function useCountUp(target: number, duration = 1100): number {
  const [value, setValue] = useState(0);
  const raf = useRef<number>(0);
  useEffect(() => {
    const t0 = performance.now();
    const tick = (now: number) => {
      let p = Math.min(1, (now - t0) / duration);
      p = 1 - Math.pow(1 - p, 3);
      setValue(target * p);
      if (p < 1) raf.current = requestAnimationFrame(tick);
    };
    raf.current = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(raf.current);
  }, [target, duration]);
  return value;
}

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

const Card = ({ children, className = '', style }: { children: React.ReactNode; className?: string; style?: React.CSSProperties }) => (
  <div className={`or-card ${className}`} style={style}>
    {children}
  </div>
);

/* ---------------- fixtures (fallback / design parity) ---------------- */

interface RecentRisk {
  id: string;
  name: string;
  crit: Criticality;
  score: number;
  meta: string;
  fw: string;
}

const FIXTURE_RECENT: RecentRisk[] = [
  { id: 'RSK-1042', name: 'Exposition RDP non filtrée sur serveur de paie', crit: 'critical', score: 9.2, meta: 'srv-paie-01', fw: 'ISO27001' },
  { id: 'RSK-1039', name: 'Absence de MFA sur comptes administrateurs cloud', crit: 'critical', score: 8.6, meta: 'aws-prod', fw: 'SOC2' },
  { id: 'RSK-1031', name: 'Chiffrement TLS obsolète sur passerelle bancaire', crit: 'high', score: 6.8, meta: 'gw-bank-02', fw: 'BCEAO' },
  { id: 'RSK-1024', name: 'Sauvegardes non testées depuis 90 jours', crit: 'high', score: 6.2, meta: 'backup-nas', fw: 'ISO27001' },
  { id: 'RSK-1018', name: 'Dépendances npm vulnérables (CVE-2024-4032)', crit: 'medium', score: 4.4, meta: 'portail-client', fw: 'NIST' },
];

/* ---------------- page ---------------- */

export const DashboardPage = () => {
  const navigate = useNavigate();
  const L = useUIStrings();
  const lang = useUIStore((s) => s.lang);
  const risks = useRiskStore((s) => s.risks);
  const total = useRiskStore((s) => s.total);
  const fetchRisks = useRiskStore((s) => s.fetchRisks);

  useEffect(() => {
    // Best-effort refresh; the widgets degrade to fixtures if this fails/empties.
    fetchRisks?.().catch(() => {});
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const hasReal = risks.length > 0;

  const critOf = (r: (typeof risks)[number]): Criticality =>
    (r.level?.toLowerCase() as Criticality) || scoreToCriticality(r.score);

  const kpis = useMemo(() => {
    if (!hasReal) return { total: 247, critical: 12, mitig: 34, resolved: 18 };
    return {
      total: total || risks.length,
      critical: risks.filter((r) => critOf(r) === 'critical').length,
      mitig: risks.filter((r) => (r.mitigations?.length ?? 0) > 0 || /progress/i.test(r.status)).length,
      resolved: risks.filter((r) => /mitigat|resolv|closed|done|accept/i.test(r.status)).length,
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [hasReal, risks, total]);

  const recent: RecentRisk[] = useMemo(() => {
    if (!hasReal) return FIXTURE_RECENT;
    return risks.slice(0, 5).map((r) => ({
      id: r.id.length > 10 ? `#${r.id.slice(0, 8)}` : r.id,
      name: r.title,
      crit: critOf(r),
      score: r.score,
      meta: r.assets?.[0]?.name ?? '—',
      fw: r.frameworks?.[0] ?? r.tags?.[0] ?? '—',
    }));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [hasReal, risks]);

  const fmt = (n: number) => Math.round(n).toLocaleString(lang === 'fr' ? 'fr-FR' : 'en-US');

  return (
    <div className="flex-1 overflow-y-auto">
      <div className="mx-auto px-5 sm:px-7 pt-6 pb-10 max-w-[1320px]" style={{ animation: 'or-fadeup .4s ease' }}>
        {/* header */}
        <div className="flex items-start justify-between flex-wrap gap-3.5 mb-[22px]">
          <div>
            <h1 className="disp text-[27px] font-bold tracking-tight text-ink">{L.greeting}</h1>
            <div className="text-[14px] text-ink-soft mt-1">{L.dashSub}</div>
          </div>
          <button
            onClick={() => navigate('/reports')}
            className="h-[38px] px-4 rounded-[10px] flex items-center gap-2 text-[13px] font-semibold text-ink hover:bg-hover transition-colors"
            style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border-strong)' }}
          >
            <FileText size={16} strokeWidth={1.75} />
            {L.genReport}
          </button>
        </div>

        {/* row 1 — score hero + kpis */}
        <div className="grid grid-cols-1 lg:grid-cols-[340px_1fr] gap-4 mb-4">
          <ScoreHero score={72} onDetails={() => navigate('/risks')} />
          <KpiGrid values={kpis} fmt={fmt} onOpen={() => navigate('/risks')} />
        </div>

        {/* row 2 — heatmap + trend */}
        <div className="grid grid-cols-1 lg:grid-cols-[1.5fr_1fr] gap-4 mb-4">
          <HeatmapCard />
          <TrendCard />
        </div>

        {/* row 3 — recent + war room */}
        <div className="grid grid-cols-1 lg:grid-cols-[1.5fr_1fr] gap-4">
          <RecentActivityCard risks={recent} onOpen={() => navigate('/risks')} />
          <WarRoomCard onJoin={() => navigate('/incidents')} />
        </div>
      </div>
    </div>
  );
};

/* ---------------- Score hero ---------------- */
function ScoreHero({ score, onDetails }: { score: number; onDetails: () => void }) {
  const L = useUIStrings();
  const val = Math.round(useCountUp(score));
  const cx = 110, cy = 112, r = 76;
  const track = arcPath(cx, cy, r, -115, 115);
  const prog = arcPath(cx, cy, r, -115, -115 + 230 * (val / 100));
  const col = val >= 70 ? 'var(--low)' : val >= 45 ? 'var(--high)' : 'var(--critical)';
  return (
    <Card>
      <div className="px-[22px] pt-5 pb-2 text-[13px] font-semibold text-ink-soft">{L.globalScore}</div>
      <div className="relative flex justify-center">
        <svg viewBox="0 0 220 150" width="220" height="150">
          <path d={track} fill="none" stroke="var(--bg-hover)" strokeWidth={14} strokeLinecap="round" />
          <path d={prog} fill="none" stroke={col} strokeWidth={14} strokeLinecap="round" style={{ filter: `drop-shadow(0 0 6px ${col})` }} />
        </svg>
        <div className="absolute left-0 right-0 text-center" style={{ top: '52px' }}>
          <div className="disp mono text-[44px] font-bold text-ink leading-none">{val}</div>
          <div className="text-[12px] text-ink-muted mt-0.5">/ 100</div>
        </div>
      </div>
      <div className="flex items-center justify-center gap-2 pt-1 pb-1.5">
        <span className="inline-flex items-center gap-1 text-[12.5px] font-semibold px-2 py-[3px] rounded-[7px]" style={{ color: 'var(--low)', background: softFill('var(--low)', 12) }}>
          <ArrowUp size={13} /> +3 pts
        </span>
        <span className="text-[12px] text-ink-muted">{L.since7}</span>
      </div>
      <button
        onClick={onDetails}
        className="mx-[22px] mb-5 mt-2 h-[34px] rounded-[9px] text-[12.5px] font-semibold text-ink hover:bg-hover transition-colors"
        style={{ width: 'calc(100% - 44px)', border: '1px solid var(--border-strong)', background: 'transparent' }}
      >
        {L.viewDetails}
      </button>
    </Card>
  );
}

/* ---------------- KPI grid ---------------- */
function KpiGrid({
  values, fmt, onOpen,
}: {
  values: { total: number; critical: number; mitig: number; resolved: number };
  fmt: (n: number) => string;
  onOpen: () => void;
}) {
  const L = useUIStrings();
  const data: { label: string; val: number; delta: string; up: boolean; danger?: boolean; icon: LucideIcon; col: string }[] = [
    { label: L.kpiTotal, val: values.total, delta: '-4%', up: false, icon: ShieldAlert, col: 'var(--accent)' },
    { label: L.kpiCrit, val: values.critical, delta: '+2', up: true, danger: true, icon: AlertTriangle, col: 'var(--critical)' },
    { label: L.kpiMiti, val: values.mitig, delta: '+6', up: true, icon: ShieldCheck, col: 'var(--high)' },
    { label: L.kpiResolved, val: values.resolved, delta: '+18', up: true, icon: CheckCircle2, col: 'var(--low)' },
  ];
  return (
    <div className="grid grid-cols-2 grid-rows-2 gap-4">
      {data.map((d) => (
        <KpiCard key={d.label} {...d} fmt={fmt} onClick={onOpen} />
      ))}
    </div>
  );
}

function KpiCard({
  label, val, delta, up, danger, icon: Icon, col, fmt, onClick,
}: {
  label: string; val: number; delta: string; up: boolean; danger?: boolean; icon: LucideIcon; col: string;
  fmt: (n: number) => string; onClick: () => void;
}) {
  const shown = Math.round(useCountUp(val));
  const deltaColor = danger ? 'var(--critical)' : 'var(--low)';
  return (
    <button onClick={onClick} className="or-card text-left p-[18px] hover:bg-hover transition-colors">
      <div className="flex items-center justify-between mb-3.5">
        <div className="w-[34px] h-[34px] rounded-[10px] flex items-center justify-center" style={{ color: col, background: softFill(col, 14) }}>
          <Icon size={18} strokeWidth={1.75} />
        </div>
        <span className="inline-flex items-center gap-0.5 text-[11.5px] font-semibold" style={{ color: deltaColor }}>
          {up ? <ArrowUp size={12} /> : <ArrowDown size={12} />}
          {delta}
        </span>
      </div>
      <div className="disp mono text-[32px] font-bold text-ink leading-none">{fmt(shown)}</div>
      <div className="text-[12.5px] text-ink-soft mt-[5px]">{label}</div>
    </button>
  );
}

/* ---------------- Heatmap ---------------- */
function HeatmapCard() {
  const L = useUIStrings();
  const counts: Record<string, number> = {
    '5-5': 4, '5-4': 2, '4-5': 3, '4-4': 1, '5-3': 1, '3-5': 2, '4-3': 2, '3-4': 3, '5-2': 0, '2-5': 1,
    '3-3': 4, '4-2': 1, '2-4': 2, '5-1': 0, '1-5': 0, '2-3': 3, '3-2': 2, '2-2': 5, '1-4': 1, '4-1': 0,
    '1-3': 2, '3-1': 1, '2-1': 2, '1-2': 3, '1-1': 6,
  };
  const cellCol = (p: number, i: number) => {
    const v = p * i;
    return v >= 15 ? 'var(--critical)' : v >= 8 ? 'var(--high)' : v >= 4 ? 'var(--medium)' : 'var(--low)';
  };
  const rows = [];
  for (let i = 5; i >= 1; i--) {
    const cells = [];
    for (let p = 1; p <= 5; p++) {
      const c = counts[`${i}-${p}`] ?? 0;
      const col = cellCol(p, i);
      cells.push(
        <div
          key={p}
          title={`P${p}×I${i} · ${c}`}
          className="aspect-square rounded-lg flex items-center justify-center"
          style={{
            background: c ? softFill(col, 18 + c * 9) : 'var(--bg-hover)',
            border: `1px solid ${c ? softFill(col, 40) : 'var(--border)'}`,
          }}
        >
          {c ? <span className="mono text-[13px] font-bold" style={{ color: col }}>{c}</span> : null}
        </div>
      );
    }
    rows.push(
      <div key={i} className="grid grid-cols-5 gap-1.5">
        {cells}
      </div>
    );
  }
  return (
    <Card style={{ padding: '18px 20px' }}>
      <div className="text-[14px] font-semibold text-ink mb-4">{L.heatTitle}</div>
      <div className="flex gap-2.5">
        <div className="flex items-center">
          <span className="text-[11px] font-semibold text-ink-muted tracking-wide" style={{ writingMode: 'vertical-rl', transform: 'rotate(180deg)' }}>
            {L.impact}
          </span>
        </div>
        <div className="flex-1 flex flex-col gap-1.5">
          <div className="flex flex-col gap-1.5">{rows}</div>
          <div className="text-center text-[11px] font-semibold text-ink-muted mt-2 tracking-wide">{L.proba}</div>
        </div>
      </div>
    </Card>
  );
}

/* ---------------- Trend ---------------- */
const TREND_SERIES: Record<string, { crit: number[]; high: number[]; med: number[] }> = {
  '7': { crit: [3, 4, 3, 5, 4, 6, 5], high: [8, 7, 9, 8, 10, 9, 11], med: [14, 15, 13, 16, 15, 17, 16] },
  '30': { crit: [2, 3, 5, 4, 6, 5, 7, 6, 8, 5], high: [6, 8, 7, 9, 8, 10, 9, 11, 10, 12], med: [12, 14, 13, 15, 16, 15, 17, 18, 17, 19] },
  '90': { crit: [1, 2, 4, 3, 5, 7, 6, 8, 7, 9, 8, 10], high: [5, 7, 6, 9, 8, 11, 10, 12, 11, 13, 12, 14], med: [10, 13, 12, 15, 14, 17, 16, 18, 17, 20, 19, 22] },
};

function TrendCard() {
  const L = useUIStrings();
  const [range, setRange] = useState<'7' | '30' | '90'>('30');
  const series = TREND_SERIES[range];
  const W = 300, H = 120, pad = 8;
  const allMax = Math.max(...series.crit, ...series.high, ...series.med);
  const line = (arr: number[]) => {
    const step = (W - pad * 2) / (arr.length - 1);
    return arr.map((v, i) => `${pad + i * step},${H - pad - (v / allMax) * (H - pad * 2)}`).join(' ');
  };
  const tab = (v: '7' | '30' | '90', lbl: string) => (
    <button
      key={v}
      onClick={() => setRange(v)}
      className="h-[26px] px-2.5 rounded-[7px] text-[11.5px] font-semibold transition-colors"
      style={{
        background: range === v ? 'var(--accent-soft)' : 'transparent',
        color: range === v ? 'var(--accent)' : 'var(--text-secondary)',
      }}
    >
      {lbl}
    </button>
  );
  const leg = (col: string, lbl: string) => (
    <span key={lbl} className="inline-flex items-center gap-1.5 text-[11px] text-ink-soft">
      <span className="w-[9px] h-[3px] rounded-sm" style={{ background: col }} />
      {lbl}
    </span>
  );
  return (
    <Card style={{ padding: '18px 20px' }}>
      <div className="flex items-center justify-between mb-2">
        <div className="text-[14px] font-semibold text-ink">{L.trendTitle}</div>
        <div className="flex gap-0.5 p-0.5 rounded-[9px]" style={{ background: 'var(--bg-hover)' }}>
          {tab('7', '7j')}
          {tab('30', '30j')}
          {tab('90', '90j')}
        </div>
      </div>
      <div className="flex gap-3.5 mb-2.5">
        {leg('var(--critical)', L.critical)}
        {leg('var(--high)', L.high)}
        {leg('var(--medium)', L.medium)}
      </div>
      <svg viewBox={`0 0 ${W} ${H}`} width="100%" height="150" preserveAspectRatio="none">
        {[1, 2, 3].map((i) => (
          <line key={i} x1={pad} x2={W - pad} y1={pad + (i * (H - pad * 2)) / 3} y2={pad + (i * (H - pad * 2)) / 3} stroke="var(--border)" strokeWidth={1} />
        ))}
        <polyline points={line(series.med)} fill="none" stroke="var(--medium)" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" />
        <polyline points={line(series.high)} fill="none" stroke="var(--high)" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" />
        <polyline points={line(series.crit)} fill="none" stroke="var(--critical)" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" />
      </svg>
    </Card>
  );
}

/* ---------------- Recent activity ---------------- */
function RecentActivityCard({ risks, onOpen }: { risks: RecentRisk[]; onOpen: () => void }) {
  const L = useUIStrings();
  return (
    <Card style={{ padding: '18px 14px' }}>
      <div className="text-[14px] font-semibold text-ink mb-2 px-2">{L.recentTitle}</div>
      <div>
        {risks.map((r) => (
          <button
            key={r.id}
            onClick={onOpen}
            className="w-full flex items-center gap-3 px-2 py-[11px] rounded-[10px] hover:bg-hover transition-colors text-left"
          >
            <span
              className="w-[9px] h-[9px] rounded-full shrink-0"
              style={{ background: critColor[r.crit], boxShadow: r.crit === 'critical' ? '0 0 7px var(--critical)' : 'none' }}
            />
            <div className="flex-1 min-w-0">
              <div className="text-[13px] font-medium text-ink truncate">{r.name}</div>
              <div className="text-[11.5px] text-ink-muted mt-0.5">{r.id} · {r.meta}</div>
            </div>
            {r.fw !== '—' && (
              <span
                className="text-[11px] font-semibold px-2 py-[3px] rounded-md shrink-0"
                style={{ color: frameworkColor[r.fw] ?? 'var(--text-secondary)', background: softFill(frameworkColor[r.fw] ?? 'var(--text-secondary)', 14) }}
              >
                {r.fw}
              </span>
            )}
            <span className="mono text-[13px] font-bold w-[34px] text-right" style={{ color: scoreColor(r.score) }}>
              {r.score.toFixed(1)}
            </span>
          </button>
        ))}
      </div>
    </Card>
  );
}

/* ---------------- War Room widget ---------------- */
function WarRoomCard({ onJoin }: { onJoin: () => void }) {
  const L = useUIStrings();
  return (
    <div
      className="rounded-[16px] p-5 flex flex-col"
      style={{
        background: 'linear-gradient(135deg,rgba(255,69,58,.1),rgba(255,69,58,.03))',
        border: '1px solid rgba(255,69,58,.28)',
      }}
    >
      <div className="flex items-center gap-2 mb-3.5">
        <span className="w-2.5 h-2.5 rounded-full" style={{ background: 'var(--critical)', animation: 'or-pulsedot 1.4s infinite' }} />
        <span className="text-[11px] font-bold tracking-[0.06em] uppercase" style={{ color: 'var(--critical)' }}>{L.warTitle}</span>
      </div>
      <div className="text-[15px] font-semibold text-ink mb-1">INC-2026-014 · Exfiltration suspectée</div>
      <div className="text-[12.5px] text-ink-soft mb-4">srv-paie-01 · Sévérité critique</div>
      <div className="flex items-center gap-4 mb-4.5">
        <div>
          <div className="disp mono text-[22px] font-bold text-ink">01:47:12</div>
          <div className="text-[11px] text-ink-muted">Durée</div>
        </div>
        <div className="flex ml-auto">
          {['AD', 'FS', 'KM'].map((x, i) => (
            <div
              key={x}
              className="w-7 h-7 rounded-full flex items-center justify-center text-[10px] font-bold text-white"
              style={{
                background: 'linear-gradient(135deg,var(--accent),var(--accent-2))',
                border: '2px solid var(--bg-elevated)',
                marginLeft: i ? '-8px' : 0,
              }}
            >
              {x}
            </div>
          ))}
        </div>
      </div>
      <button
        onClick={onJoin}
        className="mt-auto h-[38px] rounded-[10px] flex items-center justify-center gap-2 text-[13px] font-semibold text-white"
        style={{ background: 'var(--critical)' }}
      >
        <Zap size={16} /> {L.warJoin}
      </button>
    </div>
  );
}
