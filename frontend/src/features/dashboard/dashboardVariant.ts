// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Which of the redesign's four dashboard views a member gets (#901). The
// design lets the viewer switch between RSSI, risk manager, auditor and
// executive; the owner chose to drive it from the account instead, so the
// business role decides and there is no switch on screen.
//
// The four views share one layout. Only the counters on the right of the first
// row and the priority list at the bottom change.

export type DashboardVariant = 'rssi' | 'rm' | 'audit' | 'exec';

const ROLE_VARIANT: Record<string, DashboardVariant> = {
  rssi: 'rssi',
  dsi: 'rssi',
  security_analyst: 'rssi',
  viewer: 'rssi',
  risk_manager: 'rm',
  risk_owner: 'rm',
  asset_owner: 'rm',
  auditor: 'audit',
  compliance_officer: 'audit',
  internal_control: 'audit',
  executive: 'exec',
};

/** The view for a business role. Admins and unmapped roles get the RSSI view. */
export function variantFor(businessRole?: string | null): DashboardVariant {
  if (businessRole && ROLE_VARIANT[businessRole]) return ROLE_VARIANT[businessRole];
  return 'rssi';
}
