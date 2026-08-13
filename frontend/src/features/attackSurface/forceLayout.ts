// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import type { TopologyEdge, TopologyNode } from './topologyTypes';

export interface LaidOutNode {
  id: string;
  x: number;
  y: number;
  vx: number;
  vy: number;
  /** Pinned by dragging — the simulation stops moving it. */
  fixed: boolean;
  zone: string;
  r: number;
  node: TopologyNode;
}

export interface LaidOutEdge {
  id: string;
  source: LaidOutNode;
  target: LaidOutNode;
  edge: TopologyEdge;
}

/**
 * A force-directed layout that stays interactive at 2 000 nodes.
 *
 * The naive version of this is O(n²) per tick — every node repelling every other
 * node. At 2 000 nodes that is 4 million pair evaluations per frame, which is
 * roughly two orders of magnitude too slow. Two things make it tractable:
 *
 *  1. **A uniform spatial grid** for repulsion. Repulsion falls off with the
 *     square of distance, so beyond a cutoff a node's contribution is not
 *     visible; only nodes within one cell of each other are compared. Cost
 *     becomes O(n · k) with k the local density, which stays small because the
 *     other forces actively spread nodes out.
 *  2. **Zone gravity instead of a global centre.** Each cluster is pulled toward
 *     its own anchor, laid out on a ring. This is what produces the clustering
 *     the view asks for, and it also keeps local density even — which is what
 *     keeps the grid's k small. The two requirements happen to serve each other.
 *
 * Everything below is plain arithmetic on typed arrays' worth of objects: no
 * allocation inside the tick loop, because at 60fps the garbage collector is the
 * next bottleneck after the algorithm.
 */

const REPULSION = 2400;
const SPRING = 0.012;
const SPRING_LENGTH = 70;
const ZONE_GRAVITY = 0.015;
const DAMPING = 0.86;
const MAX_VELOCITY = 12;
/** Repulsion cutoff; also the spatial grid's cell size. */
const CUTOFF = 110;

export interface LayoutState {
  nodes: LaidOutNode[];
  edges: LaidOutEdge[];
  byId: Map<string, LaidOutNode>;
  zoneAnchors: Map<string, { x: number; y: number }>;
  /** Falls from 1 to 0; the layout is settled at 0. */
  alpha: number;
  mode: LayoutMode;
}

/**
 * How the graph is laid out and drawn.
 *
 *  'zones'    — the topology: nodes cluster by network zone / cloud region /
 *               category, sized by degree so hubs read as hubs. Answers "how is
 *               the estate segmented, and what connects across the segments?".
 *  'universe' — the asset universe: one gravity centre, large orbs sized and
 *               coloured by criticality. Answers "what do we own, and what
 *               matters most?".
 *
 * They are two questions about the same graph, so they are two modes of one
 * view rather than two screens with two copies of the physics.
 */
export type LayoutMode = 'zones' | 'universe';

const CRIT_RADIUS: Record<string, number> = {
  CRITICAL: 22,
  HIGH: 18,
  MEDIUM: 14,
  LOW: 11,
};

/** Node radius. Degree-driven in topology mode, criticality-driven in universe. */
function radiusFor(n: TopologyNode, mode: LayoutMode): number {
  if (mode === 'universe') {
    return CRIT_RADIUS[(n.criticality ?? 'LOW') as string] ?? 11;
  }
  return 5 + Math.min(9, Math.sqrt(n.degree ?? 0) * 2.2);
}

/**
 * Seeds a layout. Nodes start ON their anchor (jittered), not at random:
 * starting from the answer's neighbourhood is what lets the simulation settle in
 * a few hundred ticks instead of a few thousand.
 */
export function createLayout(
  nodes: TopologyNode[],
  edges: TopologyEdge[],
  width: number,
  height: number,
  mode: LayoutMode = 'zones'
): LayoutState {
  const zones = [...new Set(nodes.map((n) => n.zone ?? 'unknown'))];
  const zoneAnchors = new Map<string, { x: number; y: number }>();
  const cx = width / 2;
  const cy = height / 2;
  const ringRadius = Math.min(width, height) * 0.32;

  zones.forEach((zone, i) => {
    // Universe mode has ONE centre: that is what makes it a universe rather
    // than a set of neighbourhoods, and it is the whole visual difference.
    if (mode === 'universe' || zones.length === 1) {
      zoneAnchors.set(zone, { x: cx, y: cy });
      return;
    }
    const angle = (i / zones.length) * Math.PI * 2 - Math.PI / 2;
    zoneAnchors.set(zone, {
      x: cx + Math.cos(angle) * ringRadius,
      y: cy + Math.sin(angle) * ringRadius,
    });
  });

  const byId = new Map<string, LaidOutNode>();
  const laid: LaidOutNode[] = nodes.map((n, i) => {
    const zone = n.zone ?? 'unknown';
    const anchor = zoneAnchors.get(zone) ?? { x: cx, y: cy };
    // Deterministic jitter (golden angle) — a reload must not reshuffle a graph
    // the user has learnt the shape of.
    const a = i * 2.399963;
    const spread = 30 + Math.sqrt(i) * 6;
    const ln: LaidOutNode = {
      id: n.id as string,
      x: anchor.x + Math.cos(a) * spread,
      y: anchor.y + Math.sin(a) * spread,
      vx: 0,
      vy: 0,
      fixed: false,
      zone,
      r: radiusFor(n, mode),
      node: n,
    };
    byId.set(ln.id, ln);
    return ln;
  });

  const laidEdges: LaidOutEdge[] = [];
  for (const e of edges) {
    const s = byId.get(e.source as string);
    const t = byId.get(e.target as string);
    if (!s || !t) continue; // server already drops these; belt and braces
    laidEdges.push({ id: e.id as string, source: s, target: t, edge: e });
  }

  return { nodes: laid, edges: laidEdges, byId, zoneAnchors, alpha: 1, mode };
}

/** Advances the simulation one step. Mutates in place — no per-tick allocation. */
export function tick(state: LayoutState): void {
  const { nodes, edges, zoneAnchors } = state;
  if (state.alpha <= 0) return;

  // --- repulsion, restricted to spatial-grid neighbours ---------------------
  const grid = new Map<string, LaidOutNode[]>();
  const cell = (v: number) => Math.floor(v / CUTOFF);
  // A string key, not `cx * 100000 + cy`. Node coordinates go negative as the
  // graph spreads past the origin, and that arithmetic collides across the sign
  // boundary — key(1, -1) and key(0, 99999) are both 99999 — which silently
  // dropped repulsion between real neighbours and applied it between distant
  // ones. The symptom is nodes piling on top of each other in one corner.
  const key = (cx: number, cy: number) => `${cx},${cy}`;

  for (const n of nodes) {
    const k = key(cell(n.x), cell(n.y));
    const bucket = grid.get(k);
    if (bucket) bucket.push(n);
    else grid.set(k, [n]);
  }

  for (const n of nodes) {
    const cx = cell(n.x);
    const cy = cell(n.y);
    for (let dx = -1; dx <= 1; dx++) {
      for (let dy = -1; dy <= 1; dy++) {
        const bucket = grid.get(key(cx + dx, cy + dy));
        if (!bucket) continue;
        for (const m of bucket) {
          if (m === n) continue;
          let ddx = n.x - m.x;
          let ddy = n.y - m.y;
          let d2 = ddx * ddx + ddy * ddy;
          if (d2 > CUTOFF * CUTOFF) continue;
          if (d2 < 0.01) {
            // Exactly coincident nodes have no direction to separate along.
            // Nudge deterministically rather than dividing by zero.
            ddx = (n.id < m.id ? 1 : -1) * 0.5;
            ddy = 0.5;
            d2 = 0.5;
          }
          const d = Math.sqrt(d2);
          // Bigger nodes must push harder or they overlap: in universe mode a
          // critical asset is twice the radius of a low one.
          const scale = ((n.r + m.r) / 16) ** 2;
          const force = (REPULSION * scale) / d2;
          n.vx += (ddx / d) * force * 0.5;
          n.vy += (ddy / d) * force * 0.5;
        }
      }
    }
  }

  // --- springs along edges --------------------------------------------------
  for (const e of edges) {
    const dx = e.target.x - e.source.x;
    const dy = e.target.y - e.source.y;
    const d = Math.sqrt(dx * dx + dy * dy) || 0.01;
    const rest = SPRING_LENGTH + e.source.r + e.target.r;
    const force = (d - rest) * SPRING;
    const fx = (dx / d) * force;
    const fy = (dy / d) * force;
    e.source.vx += fx;
    e.source.vy += fy;
    e.target.vx -= fx;
    e.target.vy -= fy;
  }

  // --- zone gravity: what actually produces the clusters --------------------
  for (const n of nodes) {
    const anchor = zoneAnchors.get(n.zone);
    if (!anchor) continue;
    n.vx += (anchor.x - n.x) * ZONE_GRAVITY;
    n.vy += (anchor.y - n.y) * ZONE_GRAVITY;
  }

  // --- integrate ------------------------------------------------------------
  for (const n of nodes) {
    if (n.fixed) {
      n.vx = 0;
      n.vy = 0;
      continue;
    }
    n.vx *= DAMPING;
    n.vy *= DAMPING;
    const speed = Math.hypot(n.vx, n.vy);
    if (speed > MAX_VELOCITY) {
      n.vx = (n.vx / speed) * MAX_VELOCITY;
      n.vy = (n.vy / speed) * MAX_VELOCITY;
    }
    n.x += n.vx * state.alpha;
    n.y += n.vy * state.alpha;
  }

  state.alpha = Math.max(0, state.alpha - 0.004);
}

/** Re-heats the simulation, e.g. after a drag. */
export function reheat(state: LayoutState, to = 0.5): void {
  state.alpha = Math.max(state.alpha, to);
}

/** The node under a world-space point, or null. Topmost (last drawn) wins. */
export function hitTest(state: LayoutState, x: number, y: number): LaidOutNode | null {
  for (let i = state.nodes.length - 1; i >= 0; i--) {
    const n = state.nodes[i];
    const r = n.r + 4; // a little forgiveness for the pointer
    if ((n.x - x) ** 2 + (n.y - y) ** 2 <= r * r) return n;
  }
  return null;
}

/** Bounding box of the laid-out graph, padded. Used to fit the view. */
export function bounds(state: LayoutState, pad = 60) {
  if (state.nodes.length === 0) {
    return { minX: 0, minY: 0, maxX: 100, maxY: 100 };
  }
  let minX = Infinity;
  let minY = Infinity;
  let maxX = -Infinity;
  let maxY = -Infinity;
  for (const n of state.nodes) {
    if (n.x - n.r < minX) minX = n.x - n.r;
    if (n.y - n.r < minY) minY = n.y - n.r;
    if (n.x + n.r > maxX) maxX = n.x + n.r;
    if (n.y + n.r > maxY) maxY = n.y + n.r;
  }
  return { minX: minX - pad, minY: minY - pad, maxX: maxX + pad, maxY: maxY + pad };
}
