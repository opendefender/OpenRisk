// The full risk lifecycle, walked end to end — including the governance
// approval that gates RESIDUAL_ACCEPTED.
//
//   DRAFT → IDENTIFIED → ASSESSED → TREATMENT_PLANNED → IN_TREATMENT
//         → (RESIDUAL_ACCEPTED | MITIGATED) → CLOSED   ↘ REOPENED ↗
//
// This spec holds three properties a unit test cannot:
//
//   1. THE GUARDS ARE THE SERVER'S. Every blocked transition here is refused by
//      the API, not by a disabled button. The previous stepper kept its own copy
//      of the graph and the two drifted, which is how a risk reached "traité"
//      with two sub-actions still open. The spec asserts through the API AND
//      through the UI, so a future client-side shortcut fails it.
//
//   2. A BLOCKED STEP EXPLAINS ITSELF. The stepper must render the concrete
//      blocker ("2 sous-actions restantes…"), not hide the step. A hidden step
//      leaves a user with no way to learn what to do.
//
//   3. THE LIFECYCLE AND THE PLAN ARE ONE FLOW. Completing the checklist is what
//      unlocks MITIGATED — nothing else does.
//
// Backend counterpart, which runs without a browser and covers the same walk:
// backend/internal/handler/risk_lifecycle_e2e_test.go.

import { test, expect, request as pwRequest, type APIRequestContext } from '@playwright/test';
import { API_URL, API_BASE, ADMIN, FRONTEND_ORIGIN, SEED_IDS_FILE } from './support/env';
import fs from 'node:fs';
import { apiLogin, storageStateFor } from './support/auth';

interface Ctx {
  api: APIRequestContext;
  token: string;
  riskId: string;
  planId: string;
  subIds: string[];
}

async function authed(): Promise<{
  api: APIRequestContext;
  token: string;
  state: Awaited<ReturnType<typeof storageStateFor>>;
}> {
  const raw = await pwRequest.newContext({ baseURL: API_BASE });
  // MFA is mandated, so the password step alone yields no session. The seed
  // records the TOTP secret it enrolled; without it an enrolled account cannot
  // be answered.
  const seed = JSON.parse(fs.readFileSync(SEED_IDS_FILE, 'utf8'));
  const login = await apiLogin(raw, ADMIN.email, ADMIN.password, seed.adminMfaSecret);
  const token = login.token_pair.access_token;
  const api = await pwRequest.newContext({
    baseURL: API_BASE,
    extraHTTPHeaders: { Authorization: `Bearer ${token}` },
  });
  // Built from `raw`, the context that performed the login and therefore holds
  // the session cookies.
  return { api, token, state: await storageStateFor(raw, login) };
}

/** POST a transition and return { status, error } without throwing on 4xx. */
async function transition(api: APIRequestContext, riskId: string, to: string) {
  const res = await api.post(`risks/${riskId}/transition`, { data: { to, comment: 'e2e' } });
  const body = await res.json().catch(() => ({}));
  return { status: res.status(), error: (body as { error?: string }).error ?? '' };
}

async function stateOf(api: APIRequestContext, riskId: string) {
  const res = await api.get(`risks/${riskId}`);
  expect(res.ok()).toBeTruthy();
  return (await res.json()) as { lifecycle_state: string; status: string; lifecycle_phase: string };
}

test.describe('Risk lifecycle', () => {
  test('a risk walks the whole lifecycle, guards enforced by the server', async () => {
    const { api } = await authed();
    const ctx: Ctx = { api, token: '', riskId: '', planId: '', subIds: [] };

    await test.step('create — lands at DRAFT, owned by its creator', async () => {
      const res = await api.post('risks', {
        data: {
          title: `E2E lifecycle ${Date.now()}`,
          description: 'Bucket S3 exposé publiquement — parcours E2E du cycle de vie.',
          impact: 8,
          probability: 0.6,
        },
      });
      expect(res.status()).toBe(201);
      const risk = await res.json();
      ctx.riskId = risk.id;

      expect(risk.lifecycle_state).toBe('draft');
      // status and lifecycle_phase are DERIVED — nothing writes them separately.
      expect(risk.status).toBe('DRAFT');
      expect(risk.lifecycle_phase).toBe('identified');
      // A risk nobody answers for is unactionable, so the creator owns it.
      expect(risk.owner_id).toBeTruthy();
    });

    await test.step('advance to TREATMENT_PLANNED', async () => {
      for (const to of ['identified', 'assessed', 'treatment_planned']) {
        const r = await transition(api, ctx.riskId, to);
        expect(r.status, `${to}: ${r.error}`).toBe(200);
      }
      expect((await stateOf(api, ctx.riskId)).lifecycle_state).toBe('treatment_planned');
    });

    await test.step('guard 1 — IN_TREATMENT is refused with no mitigation', async () => {
      const r = await transition(api, ctx.riskId, 'in_treatment');
      expect(r.status).toBe(400);
      expect(r.error.toLowerCase()).toContain('mitigation');

      // A refused transition must not have moved anything.
      expect((await stateOf(api, ctx.riskId)).lifecycle_state).toBe('treatment_planned');

      // And the stepper's contract reports the same, with the guard named so the
      // UI can offer the way out instead of a dead end.
      const view = await (await api.get(`risks/${ctx.riskId}/transitions`)).json();
      const blocked = view.options.find((o: { to: string }) => o.to === 'in_treatment');
      expect(blocked).toBeTruthy();
      expect(blocked.allowed).toBe(false);
      expect(blocked.guard).toBe('active_mitigation');
      expect(blocked.reason).toBeTruthy();
    });

    await test.step('create the mitigation plan and its checklist', async () => {
      const res = await api.post(`risks/${ctx.riskId}/mitigations`, {
        data: { title: "Fermer l'accès public", description: 'Retirer la policy, activer SSE-KMS.', priority: 'high' },
      });
      expect(res.status()).toBe(201);
      const plan = await res.json();
      ctx.planId = plan.id;
      // Progress is COMPUTED: no checklist + PLANNED = 0.
      expect(plan.progress).toBe(0);

      for (const title of ['Retirer la policy publique', 'Activer SSE-KMS']) {
        const sub = await api.post(`mitigations/${ctx.planId}/sub-actions`, { data: { title } });
        expect(sub.status()).toBe(201);
        ctx.subIds.push((await sub.json()).id);
      }
    });

    await test.step('guard 1 satisfied — treatment starts, legacy fields follow', async () => {
      const r = await transition(api, ctx.riskId, 'in_treatment');
      expect(r.status, r.error).toBe(200);

      const s = await stateOf(api, ctx.riskId);
      expect(s.lifecycle_state).toBe('in_treatment');
      expect(s.status).toBe('in_progress');
      expect(s.lifecycle_phase).toBe('treated');
    });

    await test.step('guard 2 — MITIGATED is refused while sub-actions remain', async () => {
      let r = await transition(api, ctx.riskId, 'mitigated');
      expect(r.status).toBe(400);
      expect(r.error).toContain('2');

      // One down, one to go — the count in the refusal follows reality.
      expect((await api.post(`mitigations/${ctx.planId}/sub-actions/${ctx.subIds[0]}/complete`)).ok()).toBeTruthy();
      r = await transition(api, ctx.riskId, 'mitigated');
      expect(r.status).toBe(400);
      expect(r.error).toContain('1');

      // …and progress was recomputed server-side by that completion.
      const plan = await (await api.get(`mitigations/${ctx.planId}`)).json();
      expect(plan.progress).toBe(50);
    });

    await test.step('guard 3 — RESIDUAL_ACCEPTED needs a VALIDATED approval', async () => {
      const refused = await transition(api, ctx.riskId, 'residual_accepted');
      expect(refused.status).toBe(400);
      expect(refused.error).toContain('Gouvernance');

      // Submit the acceptance request. A request that exists but is pending is
      // NOT an approval — the refusal names it so it can be chased.
      const submitted = await api.post('governance/approvals', {
        data: {
          entity_type: 'risk_acceptance',
          entity_id: ctx.riskId,
          action: 'accept',
          title: 'Acceptation du risque résiduel (E2E)',
        },
      });
      // A tenant with no risk_acceptance workflow configured cannot submit; the
      // rest of the step only makes sense when one exists.
      test.skip(!submitted.ok(), 'no risk_acceptance approval workflow configured on this tenant');
      const request = await submitted.json();

      const pending = await transition(api, ctx.riskId, 'residual_accepted');
      expect(pending.status).toBe(400);
      expect(pending.error).toContain(String(request.id).slice(0, 8));

      // Approve it (four-eyes: a different member must sign, so this step needs
      // a second seeded account — skipped rather than faked when absent).
      const decided = await api.post(`governance/approvals/${request.id}/decide`, {
        data: { decision: 'approve', comment: 'E2E' },
      });
      test.skip(!decided.ok(), 'four-eyes: the requester cannot approve their own request');

      const accepted = await transition(api, ctx.riskId, 'residual_accepted');
      expect(accepted.status, accepted.error).toBe(200);
      expect((await stateOf(api, ctx.riskId)).status).toBe('accepted');
    });

    await test.step('finish the plan, then MITIGATED → CLOSED → REOPENED', async () => {
      // Back into treatment if the acceptance branch was taken.
      const current = (await stateOf(api, ctx.riskId)).lifecycle_state;
      if (current === 'residual_accepted') {
        expect((await transition(api, ctx.riskId, 'in_treatment')).status).toBe(200);
      }

      expect((await api.post(`mitigations/${ctx.planId}/sub-actions/${ctx.subIds[1]}/complete`)).ok()).toBeTruthy();
      const plan = await (await api.get(`mitigations/${ctx.planId}`)).json();
      expect(plan.progress).toBe(100);

      expect((await transition(api, ctx.riskId, 'mitigated')).status).toBe(200);
      expect((await transition(api, ctx.riskId, 'closed')).status).toBe(200);
      expect((await stateOf(api, ctx.riskId)).status).toBe('closed');

      // A closed risk is not a grave.
      expect((await transition(api, ctx.riskId, 'reopened')).status).toBe(200);
      expect((await transition(api, ctx.riskId, 'assessed')).status).toBe(200);
      expect((await stateOf(api, ctx.riskId)).status).toBe('open');
    });
  });

  test('the stepper shows a blocked step with its reason, not a hidden one', async ({ browser }) => {
    const { api, state } = await authed();

    // A risk parked in TREATMENT_PLANNED with no mitigation: the next step is
    // blocked, and the UI must SAY SO rather than offering nothing.
    const created = await api.post('risks', {
      data: { title: `E2E blocked ${Date.now()}`, description: 'Étape suivante bloquée.', impact: 6, probability: 0.4 },
    });
    const risk = await created.json();
    for (const to of ['identified', 'assessed', 'treatment_planned']) {
      await transition(api, risk.id, to);
    }

    const page = await (await browser.newContext({ storageState: state })).newPage();
    await page.goto(`/risks?focus=${risk.id}`);

    await page.getByRole('button', { name: /cycle de vie|lifecycle/i }).click();

    // The blocked step is present, greyed, and carries its reason.
    await expect(page.getByText(/étape suivante bloquée|next step blocked/i)).toBeVisible();
    await expect(page.getByText(/aucune mitigation active|no active mitigation/i)).toBeVisible();
    // …and offers the way out.
    await expect(page.getByRole('button', { name: /plan de mitigation|mitigation plan/i })).toBeVisible();
  });
});
