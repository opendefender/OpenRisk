// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Keeps a post-login target on this site. Same rule as the backend's
 * sanitiseReturnTo, which is the real gate; this copy only means a hand-edited
 * URL cannot send the user elsewhere either.
 */
export function safeNextPath(raw: string | null): string | null {
  if (!raw || !raw.startsWith('/')) return null;
  if (raw.startsWith('//') || raw.startsWith('/\\')) return null;
  return raw;
}
