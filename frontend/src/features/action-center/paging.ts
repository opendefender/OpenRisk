// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Turning a URL's ?page= into an offset the server will accept.

/**
 * The 1-based page number from the URL.
 *
 * Anything that is not a positive integer collapses to page 1 rather than
 * reaching the API: `?page=-4` would otherwise become a negative offset, which
 * the server rejects with a 400 — turning a mangled or truncated URL into an
 * error screen instead of the first page.
 */
export function pageFromParam(raw: string | null): number {
  const parsed = Number(raw);
  if (!Number.isInteger(parsed) || parsed < 1) return 1;
  return parsed;
}
