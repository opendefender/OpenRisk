// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

// Contract-first: these are aliases onto types generated from
// docs/openapi.yaml (see `npm run generate:api-types`), not hand-written
// duplicates. Regenerate src/types/openapi.generated.ts after changing
// the Compliance schemas in openapi.yaml, then this file needs no edits
// unless a schema name itself changes.
import type { components } from './openapi.generated';

export type ControlStatus = NonNullable<components['schemas']['ComplianceControl']['status']>;

export type ComplianceFramework = components['schemas']['ComplianceFramework'];
export type ComplianceControl = components['schemas']['ComplianceControl'];
export type ControlEvidence = components['schemas']['ControlEvidence'];
export type ComplianceProgress = components['schemas']['ComplianceProgress'];

export type CreateFrameworkInput = components['schemas']['CreateFrameworkInput'];
export type CreateControlInput = components['schemas']['CreateControlInput'];
export type UpdateControlInput = components['schemas']['UpdateControlInput'];

export const CONTROL_STATUSES: ControlStatus[] = [
  'not_implemented',
  'in_progress',
  'implemented',
  'not_applicable',
];
