// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, expect, it } from 'vitest';

import {
  QUOTES,
  authCopy,
  dailyQuoteIndex,
  providerLabel,
  quoteAt,
  strengthLabel,
  type AuthCopy,
  type OAuthErrorCode,
} from '../authStrings';

describe('auth quotes', () => {
  it('ships at least 8 quotes per language', () => {
    // The spec's floor. Fewer and the rotation repeats within a working week.
    expect(QUOTES.length).toBeGreaterThanOrEqual(8);
  });

  it('has both renderings and an attribution for every quote', () => {
    // A quote with a missing rendering shows blank to half the users; one without
    // an author is an unattributed claim on a security product's front door.
    for (const q of QUOTES) {
      expect(q.fr.trim(), `fr missing for "${q.en}"`).not.toBe('');
      expect(q.en.trim(), `en missing for "${q.fr}"`).not.toBe('');
      expect(q.author.trim()).not.toBe('');
    }
  });

  it('has no duplicate quotes', () => {
    const seen = new Set(QUOTES.map((q) => q.en));
    expect(seen.size).toBe(QUOTES.length);
  });

  describe('daily rotation', () => {
    it('is deterministic for a given day', () => {
      // Everyone signing in on the same day opens on the same quote, and a
      // reload does not reshuffle it — the reason this is not Math.random().
      const day = new Date(2026, 7, 7, 9, 30);
      const sameDayLater = new Date(2026, 7, 7, 23, 59);

      expect(dailyQuoteIndex(day)).toBe(dailyQuoteIndex(sameDayLater));
    });

    it('advances by one each day', () => {
      const d1 = new Date(2026, 7, 7);
      const d2 = new Date(2026, 7, 8);

      const expected = (dailyQuoteIndex(d1) + 1) % QUOTES.length;
      expect(dailyQuoteIndex(d2)).toBe(expected);
    });

    it('stays in range and wraps across a full cycle', () => {
      for (let i = 0; i < QUOTES.length * 3; i++) {
        const d = new Date(2026, 0, 1 + i);
        const idx = dailyQuoteIndex(d);
        expect(idx).toBeGreaterThanOrEqual(0);
        expect(idx).toBeLessThan(QUOTES.length);
      }
    });

    it('never indexes negatively for dates before the epoch', () => {
      // JS % keeps the sign of the dividend, so a 1969 date would index -3
      // without the correction and throw on the array read.
      const idx = dailyQuoteIndex(new Date(1969, 5, 20));
      expect(idx).toBeGreaterThanOrEqual(0);
      expect(idx).toBeLessThan(QUOTES.length);
    });

    it('quoteAt walks forward from the day’s quote and wraps', () => {
      const day = new Date(2026, 7, 7);
      const start = dailyQuoteIndex(day);

      expect(quoteAt(0, day)).toBe(QUOTES[start]);
      expect(quoteAt(1, day)).toBe(QUOTES[(start + 1) % QUOTES.length]);
      // A full lap returns to the start rather than running off the end.
      expect(quoteAt(QUOTES.length, day)).toBe(QUOTES[start]);
    });
  });
});

describe('auth copy', () => {
  const fr = authCopy('fr');
  const en = authCopy('en');

  it('defines every key in both languages', () => {
    // Guards the usual half-translated UI: a key present in one bundle and
    // missing from the other renders as `undefined` in production.
    const frKeys = Object.keys(fr).sort();
    const enKeys = Object.keys(en).sort();
    expect(frKeys).toEqual(enKeys);
  });

  it('has no empty strings', () => {
    for (const [key, value] of Object.entries(fr) as [keyof AuthCopy, unknown][]) {
      if (typeof value === 'string') {
        expect(value.trim(), `fr.${String(key)} is empty`).not.toBe('');
      }
    }
    for (const [key, value] of Object.entries(en) as [keyof AuthCopy, unknown][]) {
      if (typeof value === 'string') {
        expect(value.trim(), `en.${String(key)} is empty`).not.toBe('');
      }
    }
  });

  it('covers every OAuth error code the backend can redirect with', () => {
    // These strings are what stands between a failed OAuth round trip and a
    // blank page. A code without copy renders as undefined.
    const codes: OAuthErrorCode[] = [
      'access_denied',
      'consent_required',
      'provider_error',
      'state_missing',
      'state_invalid',
      'code_missing',
      'exchange_failed',
      'userinfo_failed',
      'unsupported_provider',
      'provider_not_configured',
      'email_unverified',
      'no_email',
      'account_disabled',
      'no_account',
      'provider_conflict',
      'internal',
    ];

    for (const code of codes) {
      expect(fr.oauth[code], `fr.oauth.${code}`).toBeTruthy();
      expect(en.oauth[code], `en.oauth.${code}`).toBeTruthy();
    }
  });

  it('names the owning provider in the conflict message', () => {
    // The whole point of the conflict branch: telling someone WHERE to sign in
    // instead of just refusing them.
    expect(fr.oauthConflictWith('Google')).toContain('Google');
    expect(en.oauthConflictWith('Google')).toContain('Google');
  });

  it('falls back to French for an unknown language', () => {
    expect(authCopy('de' as never)).toBe(fr);
  });

  it('phrases the reset acknowledgement conditionally', () => {
    // It must be truthful whether or not the address has an account — the UI
    // half of the backend's anti-enumeration contract. "We sent you an email"
    // would be a lie half the time.
    expect(fr.forgotSentBody.toLowerCase()).toContain('si un compte existe');
    expect(en.forgotSentBody.toLowerCase()).toContain('if an account exists');
  });
});

describe('strengthLabel', () => {
  it('maps every zxcvbn score to a label', () => {
    const copy = authCopy('en');
    expect(strengthLabel(copy, 0)).toBe(copy.strength0);
    expect(strengthLabel(copy, 4)).toBe(copy.strength4);
  });

  it('clamps out-of-range scores instead of returning undefined', () => {
    const copy = authCopy('en');
    expect(strengthLabel(copy, -1)).toBe(copy.strength0);
    expect(strengthLabel(copy, 99)).toBe(copy.strength4);
  });
});

describe('providerLabel', () => {
  it('uses each provider’s own branding', () => {
    expect(providerLabel('google')).toBe('Google');
    expect(providerLabel('github')).toBe('GitHub');
    // "azure" is our route parameter; users know it as Microsoft.
    expect(providerLabel('azure')).toBe('Microsoft');
  });

  it('passes unknown ids through rather than blanking them', () => {
    expect(providerLabel('okta')).toBe('okta');
  });
});
