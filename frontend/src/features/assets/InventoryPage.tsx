// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
//
// Inventory (OpenRisk.dc.html §6.12) — wired to the real /assets store. Asset table
// with type-icon, criticality badge, derived score (max of linked risks), linked-risk
// count and last-updated. Type-filter chips; create/edit modals; loading + empty states.

import { useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Atom, Plus, ChevronRight, Server, Laptop, Database, Cloud, Globe, HardDrive, Boxes, type LucideIcon } from 'lucide-react';
import { PageFrame, PageHeader, Btn, Chip, Card, CritBadge, SkeletonRows, EmptyState } from '../../shared/ui';
import { critColor, scoreColor, softFill, scoreToCriticality, type Criticality } from '../../shared/riskColors';
import { useUIStrings } from '../../shared/uiStrings';
import { useUIStore } from '../../store/uiStore';
import { useAssets } from './useAssets';
import { CreateAssetModal } from './CreateAssetModal';
import { EditAssetModal } from './EditAssetModal';
import { relTime } from '../risks/riskMap';
import type { Asset } from '../../types/asset';

const TYPE_ICON: Record<string, LucideIcon> = {
  Server: Server, Laptop: Laptop, Database: Database, SaaS: Cloud, Network: Globe, Storage: HardDrive,
};

export function InventoryPage() {
  const L = useUIStrings();
  const lang = useUIStore((s) => s.lang);
  const navigate = useNavigate();
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const { assets, isLoading } = useAssets();
  const [type, setType] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const [editing, setEditing] = useState<Asset | undefined>(undefined);

  const types = useMemo(() => [...new Set(assets.map((a) => a.type).filter(Boolean) as string[])], [assets]);
  const rows = assets.filter((a) => !type || a.type === type);

  const scoreOf = (a: Asset): number | null => {
    const rs = a.risks ?? [];
    if (!rs.length) return null;
    return Math.max(...rs.map((r) => r.score ?? 0));
  };

  const th = (t: string, right?: boolean) => (
    <th className={`text-${right ? 'right' : 'left'} text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted px-3 pb-[11px]`}>{t}</th>
  );

  return (
    <PageFrame wide>
      <PageHeader
        title={L.n_assets}
        count={`${assets.length} ${L.uniAssets}`}
        actions={
          <>
            <Btn label={tr('Vue Univers', 'Universe view')} icon={Atom} onClick={() => navigate('/assets/universe')} />
            <Btn label={tr('Nouvel actif', 'New asset')} icon={Plus} primary onClick={() => setCreating(true)} />
          </>
        }
      />

      {assets.length > 0 && (
        <div className="flex gap-2 mb-4 flex-wrap">
          <Chip label={tr('Tous', 'All')} active={!type} onClick={() => setType(null)} />
          {types.map((t) => <Chip key={t} label={t} active={type === t} onClick={() => setType(t)} />)}
        </div>
      )}

      <Card style={{ padding: '8px 8px 0', overflow: 'hidden' }}>
        {isLoading && assets.length === 0 ? (
          <SkeletonRows rows={6} />
        ) : assets.length === 0 ? (
          <EmptyState
            icon={Boxes}
            title={tr('Aucun actif inventorié', 'No assets yet')}
            sub={tr('Ajoutez vos serveurs, bases de données et services pour cartographier votre surface d’attaque.', 'Add your servers, databases and services to map your attack surface.')}
            cta={<Btn label={tr('Nouvel actif', 'New asset')} icon={Plus} primary onClick={() => setCreating(true)} />}
          />
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full border-collapse" style={{ minWidth: 720 }}>
              <thead style={{ borderBottom: '1px solid var(--border)' }}>
                <tr>{th(tr('Actif', 'Asset'))}{th('Type')}{th(L.col_crit)}{th('Score')}{th(tr('Risques', 'Risks'))}{th(L.col_mod)}{th('')}</tr>
              </thead>
              <tbody>
                {rows.map((a) => {
                  const crit = ((a.criticality ?? 'LOW').toLowerCase()) as Criticality;
                  const Icon = TYPE_ICON[a.type ?? 'Server'] ?? Server;
                  const sc = scoreOf(a);
                  const col = sc != null ? scoreColor(sc) : critColor[scoreToCriticality(0)];
                  return (
                    <tr key={a.id} onClick={() => setEditing(a)} className="cursor-pointer hover:bg-hover transition-colors" style={{ borderBottom: '1px solid var(--border)' }}>
                      <td className="px-3 py-3">
                        <div className="flex items-center gap-2.5">
                          <div className="w-8 h-8 rounded-[9px] flex items-center justify-center shrink-0" style={{ background: softFill(critColor[crit], 14), color: critColor[crit] }}><Icon size={17} /></div>
                          <div>
                            <div className="text-[13.5px] font-medium text-ink">{a.name}</div>
                            <div className="mono text-[11px] text-ink-muted">{a.owner || '—'}</div>
                          </div>
                        </div>
                      </td>
                      <td className="px-3 py-3 text-[12.5px] text-ink-soft">{a.type ?? '—'}</td>
                      <td className="px-3 py-3"><CritBadge crit={crit} /></td>
                      <td className="px-3 py-3">{sc != null ? <span className="mono text-[14px] font-bold" style={{ color: col }}>{sc.toFixed(1)}</span> : <span className="text-ink-muted">—</span>}</td>
                      <td className="px-3 py-3 text-[13px] text-ink">{a.risks?.length || '—'}</td>
                      <td className="px-3 py-3 text-[12px] text-ink-soft">{relTime(a.updated_at, lang)}</td>
                      <td className="px-3 py-3 text-right"><ChevronRight size={16} className="text-ink-muted inline" /></td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </Card>

      <CreateAssetModal isOpen={creating} onClose={() => setCreating(false)} />
      <EditAssetModal asset={editing} onClose={() => setEditing(undefined)} />
    </PageFrame>
  );
}
