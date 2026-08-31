/* Copyright (c) 2026 OpenDefender Contributors
   SPDX-License-Identifier: AGPL-3.0-only */

/**
 * Allowlist for URLs that reach an `href`.
 *
 * React 19 already neutralises `javascript:` in href — verified, including case
 * variants and leading whitespace — but it does NOT block `data:` or
 * `vbscript:`. Browsers refuse top-level navigation to `data:` and `vbscript:`
 * is long dead, so the residual risk is small; it is not zero, and the values
 * concerned come from places a tenant can influence (an ITSM ticket URL built
 * from tenant-configured integration data, a download path from an API
 * response). An allowlist costs nothing and does not depend on the framework
 * continuing to protect us.
 *
 * Returns `undefined` for anything that is not http(s), so callers render a
 * non-navigable element rather than a dangerous one.
 */
export function safeExternalUrl(raw: string | null | undefined): string | undefined {
  if (!raw) return undefined;

  const trimmed = raw.trim();
  if (!trimmed) return undefined;

  // Relative URLs stay relative: they cannot carry a scheme, so they are safe
  // and resolving them here would need a base we do not always have.
  if (trimmed.startsWith('/') && !trimmed.startsWith('//')) return trimmed;

  let parsed: URL;
  try {
    parsed = new URL(trimmed);
  } catch {
    return undefined;
  }

  return parsed.protocol === 'http:' || parsed.protocol === 'https:' ? parsed.href : undefined;
}
