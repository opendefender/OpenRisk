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
// Hand-written to match backend/internal/application/compliance/gap_analysis.go.
// Not yet in the generated OpenAPI types (follow-up: add the GapAnalysis schema
// to docs/openapi.yaml and regenerate). Fully typed — no `any`.
export interface GapControl {
  control_id: string;
  framework_id: string;
  framework_name: string;
  reference_code: string;
  name: string;
  description: string;
  status: ControlStatus;
  source_reference: string;
  evidence_count: number;
}

export interface FrameworkGapSummary {
  framework_id: string;
  framework_name: string;
  version: string;
  total: number;
  implemented: number;
  in_progress: number;
  not_implemented: number;
  not_applicable: number;
  gaps: number;
  percent_complete: number;
}

export interface GapAnalysis {
  total_controls: number;
  total_gaps: number;
  frameworks: FrameworkGapSummary[];
  gaps: GapControl[];
}

// --- Audits ("Audits") -------------------------------------------------------
// Matches backend/internal/domain/compliance_audit.go. Hand-written; follow-up:
// add to docs/openapi.yaml.
export type AuditType = 'internal' | 'external' | 'certification' | 'surveillance';
export type AuditStatus = 'planned' | 'in_progress' | 'completed' | 'cancelled';

export interface ComplianceAudit {
  id: string;
  tenant_id: string;
  title: string;
  framework_id: string | null;
  type: AuditType;
  status: AuditStatus;
  auditor: string;
  scope: string;
  summary: string;
  compliance_score: number;
  scheduled_start: string | null;
  scheduled_end: string | null;
  completed_at: string | null;
  created_by: string | null;
  created_at: string;
  updated_at: string;
}

export interface CreateAuditInput {
  title: string;
  framework_id?: string;
  type?: AuditType;
  auditor?: string;
  scope?: string;
  scheduled_start?: string;
  scheduled_end?: string;
}

export interface UpdateAuditInput {
  title?: string;
  framework_id?: string;
  type?: AuditType;
  status?: AuditStatus;
  auditor?: string;
  scope?: string;
  summary?: string;
  compliance_score?: number;
  scheduled_start?: string;
  scheduled_end?: string;
}

// --- Remediation plans ("Plans de remédiation") ------------------------------
export type RemediationPriority = 'low' | 'medium' | 'high' | 'critical';
export type RemediationStatus = 'open' | 'in_progress' | 'completed' | 'cancelled';

export interface RemediationPlan {
  id: string;
  tenant_id: string;
  title: string;
  description: string;
  control_id: string | null;
  framework_id: string | null;
  audit_id: string | null;
  priority: RemediationPriority;
  status: RemediationStatus;
  assigned_to: string | null;
  due_date: string | null;
  completed_at: string | null;
  created_by: string | null;
  created_at: string;
  updated_at: string;
  control_code?: string;
  control_name?: string;
}

export interface CreateRemediationInput {
  title: string;
  description?: string;
  control_id?: string;
  audit_id?: string;
  priority?: RemediationPriority;
  assigned_to?: string;
  due_date?: string;
}

export interface UpdateRemediationInput {
  title?: string;
  description?: string;
  priority?: RemediationPriority;
  status?: RemediationStatus;
  assigned_to?: string;
  due_date?: string;
}

export interface RemediationFilter {
  control_id?: string;
  framework_id?: string;
  audit_id?: string;
  status?: RemediationStatus;
}
