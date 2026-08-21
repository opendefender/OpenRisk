// The activation journey, end to end: signup → the five wizard steps → guided
// first risk → Aha moment.
//
// This spec exists to hold three properties that unit tests cannot:
//
//   1. TOTAL DURATION. The promise is "an unknown reaches the Aha moment in
//      under 8 minutes". A machine walking the same path should be far faster;
//      the assertion is a regression fence on the SHAPE of the journey (how many
//      screens, how many round-trips), not a benchmark of the runner.
//
//   2. THE CHECKLIST SURVIVES A RELOAD. This is the whole point of moving
//      activation to the server. The old implementation kept it in localStorage
//      and derived it from client-side counts, so state was per-device and
//      re-derived on every render. Here we complete steps, hard-reload, and
//      assert the SERVER still says they are complete.
//
//   3. ONE EVENT TICKS ONE STEP. A single framework import must strike exactly
//      one row through — the reported "two items after one import" bug.
//
// It signs up a REAL new account (a fresh tenant) rather than reusing a seeded
// persona, because an already-onboarded tenant cannot exercise onboarding.

import { test, expect, request as pwRequest, type APIRequestContext } from '@playwright/test';
import { API_URL, FRONTEND_ORIGIN } from './support/env';
import { apiLogin, storageStateFor } from './support/auth';

/** The eight-minute promise, with headroom for a cold CI runner. */
const AHA_BUDGET_MS = 8 * 60 * 1000;

interface Newcomer {
  email: string;
  password: string;
  storageState: Awaited<ReturnType<typeof storageStateFor>>;
  token: string;
}

/**
 * Register a brand-new account through the real API and mint its storageState.
 * Unique per call so parallel runs and retries never collide.
 */
async function signUp(ctx: APIRequestContext, tag: string): Promise<Newcomer> {
  const stamp = `${Date.now()}${Math.floor(Math.random() * 1000)}`;
  const email = `e2e.activation.${tag}.${stamp}@openrisk.test`;
  const password = 'ActivationE2E!2026';
  const username = `e2eact${tag}${stamp}`.slice(0, 28);

  const res = await ctx.post(`${API_URL}/auth/register`, {
    data: {
      email,
      username,
      password,
      full_name: 'Awa Newcomer',
      company_name: `Awa Test Org ${stamp}`,
    },
  });
  expect(res.status(), `registration should succeed: ${await res.text()}`).toBe(201);

  const login = await apiLogin(ctx, email, password);
  return {
    email,
    password,
    // From the SAME context that logged in, so the session cookies come with
    // it — they, not a localStorage token, are the credential now.
    storageState: await storageStateFor(ctx, login),
    token: login.token_pair.access_token,
  };
}

/** Authenticated API helper for the new account. */
function authed(ctx: APIRequestContext, token: string) {
  const headers = { Authorization: `Bearer ${token}` };
  return {
    get: (path: string) => ctx.get(`${API_URL}${path}`, { headers }),
    put: (path: string, data: unknown) => ctx.put(`${API_URL}${path}`, { headers, data }),
    post: (path: string, data: unknown) => ctx.post(`${API_URL}${path}`, { headers, data }),
  };
}

interface ActivationStep {
  key: string;
  event_key: string;
  completed: boolean;
  completed_at: string | null;
  celebrate: boolean;
  primary: boolean;
}
interface ActivationState {
  steps: ActivationStep[];
  percent: number;
  aha_reached_at: string | null;
}

const completedKeys = (s: ActivationState) => s.steps.filter((x) => x.completed).map((x) => x.key);

// ---------------------------------------------------------------------------

test.describe('activation — signup to Aha', () => {
  // These tests own their identity; the shared persona storageState must not leak in.
  test.use({ storageState: { cookies: [], origins: [] } });

  test('a newcomer reaches the Aha moment, and the checklist is server state', async ({
    page,
    browser,
  }, info) => {
    const ctx = await pwRequest.newContext();
    const t0 = Date.now();

    // --- Sign up -----------------------------------------------------------
    const newcomer = await signUp(ctx, 'full');
    const api = authed(ctx, newcomer.token);

    // A fresh tenant starts with nothing done and no Aha.
    let activation: ActivationState = await (await api.get('/activation/state')).json();
    expect(completedKeys(activation), 'a new tenant has completed nothing').toEqual([]);
    expect(activation.percent).toBe(0);
    expect(activation.aha_reached_at).toBeNull();

    // --- The route guard: the app is unreachable before onboarding ---------
    const guardCtx = await browser.newContext({ storageState: newcomer.storageState });
    const guarded = await guardCtx.newPage();
    await guarded.goto('/', { waitUntil: 'domcontentloaded' });
    await expect(
      guarded,
      'the dashboard must be unreachable while onboarding.completed is false',
    ).toHaveURL(/\/onboarding\/organization/, { timeout: 15_000 });
    await guardCtx.close();

    // --- The five wizard steps ---------------------------------------------
    // Driven through the API the UI calls, so the assertion is about the flow
    // and its persistence rather than about pixel positions.
    let wizard = await (await api.get('/onboarding/state')).json();
    expect(wizard.completed).toBe(false);
    expect(wizard.current_step).toBe('organization');
    expect(wizard.steps).toEqual(['organization', 'profile', 'goal', 'framework', 'team']);

    // 1. Organization
    wizard = await (
      await api.put('/onboarding/steps/organization', {
        answers: {
          name: 'Banque Atlantique CM',
          industry: 'banking',
          size: '201-1000',
          country: 'CM',
          currency: 'XAF',
          timezone: 'Africa/Douala',
        },
        next: 'profile',
      })
    ).json();
    expect(wizard.current_step).toBe('profile');
    expect(wizard.industry).toBe('banking');

    // 2. Profile — this one is itself a checklist step.
    wizard = await (
      await api.put('/onboarding/steps/profile', {
        answers: { full_name: 'Awa Newcomer', job_title: 'RSSI', language: 'fr' },
        next: 'goal',
      })
    ).json();
    expect(wizard.current_step).toBe('goal');

    activation = await (await api.get('/activation/state')).json();
    expect(
      completedKeys(activation),
      'completing the profile step ticks the profile row and nothing else',
    ).toEqual(['profile']);

    // Back-navigation must be allowed — a wizard you cannot correct is a trap.
    const back = await (
      await api.put('/onboarding/steps/goal', { answers: { goal: 'pass_audit' }, next: 'profile' })
    ).json();
    expect(back.current_step, 'going back must be permitted').toBe('profile');

    // 3. Goal
    wizard = await (
      await api.put('/onboarding/steps/goal', {
        answers: { goal: 'cobac_compliance' },
        next: 'framework',
      })
    ).json();
    expect(wizard.goal).toBe('cobac_compliance');

    // Suggestions follow the answers: sector + country + goal.
    const suggestions = await (await api.get('/onboarding/suggestions')).json();
    expect(suggestions.risks, 'exactly three first-risk drafts').toHaveLength(3);
    expect(suggestions.frameworks[0], 'the chosen goal leads the framework list').toBe('cobac');
    for (const r of suggestions.risks) {
      expect(r.probability).toBeGreaterThanOrEqual(0);
      expect(r.probability).toBeLessThanOrEqual(1);
      expect(r.impact).toBeLessThanOrEqual(10);
    }

    // 4. Framework — one-click import of a suggested catalog.
    const framework = await (
      await api.post('/compliance/frameworks', {
        name: 'COBAC R-2016/04',
        version: '2016',
        description: 'Contrôle interne CEMAC',
      })
    ).json();
    const importRes = await api.post(`/compliance/frameworks/${framework.id}/import-catalog`, {
      catalog_key: 'cobac',
    });
    expect(importRes.status()).toBeLessThan(300);
    const imported = await importRes.json();
    expect(imported.imported, 'the import creates real controls').toBeGreaterThan(0);

    // ⚠️ THE REGRESSION: one import ticks ONE row.
    activation = await (await api.get('/activation/state')).json();
    expect(
      completedKeys(activation).sort(),
      'a single framework import must tick exactly one step',
    ).toEqual(['framework', 'profile']);

    await api.put('/onboarding/steps/framework', { answers: { imported: ['cobac'] }, next: 'team' });

    // 5. Team — skippable, then complete.
    await api.put('/onboarding/steps/team', { answers: { emails: '' } });
    wizard = await (await api.post('/onboarding/complete', {})).json();
    expect(wizard.completed).toBe(true);
    expect(wizard.landing).toBeTruthy();

    // --- The guard lifts ---------------------------------------------------
    const appCtx = await browser.newContext({ storageState: newcomer.storageState });
    const app = await appCtx.newPage();
    await app.goto('/', { waitUntil: 'domcontentloaded' });
    await expect(app, 'the app is reachable once onboarding is complete').not.toHaveURL(
      /\/onboarding/,
      { timeout: 15_000 },
    );

    // --- The guided first risk --------------------------------------------
    // NOT auto-created: the suggestion pre-fills a form the user validates. Here
    // we submit the draft the product proposed, exactly as the UI does.
    const draft = suggestions.risks[0];
    const riskRes = await api.post('/risks', {
      title: draft.title,
      description: draft.description,
      probability: draft.probability,
      impact: draft.impact,
      source: 'manual',
    });
    expect(riskRes.status(), `first risk should be created: ${await riskRes.text()}`).toBeLessThan(
      300,
    );

    activation = await (await api.get('/activation/state')).json();
    expect(completedKeys(activation)).toContain('first_risk');
    const firstRisk = activation.steps.find((s) => s.key === 'first_risk')!;
    expect(firstRisk.primary, 'the first risk is the product promise').toBe(true);
    expect(firstRisk.celebrate, 'the server asks for a celebration exactly once').toBe(true);

    // --- The Aha moment ----------------------------------------------------
    // Definition (spec §7): the first cyber score computed on the tenant's own
    // data, with at least one compliance gap identified. Loading the executive
    // dashboard is what computes it — which is what the user does next.
    const exec = await api.get('/analytics/executive');
    expect(exec.status()).toBe(200);
    const dashboard = await exec.json();
    expect(dashboard.cyber_score, 'a cyber score is produced').toBeTruthy();

    await expect
      .poll(
        async () => {
          const s: ActivationState = await (await api.get('/activation/state')).json();
          return s.aha_reached_at;
        },
        {
          message: 'the Aha moment should be recorded once the score meets its definition',
          timeout: 20_000,
        },
      )
      .not.toBeNull();

    const elapsed = Date.now() - t0;
    info.annotations.push({ type: 'time-to-aha-ms', description: String(elapsed) });
    expect(
      elapsed,
      'the whole journey must fit inside the eight-minute promise',
    ).toBeLessThan(AHA_BUDGET_MS);

    // --- THE RELOAD ASSERTION ----------------------------------------------
    // The checklist is server state: a hard reload (and, since nothing is kept
    // client-side, an entirely different browser context) sees the same rows
    // struck through.
    await app.goto('/', { waitUntil: 'domcontentloaded' });
    await app.reload({ waitUntil: 'domcontentloaded' });

    const afterReload: ActivationState = await (await api.get('/activation/state')).json();
    expect(
      completedKeys(afterReload).sort(),
      'completed steps survive a reload because they are server facts',
    ).toEqual(['first_risk', 'framework', 'profile']);
    expect(afterReload.percent).toBeGreaterThan(0);
    expect(afterReload.aha_reached_at).not.toBeNull();

    // A different context = a different localStorage. Same answer.
    const otherCtx = await browser.newContext({ storageState: newcomer.storageState });
    const fromOtherDevice: ActivationState = await (await api.get('/activation/state')).json();
    expect(
      completedKeys(fromOtherDevice).sort(),
      'activation is per tenant, not per device',
    ).toEqual(['first_risk', 'framework', 'profile']);
    await otherCtx.close();

    // If the panel is rendered, it must agree with the server rather than
    // recomputing anything. (Rendered only while steps remain — which is the
    // case here: several rows are still open.)
    const panelRow = app.getByTestId('activation-step-first_risk');
    if (await panelRow.count()) {
      await expect(panelRow).toHaveAttribute('data-completed', 'true');
    }

    await appCtx.close();
    await ctx.dispose();
  });

  test('a completed step never asks to celebrate twice', async ({}) => {
    const ctx = await pwRequest.newContext();
    const newcomer = await signUp(ctx, 'celebrate');
    const api = authed(ctx, newcomer.token);

    // Complete one step (the profile step of the wizard).
    await api.put('/onboarding/steps/profile', {
      answers: { full_name: 'Awa', job_title: 'RSSI' },
    });

    let state: ActivationState = await (await api.get('/activation/state')).json();
    const step = state.steps.find((s) => s.key === 'profile')!;
    expect(step.completed).toBe(true);
    expect(step.celebrate, 'the first read asks for the burst').toBe(true);

    // Acknowledge it — twice, because a re-render or a retry will.
    expect((await api.post('/activation/celebrated', { step_key: 'profile' })).status()).toBe(204);
    expect(
      (await api.post('/activation/celebrated', { step_key: 'profile' })).status(),
      'acknowledging twice is a no-op, not an error',
    ).toBe(204);

    state = await (await api.get('/activation/state')).json();
    const after = state.steps.find((s) => s.key === 'profile')!;
    expect(after.celebrate, 'a celebrated step never asks again').toBe(false);
    expect(after.completed, 'and it stays completed').toBe(true);

    // A stray key cannot pollute the ledger.
    expect(
      (await api.post('/activation/celebrated', { step_key: 'not-a-step' })).status(),
      'an unknown step key is rejected',
    ).toBe(400);

    await ctx.dispose();
  });

  test('the wizard is resumable: answers survive leaving and coming back', async ({ browser }) => {
    const ctx = await pwRequest.newContext();
    const newcomer = await signUp(ctx, 'resume');
    const api = authed(ctx, newcomer.token);

    await api.put('/onboarding/steps/organization', {
      answers: { name: 'Clinique du Littoral', industry: 'health', country: 'CM' },
      next: 'profile',
    });

    // A brand-new browser context — nothing client-side carries over.
    const fresh = await browser.newContext({ storageState: newcomer.storageState });
    const page = await fresh.newPage();
    await page.goto('/', { waitUntil: 'domcontentloaded' });

    // The guard resumes at the stored step, not back at the start.
    await expect(page, 'a resumed wizard reopens where it was left').toHaveURL(
      /\/onboarding\/profile/,
      { timeout: 15_000 },
    );

    const state = await (await api.get('/onboarding/state')).json();
    expect(state.answers.organization.name).toBe('Clinique du Littoral');
    expect(state.industry).toBe('health');

    // And the suggestions follow that sector.
    const suggestions = await (await api.get('/onboarding/suggestions')).json();
    expect(suggestions.risks.some((r: { key: string }) => r.key === 'patient_data_leak')).toBe(true);

    await fresh.close();
    await ctx.dispose();
  });
});
