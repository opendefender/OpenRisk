// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, expect, it } from 'vitest';

import { baseUrlSchema, credentialsMustBeReentered } from '../vulnIntegrationSchema';

const tr = (_fr: string, en: string) => en;

describe('baseUrlSchema', () => {
  const url = baseUrlSchema(tr);

  it('accepts empty (vendor default) and full https addresses', () => {
    for (const v of ['', '  ', 'https://cloud.tenable.com', 'https://acme.atlassian.net/']) {
      expect(url.safeParse(v).success).toBe(true);
    }
  });

  it('refuses http, bare hosts and embedded credentials', () => {
    for (const v of ['http://cloud.tenable.com', 'cloud.tenable.com', 'https://u:p@host.example']) {
      expect(url.safeParse(v).success).toBe(false);
    }
  });

  it('takes a bare host:port for OpenVAS GMP', () => {
    const gmp = baseUrlSchema(tr, { gmpHost: true });
    expect(gmp.safeParse('gvm.example.com:9390').success).toBe(true);
    expect(gmp.safeParse('http://gvm.example.com:9390').success).toBe(false);
  });
});

describe('credentialsMustBeReentered', () => {
  it('asks for credentials only when saved ones would move to another host', () => {
    const a = 'https://cloud.tenable.com';
    expect(credentialsMustBeReentered(a, 'https://attacker.example', true, false)).toBe(true);
    expect(credentialsMustBeReentered(a, 'https://CLOUD.tenable.com/path', true, false)).toBe(
      false,
    );
    expect(credentialsMustBeReentered(a, 'https://attacker.example', true, true)).toBe(false);
    expect(credentialsMustBeReentered(a, 'https://attacker.example', false, false)).toBe(false);
    expect(credentialsMustBeReentered('', 'https://attacker.example', true, false)).toBe(true);
  });
});
