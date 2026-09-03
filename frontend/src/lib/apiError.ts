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
