// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Risk Champions Leaderboard (OpenRisk.dc.html §6.8): animated top-3 podium
// (crowned #1 centre-raised), a ranks 4→N table and a "your position" card.

import { useState } from 'react';
import { ArrowUp, ArrowDown } from 'lucide-react';
import { PageFrame, PageHeader, Chip, Card, Avatar, PreviewBadge } from '../../shared/ui';
import { useUIStrings } from '../../shared/uiStrings';
import { useUIStore } from '../../store/uiStore';

interface Person { name: string; init: string; dept: string; pts: number; badges: string[]; trend: number; streak: number }

export function LeaderboardPage() {
  const L = useUIStrings();
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const [period, setPeriod] = useState<'week' | 'month' | 'all'>('week');
  const fmt = (n: number) => n.toLocaleString(lang === 'fr' ? 'fr-FR' : 'en-US');

  const people: Person[] = [
    { name: 'Fatou Sy', init: 'FS', dept: tr('Sécurité Cloud', 'Cloud Security'), pts: 2840, badges: ['🛡️', '🔥', '⚡'], trend: 5, streak: 12 },
    { name: 'Amir Diallo', init: 'AD', dept: tr('Direction SSI', 'CISO Office'), pts: 2610, badges: ['👑', '🎯'], trend: 3, streak: 8 },
    { name: 'Kofi Mensah', init: 'KM', dept: tr('Infrastructure', 'Infrastructure'), pts: 2350, badges: ['⚙️', '🔥'], trend: 2, streak: 15 },
    { name: 'Léa Traoré', init: 'LT', dept: tr('Conformité', 'Compliance'), pts: 1980, badges: ['📋', '✅'], trend: -1, streak: 6 },
    { name: 'Yasmine Ba', init: 'YB', dept: tr('SOC', 'SOC'), pts: 1720, badges: ['🔍'], trend: 4, streak: 3 },
    { name: 'Omar Sylla', init: 'OS', dept: tr('DevSecOps', 'DevSecOps'), pts: 1540, badges: ['🔧'], trend: 1, streak: 9 },
    { name: 'Nadia Kone', init: 'NK', dept: tr('Audit', 'Audit'), pts: 1360, badges: ['📊'], trend: -2, streak: 4 },
  ];

  const podiumOrder = [people[1], people[0], people[2]]; // 2nd, 1st, 3rd
  const heights = [128, 164, 104];
  const medals = ['#c0c0cc', '#ffd426', '#cd9b6a'];
  const ranks = [2, 1, 3];

  return (
    <PageFrame wide>
      <PageHeader
        title={L.lbTitle}
        badge={<PreviewBadge label={tr('Aperçu', 'Preview')} />}
        actions={
          <>
            <Chip label={L.lbWeek} active={period === 'week'} onClick={() => setPeriod('week')} />
            <Chip label={L.lbMonth} active={period === 'month'} onClick={() => setPeriod('month')} />
            <Chip label={L.lbAll} active={period === 'all'} onClick={() => setPeriod('all')} />
          </>
        }
      />
      <div className="text-[13.5px] text-ink-soft -mt-2 mb-1.5">{L.lbSub}</div>

      {/* podium */}
      <div className="flex items-end justify-center gap-[18px] pt-5 pb-[30px]">
        {podiumOrder.map((p, i) => (
          <div key={p.init} className="flex flex-col items-center w-[150px]" style={{ animation: `or-fadeup .5s ${i * 0.12}s both` }}>
            {ranks[i] === 1 && <div className="text-[26px] mb-0.5" style={{ animation: 'or-float 3s ease-in-out infinite' }}>👑</div>}
            <div className="relative mb-3">
              <div
                className="rounded-full flex items-center justify-center font-bold text-white"
                style={{
                  width: ranks[i] === 1 ? 72 : 60, height: ranks[i] === 1 ? 72 : 60,
                  fontSize: ranks[i] === 1 ? 24 : 20,
                  background: 'linear-gradient(135deg,var(--accent),var(--accent-2))',
                  border: `3px solid ${medals[i]}`, boxShadow: `0 4px 18px ${medals[i]}66`,
                }}
              >
                {p.init}
              </div>
              <div className="absolute left-1/2 -translate-x-1/2 rounded-full flex items-center justify-center font-extrabold" style={{ bottom: -6, width: 22, height: 22, background: medals[i], color: '#1a1a1a', fontSize: 11 }}>{ranks[i]}</div>
            </div>
            <div className="text-[13.5px] font-semibold text-ink text-center">{p.name}</div>
            <div className="disp mono text-[17px] font-bold text-ink" style={{ margin: '3px 0 12px' }}>{fmt(p.pts)}</div>
            <div className="w-full flex items-start justify-center pt-3.5" style={{ height: heights[i], borderRadius: '14px 14px 0 0', background: `linear-gradient(180deg,color-mix(in srgb,${medals[i]} 22%,transparent),transparent)`, border: '1px solid var(--border)', borderBottom: 'none' }}>
              <span className="text-[22px]">{p.badges.slice(0, 2).join(' ')}</span>
            </div>
          </div>
        ))}
      </div>

      {/* ranks 4..N */}
      <Card style={{ padding: '10px 14px 4px', overflow: 'hidden', marginBottom: 16 }}>
        <div className="overflow-x-auto">
          <table className="w-full border-collapse" style={{ minWidth: 620 }}>
            <thead>
              <tr>
                {['#', tr('Champion', 'Champion'), 'Badges'].map((t) => <th key={t} className="text-left text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted px-3 pb-[11px]">{t}</th>)}
                <th className="text-center text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted px-3 pb-[11px]">{tr('Tendance', 'Trend')}</th>
                <th className="text-center text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted px-3 pb-[11px]">Streak</th>
                <th className="text-right text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted px-3 pb-[11px]">{L.lbPoints}</th>
              </tr>
            </thead>
            <tbody>
              {people.slice(3).map((p, i) => (
                <tr key={p.init} style={{ borderTop: '1px solid var(--border)' }}>
                  <td className="px-3 py-3 w-11"><span className="mono text-[14px] font-bold text-ink-muted">#{i + 4}</span></td>
                  <td className="px-3 py-3">
                    <div className="flex items-center gap-2.5">
                      <Avatar initials={p.init} size={32} />
                      <div>
                        <div className="text-[13.5px] font-medium text-ink">{p.name}</div>
                        <div className="text-[11.5px] text-ink-muted">{p.dept}</div>
                      </div>
                    </div>
                  </td>
                  <td className="px-3 py-3"><span className="text-[15px]">{p.badges.join(' ')}</span></td>
                  <td className="px-3 py-3 text-center">
                    <span className="inline-flex items-center gap-0.5 text-[12px] font-semibold" style={{ color: p.trend >= 0 ? 'var(--low)' : 'var(--critical)' }}>
                      {p.trend >= 0 ? <ArrowUp size={12} /> : <ArrowDown size={12} />}{Math.abs(p.trend)}
                    </span>
                  </td>
                  <td className="px-3 py-3 text-center text-[12.5px] text-ink-soft">🔥 {p.streak}</td>
                  <td className="px-3 py-3 text-right"><span className="mono text-[14px] font-bold text-ink">{fmt(p.pts)}</span></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Card>

      {/* your position */}
      <div className="flex items-center gap-[18px] rounded-[16px] px-[22px] py-[18px]" style={{ background: 'linear-gradient(135deg,var(--accent-soft),transparent)', border: '1px solid var(--accent-line)' }}>
        <div className="text-center">
          <div className="disp mono text-[24px] font-bold" style={{ color: 'var(--accent)' }}>#2</div>
          <div className="text-[10.5px] text-ink-muted">{L.lbYou}</div>
        </div>
        <Avatar initials="AD" size={40} />
        <div className="flex-1 min-w-0">
          <div className="text-[14px] font-semibold text-ink">Amir Diallo</div>
          <div className="text-[12.5px] text-ink-soft mb-2">2 610 {L.lbPoints} · 230 {L.lbNext} Fatou Sy</div>
          <div className="h-1.5 rounded-md overflow-hidden" style={{ background: 'var(--bg-hover)' }}>
            <div className="h-full rounded-md" style={{ width: '82%', background: 'linear-gradient(90deg,var(--accent),var(--accent-2))' }} />
          </div>
        </div>
        <div className="text-[26px]">👑</div>
      </div>
    </PageFrame>
  );
}
