// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Contract-first: aliases onto the types generated from docs/openapi.yaml.
import type { components } from '../../types/openapi.generated';

export type AssetTopology = components['schemas']['AssetTopology'];
export type TopologyNode = components['schemas']['TopologyNode'];
export type TopologyEdge = components['schemas']['TopologyEdge'];
export type TopologyZone = components['schemas']['TopologyZone'];
export type CompromiseChain = components['schemas']['CompromiseChain'];
export type TopologyEdgeType = NonNullable<TopologyEdge['type']>;

/** Legend order — the same four the server folds every stored relation onto. */
export const TOPOLOGY_EDGE_TYPES: TopologyEdgeType[] = [
  'depends_on',
  'hosted_on',
  'connects_to',
  'processes_data_of',
];

export const EDGE_LABELS: Record<TopologyEdgeType, string> = {
  depends_on: 'dépend de',
  hosted_on: 'hébergé sur',
  connects_to: 'se connecte à',
  processes_data_of: 'traite les données de',
};

/**
 * Edge styling. Colour is NOT used to distinguish edge types: the criticality
 * and exposure palettes already own colour in this view, and a third colour
 * dimension on the lines would collide with both. Dash pattern carries the type.
 */
export const EDGE_DASH: Record<TopologyEdgeType, number[]> = {
  depends_on: [],
  hosted_on: [6, 3],
  connects_to: [2, 3],
  processes_data_of: [9, 3, 2, 3],
};

/** How the graph is coloured. */
export type ColorMode = 'criticality' | 'exposure';
