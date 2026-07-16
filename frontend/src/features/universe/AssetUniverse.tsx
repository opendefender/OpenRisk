// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
//
// Asset Universe — the dependency cartography ("cartographie des dépendances
// entre actifs"). A force-directed graph on a raw <canvas> with an IN-HOUSE
// physics simulation (no D3 / no graph library): pairwise repulsion 3000/d²
// with separation, link springs (d-96)·0.018, gravity to centre 0.005, damping
// 0.85, 160 pre-warm iterations. Drag a node (pins it), pan the background,
// wheel-zoom about the cursor (0.3–4×), click to select. DPR-aware; respects
// prefers-reduced-motion.
//
// It is now wired to REAL data: nodes come from /assets, edges from
// /asset-dependencies. The side panel is a live dependency editor (add/remove
// edges), gated by the `assets:update` permission.

import { useEffect, useMemo, useRef, useState } from 'react';
import {
  Atom, Server, Filter, Plus, Minus, Globe, Database, Cloud, Laptop, HardDrive,
  AppWindow, Users, Building2, Boxes, X, Trash2, Link2, type LucideIcon,
} from 'lucide-react';
import { useUIStore } from '../../store/uiStore';
import { useUIStrings } from '../../shared/uiStrings';
import { CritBadge, softFill } from '../../shared/ui';
import { scoreColor, critColor, type Criticality } from '../../shared/riskColors';
import { useAuthStore } from '../../hooks/useAuthStore';
import { useNavigate } from 'react-router-dom';
import { useAssets } from '../assets/useAssets';
import { useAssetDependencies } from '../assets/useAssetDependencies';
import { useToast } from '../../hooks/useToast';
import { DEPENDENCY_TYPES, type Asset, type AssetDependency, type DependencyType } from '../../types/asset';

const RADIUS: Record<Criticality, number> = { critical: 26, high: 21, medium: 17, low: 13 };
const CM: Record<Criticality, string> = { critical: '#ff453a', high: '#ff9f0a', medium: '#ffd426', low: '#30d158' };

// Map a free-form asset.type onto a category icon. Unknown types fall back to Boxes.
const TYPE_ICON: Record<string, LucideIcon> = {
  Server: Server, Application: AppWindow, Cloud: Cloud, Database: Database, SaaS: Cloud,
  Storage: HardDrive, Network: Globe, Laptop: Laptop, Data: Database, User: Users, Supplier: Building2,
};
const iconFor = (type?: string): LucideIcon => TYPE_ICON[type ?? ''] ?? Boxes;

// Relationship labels for the cartography (self-contained FR/EN, no i18n churn).
const DEP_LABELS: Record<DependencyType, { fr: string; en: string }> = {
  depends_on: { fr: 'dépend de', en: 'depends on' },
  runs_on: { fr: "s'exécute sur", en: 'runs on' },
  connects_to: { fr: 'se connecte à', en: 'connects to' },
  hosted_by: { fr: 'hébergé par', en: 'hosted by' },
  stores_data_in: { fr: 'stocke ses données dans', en: 'stores data in' },
  authenticates_via: { fr: "s'authentifie via", en: 'authenticates via' },
  backs_up_to: { fr: 'sauvegardé vers', en: 'backs up to' },
  managed_by: { fr: 'géré par', en: 'managed by' },
};

const critOf = (a: Asset): Criticality => ((a.criticality ?? 'MEDIUM').toLowerCase() as Criticality);
const scoreOf = (a: Asset): number => {
  const rs = a.risks ?? [];
  return rs.length ? Math.max(...rs.map((r) => r.score ?? 0)) : 0;
};

interface GNode { id: string; name: string; type: string; crit: Criticality; score: number; riskCount: number; owner: string }
interface SimNode extends GNode { r: number; x: number; y: number; vx: number; vy: number; fixed?: boolean }
interface SimLink { s: SimNode; t: SimNode }

export function AssetUniverse() {
  const L = useUIStrings();
  const lang = useUIStore((s) => s.lang);
  const theme = useUIStore((s) => s.theme);
  const navigate = useNavigate();
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);

  const { assets, isLoading } = useAssets();
  const { dependencies, createDependency, deleteDependency } = useAssetDependencies();
  const canEdit = useAuthStore((s) => s.hasPermission('assets:update'));
  const toast = useToast();

  const [selectedId, setSelectedId] = useState<string | null>(null);

  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const nodesRef = useRef<SimNode[]>([]);
  const linksRef = useRef<SimLink[]>([]);
  const viewRef = useRef({ x: 0, y: 0, k: 1 });
  const selRef = useRef<string | null>(null);
  const themeRef = useRef(theme);
  const rafRef = useRef<number>(0);

  useEffect(() => { themeRef.current = theme; }, [theme]);
  useEffect(() => { selRef.current = selectedId; }, [selectedId]);

  // Build the graph from real data. gnodes = assets, glinks = dependencies whose
  // endpoints both resolve to a known asset.
  const { gnodes, glinks, sig } = useMemo(() => {
    const gnodes: GNode[] = assets.map((a) => ({
      id: a.id as string,
      name: a.name ?? '—',
      type: a.type ?? '',
      crit: critOf(a),
      score: scoreOf(a),
      riskCount: a.risks?.length ?? 0,
      owner: a.owner ?? '',
    }));
    const known = new Set(gnodes.map((n) => n.id));
    const glinks = (dependencies as AssetDependency[])
      .filter((d) => known.has(d.source_asset_id as string) && known.has(d.target_asset_id as string))
      .map((d) => [d.source_asset_id as string, d.target_asset_id as string] as [string, string]);
    const sig = gnodes.map((n) => n.id + n.crit).join('|') + '::' + glinks.map((l) => l.join('>')).join('|');
    return { gnodes, glinks, sig };
  }, [assets, dependencies]);

  const selectedAsset = useMemo(() => assets.find((a) => a.id === selectedId), [assets, selectedId]);

  // ---- physics ----
  const tickSim = (cx: number, cy: number) => {
    const N = nodesRef.current, Lk = linksRef.current;
    for (let i = 0; i < N.length; i++) {
      const a = N[i];
      for (let j = i + 1; j < N.length; j++) {
        const b = N[j];
        const dx = b.x - a.x, dy = b.y - a.y;
        const d2 = dx * dx + dy * dy || 1;
        const d = Math.sqrt(d2);
        const min = a.r + b.r + 16;
        let f = 3000 / d2;
        if (d < min) f += ((min - d) * 0.6) / d;
        const fx = (dx / d) * f, fy = (dy / d) * f;
        if (!a.fixed) { a.vx -= fx; a.vy -= fy; }
        if (!b.fixed) { b.vx += fx; b.vy += fy; }
      }
    }
    Lk.forEach((l) => {
      const dx = l.t.x - l.s.x, dy = l.t.y - l.s.y;
      const d = Math.sqrt(dx * dx + dy * dy) || 1;
      const f = (d - 96) * 0.018;
      const fx = (dx / d) * f, fy = (dy / d) * f;
      if (!l.s.fixed) { l.s.vx += fx; l.s.vy += fy; }
      if (!l.t.fixed) { l.t.vx -= fx; l.t.vy -= fy; }
    });
    N.forEach((n) => {
      if (n.fixed) return;
      n.vx += (cx - n.x) * 0.005; n.vy += (cy - n.y) * 0.005;
      n.vx *= 0.85; n.vy *= 0.85; n.x += n.vx; n.y += n.vy;
    });
  };

  const drawSim = () => {
    const c = canvasRef.current; if (!c) return;
    const ctx = c.getContext('2d'); if (!ctx) return;
    const dpr = window.devicePixelRatio || 1;
    const rect = c.getBoundingClientRect();
    if (c.width !== Math.round(rect.width * dpr)) { c.width = Math.round(rect.width * dpr); c.height = Math.round(rect.height * dpr); }
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.clearRect(0, 0, rect.width, rect.height);
    const v = viewRef.current; ctx.translate(v.x, v.y); ctx.scale(v.k, v.k);
    const dark = themeRef.current === 'dark';
    const sel = selRef.current;
    const neigh: Record<string, 1> = {};
    if (sel) linksRef.current.forEach((l) => { if (l.s.id === sel) neigh[l.t.id] = 1; if (l.t.id === sel) neigh[l.s.id] = 1; });
    linksRef.current.forEach((l) => {
      const hl = !!sel && (l.s.id === sel || l.t.id === sel);
      ctx.beginPath(); ctx.moveTo(l.s.x, l.s.y); ctx.lineTo(l.t.x, l.t.y);
      ctx.strokeStyle = hl ? 'rgba(10,132,255,0.55)' : dark ? 'rgba(150,150,170,0.16)' : 'rgba(80,80,100,0.14)';
      ctx.lineWidth = hl ? 1.8 : 1; ctx.stroke();
    });
    nodesRef.current.forEach((n) => {
      const col = CM[n.crit];
      const dim = !!sel && sel !== n.id && !neigh[n.id];
      ctx.globalAlpha = dim ? 0.28 : 1;
      const g = ctx.createRadialGradient(n.x, n.y, 0, n.x, n.y, n.r * 2.3);
      g.addColorStop(0, col + (n.crit === 'critical' ? 'aa' : '77')); g.addColorStop(1, col + '00');
      ctx.fillStyle = g; ctx.beginPath(); ctx.arc(n.x, n.y, n.r * 2.3, 0, 7); ctx.fill();
      ctx.beginPath(); ctx.arc(n.x, n.y, n.r, 0, 7);
      ctx.fillStyle = dark ? col + '33' : col + '22'; ctx.fill();
      ctx.lineWidth = n.id === sel ? 3 : 1.7; ctx.strokeStyle = col; ctx.stroke();
      ctx.globalAlpha = dim ? 0.25 : 0.95;
      ctx.fillStyle = dark ? '#d2d2da' : '#3a3a44';
      ctx.font = '600 10.5px Inter,sans-serif'; ctx.textAlign = 'center';
      ctx.fillText(n.name, n.x, n.y + n.r + 13); ctx.globalAlpha = 1;
    });
  };

  // ---- setup / teardown (re-inits whenever the real graph changes) ----
  useEffect(() => {
    const canvas = canvasRef.current; if (!canvas) return;
    if (gnodes.length === 0) { nodesRef.current = []; linksRef.current = []; return; }

    const nodes: SimNode[] = gnodes.map((n) => ({ ...n, r: RADIUS[n.crit], x: 0, y: 0, vx: 0, vy: 0 }));
    const idx: Record<string, SimNode> = {}; nodes.forEach((n) => (idx[n.id] = n));
    const links: SimLink[] = glinks
      .map(([a, b]) => ({ s: idx[a], t: idx[b] }))
      .filter((l) => l.s && l.t);
    nodesRef.current = nodes; linksRef.current = links;

    const rect = canvas.getBoundingClientRect();
    const cx = rect.width / 2 || 500, cy = rect.height / 2 || 350;
    nodes.forEach((n, i) => {
      const ang = (i / nodes.length) * Math.PI * 2;
      n.x = cx + Math.cos(ang) * 180 + (Math.random() * 30 - 15);
      n.y = cy + Math.sin(ang) * 180 + (Math.random() * 30 - 15);
    });
    viewRef.current = { x: 0, y: 0, k: 1 };
    for (let i = 0; i < 160; i++) tickSim(cx, cy);

    const toWorld = (e: MouseEvent) => {
      const rc = canvas.getBoundingClientRect(); const v = viewRef.current;
      return { x: (e.clientX - rc.left - v.x) / v.k, y: (e.clientY - rc.top - v.y) / v.k };
    };
    const hit = (wx: number, wy: number) => nodesRef.current.find((n) => {
      const dx = n.x - wx, dy = n.y - wy; return dx * dx + dy * dy <= (n.r + 4) * (n.r + 4);
    });
    let drag: { n: SimNode; dx: number; dy: number } | null = null;
    let pan: { x: number; y: number; vx: number; vy: number } | null = null;
    let moved = false;
    const md = (e: MouseEvent) => {
      const w = toWorld(e); const n = hit(w.x, w.y); moved = false;
      if (n) { drag = { n, dx: n.x - w.x, dy: n.y - w.y }; n.fixed = true; }
      else pan = { x: e.clientX, y: e.clientY, vx: viewRef.current.x, vy: viewRef.current.y };
    };
    const mm = (e: MouseEvent) => {
      const w = toWorld(e);
      if (drag) { drag.n.x = w.x + drag.dx; drag.n.y = w.y + drag.dy; drag.n.vx = 0; drag.n.vy = 0; moved = true; }
      else if (pan) { viewRef.current.x = pan.vx + (e.clientX - pan.x); viewRef.current.y = pan.vy + (e.clientY - pan.y); moved = true; }
      else { const n = hit(w.x, w.y); canvas.style.cursor = n ? 'pointer' : 'grab'; }
    };
    const mu = () => { if (drag) drag.n.fixed = false; drag = null; pan = null; };
    const click = (e: MouseEvent) => {
      if (moved) return;
      const w = toWorld(e); const n = hit(w.x, w.y);
      setSelectedId(n ? n.id : null);
    };
    const wheel = (e: WheelEvent) => {
      e.preventDefault();
      const rc = canvas.getBoundingClientRect(); const mx = e.clientX - rc.left, my = e.clientY - rc.top;
      const f = e.deltaY < 0 ? 1.1 : 0.9; const v = viewRef.current;
      const nk = Math.max(0.3, Math.min(4, v.k * f)); const r = nk / v.k;
      v.x = mx - (mx - v.x) * r; v.y = my - (my - v.y) * r; v.k = nk;
    };
    canvas.addEventListener('mousedown', md);
    window.addEventListener('mousemove', mm);
    window.addEventListener('mouseup', mu);
    canvas.addEventListener('click', click);
    canvas.addEventListener('wheel', wheel, { passive: false });

    const reduce = window.matchMedia?.('(prefers-reduced-motion:reduce)').matches;
    const loop = () => {
      const r = canvas.getBoundingClientRect();
      if (!reduce) tickSim(r.width / 2, r.height / 2);
      drawSim();
      rafRef.current = requestAnimationFrame(loop);
    };
    rafRef.current = requestAnimationFrame(loop);

    return () => {
      cancelAnimationFrame(rafRef.current);
      canvas.removeEventListener('mousedown', md);
      window.removeEventListener('mousemove', mm);
      window.removeEventListener('mouseup', mu);
      canvas.removeEventListener('click', click);
      canvas.removeEventListener('wheel', wheel);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sig]);

  const zoom = (f: number) => {
    const c = canvasRef.current; if (!c) return;
    const rect = c.getBoundingClientRect(); const cx = rect.width / 2, cy = rect.height / 2;
    const v = viewRef.current; v.x = cx - (cx - v.x) * f; v.y = cy - (cy - v.y) * f; v.k = Math.max(0.3, Math.min(4, v.k * f));
  };
  const resetView = () => { viewRef.current = { x: 0, y: 0, k: 1 }; setSelectedId(null); };

  const critCount = useMemo(() => gnodes.filter((n) => n.crit === 'critical' || n.crit === 'high').length, [gnodes]);

  const addDependency = async (targetId: string, type: DependencyType) => {
    if (!selectedId) return;
    try {
      await createDependency.mutateAsync({ source_asset_id: selectedId, target_asset_id: targetId, type });
      toast.success(tr('Dépendance ajoutée', 'Dependency added'));
    } catch {
      toast.error(tr("Impossible d'ajouter la dépendance", 'Could not add dependency'));
    }
  };
  const removeDependency = async (id: string) => {
    try {
      await deleteDependency.mutateAsync(id);
      toast.success(tr('Dépendance supprimée', 'Dependency removed'));
    } catch {
      toast.error(tr('Impossible de supprimer la dépendance', 'Could not remove dependency'));
    }
  };

  return (
    <div className="relative w-full overflow-hidden" style={{ height: 'calc(100vh - 58px)', background: 'radial-gradient(circle at 50% 40%, color-mix(in srgb,var(--accent) 6%,var(--bg-app)), var(--bg-app))' }}>
      {/* toolbar */}
      <div className="absolute top-4 left-4 right-4 z-10 h-[50px] flex items-center gap-3.5 px-3.5 glass rounded-[14px] shadow-card-md">
        <div className="flex items-center gap-2.5 min-w-0">
          <span className="disp text-[14px] font-semibold text-ink whitespace-nowrap">{L.uniTitle}</span>
          <span className="w-px h-[18px] shrink-0" style={{ background: 'var(--border-strong)' }} />
          <span className="text-[12px] text-ink-soft whitespace-nowrap hidden sm:inline">{gnodes.length} {L.uniAssets} · {glinks.length} {L.uniLinks}</span>
        </div>
        <div className="flex gap-1.5 ml-auto">
          <button title={tr('Réinitialiser la vue', 'Reset view')} className="w-9 h-9 rounded-[10px] flex items-center justify-center text-ink-soft hover:bg-hover" style={{ border: '1px solid var(--border-strong)' }}><Filter size={16} /></button>
          <button onClick={resetView} title={tr('Recentrer', 'Recenter')} className="w-9 h-9 rounded-[10px] flex items-center justify-center text-ink-soft hover:bg-hover" style={{ border: '1px solid var(--border-strong)' }}><Atom size={16} /></button>
        </div>
      </div>

      <canvas ref={canvasRef} className="absolute inset-0 w-full h-full block" style={{ cursor: 'grab' }} />

      {/* loading / empty overlays */}
      {isLoading && gnodes.length === 0 && (
        <div className="absolute inset-0 flex items-center justify-center pointer-events-none">
          <div className="text-[13px] text-ink-soft">{tr('Chargement de la cartographie…', 'Loading the map…')}</div>
        </div>
      )}
      {!isLoading && gnodes.length === 0 && (
        <div className="absolute inset-0 flex flex-col items-center justify-center gap-3 text-center px-6">
          <div style={{ color: 'var(--accent)' }}><Atom size={46} /></div>
          <div className="text-[15px] font-semibold text-ink">{tr('Aucun actif à cartographier', 'No assets to map')}</div>
          <div className="text-[13px] text-ink-soft max-w-sm">{tr('Ajoutez des actifs à votre inventaire, puis reliez-les pour visualiser leurs dépendances.', 'Add assets to your inventory, then link them to visualise their dependencies.')}</div>
          <button onClick={() => navigate('/assets')} className="mt-1 h-[36px] px-4 rounded-[10px] text-[12.5px] font-semibold text-white" style={{ background: 'linear-gradient(135deg,var(--accent),var(--accent-hover))' }}>{tr("Aller à l'inventaire", 'Go to inventory')}</button>
        </div>
      )}

      {/* zoom controls */}
      {gnodes.length > 0 && (
        <div className="absolute right-4 z-10 flex flex-col gap-1 p-[5px] glass rounded-[12px] shadow-card-md" style={{ bottom: 44 }}>
          {[['+', () => zoom(1.25), Plus], ['-', () => zoom(0.8), Minus], ['r', resetView, Atom]].map(([k, fn, Icon]) => (
            <button key={k as string} onClick={fn as () => void} className="w-[34px] h-[34px] rounded-[9px] flex items-center justify-center text-ink-soft hover:bg-hover">
              {(() => { const I = Icon as LucideIcon; return <I size={17} />; })()}
            </button>
          ))}
        </div>
      )}

      {/* status bar */}
      <div className="absolute bottom-0 left-0 right-0 z-10 h-8 flex items-center gap-4 px-[18px] glass text-[11.5px] text-ink-soft" style={{ borderTop: '1px solid var(--border)' }}>
        <span>{gnodes.length} {L.uniAssets}</span>
        <span style={{ color: 'var(--critical)' }}>{critCount} {tr('critiques/élevés', 'critical/high')}</span>
        <span>{glinks.length} {L.uniLinks}</span>
      </div>

      {selectedAsset && (
        <AssetPanel
          asset={selectedAsset}
          assets={assets}
          dependencies={dependencies}
          canEdit={canEdit}
          busy={createDependency.isPending || deleteDependency.isPending}
          onClose={resetView}
          onSelect={setSelectedId}
          onView={() => navigate('/assets')}
          onAdd={addDependency}
          onRemove={removeDependency}
        />
      )}
    </div>
  );
}

interface PanelProps {
  asset: Asset;
  assets: Asset[];
  dependencies: AssetDependency[];
  canEdit: boolean;
  busy: boolean;
  onClose: () => void;
  onSelect: (id: string) => void;
  onView: () => void;
  onAdd: (targetId: string, type: DependencyType) => void;
  onRemove: (id: string) => void;
}

function AssetPanel({ asset, assets, dependencies, canEdit, busy, onClose, onSelect, onView, onAdd, onRemove }: PanelProps) {
  const L = useUIStrings();
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const crit = critOf(asset);
  const col = critColor[crit];
  const Icon = iconFor(asset.type);
  const score = scoreOf(asset);
  const nameOf = (id?: string) => assets.find((a) => a.id === id)?.name ?? '—';
  const depLabel = (t?: DependencyType) => (t ? DEP_LABELS[t][lang === 'fr' ? 'fr' : 'en'] : '');

  const outgoing = dependencies.filter((d) => d.source_asset_id === asset.id);
  const incoming = dependencies.filter((d) => d.target_asset_id === asset.id);

  const [target, setTarget] = useState('');
  const [relType, setRelType] = useState<DependencyType>('depends_on');

  // Candidate targets: every other asset not already linked (this asset as source).
  const linkedTargetIds = new Set(outgoing.map((d) => d.target_asset_id));
  const candidates = assets.filter((a) => a.id !== asset.id && !linkedTargetIds.has(a.id));

  const submitAdd = () => {
    if (!target) return;
    onAdd(target, relType);
    setTarget('');
    setRelType('depends_on');
  };

  const Section = ({ title, children }: { title: string; children: React.ReactNode }) => (
    <div className="px-5 py-4" style={{ borderTop: '1px solid var(--border)' }}>
      <div className="text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted mb-2.5">{title}</div>
      {children}
    </div>
  );
  const KV = ({ k, v }: { k: string; v: string }) => (
    <div className="flex justify-between py-[5px] text-[13px]"><span className="text-ink-soft">{k}</span><span className="mono text-ink font-medium">{v}</span></div>
  );

  const EdgeRow = ({ dep, dir }: { dep: AssetDependency; dir: 'out' | 'in' }) => {
    const otherId = dir === 'out' ? dep.target_asset_id : dep.source_asset_id;
    return (
      <div className="flex items-center gap-2 py-[6px]">
        <button onClick={() => otherId && onSelect(otherId)} className="flex items-center gap-1.5 min-w-0 flex-1 text-left">
          <span className="w-1.5 h-1.5 rounded-full shrink-0" style={{ background: dir === 'out' ? 'var(--accent)' : 'var(--high)' }} />
          <span className="text-[12.5px] text-ink truncate">
            <span className="text-ink-muted">{dir === 'out' ? depLabel(dep.type) : `${tr('requis par', 'required by')}`} </span>
            {nameOf(otherId)}
            {dir === 'in' && <span className="text-ink-muted"> ({depLabel(dep.type)})</span>}
          </span>
        </button>
        {canEdit && (
          <button onClick={() => dep.id && onRemove(dep.id)} disabled={busy} className="w-6 h-6 rounded-md flex items-center justify-center text-ink-muted hover:text-[var(--critical)] hover:bg-hover shrink-0" title={tr('Supprimer', 'Remove')}>
            <Trash2 size={13} />
          </button>
        )}
      </div>
    );
  };

  const selectCls = 'w-full h-9 rounded-lg px-2.5 text-[12.5px] text-ink outline-none';
  const selectStyle = { border: '1px solid var(--border-strong)', background: 'var(--bg-elevated)' } as React.CSSProperties;

  return (
    <div className="absolute right-4 z-20 flex flex-col overflow-hidden glass-strong rounded-[16px] shadow-card-lg" style={{ top: 80, bottom: 44, width: 360, animation: 'or-scalein .22s cubic-bezier(.2,.8,.2,1)' }}>
      <div className="px-5 pt-[18px] pb-4">
        <div className="flex items-start gap-3">
          <div className="w-[42px] h-[42px] rounded-xl flex items-center justify-center shrink-0" style={{ background: softFill(col, 16), color: col }}><Icon size={22} /></div>
          <div className="flex-1 min-w-0">
            <div className="disp text-[16px] font-bold text-ink truncate">{asset.name}</div>
            <div className="mt-[5px]"><CritBadge crit={crit} /></div>
          </div>
          <button onClick={onClose} className="w-7 h-7 rounded-lg flex items-center justify-center text-ink-soft shrink-0" style={{ background: 'var(--bg-hover)' }}><X size={16} /></button>
        </div>
        <div className="flex items-center gap-3 mt-3.5 p-3 rounded-xl" style={{ background: 'var(--bg-hover)' }}>
          <span className="mono text-[26px] font-bold" style={{ color: scoreColor(score) }}>{score.toFixed(1)}</span>
          <span className="text-[12px] text-ink-soft leading-tight">{L.aggScore ?? tr('Score agrégé', 'Aggregate score')}</span>
        </div>
      </div>

      <div className="flex-1 overflow-y-auto">
        <Section title={tr('Informations', 'Info')}>
          <KV k="Type" v={asset.type || '—'} />
          <KV k={L.col_owner ?? tr('Responsable', 'Owner')} v={asset.owner || '—'} />
          <KV k={tr('Risques ouverts', 'Open risks')} v={String(asset.risks?.length ?? 0)} />
        </Section>

        <Section title={`${tr('Dépendances', 'Dependencies')} · ${outgoing.length + incoming.length}`}>
          {outgoing.length === 0 && incoming.length === 0 && (
            <div className="text-[12px] text-ink-muted py-1">{tr('Aucune dépendance déclarée.', 'No dependencies declared.')}</div>
          )}
          {outgoing.map((d) => <EdgeRow key={d.id} dep={d} dir="out" />)}
          {incoming.map((d) => <EdgeRow key={d.id} dep={d} dir="in" />)}

          {canEdit && (
            <div className="mt-3 pt-3 space-y-2" style={{ borderTop: '1px dashed var(--border)' }}>
              <div className="text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted flex items-center gap-1.5"><Link2 size={12} /> {tr('Ajouter une dépendance', 'Add a dependency')}</div>
              <select value={relType} onChange={(e) => setRelType(e.target.value as DependencyType)} className={selectCls} style={selectStyle}>
                {DEPENDENCY_TYPES.map((t) => <option key={t} value={t}>{depLabel(t)}</option>)}
              </select>
              <select value={target} onChange={(e) => setTarget(e.target.value)} className={selectCls} style={selectStyle}>
                <option value="">{candidates.length ? tr('Choisir un actif cible…', 'Choose a target asset…') : tr('Aucun actif disponible', 'No asset available')}</option>
                {candidates.map((a) => <option key={a.id} value={a.id}>{a.name}</option>)}
              </select>
              <button onClick={submitAdd} disabled={!target || busy} className="w-full h-9 rounded-lg text-[12.5px] font-semibold text-white disabled:opacity-50 flex items-center justify-center gap-1.5" style={{ background: 'linear-gradient(135deg,var(--accent),var(--accent-hover))' }}>
                <Plus size={14} /> {tr('Relier', 'Link')}
              </button>
            </div>
          )}
        </Section>
      </div>

      <div className="px-[18px] py-3.5" style={{ borderTop: '1px solid var(--border)' }}>
        <button onClick={onView} className="w-full h-[38px] rounded-[10px] text-[12.5px] font-semibold text-white" style={{ background: 'linear-gradient(135deg,var(--accent),var(--accent-hover))' }}>{L.viewFull ?? tr("Voir dans l'inventaire", 'View in inventory')}</button>
      </div>
    </div>
  );
}
