// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Role → dashboard persona mapping (UX-2). The dashboard adapts what it leads with
// to the member's GRC job, so each profession opens on what it actually needs:
//   RSSI / risk owner  → risk posture & exposure
//   security analyst    → vulnerabilities
//   auditor / compliance→ controls coverage
//   executive           → cost & KPIs (no technical detail)
//   DSI / asset owner   → the estate
//   viewer              → a light read-only overview
// Admins and unknown roles fall back to the full posture view.

export type PersonaKey = 'posture' | 'analyst' | 'audit' | 'exec' | 'estate' | 'viewer';

const ROLE_PERSONA: Record<string, PersonaKey> = {
  rssi: 'posture',
  risk_manager: 'posture',
  risk_owner: 'posture',
  security_analyst: 'analyst',
  auditor: 'audit',
  compliance_officer: 'audit',
  internal_control: 'audit',
  executive: 'exec',
  dsi: 'estate',
  asset_owner: 'estate',
  viewer: 'viewer',
};

/** Resolve the dashboard persona for a business role ('posture' by default). */
export function personaFor(businessRole?: string): PersonaKey {
  if (businessRole && ROLE_PERSONA[businessRole]) return ROLE_PERSONA[businessRole];
  return 'posture';
}
