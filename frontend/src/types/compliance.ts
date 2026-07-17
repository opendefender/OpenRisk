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

export type ComplianceCatalogSummary = components['schemas']['ComplianceCatalogSummary'];
export type ImportCatalogInput = components['schemas']['ImportCatalogInput'];
export type ImportCatalogResult = components['schemas']['ImportCatalogResult'];

export const CONTROL_STATUSES: ControlStatus[] = [
  'not_implemented',
  'in_progress',
  'implemented',
  'not_applicable',
];

// --- Gap analysis ("analyse d'écarts") --------------------------------------
// Contract-first aliases (see the header note) — the GapAnalysis/Audit/
// Remediation/ControlMapping schemas now live in docs/openapi.yaml and are
// regenerated into openapi.generated.ts. Enum aliases are derived from the
// generated object schemas so there is a single source of truth.
export type GapControl = components['schemas']['GapControl'];
export type FrameworkGapSummary = components['schemas']['FrameworkGapSummary'];
export type GapAnalysis = components['schemas']['GapAnalysis'];

// --- Audits ("Audits") -------------------------------------------------------
export type ComplianceAudit = components['schemas']['ComplianceAudit'];
export type CreateAuditInput = components['schemas']['CreateAuditInput'];
export type UpdateAuditInput = components['schemas']['UpdateAuditInput'];
export type AuditType = NonNullable<ComplianceAudit['type']>;
export type AuditStatus = NonNullable<ComplianceAudit['status']>;

// --- Remediation plans ("Plans de remédiation") ------------------------------
export type RemediationPlan = components['schemas']['RemediationPlan'];
export type CreateRemediationInput = components['schemas']['CreateRemediationInput'];
export type UpdateRemediationInput = components['schemas']['UpdateRemediationInput'];
export type RemediationPriority = NonNullable<RemediationPlan['priority']>;
export type RemediationStatus = NonNullable<RemediationPlan['status']>;

// Query filter — not a request/response body, so it stays hand-written.
export interface RemediationFilter {
  control_id?: string;
  framework_id?: string;
  audit_id?: string;
  status?: RemediationStatus;
}

// --- Cross-framework control mappings ---------------------------------------
export type ControlMapping = components['schemas']['ControlMapping'];
export type CreateControlMappingInput = components['schemas']['CreateControlMappingInput'];
export type MappingRelation = NonNullable<ControlMapping['relation']>;
