// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

// Hand-written types mirroring the backend's board-report JSON (domain.BoardReport
// + application/board snapshots). Kept in sync with backend/internal/domain/board_report.go
// and documented in docs/openapi.yaml. Matches the Risk/Mitigation modules, which
// are not yet on the generated OpenAPI client.

export type BoardReportStatus = 'draft' | 'approved';
export type BoardLocale = 'fr' | 'en';

export interface FrameworkSnapshot {
  name: string;
  version: string;
  total: number;
  applicable: number;
  implemented: number;
  percent_complete: number;
}

export interface BoardReport {
  id: string;
  tenant_id: string;
  title: string;
  organization_name: string;
  period_label: string;
  locale: BoardLocale;
  status: BoardReportStatus;

  risks_critical: number;
  risks_high: number;
  risks_medium: number;
  risks_low: number;
  risks_total: number;
  financial_exposure_fcfa: number;
  overall_compliance_percent: number;
  frameworks_snapshot: FrameworkSnapshot[] | null;

  executive_summary: string;
  risk_commentary: string;
  compliance_commentary: string;
  financial_commentary: string;
  recommendations: string[];

  generated_by_model: string;
  created_by: string;
  approved_by: string | null;
  approved_at: string | null;
  created_at: string;
  updated_at: string;
}

export interface GenerateBoardReportInput {
  period_label?: string;
  locale?: BoardLocale;
}

export interface UpdateBoardReportInput {
  title?: string;
  executive_summary?: string;
  risk_commentary?: string;
  compliance_commentary?: string;
  financial_commentary?: string;
  recommendations?: string[];
}
