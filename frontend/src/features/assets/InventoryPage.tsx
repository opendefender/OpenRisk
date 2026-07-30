// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Inventory (OpenRisk.dc.html §6.12) — wired to the real /assets store. Asset table
// with type-icon, criticality badge, derived score (max of linked risks), linked-risk
// count and last-updated. Type-filter chips; create/edit modals; loading + empty states.

import { useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Atom, Plus, Server, Laptop, Database, Cloud, Globe, HardDrive, Boxes, AppWindow, Users, Building2, type LucideIcon } from 'lucide-react';
import { PageFrame, PageHeader, Btn, Chip, Card, CritBadge, SkeletonRows, EmptyState } from '../../shared/ui';
import { DataTable, type Column } from '../../shared/DataTable';
import { critColor, scoreColor, softFill, type Criticality } from '../../shared/riskColors';
import { useUIStrings } from '../../shared/uiStrings';
import { useUIStore } from '../../store/uiStore';
import { useAssets } from './useAssets';
import { CreateAssetModal } from './CreateAssetModal';
import { EditAssetModal } from './EditAssetModal';
import { AssetHistoryDrawer } from './AssetHistoryDrawer';
import { useFocusParam } from '../../shared/useFocusParam';
import { relTime } from '../risks/riskMap';
import type { Asset } from '../../types/asset';

const TYPE_ICON: Record<string, LucideIcon> = {
  Server: Server, Application: AppWindow, Cloud: Cloud, Database: Database, SaaS: Cloud,
  Storage: HardDrive, Network: Globe, Laptop: Laptop, Data: Database, User: Users, Supplier: Building2,
};

// Derived asset score = the max score of its linked risks (null when none).
const scoreOf = (a: Asset): number | null => {
  const rs = a.risks ?? [];
  if (!rs.length) return null;
  return Math.max(...rs.map((r) => r.score ?? 0));
};
const CRIT_RANK: Record<string, number> = { critical: 4, high: 3, medium: 2, low: 1 };

export function InventoryPage() {
  const L = useUIStrings();
  const lang = useUIStore((s) => s.lang);
  const navigate = useNavigate();
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const { assets, isLoading } = useAssets();
  const [type, setType] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const [editing, setEditing] = useState<Asset | undefined>(undefined);
  const [historyAssetId, setHistoryAssetId] = useState<string | null>(null);

  // Deep-link from universal search (/assets?focus=<id>) → open that asset's editor
  // once it's present in the loaded list.
  const { focusId, clearFocus } = useFocusParam();
  useEffect(() => {
    if (!focusId) return;
    const a = assets.find((x) => x.id === focusId);
    if (a) {
      setEditing(a);
      clearFocus();
    }
  }, [focusId, assets, clearFocus]);

  const types = useMemo(() => [...new Set(assets.map((a) => a.type).filter(Boolean) as string[])], [assets]);
  const rows = assets.filter((a) => !type || a.type === type);

  // Kit adoption (docs/UI_ELEVATION §6): dense, sortable, density-aware DataTable with
  // a frozen Asset column. Default sort surfaces the most critical assets first.
  const columns: Column<Asset>[] = useMemo(() => [
    {
      key: 'name', header: tr('Actif', 'Asset'), frozen: true, sortValue: (a) => (a.name ?? '').toLowerCase(),
      render: (a) => {
        const crit = ((a.criticality ?? 'LOW').toLowerCase()) as Criticality;
        const Icon = TYPE_ICON[a.type ?? 'Server'] ?? Server;
        return (
          <div className="flex items-center gap-2.5">
            <div className="w-8 h-8 rounded-[9px] flex items-center justify-center shrink-0" style={{ background: softFill(critColor[crit], 14), color: critColor[crit] }}><Icon size={17} /></div>
            <div>
              <div className="text-[13.5px] font-medium text-ink">{a.name}</div>
              <div className="mono text-[11px] text-ink-muted">{a.owner || '—'}</div>
            </div>
          </div>
        );
      },
    },
    { key: 'type', header: 'Type', sortValue: (a) => a.type ?? '', render: (a) => <span className="text-[12.5px] text-ink-soft">{a.type ?? '—'}</span> },
    { key: 'crit', header: L.col_crit, sortValue: (a) => CRIT_RANK[(a.criticality ?? 'LOW').toLowerCase()] ?? 0, render: (a) => <CritBadge crit={((a.criticality ?? 'LOW').toLowerCase()) as Criticality} /> },
    { key: 'score', header: 'Score', align: 'right', sortValue: (a) => scoreOf(a) ?? -1, render: (a) => { const sc = scoreOf(a); return sc != null ? <span className="mono text-[14px] font-bold" style={{ color: scoreColor(sc) }}>{sc.toFixed(1)}</span> : <span className="text-ink-muted">—</span>; } },
    { key: 'risks', header: tr('Risques', 'Risks'), align: 'right', sortValue: (a) => a.risks?.length ?? 0, render: (a) => <span className="text-[13px] text-ink">{a.risks?.length || '—'}</span> },
    { key: 'mod', header: L.col_mod, sortValue: (a) => new Date(a.updated_at ?? 0).getTime(), render: (a) => <span className="text-[12px] text-ink-soft">{relTime(a.updated_at, lang)}</span> },
    // eslint-disable-next-line react-hooks/exhaustive-deps
  ], [lang]);

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
          <DataTable
            rows={rows}
            columns={columns}
            rowKey={(a) => a.id as string}
            onRowClick={(a) => setEditing(a)}
            minWidth={720}
            initialSort={{ key: 'crit', dir: 'desc' }}
            empty={<EmptyState icon={Boxes} title={tr('Aucun résultat', 'No results')} sub={tr('Aucun actif ne correspond à ce filtre.', 'No asset matches this filter.')} />}
          />
        )}
      </Card>

      <CreateAssetModal isOpen={creating} onClose={() => setCreating(false)} />
      <EditAssetModal
        asset={editing}
        onClose={() => setEditing(undefined)}
        onShowHistory={(id) => { setEditing(undefined); setHistoryAssetId(id); }}
      />
      <AssetHistoryDrawer assetId={historyAssetId} onClose={() => setHistoryAssetId(null)} />
    </PageFrame>
  );
}
