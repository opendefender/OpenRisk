// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Keeps a post-login target on this site. Same rule as the backend's
 * sanitiseReturnTo, which is the real gate; this copy only means a hand-edited
 * URL cannot send the user elsewhere either.
 */
export function safeNextPath(raw: string | null): string | null {
  if (!raw || !raw.startsWith('/') || raw.startsWith('//')) return null;
  // Browsers strip tab and newline from URLs, so "/\t/evil.com" would become
  // "//evil.com". Refuse control characters and backslashes anywhere.
  for (const ch of raw) {
    const code = ch.charCodeAt(0);
    if (code < 0x20 || code === 0x7f || ch === '\\') return null;
  }
  // Last word goes to the URL parser itself: whatever it resolves to must stay
  // on this origin.
  try {
    if (new URL(raw, window.location.origin).origin !== window.location.origin) return null;
  } catch {
    return null;
  }
  return raw;
}
