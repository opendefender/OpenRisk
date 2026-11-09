// Accessibility (WCAG 2.2 AA — task §1): axe-core across the primary screens (incl.
// a 404). Always attaches the full violations report as an artifact. The gate is
// zero serious/critical; screens that violate today are quarantined in A11Y_KNOWN
// (test.fixme with a bug id) so the gate stays green while violations stay named.

import { test, expect, request as pwRequest, type Page, type TestInfo } from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';
import { authFileFor } from './support/env';
import { signUp, authed } from './support/newcomer';

test.use({ storageState: authFileFor('admin') });

const SCREENS: { path: string; name: string }[] = [
  { path: '/', name: 'Dashboard' },
  { path: '/risks', name: 'Risk register' },
  { path: '/compliance', name: 'Compliance' },
  { path: '/vulnerabilities', name: 'Vulnerabilities' },
  { path: '/analytics', name: 'Executive dashboard' },
  { path: '/analytics/financial', name: 'Financial dashboard' },
  { path: '/assets', name: 'Asset inventory' },
  { path: '/mitigations', name: 'Mitigations' },
  { path: '/incidents', name: 'Incidents' },
  { path: '/automation/rules', name: 'Automation' },
  { path: '/governance', name: 'Governance' },
  { path: '/reports', name: 'Reports' },
  { path: '/settings', name: 'Settings' },
  { path: '/settings?tab=billing', name: 'Billing' },
  // #438 criterion 12. The seeded admin has risks and controls, so this renders
  // the REVEAL rather than the error state — scanning the error screen would
  // pass while leaving the screen a customer actually sees unchecked.
  { path: '/posture', name: 'Posture reveal' },
  { path: '/this-route-does-not-exist', name: '404 page' },
];

// path -> OR-BUG id for screens with unresolved serious/critical violations.
// Empty: OR-BUG-011 (contrast) and OR-BUG-012 (labels/button-name) were fixed and
// the 6 key screens now pass axe WCAG 2.1 AA — the gate enforces it going forward.
const A11Y_KNOWN: Record<string, string> = {};

/**
 * One axe pass. Always attaches the full report; the gate is zero
 * serious/critical.
 */
async function expectNoBlockingViolations(page: Page, info: TestInfo, label: string, path: string) {
  const results = await new AxeBuilder({ page })
    // WCAG 2.2 AA (task §1): include the 2.2 rule tags on top of 2.0/2.1.
    .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22a', 'wcag22aa'])
    .analyze();

  await info.attach(`axe-${label}.json`, {
    body: JSON.stringify(results.violations, null, 2),
    contentType: 'application/json',
  });

  const blocking = results.violations.filter((v) => v.impact === 'serious' || v.impact === 'critical');
  info.annotations.push({
    type: 'a11y-summary',
    description: `${path} :: ${results.violations.length} total, ${blocking.length} serious/critical [${blocking.map((v) => v.id).join(', ')}]`,
  });

  expect(
    blocking,
    `serious/critical WCAG 2.1 AA violations on ${path}:\n${blocking.map((v) => `${v.id}: ${v.help}`).join('\n')}`,
  ).toEqual([]);
}

for (const screen of SCREENS) {
  test(`a11y: ${screen.name} (${screen.path})`, async ({ page }, info) => {
    test.fixme(screen.path in A11Y_KNOWN, A11Y_KNOWN[screen.path]);
    await page.goto(screen.path, { waitUntil: 'domcontentloaded' });
    await page.waitForLoadState('networkidle', { timeout: 15_000 }).catch(() => {});

    await expectNoBlockingViolations(page, info, screen.name, screen.path);
  });
}

// ---------------------------------------------------------------------------
// Onboarding (#234, criterion 7).
//
// These five routes and the activation checklist were absent from the sweep,
// which means the first screens a new customer ever sees were the only ones
// never checked. They cannot be added to SCREENS above: that block runs as the
// seeded admin, who is past the wizard, and OnboardingCompletedRedirect would
// bounce every /onboarding/* navigation to the dashboard — the sweep would pass
// while scanning the wrong page. So this block signs up its own tenant.
// ---------------------------------------------------------------------------

// #438 re-sequenced the tunnel: `profile` merged into `organization`, `team`
// moved to the Posture Reveal, and `score` + `cover` were added.
const WIZARD_STEPS = ['organization', 'goal', 'framework', 'score', 'cover'] as const;

test.describe('a11y — onboarding', () => {
  // This block owns its identity; the shared admin storageState must not leak in.
  test.use({ storageState: { cookies: [], origins: [] } });

  // ONE tenant for all five routes. The wizard guard only redirects a user who
  // has COMPLETED it, so an unfinished account can open any step directly — and
  // signing up five separate tenants in a row trips the registration rate limit.
  let wizard: Awaited<ReturnType<typeof signUp>>;
  test.beforeAll(async () => {
    const api = await pwRequest.newContext();
    wizard = await signUp(api, 'a11ywiz');
    await api.dispose();
  });

  for (const step of WIZARD_STEPS) {
    test(`a11y: onboarding wizard — ${step} (/onboarding/${step})`, async ({ browser }, info) => {
      const ctx = await browser.newContext({ storageState: wizard.storageState });
      const page = await ctx.newPage();

      await page.goto(`/onboarding/${step}`, { waitUntil: 'domcontentloaded' });
      await page.waitForLoadState('networkidle', { timeout: 15_000 }).catch(() => {});

      // Guard against the silent-redirect trap this block exists to avoid: if we
      // are not on the step we asked for, the scan below would be meaningless.
      await expect(page, 'the wizard step must actually be on screen').toHaveURL(
        new RegExp(`/onboarding/${step}`),
        { timeout: 15_000 },
      );

      await expectNoBlockingViolations(page, info, `onboarding-${step}`, `/onboarding/${step}`);

      await ctx.close();
    });
  }

  test('a11y: activation checklist (dashboard, steps outstanding)', async ({ browser }, info) => {
    const api = await pwRequest.newContext();
    const newcomer = await signUp(api, 'a11ychecklist');
    const client = authed(api, newcomer.token);

    // Finish the wizard so the guard lifts, but complete nothing else: the
    // checklist only renders while steps remain outstanding, so a fully
    // activated tenant would give us a dashboard with no checklist to scan.
    await client.put('/onboarding/steps/organization', {
      answers: {
        name: 'Awa Test Org',
        industry: 'banking',
        full_name: 'Awa Newcomer',
        job_title: 'RSSI',
      },
    });
    await client.post('/onboarding/complete', {});

    const ctx = await browser.newContext({ storageState: newcomer.storageState });
    const page = await ctx.newPage();
    await page.goto('/', { waitUntil: 'domcontentloaded' });
    await page.waitForLoadState('networkidle', { timeout: 15_000 }).catch(() => {});

    await expect(page, 'the guard must have lifted').not.toHaveURL(/\/onboarding/, {
      timeout: 15_000,
    });
    await expect(
      page.getByTestId('activation-step-first_risk'),
      'the checklist must be on screen for the scan to mean anything',
    ).toBeVisible({ timeout: 15_000 });

    await expectNoBlockingViolations(page, info, 'activation-checklist', '/ (checklist)');

    await ctx.close();
    await api.dispose();
  });
});

// ---------------------------------------------------------------------------
// #438 criterion 12 — the recognition screen.
//
// It cannot join SCREENS: it lives under /onboarding, and the seeded admin has
// completed the wizard, so OnboardingCompletedRedirect would bounce the scan to
// the dashboard and pass while checking the wrong page. It also needs a tenant
// that HOLDS data while its wizard is unfinished — the exact population #234
// backfilled — so this block builds one.
// ---------------------------------------------------------------------------

test.describe('a11y — recognition (#438)', () => {
  test.use({ storageState: { cookies: [], origins: [] } });

  test('a11y: recognition (/onboarding/recognition)', async ({ browser }, info) => {
    const api = await pwRequest.newContext();
    const tenant = await signUp(api, 'a11yrecog');
    const client = authed(api, tenant.token);

    // One real risk is enough to make the tenant "recognised" server-side. The
    // wizard is deliberately NOT completed: recognition is the screen that
    // REPLACES the tunnel, so a completed tenant would never reach it.
    const created = await client.post('/risks', {
      title: 'Risque préexistant',
      description: 'Créé avant que le tunnel existe.',
      probability: 0.4,
      impact: 7,
    });
    expect(
      [200, 201].includes(created.status()),
      `seeding a risk should succeed: ${await created.text()}`,
    ).toBeTruthy();

    const ctx = await browser.newContext({ storageState: tenant.storageState });
    const page = await ctx.newPage();
    await page.goto('/onboarding/recognition', { waitUntil: 'domcontentloaded' });
    await page.waitForLoadState('networkidle', { timeout: 15_000 }).catch(() => {});

    // The silent-redirect trap again: a scan of the wrong screen is worse than
    // no scan, because it reports green.
    await expect(page, 'the recognition screen must actually be on screen').toHaveURL(
      /\/onboarding\/recognition/,
      { timeout: 15_000 },
    );
    await expect(page.getByTestId('recognition')).toBeVisible({ timeout: 15_000 });

    await expectNoBlockingViolations(
      page,
      info,
      'onboarding-recognition',
      '/onboarding/recognition',
    );

    await ctx.close();
    await api.dispose();
  });
});

// ---------------------------------------------------------------------------
// #783 — hosted plan caps (D-078).
//
// Two states the plan gates added were outside the sweep: the cap notice
// (PlanCapNotice, here inside the framework dialog) and the locked, blurred
// preview FeatureGate renders on a plan without the feature (Threat Intel).
//
// Both exist ONLY on a hosted deployment: a self-hosted install resolves every
// tenant to PlanSelfHosted, which has no caps and no locks (ADR 0007). The CI
// e2e stack boots self_hosted, so this block detects the plan of the tenant it
// just signed up and SKIPS, loudly, when it is not Free. A skip is not a pass:
// run it against a stack started with OPENRISK_DEPLOYMENT=hosted.
// ---------------------------------------------------------------------------

test.describe('a11y — hosted plan caps (#783)', () => {
  test.use({ storageState: { cookies: [], origins: [] } });

  let free: Awaited<ReturnType<typeof signUp>>;
  let plan = '';
  test.beforeAll(async () => {
    const api = await pwRequest.newContext();
    free = await signUp(api, 'a11ycaps');
    const client = authed(api, free.token);
    // Past the wizard so the guard does not bounce /compliance and /threat-intel.
    await client.put('/onboarding/steps/organization', {
      answers: {
        name: 'Awa Test Org',
        industry: 'banking',
        full_name: 'Awa Newcomer',
        job_title: 'RSSI',
      },
    });
    await client.post('/onboarding/complete', {});
    const snapshot = await client.get('/entitlements');
    plan = snapshot.ok() ? String(((await snapshot.json()) as { plan?: string }).plan ?? '') : '';
    if (plan === 'free') {
      // Free keeps ONE framework active: this one fills the slot, so the dialog
      // opens at the cap.
      const created = await client.post('/compliance/frameworks', {
        name: 'Cap filler',
      });
      expect(
        [200, 201].includes(created.status()),
        `seeding the framework should succeed: ${await created.text()}`,
      ).toBeTruthy();
    }
    await api.dispose();
  });

  test.beforeEach(() => {
    test.skip(
      plan !== 'free',
      `the tenant resolved to plan "${plan}": caps and locks exist only on a hosted deployment (OPENRISK_DEPLOYMENT=hosted); this is NOT a pass`,
    );
  });

  test('a11y: cap notice in the framework dialog, Escape returns focus to its trigger', async ({
    browser,
  }, info) => {
    const ctx = await browser.newContext({ storageState: free.storageState });
    const page = await ctx.newPage();
    await page.goto('/compliance', { waitUntil: 'domcontentloaded' });
    await page.waitForLoadState('networkidle', { timeout: 15_000 }).catch(() => {});

    const trigger = page.getByRole('button', { name: /^(Nouveau|New)$/ });
    await trigger.focus();
    await page.keyboard.press('Enter');

    const dialog = page.getByRole('dialog');
    await expect(dialog, 'the create-framework dialog must open from the keyboard').toBeVisible();
    await expect(
      dialog.getByTestId('plan-cap-frameworks'),
      'the cap notice must be on screen for the scan to mean anything',
    ).toBeVisible();
    expect(
      await page.evaluate(() => !!document.activeElement?.closest('[role="dialog"]')),
      'focus must be inside the dialog once it is open',
    ).toBe(true);

    await expectNoBlockingViolations(page, info, 'plan-cap-notice-dialog', '/compliance (cap)');

    await page.keyboard.press('Escape');
    await expect(dialog, 'Escape closes the dialog').toBeHidden();
    await expect(trigger, 'focus returns to the trigger that opened the dialog').toBeFocused();

    await ctx.close();
  });

  test('a11y: locked Threat Intel preview, keyboard never enters the blurred content', async ({
    browser,
  }, info) => {
    const ctx = await browser.newContext({ storageState: free.storageState });
    const page = await ctx.newPage();
    await page.goto('/threat-intel', { waitUntil: 'domcontentloaded' });
    await page.waitForLoadState('networkidle', { timeout: 15_000 }).catch(() => {});

    const preview = page.locator('[inert][aria-hidden="true"]');
    await expect(preview, 'the blurred preview must be on screen (plan is locked)').toHaveCount(1, {
      timeout: 15_000,
    });

    await expectNoBlockingViolations(page, info, 'locked-threat-intel', '/threat-intel (locked)');

    // Walk the tab order well past the page's own controls: no stop may land
    // inside the inert preview, and the upgrade CTA must be one of the stops.
    let reachedCta = false;
    for (let i = 0; i < 60; i++) {
      await page.keyboard.press('Tab');
      const stop = await page.evaluate(() => {
        const el = document.activeElement;
        return {
          insidePreview: !!el?.closest('[inert]'),
          label: (el?.textContent ?? '').trim().slice(0, 60),
        };
      });
      expect(
        stop.insidePreview,
        `Tab stop ${i + 1} ("${stop.label}") is inside the locked preview`,
      ).toBe(false);
      if (/Pro|Business|Enterprise/.test(stop.label)) reachedCta = true;
    }
    expect(reachedCta, 'the upgrade CTA must be reachable by keyboard').toBe(true);

    await ctx.close();
  });
});
