// Accessibility (UX — WCAG 2.1 AA): axe-core on 6 key screens. Always attaches the
// full violations report as an artifact. The DoD target is zero serious/critical;
// screens that violate today are quarantined in A11Y_KNOWN (test.fixme with a bug
// id) so the gate stays green while the violations stay named in the audit.

import { test, expect } from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';
import { authFileFor } from './support/env';

test.use({ storageState: authFileFor('admin') });

const SCREENS: { path: string; name: string }[] = [
  { path: '/', name: 'Dashboard' },
  { path: '/risks', name: 'Risk register' },
  { path: '/compliance', name: 'Compliance' },
  { path: '/vulnerabilities', name: 'Vulnerabilities' },
  { path: '/analytics', name: 'Executive dashboard' },
  { path: '/settings', name: 'Settings' },
];

// path -> OR-BUG id for screens with unresolved serious/critical violations.
// Empty: OR-BUG-011 (contrast) and OR-BUG-012 (labels/button-name) were fixed and
// the 6 key screens now pass axe WCAG 2.1 AA — the gate enforces it going forward.
const A11Y_KNOWN: Record<string, string> = {};

for (const screen of SCREENS) {
  test(`a11y: ${screen.name} (${screen.path})`, async ({ page }, info) => {
    test.fixme(screen.path in A11Y_KNOWN, A11Y_KNOWN[screen.path]);
    await page.goto(screen.path, { waitUntil: 'domcontentloaded' });
    await page.waitForLoadState('networkidle', { timeout: 15_000 }).catch(() => {});

    const results = await new AxeBuilder({ page })
      .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
      .analyze();

    await info.attach(`axe-${screen.name}.json`, {
      body: JSON.stringify(results.violations, null, 2),
      contentType: 'application/json',
    });

    const blocking = results.violations.filter((v) => v.impact === 'serious' || v.impact === 'critical');
    info.annotations.push({
      type: 'a11y-summary',
      description: `${screen.path} :: ${results.violations.length} total, ${blocking.length} serious/critical [${blocking.map((v) => v.id).join(', ')}]`,
    });

    expect(
      blocking,
      `serious/critical WCAG 2.1 AA violations on ${screen.path}:\n${blocking.map((v) => `${v.id}: ${v.help}`).join('\n')}`,
    ).toEqual([]);
  });
}
