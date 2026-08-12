// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { bounds, type LayoutState } from './forceLayout';
import { EDGE_DASH, type ColorMode, type TopologyEdgeType } from './topologyTypes';

/**
 * Export the topology as SVG or PNG.
 *
 * The SVG is BUILT FROM THE LAYOUT, not screenshotted from the canvas: an export
 * that only captures the current viewport would silently crop the graph the user
 * is trying to put in a report, and would rasterise text that should stay
 * selectable. The PNG is then rendered from that same SVG, so both formats show
 * the same complete picture.
 */

interface ExportOptions {
  colorOf: (nodeId: string) => string;
  /** Node ids to highlight (the compromise chain), if any. */
  highlighted?: Set<string>;
  /** Edge ids on the highlighted chain. */
  highlightedEdges?: Set<string>;
  /** Solid background — a transparent PNG dropped in a light report is unreadable. */
  background: string;
  /** Ink colour for labels and edges. */
  ink: string;
  title?: string;
}

function escapeXml(s: string): string {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

/** Builds a standalone SVG document for the whole graph. */
export function topologyToSvg(state: LayoutState, opts: ExportOptions): string {
  const b = bounds(state, 40);
  const w = Math.max(1, Math.round(b.maxX - b.minX));
  const h = Math.max(1, Math.round(b.maxY - b.minY));
  const dim = state.nodes.length > 0 && (opts.highlighted?.size ?? 0) > 0;

  const parts: string[] = [];
  parts.push(
    `<svg xmlns="http://www.w3.org/2000/svg" width="${w}" height="${h}" ` +
      `viewBox="${b.minX} ${b.minY} ${w} ${h}">`
  );
  parts.push(`<rect x="${b.minX}" y="${b.minY}" width="${w}" height="${h}" fill="${opts.background}"/>`);

  // Edges first, so nodes sit on top of them.
  parts.push('<g stroke-linecap="round" fill="none">');
  for (const e of state.edges) {
    const on = !dim || opts.highlightedEdges?.has(e.id);
    const dash = EDGE_DASH[(e.edge.type ?? 'depends_on') as TopologyEdgeType];
    parts.push(
      `<path d="M${e.source.x.toFixed(1)} ${e.source.y.toFixed(1)}L${e.target.x.toFixed(1)} ${e.target.y.toFixed(1)}" ` +
        `stroke="${opts.ink}" stroke-opacity="${on ? 0.45 : 0.08}" stroke-width="${on ? 1.4 : 1}"` +
        (dash.length ? ` stroke-dasharray="${dash.join(' ')}"` : '') +
        '/>'
    );
  }
  parts.push('</g>');

  parts.push('<g>');
  for (const n of state.nodes) {
    const on = !dim || opts.highlighted?.has(n.id);
    parts.push(
      `<circle cx="${n.x.toFixed(1)}" cy="${n.y.toFixed(1)}" r="${n.r.toFixed(1)}" ` +
        `fill="${opts.colorOf(n.id)}" fill-opacity="${on ? 1 : 0.15}"/>`
    );
  }
  parts.push('</g>');

  // Labels only for nodes big enough to carry one — labelling 2 000 nodes
  // produces an unreadable smear, and the export is meant to be read.
  parts.push(`<g font-family="ui-sans-serif, system-ui, sans-serif" font-size="9" fill="${opts.ink}">`);
  for (const n of state.nodes) {
    if (n.r < 7 && state.nodes.length > 120) continue;
    const on = !dim || opts.highlighted?.has(n.id);
    if (!on) continue;
    parts.push(
      `<text x="${(n.x + n.r + 3).toFixed(1)}" y="${(n.y + 3).toFixed(1)}" fill-opacity="0.8">` +
        `${escapeXml(n.node.name ?? '')}</text>`
    );
  }
  parts.push('</g>');

  if (opts.title) {
    parts.push(
      `<text x="${b.minX + 16}" y="${b.minY + 26}" font-family="ui-sans-serif, system-ui, sans-serif" ` +
        `font-size="14" font-weight="600" fill="${opts.ink}">${escapeXml(opts.title)}</text>`
    );
  }

  parts.push('</svg>');
  return parts.join('');
}

function triggerDownload(blob: Blob, filename: string): void {
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

export function downloadSvg(state: LayoutState, opts: ExportOptions, filename = 'topologie.svg'): void {
  triggerDownload(new Blob([topologyToSvg(state, opts)], { type: 'image/svg+xml' }), filename);
}

/**
 * Renders the exported SVG to a PNG at 2× for legibility on a normal screen.
 * Returns a promise so the caller can show progress and, more importantly,
 * report a failure rather than leaving a button that did nothing.
 */
export async function downloadPng(
  state: LayoutState,
  opts: ExportOptions,
  filename = 'topologie.png'
): Promise<void> {
  const svg = topologyToSvg(state, opts);
  const b = bounds(state, 40);
  const w = Math.max(1, Math.round(b.maxX - b.minX));
  const h = Math.max(1, Math.round(b.maxY - b.minY));
  const scale = 2;

  const url = URL.createObjectURL(new Blob([svg], { type: 'image/svg+xml' }));
  try {
    const img = new Image();
    await new Promise<void>((resolve, reject) => {
      img.onload = () => resolve();
      img.onerror = () => reject(new Error("Le rendu PNG a échoué."));
      img.src = url;
    });

    const canvas = document.createElement('canvas');
    canvas.width = w * scale;
    canvas.height = h * scale;
    const ctx = canvas.getContext('2d');
    if (!ctx) throw new Error("Le canvas n'est pas disponible.");
    ctx.fillStyle = opts.background;
    ctx.fillRect(0, 0, canvas.width, canvas.height);
    ctx.drawImage(img, 0, 0, canvas.width, canvas.height);

    const blob = await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, 'image/png'));
    if (!blob) throw new Error("Le PNG n'a pas pu être produit.");
    triggerDownload(blob, filename);
  } finally {
    URL.revokeObjectURL(url);
  }
}
