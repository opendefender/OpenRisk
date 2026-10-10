// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Pure helpers of the topology page (#907): a node's neighbourhood, what to
// highlight, asset search. The paths themselves come from the server
// (exposure_paths), computed with the compromise chain's rule.

import type { TopologyEdge, TopologyNode } from './topologyTypes';

export interface Neighbourhood {
  /** Assets this one depends on (edge source → target, this is the source). */
  up: string[];
  /** Assets that depend on this one. */
  down: string[];
  edges: string[];
}

export function neighbourhood(edges: readonly TopologyEdge[], id: string): Neighbourhood {
  const up: string[] = [];
  const down: string[] = [];
  const ids: string[] = [];
  for (const e of edges) {
    if (e.source === id) {
      up.push(e.target as string);
      ids.push(e.id as string);
    } else if (e.target === id) {
      down.push(e.source as string);
      ids.push(e.id as string);
    }
  }
  return { up: [...new Set(up)], down: [...new Set(down)], edges: ids };
}

export interface Highlight {
  nodes: Set<string>;
  edges: Set<string>;
}

export interface ExposurePathLike {
  asset_ids: string[];
  edge_ids: string[];
  hops: number;
}

/**
 * What the canvas keeps bright, in order of intent: a compromise chain the
 * user asked for, then the internet paths (one, or all when the action is on),
 * then the selected node's neighbourhood. Null means "everything".
 */
export function highlightFor(opts: {
  chain?: { nodes: Set<string>; edges: Set<string> } | null;
  paths?: readonly ExposurePathLike[] | null;
  selectedId?: string | null;
  edges: readonly TopologyEdge[];
}): Highlight | null {
  if (opts.chain) return opts.chain;
  if (opts.paths && opts.paths.length > 0) {
    const nodes = new Set<string>();
    const edges = new Set<string>();
    for (const p of opts.paths) {
      p.asset_ids.forEach((id) => nodes.add(id));
      p.edge_ids.forEach((id) => edges.add(id));
    }
    return { nodes, edges };
  }
  if (opts.selectedId) {
    const n = neighbourhood(opts.edges, opts.selectedId);
    return {
      nodes: new Set([opts.selectedId, ...n.up, ...n.down]),
      edges: new Set(n.edges),
    };
  }
  return null;
}

/** Assets whose name contains the query, accent- and case-insensitive. */
export function searchNodes(nodes: readonly TopologyNode[], q: string, limit = 8): TopologyNode[] {
  const fold = (s: string) =>
    s
      .normalize('NFD')
      .replace(/\p{Diacritic}/gu, '')
      .toLowerCase();
  const needle = fold(q.trim());
  if (!needle) return [];
  return nodes
    .filter((n) => fold(n.name ?? '').includes(needle))
    .sort((a, b) => {
      const sa = fold(a.name ?? '').startsWith(needle) ? 0 : 1;
      const sb = fold(b.name ?? '').startsWith(needle) ? 0 : 1;
      return sa - sb || (a.name ?? '').localeCompare(b.name ?? '');
    })
    .slice(0, limit);
}

/* ------------------------------------------------------------- labels */

export interface LabelCandidate {
  id: string;
  /** Node centre and radius, in the same units as width and height. */
  x: number;
  y: number;
  r: number;
  width: number;
  height: number;
  /** Higher first. A must-show label is placed even when it collides. */
  priority: number;
  mustShow?: boolean;
}

export interface PlacedLabel {
  id: string;
  /** Left edge and baseline of the text. */
  x: number;
  y: number;
}

interface Box {
  x0: number;
  y0: number;
  x1: number;
  y1: number;
}

const overlaps = (a: Box, b: Box) => a.x0 < b.x1 && b.x0 < a.x1 && a.y0 < b.y1 && b.y0 < a.y1;

/**
 * Places labels without overlapping each other or another node (#907): each
 * label tries the right of its node, then the left, above and below, in
 * priority order. A label with nowhere to go is dropped, unless it must show
 * (hovered, selected), in which case it takes the right and may overlap.
 */
export function placeLabels(
  labels: readonly LabelCandidate[],
  nodes: readonly { x: number; y: number; r: number }[],
  gap = 4,
): PlacedLabel[] {
  const taken: Box[] = nodes.map((n) => ({
    x0: n.x - n.r,
    y0: n.y - n.r,
    x1: n.x + n.r,
    y1: n.y + n.r,
  }));
  const out: PlacedLabel[] = [];
  const order = [...labels].sort((a, b) => b.priority - a.priority);
  for (const l of order) {
    const h = l.height;
    const spots: Box[] = [
      { x0: l.x + l.r + gap, y0: l.y - h / 2, x1: l.x + l.r + gap + l.width, y1: l.y + h / 2 },
      { x0: l.x - l.r - gap - l.width, y0: l.y - h / 2, x1: l.x - l.r - gap, y1: l.y + h / 2 },
      {
        x0: l.x - l.width / 2,
        y0: l.y - l.r - gap - h,
        x1: l.x + l.width / 2,
        y1: l.y - l.r - gap,
      },
      {
        x0: l.x - l.width / 2,
        y0: l.y + l.r + gap,
        x1: l.x + l.width / 2,
        y1: l.y + l.r + gap + h,
      },
    ];
    // Its own node does not block it.
    const own = (b: Box) =>
      b.x0 === l.x - l.r && b.y0 === l.y - l.r && b.x1 === l.x + l.r && b.y1 === l.y + l.r;
    const free = spots.find((s) => !taken.some((b) => !own(b) && overlaps(s, b)));
    const spot = free ?? (l.mustShow ? spots[0] : undefined);
    if (!spot) continue;
    taken.push(spot);
    out.push({ id: l.id, x: spot.x0, y: spot.y1 - h * 0.22 });
  }
  return out;
}
