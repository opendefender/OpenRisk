// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

import { AxiosError, type AxiosResponse } from 'axios';
import { describe, expect, it } from 'vitest';
import enJson from '../../locales/en.json';
import frJson from '../../locales/fr.json';
import { translate, type Catalog } from '../../i18n/translate';
import { apiErrorCode, downloadErrorMessage } from '../apiError';

const catalogs = { en: enJson as Catalog, fr: frJson as Catalog };
const tFor = (locale: 'fr' | 'en') => (key: string) => translate(catalogs, locale, key);
const FALLBACK = 'Could not download';

function blobError(body: string, status = 409): AxiosError {
  const response = { data: new Blob([body]), status } as AxiosResponse;
  return new AxiosError('failed', 'ERR_BAD_REQUEST', undefined, undefined, response);
}

describe('download error of a blob response', () => {
  const body = JSON.stringify({
    error: 'English server text',
    code: 'evidence_integrity_mismatch',
  });

  it('reads the code out of a Blob body', async () => {
    expect(await apiErrorCode(blobError(body))).toBe('evidence_integrity_mismatch');
  });

  it('shows the French message when the language is fr', async () => {
    const msg = await downloadErrorMessage(blobError(body), tFor('fr'), FALLBACK);
    expect(msg).toBe(frJson.errors.evidence_integrity_mismatch);
    expect(msg).not.toContain('English server text');
  });

  it('shows the English message when the language is en', async () => {
    const msg = await downloadErrorMessage(blobError(body), tFor('en'), FALLBACK);
    expect(msg).toBe(enJson.errors.evidence_integrity_mismatch);
  });

  it('falls back to the generic message for a non-JSON Blob', async () => {
    expect(
      await downloadErrorMessage(blobError('<html>boom</html>', 500), tFor('en'), FALLBACK),
    ).toBe(FALLBACK);
  });

  it('falls back for an unknown code and for a non-axios error', async () => {
    const other = JSON.stringify({ code: 'something_else' });
    expect(await downloadErrorMessage(blobError(other), tFor('en'), FALLBACK)).toBe(FALLBACK);
    expect(await downloadErrorMessage(new Error('x'), tFor('en'), FALLBACK)).toBe(FALLBACK);
  });
});
