// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).
//
// Topology on the October 2026 redesign (#907): how assets depend on each
// other and which routes lead from the internet to a critical asset.
//
// The design's frame (grid canvas, legend below, side panel with exposure
// paths or the selected node) around our graph engine, which keeps zoom and
// pan, dragging, the Topologie / Univers layouts, colour by criticality or
// exposure, zone and criticality filters, the compromise chain, dependency
// editing and SVG / PNG export. Added per the owner's review (D-069 D): search
// and centre on an asset, labels that adapt to zoom and appear on hover and
// selection, the neighbourhood isolated on selection, and a list view of the
// same data for keyboard and screen-reader users.

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import {
  AlertTriangle,
  Download,
  Maximize2,
  Minus,
  Network,
  Plus,
  Route,
  Search,
  SlidersHorizontal,
} from 'lucide-react';

import { useI18n } from '../../hooks/useI18n';
import { useAuthStore } from '../../hooks/useAuthStore';
import { useToast } from '../../hooks/useToast';
import { apiErrorMessage } from '../../lib/apiError';
import { Button, Menu, Popover, TabPanel, Tabs } from '../../shared/ds';
import { PageFrame, PageHeader } from '../../shared/ui';
import { critColor, type Criticality } from '../../shared/riskColors';
import { BlockError, BlockSkeleton } from '../dashboard/console/Panel';
import { useDrawerController } from '../entity-drawer/drawerState';
import { useAssetDependencies } from '../assets/useAssetDependencies';
import { topologyService } from './topologyService';
import {
  bounds,
  createLayout,
  hitTest,
  reheat,
  tick,
  type LaidOutNode,
  type LayoutMode,
  type LayoutState,
} from './forceLayout';
import { downloadPng, downloadSvg } from './topologyExport';
import {
  EDGE_DASH,
  TOPOLOGY_EDGE_TYPES,
  type ColorMode,
  type CompromiseChain,
  type TopologyEdgeType,
  type TopologyNode,
} from './topologyTypes';
import { highlightFor, neighbourhood, searchNodes, type Highlight } from './topologyModel';
import { DependencyEditor, NodePanel, PathsPanel } from './TopologyPanel';
import { TopologyList } from './TopologyList';

const EXPOSED_COLOR = 'var(--risk-critical)';
const INTERNAL_COLOR = 'var(--risk-low)';
const CRITS = ['CRITICAL', 'HIGH', 'MEDIUM', 'LOW'] as const;

/** Resolves a CSS variable to a concrete colour — canvas and the export cannot
 *  carry `var(--x)`. */
function resolveColor(cssVar: string, root: HTMLElement): string {
  const name = cssVar.match(/var\((--[^)]+)\)/)?.[1];
  if (!name) return cssVar;
  const resolved = getComputedStyle(root).getPropertyValue(name).trim();
  // An unresolvable token falls back to another TOKEN, never a literal: the
  // fallback has to follow the theme too.
  return resolved || getComputedStyle(root).getPropertyValue('--fg-muted').trim();
}

type View = 'graph' | 'list';

export default function TopologyView() {
  const { t } = useI18n();
  const toast = useToast();
  const { open: openDrawer } = useDrawerController();
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const wrapRef = useRef<HTMLDivElement | null>(null);
  const layoutRef = useRef<LayoutState | null>(null);
  const rafRef = useRef<number | null>(null);
  // The camera lives in a ref: it changes on every wheel or drag event, and a
  // render per event would be the whole frame budget.
  const camRef = useRef({ x: 0, y: 0, k: 1 });
  const hoverRef = useRef<string | null>(null);
  // The camera follows the layout while it settles, until the user takes it
  // (wheel, pan, zoom buttons, centring on a node): framing once at the start
  // caught the graph before it spread and left it small in a corner.
  const userCamRef = useRef(false);

  const [view, setView] = useState<View>('graph');
  const [colorMode, setColorMode] = useState<ColorMode>('criticality');
  const [layoutMode, setLayoutMode] = useState<LayoutMode>('zones');
  const [zoneFilter, setZoneFilter] = useState('');
  // Unticking a level drops those assets and every edge ending on one.
  const [critFilter, setCritFilter] = useState<Set<string>>(new Set(CRITS));
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [chainOrigin, setChainOrigin] = useState<string | null>(null);
  const [pathsOn, setPathsOn] = useState(false);
  const [activePath, setActivePath] = useState<number | null>(null);
  const [query, setQuery] = useState('');
  const [exporting, setExporting] = useState(false);

  const canEdit = useAuthStore((st) => st.hasPermission('assets:update'));
  const { dependencies, createDependency, deleteDependency } = useAssetDependencies();

  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: ['attack-surface', 'topology'],
    queryFn: () => topologyService.get(),
  });
  const { data: chain } = useQuery<CompromiseChain>({
    queryKey: ['attack-surface', 'chain', chainOrigin],
    queryFn: () => topologyService.compromiseChain(chainOrigin as string),
    enabled: !!chainOrigin,
  });

  const nodes = useMemo(() => data?.nodes ?? [], [data]);
  const edges = useMemo(() => data?.edges ?? [], [data]);
  const paths = useMemo(
    () =>
      (data?.exposure_paths ?? []).map((p) => ({
        asset_ids: p.asset_ids as string[],
        edge_ids: p.edge_ids as string[],
        hops: p.hops,
      })),
    [data],
  );
  const byId = useMemo(() => new Map(nodes.map((n) => [n.id as string, n])), [nodes]);
  const nameOf = useCallback((id: string) => byId.get(id)?.name ?? id.slice(0, 8), [byId]);
  const selected = selectedId ? byId.get(selectedId) : undefined;

  const visible = useCallback(
    (n: TopologyNode) =>
      (!zoneFilter || n.zone === zoneFilter) && critFilter.has((n.criticality ?? 'LOW') as string),
    [zoneFilter, critFilter],
  );
  const visibleCount = useMemo(() => nodes.filter(visible).length, [nodes, visible]);

  const highlight: Highlight | null = useMemo(() => {
    const chainSet = chain
      ? {
          nodes: new Set<string>([
            chain.origin_id as string,
            ...(chain.impacted ?? []).map((h) => h.asset_id as string),
            ...(chain.reachable ?? []).map((h) => h.asset_id as string),
          ]),
          edges: new Set((chain.edge_ids ?? []) as string[]),
        }
      : null;
    return highlightFor({
      chain: chainSet,
      paths: activePath !== null ? [paths[activePath]] : pathsOn ? paths : null,
      selectedId,
      edges,
    });
  }, [chain, paths, activePath, pathsOn, selectedId, edges]);

  const colorOf = useCallback(
    (n: TopologyNode): string => {
      if (colorMode === 'exposure') return n.internet_exposed ? EXPOSED_COLOR : INTERNAL_COLOR;
      const crit = ((n.criticality ?? 'LOW') as string).toLowerCase() as Criticality;
      return critColor[crit] ?? critColor.low;
    },
    [colorMode],
  );

  const fitToView = useCallback(() => {
    const st = layoutRef.current;
    const wrap = wrapRef.current;
    if (!st || !wrap) return;
    const b = bounds(st, 40);
    const w = wrap.clientWidth || 900;
    const h = wrap.clientHeight || 600;
    const k = Math.min(w / (b.maxX - b.minX), h / (b.maxY - b.minY), 2.5);
    camRef.current = {
      k,
      x: w / 2 - ((b.minX + b.maxX) / 2) * k,
      y: h / 2 - ((b.minY + b.maxY) / 2) * k,
    };
  }, []);

  // --- build / rebuild the layout when the data or the filters change -------
  useEffect(() => {
    const wrap = wrapRef.current;
    if (!wrap || nodes.length === 0 || view !== 'graph') {
      layoutRef.current = null;
      return;
    }
    const vn = nodes.filter(visible);
    const ids = new Set(vn.map((n) => n.id as string));
    const ve = edges.filter((e) => ids.has(e.source as string) && ids.has(e.target as string));
    layoutRef.current = createLayout(
      vn,
      ve,
      wrap.clientWidth || 900,
      wrap.clientHeight || 520,
      layoutMode,
    );
    userCamRef.current = false;
    fitToView();
  }, [nodes, edges, visible, layoutMode, view, fitToView]);

  // --- render loop ----------------------------------------------------------
  useEffect(() => {
    const draw = () => {
      const canvas = canvasRef.current;
      const wrap = wrapRef.current;
      const st = layoutRef.current;
      if (canvas && wrap && st) {
        const dpr = window.devicePixelRatio || 1;
        const w = wrap.clientWidth;
        const h = wrap.clientHeight;
        if (canvas.width !== w * dpr || canvas.height !== h * dpr) {
          canvas.width = w * dpr;
          canvas.height = h * dpr;
          canvas.style.width = `${w}px`;
          canvas.style.height = `${h}px`;
        }
        const ctx = canvas.getContext('2d');
        if (ctx) {
          tick(st);
          if (!userCamRef.current && st.alpha > 0) fitToView();
          paint(ctx, st, dpr, w, h);
        }
      }
      rafRef.current = requestAnimationFrame(draw);
    };
    rafRef.current = requestAnimationFrame(draw);
    return () => {
      if (rafRef.current) cancelAnimationFrame(rafRef.current);
    };
  });

  const paint = (
    ctx: CanvasRenderingContext2D,
    st: LayoutState,
    dpr: number,
    w: number,
    h: number,
  ) => {
    const root = document.documentElement;
    const ink = resolveColor('var(--fg-primary)', root);
    const edgeColor = resolveColor('var(--graph-edge)', root);
    const activeEdge = resolveColor('var(--graph-edge-active)', root);
    const nodeFill = resolveColor('var(--graph-node)', root);
    const accent = resolveColor('var(--accent)', root);
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.clearRect(0, 0, w, h);
    const cam = camRef.current;
    ctx.save();
    ctx.translate(cam.x, cam.y);
    ctx.scale(cam.k, cam.k);

    const hl = highlight;
    // Edges: dash carries the relation; colour stays with the node scale.
    ctx.lineCap = 'round';
    for (const e of st.edges) {
      const on = !hl || hl.edges.has(e.id);
      ctx.globalAlpha = on ? (hl ? 0.95 : 0.6) : 0.08;
      ctx.strokeStyle = hl && on ? activeEdge : edgeColor;
      ctx.lineWidth = (hl && on ? 2 : 1.2) / cam.k;
      const dash = EDGE_DASH[(e.edge.type ?? 'depends_on') as TopologyEdgeType] ?? [];
      ctx.setLineDash(dash.map((d) => d / cam.k));
      ctx.beginPath();
      ctx.moveTo(e.source.x, e.source.y);
      ctx.lineTo(e.target.x, e.target.y);
      ctx.stroke();
    }
    ctx.setLineDash([]);

    // Nodes: the design's rings, coloured by criticality or exposure.
    for (const n of st.nodes) {
      const on = !hl || hl.nodes.has(n.id);
      ctx.globalAlpha = on ? 1 : 0.15;
      ctx.fillStyle = nodeFill;
      ctx.strokeStyle = resolveColor(colorOf(n.node), root);
      ctx.lineWidth = 2.5 / cam.k;
      ctx.beginPath();
      ctx.arc(n.x, n.y, n.r, 0, Math.PI * 2);
      ctx.fill();
      ctx.stroke();
      if (n.id === selectedId || n.id === chainOrigin) {
        ctx.globalAlpha = 1;
        ctx.strokeStyle = n.id === selectedId ? accent : ink;
        ctx.lineWidth = 2 / cam.k;
        ctx.beginPath();
        ctx.arc(n.x, n.y, n.r + 5 / cam.k, 0, Math.PI * 2);
        ctx.stroke();
      }
    }

    // Labels adapt: every label once zoomed in enough to read them; otherwise
    // only the hovered and selected nodes, and those an isolation keeps bright.
    const labelZoom = st.mode === 'universe' ? 0.3 : 0.55;
    const all = cam.k > labelZoom;
    ctx.fillStyle = ink;
    ctx.font = `${11.5 / cam.k}px ui-sans-serif, system-ui, sans-serif`;
    for (const n of st.nodes) {
      const lit = hl?.nodes.has(n.id) ?? false;
      const show =
        n.id === hoverRef.current || n.id === selectedId || (hl ? lit && hl.nodes.size <= 24 : all);
      if (!show || (hl && !lit && n.id !== hoverRef.current)) continue;
      ctx.globalAlpha = 0.92;
      ctx.fillText(n.node.name ?? '', n.x + n.r + 4 / cam.k, n.y + 4 / cam.k);
    }
    ctx.globalAlpha = 1;
    ctx.restore();
  };

  // --- pointer: pan, zoom, drag, select, hover -------------------------------
  const dragRef = useRef<{
    node: LaidOutNode | null;
    lastX: number;
    lastY: number;
    moved: boolean;
  } | null>(null);
  const toWorld = (clientX: number, clientY: number) => {
    const rect = canvasRef.current?.getBoundingClientRect();
    const cam = camRef.current;
    return {
      x: (clientX - (rect?.left ?? 0) - cam.x) / cam.k,
      y: (clientY - (rect?.top ?? 0) - cam.y) / cam.k,
    };
  };
  const onPointerDown = (e: React.PointerEvent) => {
    const st = layoutRef.current;
    if (!st) return;
    (e.target as Element).setPointerCapture?.(e.pointerId);
    const { x, y } = toWorld(e.clientX, e.clientY);
    const hit = hitTest(st, x, y);
    dragRef.current = { node: hit, lastX: e.clientX, lastY: e.clientY, moved: false };
    if (hit) hit.fixed = true;
  };
  const onPointerMove = (e: React.PointerEvent) => {
    const st = layoutRef.current;
    const drag = dragRef.current;
    if (!st) return;
    if (!drag) {
      const { x, y } = toWorld(e.clientX, e.clientY);
      const hit = hitTest(st, x, y);
      hoverRef.current = hit?.id ?? null;
      if (canvasRef.current) canvasRef.current.style.cursor = hit ? 'pointer' : 'grab';
      return;
    }
    const dx = e.clientX - drag.lastX;
    const dy = e.clientY - drag.lastY;
    if (Math.abs(dx) + Math.abs(dy) > 2) drag.moved = true;
    drag.lastX = e.clientX;
    drag.lastY = e.clientY;
    if (drag.node) {
      drag.node.x += dx / camRef.current.k;
      drag.node.y += dy / camRef.current.k;
      reheat(st, 0.35);
    } else {
      userCamRef.current = true;
      camRef.current.x += dx;
      camRef.current.y += dy;
    }
  };
  const onPointerUp = () => {
    const drag = dragRef.current;
    if (drag) {
      if (drag.node) {
        drag.node.fixed = false;
        if (!drag.moved) select(drag.node.id);
      } else if (!drag.moved) {
        // A click on empty canvas lets go of the selection.
        setSelectedId(null);
      }
    }
    dragRef.current = null;
  };

  const zoomBy = (factor: number, cx?: number, cy?: number) => {
    const wrap = wrapRef.current;
    const cam = camRef.current;
    const px = cx ?? (wrap?.clientWidth ?? 0) / 2;
    const py = cy ?? (wrap?.clientHeight ?? 0) / 2;
    userCamRef.current = true;
    const next = Math.min(4, Math.max(0.08, cam.k * factor));
    cam.x = px - ((px - cam.x) / cam.k) * next;
    cam.y = py - ((py - cam.y) / cam.k) * next;
    cam.k = next;
  };
  const onWheel = (e: React.WheelEvent) => {
    const rect = canvasRef.current?.getBoundingClientRect();
    zoomBy(
      e.deltaY < 0 ? 1.12 : 1 / 1.12,
      e.clientX - (rect?.left ?? 0),
      e.clientY - (rect?.top ?? 0),
    );
  };

  /** Select a node and bring it to the middle of the canvas. */
  const select = (id: string) => {
    setSelectedId(id);
    setActivePath(null);
    const n = layoutRef.current?.byId.get(id);
    const wrap = wrapRef.current;
    if (n && wrap) {
      userCamRef.current = true;
      const cam = camRef.current;
      cam.k = Math.max(cam.k, 1.1);
      cam.x = wrap.clientWidth / 2 - n.x * cam.k;
      cam.y = wrap.clientHeight / 2 - n.y * cam.k;
    }
  };
  const showFromList = (id: string) => {
    const n = byId.get(id);
    if (n && !visible(n)) {
      setZoneFilter('');
      setCritFilter(new Set(CRITS));
    }
    setView('graph');
    // Wait one frame for the layout to exist, then centre.
    requestAnimationFrame(() => requestAnimationFrame(() => select(id)));
  };

  // --- export ---------------------------------------------------------------
  const doExport = async (format: 'svg' | 'png') => {
    const st = layoutRef.current;
    if (!st) return;
    const root = document.documentElement;
    const colors = new Map<string, string>();
    for (const n of st.nodes) colors.set(n.id, resolveColor(colorOf(n.node), root));
    const opts = {
      colorOf: (id: string) => colors.get(id) ?? resolveColor('var(--fg-muted)', root),
      highlighted: highlight?.nodes,
      highlightedEdges: highlight?.edges,
      background: resolveColor('var(--surface-1)', root),
      ink: resolveColor('var(--fg-primary)', root),
      title: `${t('topology.title')} — ${st.nodes.length}`,
    };
    setExporting(true);
    try {
      if (format === 'svg') downloadSvg(st, opts);
      else await downloadPng(st, opts);
      toast.success(t('topology.exported', { format: format.toUpperCase() }));
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t('topology.exportFailed'));
    } finally {
      setExporting(false);
    }
  };

  const zones = data?.zones ?? [];
  const zoneLabel = (key?: string) => zones.find((z) => z.key === key)?.label ?? key ?? '';
  const results = useMemo(() => searchNodes(nodes, query), [nodes, query]);
  const nb = selectedId ? neighbourhood(edges, selectedId) : null;
  const anyExposed = nodes.some((n) => n.internet_exposed);
  const legend =
    colorMode === 'exposure'
      ? [
          { c: EXPOSED_COLOR, label: t('topology.legendExposure.exposed') },
          { c: INTERNAL_COLOR, label: t('topology.legendExposure.internal') },
        ]
      : (['critical', 'high', 'medium', 'low'] as const).map((c) => ({
          c: critColor[c],
          label: t(`topology.legendCrit.${c}`),
        }));

  return (
    <PageFrame wide>
      <PageHeader
        className="!mb-4"
        title={t('topology.title')}
        subtitle={t('topology.subtitle')}
        actions={
          <>
            <Menu
              label={t('topology.export')}
              trigger={
                <Button
                  variant="ghost"
                  icon={Download}
                  loading={exporting}
                  aria-label={t('topology.export')}
                />
              }
              items={[
                { label: t('topology.exportSvg'), onSelect: () => void doExport('svg') },
                { label: t('topology.exportPng'), onSelect: () => void doExport('png') },
              ]}
            />
            <Button
              variant={pathsOn ? 'primary' : 'secondary'}
              icon={Route}
              aria-pressed={pathsOn}
              onClick={() => {
                setPathsOn((v) => !v);
                setActivePath(null);
                setSelectedId(null);
                setChainOrigin(null);
              }}
              data-testid="topo-paths-toggle"
            >
              {t('topology.paths')}
            </Button>
          </>
        }
      />

      {data?.truncated && (
        <div className="mb-3 flex items-start gap-2 rounded-[12px] border border-border-subtle px-3 py-2 text-[13px] text-ink-soft bg-surface-2">
          <AlertTriangle
            size={16}
            className="mt-0.5 shrink-0 text-warning-text"
            aria-hidden="true"
          />
          <span>{t('topology.truncated', { limit: data.node_limit ?? 0 })}</span>
        </div>
      )}

      <div className="flex flex-wrap items-center gap-2 mb-3">
        <SearchBox
          query={query}
          onQuery={setQuery}
          results={results}
          onPick={(id) => {
            setQuery('');
            showFromList(id);
          }}
        />
        <Tabs
          id="topo-view"
          items={[
            { id: 'graph', label: t('topology.graph') },
            { id: 'list', label: t('topology.list') },
          ]}
          value={view}
          onChange={(v) => setView(v as View)}
          label={t('topology.viewLabel')}
        />
        <span className="flex-1" />
        <span className="text-[12px] text-ink-muted" data-testid="topo-count">
          {data
            ? visibleCount !== nodes.length
              ? t('topology.countFiltered', { shown: visibleCount, total: nodes.length })
              : t('topology.count', { count: nodes.length, links: edges.length })
            : null}
        </span>
        <Popover
          label={t('topology.display')}
          placement="bottom"
          trigger={
            <Button variant="secondary" size="sm" icon={SlidersHorizontal}>
              {t('topology.display')}
            </Button>
          }
        >
          <DisplaySettings
            layoutMode={layoutMode}
            setLayoutMode={setLayoutMode}
            colorMode={colorMode}
            setColorMode={setColorMode}
            zones={zones}
            zoneFilter={zoneFilter}
            setZoneFilter={setZoneFilter}
            critFilter={critFilter}
            setCritFilter={setCritFilter}
          />
        </Popover>
      </div>

      <div className="flex flex-wrap gap-4 items-stretch">
        <section className="flex-[2_1_600px] min-w-0 rounded-[14px] border border-border-subtle bg-surface-1 overflow-hidden flex flex-col">
          <TabPanel tabsId="topo-view" id={view} active>
            {isLoading ? (
              <div className="p-5">
                <BlockSkeleton lines={8} height={36} />
              </div>
            ) : isError ? (
              <div className="p-5">
                <BlockError onRetry={() => void refetch()} />
              </div>
            ) : nodes.length === 0 ? (
              <div className="flex h-[420px] flex-col items-center justify-center gap-2 text-center px-6">
                <Network size={32} className="text-ink-muted" aria-hidden="true" />
                <p className="m-0 text-sm text-ink-soft">{t('topology.empty')}</p>
                <p className="m-0 text-[12px] text-ink-muted">{t('topology.emptyHint')}</p>
              </div>
            ) : view === 'list' ? (
              <TopologyList
                nodes={nodes.filter(visible)}
                edges={edges}
                nameOf={nameOf}
                onShow={showFromList}
              />
            ) : (
              <div className="relative">
                <div
                  ref={wrapRef}
                  className="h-[min(62vh,560px)] min-h-[360px] w-full"
                  style={{
                    backgroundImage:
                      'linear-gradient(var(--grid-line) 1px, transparent 1px), linear-gradient(90deg, var(--grid-line) 1px, transparent 1px)',
                    backgroundSize: '32px 32px',
                  }}
                >
                  <canvas
                    ref={canvasRef}
                    role="img"
                    aria-label={t('topology.canvasLabel')}
                    className="h-full w-full touch-none"
                    style={{ cursor: 'grab' }}
                    onPointerDown={onPointerDown}
                    onPointerMove={onPointerMove}
                    onPointerUp={onPointerUp}
                    onPointerCancel={onPointerUp}
                    onPointerLeave={() => {
                      hoverRef.current = null;
                    }}
                    onWheel={onWheel}
                    data-testid="topo-canvas"
                  />
                </div>
                <div className="absolute top-3 left-3 flex flex-col gap-1">
                  <MapBtn title={t('topology.zoomIn')} onClick={() => zoomBy(1.25)}>
                    <Plus size={15} />
                  </MapBtn>
                  <MapBtn title={t('topology.zoomOut')} onClick={() => zoomBy(1 / 1.25)}>
                    <Minus size={15} />
                  </MapBtn>
                  <MapBtn title={t('topology.fit')} onClick={fitToView}>
                    <Maximize2 size={15} />
                  </MapBtn>
                </div>
              </div>
            )}
          </TabPanel>
          {nodes.length > 0 && (
            <div className="mt-auto flex flex-wrap items-center gap-x-4 gap-y-1.5 px-4 py-2.5 border-t border-border-subtle text-[11.5px] text-ink-muted bg-surface-1">
              {legend.map((l) => (
                <span key={l.label} className="flex items-center gap-1.5">
                  <span
                    className="w-2.5 h-2.5 rounded-full border-2"
                    style={{ borderColor: l.c }}
                  />
                  {l.label}
                </span>
              ))}
              <span className="flex-1" />
              {TOPOLOGY_EDGE_TYPES.map((ty) => (
                <span key={ty} className="flex items-center gap-1.5">
                  <svg width="22" height="6" aria-hidden="true">
                    <line
                      x1="0"
                      y1="3"
                      x2="22"
                      y2="3"
                      stroke="currentColor"
                      strokeWidth="1.5"
                      strokeDasharray={EDGE_DASH[ty].join(' ') || undefined}
                    />
                  </svg>
                  {t(`topology.rel.${ty}`)}
                </span>
              ))}
            </div>
          )}
        </section>

        <div className="flex-[1_1_300px] min-w-[280px]">
          {selected && nb ? (
            <NodePanel
              node={selected}
              zoneLabel={zoneLabel(selected.zone as string)}
              slug=""
              up={nb.up}
              down={nb.down}
              nameOf={nameOf}
              onSelect={select}
              onClose={() => setSelectedId(null)}
              onOpenAsset={() => openDrawer('asset', selected.id as string)}
              chainOn={chainOrigin === selected.id}
              chainCounts={
                chain && chainOrigin === selected.id
                  ? {
                      impacted: chain.impacted?.length ?? 0,
                      reachable: chain.reachable?.length ?? 0,
                    }
                  : null
              }
              onChain={() =>
                setChainOrigin(chainOrigin === selected.id ? null : (selected.id as string))
              }
              editor={
                <DependencyEditor
                  assetId={selected.id as string}
                  nodes={nodes}
                  dependencies={dependencies}
                  canEdit={canEdit}
                  busy={createDependency.isPending || deleteDependency.isPending}
                  onCreate={async (targetId, type) => {
                    try {
                      await createDependency.mutateAsync({
                        source_asset_id: selected.id,
                        target_asset_id: targetId,
                        type,
                      });
                      await refetch();
                      toast.success(t('topology.depAdded'));
                    } catch (err) {
                      toast.error(apiErrorMessage(err) || t('topology.depAddFailed'));
                    }
                  }}
                  onDelete={async (id) => {
                    try {
                      await deleteDependency.mutateAsync(id);
                      await refetch();
                    } catch (err) {
                      toast.error(apiErrorMessage(err) || t('topology.depRemoveFailed'));
                    }
                  }}
                />
              }
            />
          ) : isLoading ? (
            <div className="bg-surface-1 border border-border-subtle rounded-[14px] p-5">
              <BlockSkeleton lines={5} height={28} />
            </div>
          ) : data ? (
            <PathsPanel
              paths={paths}
              nameOf={nameOf}
              critOf={(id) => (byId.get(id)?.criticality ?? 'LOW') as string}
              anyExposed={anyExposed}
              activeIndex={activePath}
              onPick={(i) => {
                setActivePath(i);
                setChainOrigin(null);
              }}
            />
          ) : null}
        </div>
      </div>
    </PageFrame>
  );
}

/* --------------------------------------------------------------- search */

function SearchBox({
  query,
  onQuery,
  results,
  onPick,
}: {
  query: string;
  onQuery: (q: string) => void;
  results: TopologyNode[];
  onPick: (id: string) => void;
}) {
  const { t } = useI18n();
  const [active, setActive] = useState(0);
  const open = query.trim().length > 0;
  return (
    <div className="relative flex-[0_1_280px] min-w-[200px]">
      <Search
        size={15}
        className="absolute left-2.5 top-1/2 -translate-y-1/2 text-ink-muted pointer-events-none"
        aria-hidden="true"
      />
      <input
        type="search"
        role="combobox"
        aria-expanded={open}
        aria-controls="topo-search-list"
        aria-activedescendant={
          open && results[active] ? `topo-opt-${results[active].id}` : undefined
        }
        aria-label={t('topology.searchLabel')}
        placeholder={t('topology.search')}
        value={query}
        onChange={(e) => {
          onQuery(e.target.value);
          setActive(0);
        }}
        onKeyDown={(e) => {
          if (e.key === 'ArrowDown') {
            e.preventDefault();
            setActive((a) => Math.min(a + 1, results.length - 1));
          } else if (e.key === 'ArrowUp') {
            e.preventDefault();
            setActive((a) => Math.max(a - 1, 0));
          } else if (e.key === 'Enter' && results[active]) {
            e.preventDefault();
            onPick(results[active].id as string);
          } else if (e.key === 'Escape') {
            onQuery('');
          }
        }}
        className="w-full h-9 pl-8 pr-2 rounded-[10px] text-[13px] text-ink bg-surface-1 border border-border-default"
        data-testid="topo-search"
      />
      {open && (
        <ul
          id="topo-search-list"
          role="listbox"
          className="absolute z-20 mt-1 w-full max-h-72 overflow-auto m-0 p-1 list-none rounded-[10px] border border-border-subtle bg-surface-2 shadow-lg"
        >
          {results.length === 0 ? (
            <li className="px-2.5 py-2 text-[12.5px] text-ink-muted">{t('topology.noMatch')}</li>
          ) : (
            results.map((n, i) => (
              <li
                key={n.id}
                id={`topo-opt-${n.id}`}
                role="option"
                aria-selected={i === active}
                onMouseDown={(e) => {
                  e.preventDefault();
                  onPick(n.id as string);
                }}
                onMouseEnter={() => setActive(i)}
                className={`flex items-center gap-2 px-2.5 py-1.5 rounded-[8px] text-[13px] cursor-pointer ${
                  i === active ? 'bg-surface-3 text-ink' : 'text-ink-soft'
                }`}
              >
                <span
                  className="w-2 h-2 rounded-full shrink-0"
                  style={{
                    background:
                      critColor[((n.criticality ?? 'LOW') as string).toLowerCase() as Criticality],
                  }}
                />
                <span className="truncate">{n.name}</span>
              </li>
            ))
          )}
        </ul>
      )}
    </div>
  );
}

/* ------------------------------------------------------- display popover */

function DisplaySettings({
  layoutMode,
  setLayoutMode,
  colorMode,
  setColorMode,
  zones,
  zoneFilter,
  setZoneFilter,
  critFilter,
  setCritFilter,
}: {
  layoutMode: LayoutMode;
  setLayoutMode: (m: LayoutMode) => void;
  colorMode: ColorMode;
  setColorMode: (m: ColorMode) => void;
  zones: { key?: string; label?: string; count?: number }[];
  zoneFilter: string;
  setZoneFilter: (z: string) => void;
  critFilter: Set<string>;
  setCritFilter: (f: Set<string>) => void;
}) {
  const { t } = useI18n();
  const group = 'text-[10.5px] tracking-[.06em] uppercase font-semibold text-ink-muted mb-1.5';
  return (
    <div className="grid gap-3 p-3 w-[300px]" data-testid="topo-display">
      <div>
        <div className={group}>{t('topology.layout')}</div>
        <div className="flex gap-1.5">
          <Opt on={layoutMode === 'zones'} onClick={() => setLayoutMode('zones')}>
            {t('topology.layoutZones')}
          </Opt>
          <Opt on={layoutMode === 'universe'} onClick={() => setLayoutMode('universe')}>
            {t('topology.layoutUniverse')}
          </Opt>
        </div>
      </div>
      <div>
        <div className={group}>{t('topology.color')}</div>
        <div className="flex gap-1.5">
          <Opt on={colorMode === 'criticality'} onClick={() => setColorMode('criticality')}>
            {t('topology.colorCrit')}
          </Opt>
          <Opt on={colorMode === 'exposure'} onClick={() => setColorMode('exposure')}>
            {t('topology.colorExposure')}
          </Opt>
        </div>
      </div>
      <div>
        <div className={group}>{t('topology.criticality')}</div>
        <div className="flex flex-wrap gap-1.5">
          {CRITS.map((c) => (
            <Opt
              key={c}
              on={critFilter.has(c)}
              onClick={() => {
                const next = new Set(critFilter);
                if (next.has(c)) next.delete(c);
                else next.add(c);
                setCritFilter(next);
              }}
            >
              {t(`topology.crit.${c}`)}
            </Opt>
          ))}
        </div>
      </div>
      {zones.length > 0 && (
        <div>
          <div className={group}>{t('topology.zone')}</div>
          <div className="flex flex-wrap gap-1.5 max-h-32 overflow-auto">
            <Opt on={!zoneFilter} onClick={() => setZoneFilter('')}>
              {t('topology.allZones')}
            </Opt>
            {zones.map((z) => (
              <Opt
                key={z.key}
                on={zoneFilter === z.key}
                onClick={() => setZoneFilter(zoneFilter === z.key ? '' : (z.key as string))}
              >
                {z.label} {z.count}
              </Opt>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

function Opt({
  on,
  onClick,
  children,
}: {
  on: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      aria-pressed={on}
      onClick={onClick}
      className={`h-7 px-2.5 rounded-full border text-[12px] font-semibold ${
        on
          ? 'bg-surface-3 border-border-strong text-ink'
          : 'border-border-default text-ink-soft hover:border-border-strong'
      }`}
    >
      {children}
    </button>
  );
}

function MapBtn({
  children,
  onClick,
  title,
}: {
  children: React.ReactNode;
  onClick: () => void;
  title: string;
}) {
  return (
    <button
      type="button"
      title={title}
      aria-label={title}
      onClick={onClick}
      className="flex h-8 w-8 items-center justify-center rounded-[8px] border border-border-subtle bg-surface-2 text-ink-soft hover:text-ink"
    >
      {children}
    </button>
  );
}
