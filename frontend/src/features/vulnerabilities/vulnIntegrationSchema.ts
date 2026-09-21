// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Client-side rules for the integration and ticketing base URL (#573). They
// mirror SaveIntegration / SaveTicketing; the server remains the authority and
// also refuses private, loopback and link-local addresses.

import { z } from 'zod';

type Tr = (fr: string, en: string) => string;

function parse(raw: string): URL | null {
  try {
    return new URL(raw);
  } catch {
    return null;
  }
}

/**
 * The base URL: empty (vendor default), or a full https:// address. With
 * `gmpHost` (OpenVAS) a bare `host:port` is expected instead of a URL.
 */
export function baseUrlSchema(tr: Tr, opts: { gmpHost?: boolean } = {}) {
  return z
    .string()
    .trim()
    .max(512, tr('512 caractères au plus.', 'At most 512 characters.'))
    .refine(
      (v) => {
        if (v === '') return true;
        const u = parse(opts.gmpHost && !v.includes('://') ? `https://${v}` : v);
        return u !== null && u.protocol === 'https:' && u.hostname !== '' && u.username === '';
      },
      opts.gmpHost
        ? tr(
            'Hôte attendu, ex. gvm.example.com:9390.',
            'A host is expected, e.g. gvm.example.com:9390.',
          )
        : tr('Adresse complète en https:// attendue.', 'A full https:// address is expected.'),
    );
}

/** scheme://host:port a URL sends requests to ('' when unset). */
function origin(raw: string): string {
  const v = raw.trim();
  if (v === '') return '';
  const u = parse(v.includes('://') ? v : `https://${v}`);
  return u ? `${u.protocol}//${u.host}`.toLowerCase() : v;
}

/**
 * Stored credentials only go to the host they were entered for: moving the
 * base URL to another host requires typing them again (server rule, #573).
 */
export function credentialsMustBeReentered(
  savedUrl: string,
  nextUrl: string,
  hasSavedCredentials: boolean,
  enteredCredentials: boolean,
): boolean {
  return hasSavedCredentials && !enteredCredentials && origin(savedUrl) !== origin(nextUrl);
}
