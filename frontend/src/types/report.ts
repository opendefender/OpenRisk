// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

export type ReportType =
  | 'executive_summary'
  | 'compliance_framework'
  | 'board'
  | 'risk_register'
  | 'incident'
  | 'audit';

export type ReportFormat = 'pdf' | 'docx' | 'xlsx';

/** The DOCUMENT's language, chosen independently of the interface's. */
export type ReportLocale = 'fr' | 'en';

/** Whether the bytes exist yet. */
export type ReportRunState = 'queued' | 'running' | 'succeeded' | 'failed';

/** Whether a human has accepted the document. A different question. */
export type ReportLifecycle = 'draft' | 'in_review' | 'approved' | 'published';

export interface ReportComment {
  id: string;
  report_id: string;
  author_id: string;
  author_email?: string;
  body: string;
  /** The lifecycle move this comment accompanied; empty for a plain remark. */
  transition?: ReportLifecycle;
  created_at: string;
}

export interface Report {
  id: string;
  type: ReportType;
  format: ReportFormat;
  locale: ReportLocale;
  template_key: string;
  template_version: string;
  title: string;
  run_state: ReportRunState;
  progress: number;
  step: string;
  error?: string;
  lifecycle: ReportLifecycle;
  filename?: string;
  content_type?: string;
  size_bytes?: number;
  /** SHA-256 of the bytes served: what verification recomputes. */
  content_hash?: string;
  /**
   * SHA-256 of what the report SAYS, and the value printed on the document.
   * Necessarily a different number from content_hash — a file cannot carry the
   * hash of itself — and stable across formats, so two people holding a PDF and
   * a spreadsheet can tell they have the same report.
   */
  content_fingerprint?: string;
  version: number;
  supersedes?: string;
  requested_by: string;
  requested_by_email?: string;
  approved_by?: string;
  approved_by_email?: string;
  approved_at?: string;
  published_at?: string;
  created_at: string;
  completed_at?: string;
  comments?: ReportComment[];
}

export interface ReportFormatOption {
  key: ReportFormat;
  label: string;
}

/** What extra question the configurator must ask for this type. */
export type ReportScope = 'framework' | 'audit' | 'period' | 'none';

export interface ReportTypeOption {
  type: ReportType;
  template_key: string;
  template_version: string;
  title: string;
  description: string;
  formats: ReportFormatOption[];
  scope: ReportScope;
}

export interface ReportCatalogue {
  types: ReportTypeOption[];
  locales: { key: ReportLocale; label: string }[];
}

export interface CreateReportInput {
  type: ReportType;
  format: ReportFormat;
  locale: ReportLocale;
  from?: string;
  to?: string;
  framework_id?: string;
  audit_id?: string;
  recipients?: string[];
  /** Marks this as a new version of an existing report. */
  supersedes?: string;
}

export interface ReportVerification {
  report_id: string;
  content_hash: string;
  recomputed_hash: string;
  content_fingerprint: string;
  intact: boolean;
  size_bytes: number;
  message: string;
}

export interface ReportVersionSummary {
  id: string;
  version: number;
  created_at: string;
  format: ReportFormat;
  locale: ReportLocale;
  template: string;
  lifecycle: ReportLifecycle;
  content_hash: string;
  size_bytes: number;
}

export interface ReportVersionDiff {
  from: ReportVersionSummary;
  to: ReportVersionSummary;
  changes: string[];
  same_document: boolean;
}
