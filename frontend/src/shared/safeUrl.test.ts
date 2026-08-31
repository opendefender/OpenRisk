/* Copyright (c) 2026 OpenDefender Contributors
   SPDX-License-Identifier: AGPL-3.0-only */

import { describe, it, expect } from 'vitest';
import { safeExternalUrl } from './safeUrl';

describe('safeExternalUrl', () => {
  it('allows http and https', () => {
    expect(safeExternalUrl('https://jira.example.com/browse/SEC-42')).toBe(
      'https://jira.example.com/browse/SEC-42',
    );
    expect(safeExternalUrl('http://internal.local/incident/7')).toBe('http://internal.local/incident/7');
  });

  it('rejects the schemes React does not block', () => {
    // React 19 neutralises javascript: itself, but not these — verified against
    // react-dom/server, which is why the allowlist exists.
    expect(safeExternalUrl('data:text/html,<script>alert(1)</script>')).toBeUndefined();
    expect(safeExternalUrl('vbscript:msgbox(1)')).toBeUndefined();
  });

  it('rejects javascript: in every disguise', () => {
    expect(safeExternalUrl('javascript:alert(1)')).toBeUndefined();
    expect(safeExternalUrl('JaVaScRiPt:alert(1)')).toBeUndefined();
    expect(safeExternalUrl('  javascript:alert(1)')).toBeUndefined();
  });

  it('rejects empty and malformed input', () => {
    expect(safeExternalUrl(null)).toBeUndefined();
    expect(safeExternalUrl(undefined)).toBeUndefined();
    expect(safeExternalUrl('')).toBeUndefined();
    expect(safeExternalUrl('   ')).toBeUndefined();
    expect(safeExternalUrl('not a url')).toBeUndefined();
  });

  it('keeps same-origin relative paths', () => {
    expect(safeExternalUrl('/agents/openrisk-agent-linux')).toBe('/agents/openrisk-agent-linux');
  });

  it('rejects protocol-relative URLs, which inherit the scheme and can be hijacked', () => {
    expect(safeExternalUrl('//evil.example.com/payload')).toBeUndefined();
  });
});
