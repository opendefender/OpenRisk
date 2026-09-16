// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The questionnaire link is `/vendor-questionnaire#<token>` (ADR 0004 D4). The
// fragment is never sent to the server when the page loads, which is why the
// token lives there and not in the path or the query string.

/** The token from `#<token>`. A malformed escape is an invalid link, not a crash. */
export function tokenFromHash(hash: string): string {
  try {
    return decodeURIComponent(hash.replace(/^#/, '')).trim();
  } catch {
    return '';
  }
}
