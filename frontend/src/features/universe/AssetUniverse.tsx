// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
//
// Asset Universe (OpenRisk.dc.html §6.6 + §7) — the signature feature. A
// force-directed graph on a raw <canvas>, with an IN-HOUSE physics simulation
// (no D3 / no graph library): pairwise repulsion 3000/d² with separation, link
// springs (d-96)·0.018, gravity to centre 0.005, damping 0.85, 160 pre-warm
// iterations. Drag a node (pins it), pan the background, wheel-zoom about the
// cursor (0.3–4×), click to select. DPR-aware; respects prefers-reduced-motion.

import { useEffect, useMemo, useRef, useState } from 'react';
import { Atom, Server, Grid3x3, LayoutDashboard, Filter, Plus, Minus, Globe, Database, Cloud, Laptop, X, type LucideIcon } from 'lucide-react';
import { useUIStore } from '../../store/uiStore';
import { useUIStrings } from '../../shared/uiStrings';
import { CritBadge, softFill, PreviewBadge } from '../../shared/ui';
import { scoreColor, critColor } from '../../shared/riskColors';
import { UNI_NODES, UNI_LINKS, type UniNode } from '../../shared/fixtures';
import { useNavigate } from 'react-router-dom';

const RADIUS: Record<string, number> = { critical: 26, high: 21, medium: 17, low: 13 };
const CM: Record<string, string> = { critical: '#ff453a', high: '#ff9f0a', medium: '#ffd426', low: '#30d158' };
const TYPE_ICON: Record<string, LucideIcon> = { globe: Globe, server: Server, database: Database, cloud: Cloud, laptop: Laptop };

interface SimNode extends UniNode { r: number; x: number; y: number; vx: number; vy: number; fixed?: boolean }
interface SimLink { s: SimNode; t: SimNode }

export function AssetUniverse() {
  const L = useUIStrings();
  const theme = useUIStore((s) => s.theme);
  const navigate = useNavigate();
  const [uniView, setUniView] = useState<'universe' | 'topology' | 'bubbles' | 'hierarchy'>('universe');
  const [selected, setSelected] = useState<SimNode | null>(null);

  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const nodesRef = useRef<SimNode[]>([]);
  const linksRef = useRef<SimLink[]>([]);
  const idxRef = useRef<Record<string, SimNode>>({});
  const viewRef = useRef({ x: 0, y: 0, k: 1 });
  const selRef = useRef<string | null>(null);
  const themeRef = useRef(theme);
  const rafRef = useRef<number>(0);

  useEffect(() => { themeRef.current = theme; }, [theme]);

  const views: [typeof uniView, string, LucideIcon][] = [
    ['universe', L.v_universe, Atom], ['topology', L.v_topology, Server],
    ['bubbles', L.v_bubbles, Grid3x3], ['hierarchy', L.v_hierarchy, LayoutDashboard],
  ];

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

  // ---- setup / teardown ----
  useEffect(() => {
    if (uniView !== 'universe') return;
    const canvas = canvasRef.current; if (!canvas) return;
    const nodes: SimNode[] = UNI_NODES.map((n) => ({ ...n, r: RADIUS[n.crit], x: 0, y: 0, vx: 0, vy: 0 }));
    const idx: Record<string, SimNode> = {}; nodes.forEach((n) => (idx[n.id] = n));
    const links: SimLink[] = UNI_LINKS.map(([a, b]) => ({ s: idx[a], t: idx[b] }));
    nodesRef.current = nodes; linksRef.current = links; idxRef.current = idx;
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
      selRef.current = n ? n.id : null; setSelected(n ?? null);
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
  }, [uniView]);

  const zoom = (f: number) => {
    const c = canvasRef.current; if (!c) return;
    const rect = c.getBoundingClientRect(); const cx = rect.width / 2, cy = rect.height / 2;
    const v = viewRef.current; v.x = cx - (cx - v.x) * f; v.y = cy - (cy - v.y) * f; v.k = Math.max(0.3, Math.min(4, v.k * f));
  };
  const resetView = () => { viewRef.current = { x: 0, y: 0, k: 1 }; selRef.current = null; setSelected(null); };

  const critCount = useMemo(() => UNI_NODES.filter((n) => n.crit === 'critical' || n.crit === 'high').length, []);

  return (
    <div className="relative w-full overflow-hidden" style={{ height: 'calc(100vh - 58px)', background: 'radial-gradient(circle at 50% 40%, color-mix(in srgb,var(--accent) 6%,var(--bg-app)), var(--bg-app))' }}>
      {/* toolbar */}
      <div className="absolute top-4 left-4 right-4 z-10 h-[50px] flex items-center gap-3.5 px-3.5 glass rounded-[14px] shadow-card-md">
        <div className="flex items-center gap-2.5 min-w-0">
          <span className="disp text-[14px] font-semibold text-ink whitespace-nowrap">{L.uniTitle}</span>
          <span className="w-px h-[18px] shrink-0" style={{ background: 'var(--border-strong)' }} />
          <span className="text-[12px] text-ink-soft whitespace-nowrap hidden sm:inline">{UNI_NODES.length} {L.uniAssets} · {UNI_LINKS.length} {L.uniLinks}</span>
          <span className="hidden md:inline"><PreviewBadge label={L.soon === 'Coming soon' ? 'Preview' : 'Aperçu'} /></span>
        </div>
        <div className="flex gap-[3px] mx-auto p-[3px] rounded-[10px]" style={{ background: 'var(--bg-hover)' }}>
          {views.map(([k, lbl, Icon]) => (
            <button
              key={k}
              onClick={() => setUniView(k)}
              className="h-[30px] px-3 rounded-lg text-[12px] font-semibold inline-flex items-center gap-1.5 transition-all"
              style={{ background: uniView === k ? 'linear-gradient(135deg,var(--accent),var(--accent-hover))' : 'transparent', color: uniView === k ? '#fff' : 'var(--text-secondary)' }}
            >
              <Icon size={14} /> <span className="hidden md:inline">{lbl}</span>
            </button>
          ))}
        </div>
        <div className="flex gap-1.5">
          <button className="w-9 h-9 rounded-[10px] flex items-center justify-center text-ink-soft hover:bg-hover" style={{ border: '1px solid var(--border-strong)' }}><Filter size={16} /></button>
          <button onClick={resetView} className="w-9 h-9 rounded-[10px] flex items-center justify-center text-ink-soft hover:bg-hover" style={{ border: '1px solid var(--border-strong)' }}><Atom size={16} /></button>
        </div>
      </div>

      {uniView === 'universe' ? (
        <canvas ref={canvasRef} className="absolute inset-0 w-full h-full block" style={{ cursor: 'grab' }} />
      ) : (
        <div className="absolute inset-0 flex flex-col items-center justify-center gap-3 text-center">
          <div style={{ color: 'var(--accent)' }}><Atom size={48} /></div>
          <div className="text-[15px] font-semibold text-ink">{L.soon}</div>
          <div className="text-[13px] text-ink-soft">{views.find((v) => v[0] === uniView)?.[1]}</div>
        </div>
      )}

      {/* zoom controls */}
      <div className="absolute right-4 z-10 flex flex-col gap-1 p-[5px] glass rounded-[12px] shadow-card-md" style={{ bottom: 44 }}>
        {[['+', () => zoom(1.25), Plus], ['-', () => zoom(0.8), Minus], ['r', resetView, Atom]].map(([k, fn, Icon]) => (
          <button key={k as string} onClick={fn as () => void} className="w-[34px] h-[34px] rounded-[9px] flex items-center justify-center text-ink-soft hover:bg-hover">
            {(() => { const I = Icon as LucideIcon; return <I size={17} />; })()}
          </button>
        ))}
      </div>

      {/* status bar */}
      <div className="absolute bottom-0 left-0 right-0 z-10 h-8 flex items-center gap-4 px-[18px] glass text-[11.5px] text-ink-soft" style={{ borderTop: '1px solid var(--border)' }}>
        <span>{UNI_NODES.length} {L.uniAssets}</span>
        <span style={{ color: 'var(--critical)' }}>{critCount} {L.critical.toLowerCase()}</span>
        <span style={{ color: 'var(--high)' }}>8 CVE</span>
        <span className="ml-auto hidden sm:inline">{L.lastScan} · il y a 3h</span>
      </div>

      {selected && <AssetPanel node={selected} onClose={resetView} onSelect={(n) => { selRef.current = n.id; setSelected(n); }} onView={() => navigate('/assets')} />}
    </div>
  );
}

function AssetPanel({ node, onClose, onSelect, onView }: { node: SimNode; onClose: () => void; onSelect: (n: SimNode) => void; onView: () => void }) {
  const L = useUIStrings();
  const lang = useUIStore((s) => s.lang);
  const col = critColor[node.crit];
  const Icon = TYPE_ICON[node.type] ?? Server;
  const conns = UNI_LINKS.filter(([a, b]) => a === node.id || b === node.id).map(([a, b]) => (a === node.id ? b : a)).slice(0, 6)
    .map((id) => UNI_NODES.find((n) => n.id === id)).filter(Boolean) as UniNode[];

  const Section = ({ title, children }: { title: string; children: React.ReactNode }) => (
    <div className="px-5 py-4" style={{ borderTop: '1px solid var(--border)' }}>
      <div className="text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted mb-2.5">{title}</div>
      {children}
    </div>
  );
  const KV = ({ k, v }: { k: string; v: string }) => (
    <div className="flex justify-between py-[5px] text-[13px]"><span className="text-ink-soft">{k}</span><span className="mono text-ink font-medium">{v}</span></div>
  );

  return (
    <div className="absolute right-4 z-20 flex flex-col overflow-hidden glass-strong rounded-[16px] shadow-card-lg" style={{ top: 80, bottom: 44, width: 340, animation: 'or-scalein .22s cubic-bezier(.2,.8,.2,1)' }}>
      <div className="px-5 pt-[18px] pb-4">
        <div className="flex items-start gap-3">
          <div className="w-[42px] h-[42px] rounded-xl flex items-center justify-center shrink-0" style={{ background: softFill(col, 16), color: col }}><Icon size={22} /></div>
          <div className="flex-1 min-w-0">
            <div className="disp text-[16px] font-bold text-ink truncate">{node.name}</div>
            <div className="mt-[5px]"><CritBadge crit={node.crit} /></div>
          </div>
          <button onClick={onClose} className="w-7 h-7 rounded-lg flex items-center justify-center text-ink-soft shrink-0" style={{ background: 'var(--bg-hover)' }}><X size={16} /></button>
        </div>
        <div className="flex items-center gap-3 mt-3.5 p-3 rounded-xl" style={{ background: 'var(--bg-hover)' }}>
          <span className="mono text-[26px] font-bold" style={{ color: scoreColor(node.score) }}>{node.score.toFixed(1)}</span>
          <span className="text-[12px] text-ink-soft leading-tight">{L.aggScore}</span>
        </div>
      </div>
      <div className="flex-1 overflow-y-auto">
        <Section title={lang === 'fr' ? 'Informations' : 'Info'}>
          <KV k="IP" v={node.ip} /><KV k="OS" v={node.os} /><KV k="Env" v={node.env} /><KV k={L.col_owner} v="AD" />
        </Section>
        <Section title={`${L.n_risks} · CVE`}>
          <KV k={lang === 'fr' ? 'Risques ouverts' : 'Open risks'} v={String(node.riskCount)} /><KV k="CVE" v={String(node.cveCount)} />
        </Section>
        {conns.length > 0 && (
          <Section title={L.connections}>
            <div className="flex flex-wrap gap-1.5">
              {conns.map((c) => (
                <button key={c.id} onClick={() => onSelect(c as SimNode)} className="text-[11.5px] font-medium px-[9px] py-[5px] rounded-lg inline-flex items-center gap-1.5" style={{ border: '1px solid var(--border)', background: 'var(--bg-elevated)', color: 'var(--text-primary)' }}>
                  <span className="w-1.5 h-1.5 rounded-full" style={{ background: critColor[c.crit] }} />{c.name}
                </button>
              ))}
            </div>
          </Section>
        )}
      </div>
      <div className="px-[18px] py-3.5" style={{ borderTop: '1px solid var(--border)' }}>
        <button onClick={onView} className="w-full h-[38px] rounded-[10px] text-[12.5px] font-semibold text-white" style={{ background: 'linear-gradient(135deg,var(--accent),var(--accent-hover))' }}>{L.viewFull}</button>
      </div>
    </div>
  );
}
