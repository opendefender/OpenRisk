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
