// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Contract-first: aliases onto types generated from docs/openapi.yaml (see
// `npm run generate:api-types`), not hand-written duplicates.
import type { components } from './openapi.generated';

export type AssetCriticality = NonNullable<components['schemas']['Asset']['criticality']>;

export type Asset = components['schemas']['Asset'];
export type AssetSnapshot = components['schemas']['AssetSnapshot'];
export type CreateAssetInput = components['schemas']['CreateAssetInput'];
export type UpdateAssetInput = components['schemas']['UpdateAssetInput'];

export type AssetDependency = components['schemas']['AssetDependency'];
export type CreateAssetDependencyInput = components['schemas']['CreateAssetDependencyInput'];
export type DependencyType = NonNullable<AssetDependency['type']>;

export const ASSET_CRITICALITIES: AssetCriticality[] = ['LOW', 'MEDIUM', 'HIGH', 'CRITICAL'];

// Canonical inventory taxonomy — covers the categories a GRC inventory must
// classify: servers, applications, cloud, data, users, suppliers (plus the
// finer-grained legacy types). Scanner-imported assets may carry other free
// strings; the UI falls back to a generic icon for those.
export const ASSET_TYPES = [
  'Server',
  'Application',
  'Cloud',
  'Database',
  'SaaS',
  'Storage',
  'Network',
  'Laptop',
  'Data',
  'User',
  'Supplier',
] as const;

// Relationship vocabulary for the dependency cartography. Order = display order.
export const DEPENDENCY_TYPES: DependencyType[] = [
  'depends_on',
  'runs_on',
  'connects_to',
  'hosted_by',
  'stores_data_in',
  'authenticates_via',
  'backs_up_to',
  'managed_by',
];
