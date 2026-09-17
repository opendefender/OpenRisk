// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Which organisation name the tunnel may show as a starting value (#716).
//
// Sign-up asks for no company, so the account is created under the person's own
// name — a placeholder, not an answer. Pre-filling the organisation step with it
// meant most people pressed Continue and kept it, and the danger zone then asked
// them to type "Alex Dembele — espace" to confirm a deletion. A placeholder is
// never shown as if it were the organisation's name; a real one still is.

// Organisations created before #716 carry one of these; they are not migrated,
// so the tunnel has to recognise them too.
const LEGACY_SUFFIXES = [' — espace', ' — workspace'];

function norm(s: string): string {
  return s.trim().replace(/\s+/g, ' ').toLocaleLowerCase();
}

/** True when `orgName` is the name sign-up invented, not one anybody chose. */
export function isProvisionalOrgName(orgName: string, fullName: string | undefined): boolean {
  const name = norm(orgName);
  if (!name) return true;
  if (LEGACY_SUFFIXES.some((s) => name.endsWith(norm(s)))) return true;
  return fullName !== undefined && norm(fullName) !== '' && name === norm(fullName);
}

/** The value the organisation field opens on when nothing was stored yet. */
export function initialOrgName(orgName: string | undefined, fullName: string | undefined): string {
  if (!orgName || isProvisionalOrgName(orgName, fullName)) return '';
  return orgName;
}
