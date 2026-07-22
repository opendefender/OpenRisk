// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import type { ComplianceControl, ComplianceProgress, ControlStatus } from '../../types/compliance';

// Mirrors backend/internal/application/compliance/get_compliance_progress.go's
// GetComplianceProgressUseCase.Execute — keep both in sync if the formula
// changes. Computed client-side here (rather than calling
// GET /compliance/frameworks/{id}/progress) because the control list is
// already loaded on the page that needs this, and computing it locally
// keeps the gauge in sync with in-flight optimistic status updates
// without a redundant round-trip.
export function computeComplianceProgress(frameworkId: string, controls: ComplianceControl[]): ComplianceProgress {
  const byStatus: Record<string, number> = {};
  for (const control of controls) {
    const status = control.status ?? 'not_implemented';
    byStatus[status] = (byStatus[status] ?? 0) + 1;
  }

  const total = controls.length;
  const notApplicable = byStatus['not_applicable' satisfies ControlStatus] ?? 0;
  const implemented = byStatus['implemented' satisfies ControlStatus] ?? 0;
  const applicable = total - notApplicable;
  const percentComplete = applicable > 0 ? (implemented / applicable) * 100 : 0;

  return {
    framework_id: frameworkId,
    total,
    by_status: byStatus,
    applicable,
    percent_complete: percentComplete,
  };
}
