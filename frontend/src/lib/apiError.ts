// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { isAxiosError } from 'axios';

/**
 * The server's own message for a failed request, or '' when it did not send one.
 *
 * This exists because generic client-side copy ("failed to create asset") throws
 * away the only part of a validation failure worth reading. The server names the
 * offending field ("Nom d'hôte is required", "port must be at most 65535");
 * callers should show that and fall back to their generic string only when there
 * is genuinely nothing to show.
 */
export function apiErrorMessage(err: unknown): string {
  if (!isAxiosError(err)) return '';
  const data = err.response?.data as
    { error?: unknown; message?: unknown; details?: unknown } | undefined;
  if (!data) return '';
  for (const candidate of [data.message, data.error, data.details]) {
    if (typeof candidate === 'string' && candidate.trim() !== '') {
      return candidate;
    }
  }
  return '';
}

/** Stable machine code the backend sends with a refused tampered-evidence download. */
export const EVIDENCE_INTEGRITY_MISMATCH = 'evidence_integrity_mismatch';

/** Blob.text() where it exists; FileReader elsewhere (older jsdom, some WebViews). */
function blobText(blob: Blob): Promise<string> {
  if (typeof blob.text === 'function') return blob.text();
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(typeof reader.result === 'string' ? reader.result : '');
    reader.onerror = () => reject(reader.error);
    reader.readAsText(blob);
  });
}

/**
 * The machine `code` of a failed request, or '' when there is none.
 *
 * Downloads use `responseType: 'blob'`, so on an error status axios hands back
 * the JSON body as a Blob and `data.code` is undefined. This reads the Blob as
 * text and parses it; anything that is not a JSON object with a string `code`
 * yields '' so the caller falls back to its generic message.
 */
export async function apiErrorCode(err: unknown): Promise<string> {
  if (!isAxiosError(err)) return '';
  const data: unknown = err.response?.data;
  let body: unknown = data;
  if (typeof Blob !== 'undefined' && data instanceof Blob) {
    try {
      body = JSON.parse(await blobText(data));
    } catch {
      return '';
    }
  }
  if (typeof body !== 'object' || body === null) return '';
  const code = (body as { code?: unknown }).code;
  return typeof code === 'string' ? code : '';
}

/**
 * The localized message for a failed download: the integrity-refusal copy when
 * the server says so (never the server's English text), else `fallback`.
 */
export async function downloadErrorMessage(
  err: unknown,
  t: (key: string) => string,
  fallback: string,
): Promise<string> {
  const code = await apiErrorCode(err);
  return code === EVIDENCE_INTEGRITY_MISMATCH ? t(`errors.${code}`) : fallback;
}
