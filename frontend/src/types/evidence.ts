// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// The evidence library: one proof artifact answering N controls, with a
// collection date, an expiry and a review verdict.

export type EvidenceType = 'document' | 'capture' | 'configuration' | 'attestation' | 'log';
export type EvidenceReview = 'pending' | 'accepted' | 'rejected';
export type EvidenceSource = 'manual' | 'integration' | 'scanner' | 'automation';

/**
 * The state an auditor reads. DERIVED by the server from the expiry date and the
 * review verdict — never sent by the client, and never stored, so it cannot go
 * stale between a worker run and a page load.
 */
export type EvidenceStatus = 'valid' | 'expiring_soon' | 'expired' | 'rejected' | 'pending';

export interface EvidenceControlRef {
  control_id: string;
  reference_code: string;
  name: string;
  framework_id: string;
  framework_name: string;
}

export interface Evidence {
  id: string;
  tenant_id: string;
  title: string;
  type: EvidenceType;
  description: string;
  file_ref: string;
  filename: string;
  external_url: string;
  /** When the proof was TAKEN, which is not when the row was written. */
  collected_at: string;
  valid_until: string | null;
  collected_by: string | null;
  collected_by_email?: string;
  review: EvidenceReview;
  review_note: string;
  reviewed_by: string | null;
  reviewed_at: string | null;
  source: EvidenceSource;
  source_detail: string;
  status: EvidenceStatus;
  /** Negative once expired; absent when the artifact never expires. */
  days_until_expiry?: number;
  control_ids: string[];
  controls?: EvidenceControlRef[];
  created_at: string;
  updated_at: string;
}

/** Counts by status across the WHOLE filtered set, not just the page. */
export interface EvidenceStatusSummary {
  valid: number;
  expiring_soon: number;
  expired: number;
  rejected: number;
  pending: number;
}

export interface EvidenceListResult {
  items: Evidence[];
  total: number;
  summary: EvidenceStatusSummary;
}

export interface EvidenceFilter {
  type?: EvidenceType;
  review?: EvidenceReview;
  control_id?: string;
  framework_id?: string;
  q?: string;
  limit?: number;
  offset?: number;
}

export interface CreateEvidenceInput {
  title: string;
  type?: EvidenceType;
  description?: string;
  external_url?: string;
  collected_at?: string;
  valid_until?: string;
  control_ids?: string[];
  /** When present the artifact is uploaded as multipart. */
  file?: File;
}

export interface UpdateEvidenceInput {
  title?: string;
  type?: EvidenceType;
  description?: string;
  external_url?: string;
  collected_at?: string;
  /** An empty string clears the expiry; omitting the key leaves it alone. */
  valid_until?: string;
}

/**
 * Why a control lacks proof. "Never evidenced" and "the proof went stale" are
 * different jobs — the second is usually smaller, and is invisible in tools that
 * only count attachments.
 */
export type MissingKind = 'covered' | 'no_evidence' | 'stale_evidence' | 'expiring_soon';

export interface MissingControl {
  control_id: string;
  reference_code: string;
  name: string;
  control_status: string;
  kind: MissingKind;
  /** The gap between these two numbers is the story: "4 documents, none valid". */
  total_evidence: number;
  covering_evidence: number;
  nearest_expiry?: string;
}

export interface FrameworkEvidenceCoverage {
  framework_id: string;
  framework_name: string;
  version: string;
  total_controls: number;
  covered_controls: number;
  no_evidence: number;
  stale_evidence: number;
  expiring_soon: number;
  percent_covered: number;
  missing: MissingControl[];
}
