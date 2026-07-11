// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
//
// Inventory (OpenRisk.dc.html §6.12): asset table with type-icon + IP, criticality
// badge, score, risk/CVE counts, env and last scan. Type-filter chips; rows open
// the Asset Universe.

import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Atom, Plus, ChevronRight, Globe, Server, Database, Cloud, Laptop, type LucideIcon } from 'lucide-react';
import { PageFrame, PageHeader, Btn, Chip, Card, CritBadge } from '../../shared/ui';
import { critColor, scoreColor, softFill } from '../../shared/riskColors';
import { UNI_NODES, type NodeType } from '../../shared/fixtures';
import { useUIStrings } from '../../shared/uiStrings';
import { useUIStore } from '../../store/uiStore';

const TYPE_ICON: Record<NodeType, LucideIcon> = { globe: Globe, server: Server, database: Database, cloud: Cloud, laptop: Laptop };

export function InventoryPage() {
  const L = useUIStrings();
  const lang = useUIStore((s) => s.lang);
  const navigate = useNavigate();
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const [type, setType] = useState<NodeType | null>(null);

  const types = [...new Set(UNI_NODES.map((n) => n.type))];
  const rows = UNI_NODES.filter((n) => n.name !== 'Internet').filter((n) => !type || n.type === type);
  const th = (t: string, right?: boolean) => (
    <th className={`text-${right ? 'right' : 'left'} text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted px-3 pb-[11px]`}>{t}</th>
  );

  return (
    <PageFrame wide>
      <PageHeader
        title={L.n_assets}
        count={`${rows.length} ${L.uniAssets}`}
        actions={
          <>
            <Btn label={tr('Vue Univers', 'Universe view')} icon={Atom} onClick={() => navigate('/assets/universe')} />
            <Btn label={L.newRisk} icon={Plus} primary onClick={() => navigate('/risks')} />
          </>
        }
      />
      <div className="flex gap-2 mb-4 flex-wrap">
        <Chip label={tr('Tous', 'All')} active={!type} onClick={() => setType(null)} />
        {types.map((t) => <Chip key={t} label={t.charAt(0).toUpperCase() + t.slice(1)} active={type === t} onClick={() => setType(t)} />)}
      </div>
      <Card style={{ padding: '8px 8px 0', overflow: 'hidden' }}>
        <div className="overflow-x-auto">
          <table className="w-full border-collapse" style={{ minWidth: 820 }}>
            <thead style={{ borderBottom: '1px solid var(--border)' }}>
              <tr>{th(tr('Actif', 'Asset'))}{th('Type')}{th(L.col_crit)}{th('Score')}{th(tr('Risques', 'Risks'))}{th('CVE')}{th('Env')}{th(L.lastScan)}{th('')}</tr>
            </thead>
            <tbody>
              {rows.map((n) => {
                const Icon = TYPE_ICON[n.type];
                return (
                  <tr key={n.id} onClick={() => navigate('/assets/universe')} className="cursor-pointer hover:bg-hover transition-colors" style={{ borderBottom: '1px solid var(--border)' }}>
                    <td className="px-3 py-3">
                      <div className="flex items-center gap-2.5">
                        <div className="w-8 h-8 rounded-[9px] flex items-center justify-center shrink-0" style={{ background: softFill(critColor[n.crit], 14), color: critColor[n.crit] }}><Icon size={17} /></div>
                        <div>
                          <div className="text-[13.5px] font-medium text-ink">{n.name}</div>
                          <div className="mono text-[11px] text-ink-muted">{n.ip}</div>
                        </div>
                      </div>
                    </td>
                    <td className="px-3 py-3 text-[12.5px] text-ink-soft capitalize">{n.type}</td>
                    <td className="px-3 py-3"><CritBadge crit={n.crit} /></td>
                    <td className="px-3 py-3"><span className="mono text-[14px] font-bold" style={{ color: scoreColor(n.score) }}>{n.score.toFixed(1)}</span></td>
                    <td className="px-3 py-3 text-[13px] text-ink">{n.riskCount || '—'}</td>
                    <td className="px-3 py-3">
                      {n.cveCount ? <span className="text-[12px] font-semibold px-2 py-0.5 rounded-md" style={{ color: 'var(--high)', background: softFill('var(--high)', 14) }}>{n.cveCount}</span> : <span className="text-ink-muted">—</span>}
                    </td>
                    <td className="px-3 py-3 text-[12.5px] text-ink-soft">{n.env}</td>
                    <td className="px-3 py-3 text-[12px] text-ink-muted">{n.lastScan}</td>
                    <td className="px-3 py-3 text-right"><ChevronRight size={16} className="text-ink-muted inline" /></td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </Card>
    </PageFrame>
  );
}
