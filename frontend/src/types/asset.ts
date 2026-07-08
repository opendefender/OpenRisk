// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

// Contract-first: aliases onto types generated from docs/openapi.yaml (see
// `npm run generate:api-types`), not hand-written duplicates.
import type { components } from './openapi.generated';

export type AssetCriticality = NonNullable<components['schemas']['Asset']['criticality']>;

export type Asset = components['schemas']['Asset'];
export type AssetSnapshot = components['schemas']['AssetSnapshot'];
export type CreateAssetInput = components['schemas']['CreateAssetInput'];
export type UpdateAssetInput = components['schemas']['UpdateAssetInput'];

export const ASSET_CRITICALITIES: AssetCriticality[] = ['LOW', 'MEDIUM', 'HIGH', 'CRITICAL'];
export const ASSET_TYPES = ['Server', 'Laptop', 'Database', 'SaaS', 'Network', 'Storage'] as const;
