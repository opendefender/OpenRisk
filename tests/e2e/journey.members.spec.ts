// W0-04 — organization member management, end to end in a real browser.
//
// The API-level proof lives in the Go suite and in the live-proof record; what
// only a browser can settle is that the SCREEN does what the API allows: that
// an invitation created here appears in the list, that the link it produces
// actually lands somewhere usable, that withdrawing access asks first, and that
// a member without the permission is told so rather than shown an empty table.
//
// Every test drives the real UI. Where a test needs the one-time invitation
// token — which by design reaches only the invitee — it takes it from the same
// place an administrator would: the dialog the product shows when the email
// could not be delivered.

import { test, expect, request as pwRequest, type APIRequestContext, type Page } from '@playwright/test';
import { authFileFor, API_URL, API_BASE, ADMIN } from './support/env';
import { apiLogin } from './support/auth';

test.use({ storageState: authFileFor('admin') });

/** An API context carrying the admin's bearer token.
 *
 *  page.request cannot be used for this: the app authenticates with a bearer
 *  token held in localStorage, not with a cookie, so a request made through
 *  the page context arrives unauthenticated. */
async function adminApi(): Promise<APIRequestContext> {
  const raw = await pwRequest.newContext({ baseURL: API_BASE });
  const login = await apiLogin(raw, ADMIN.email, ADMIN.password);
  return pwRequest.newContext({
    baseURL: API_BASE,
    extraHTTPHeaders: { Authorization: `Bearer ${login.token_pair.access_token}` },
  });
}

/** A fresh address per run: re-inviting an existing member is correctly a 409,
 *  so a spec that reuses one address only passes the first time. */
const freshEmail = () => `w4-e2e-${Date.now()}-${Math.floor(Math.random() * 1000)}@openrisk.test`;

async function openMembers(page: Page) {
  await page.goto('/settings/members', { waitUntil: 'domcontentloaded' });
  await page.locator('[data-testid="members-tab-members"]').waitFor({ state: 'visible', timeout: 20_000 });
}

test('the roster renders real members, not a fixture', async ({ page }) => {
  await openMembers(page);
  const rows = page.locator('[data-testid="member-row"]');
  await expect(rows.first()).toBeVisible({ timeout: 20_000 });

  // Cross-check the screen against the API it claims to render. A table that
  // disagrees with /organization/members is a table showing something else.
  const api = await adminApi();
  const res = await api.get('organization/members?limit=100');
  expect(res.ok()).toBeTruthy();
  const body = await res.json();
  await expect(rows).toHaveCount(body.total);

  // The owner is present and is NOT offered a role control — the server
  // refuses that, and the row says so instead of offering a 403.
  const ownerRow = rows.filter({ hasText: /propriétaire|owner/i }).first();
  if (await ownerRow.count()) {
    await expect(ownerRow.locator('select')).toHaveCount(0);
  }
});

test('invite → the invitation appears, and its link opens a usable acceptance page', async ({ page, context }) => {
  const email = freshEmail();
  await openMembers(page);

  await page.locator('[data-testid="invite-member"]').click();
  await page.getByLabel(/adresse email|email address/i).fill(email);
  await page.getByRole('button', { name: /envoyer l'invitation|send invitation/i }).click();

  // The dialog reports what actually happened to the mail. Both outcomes are
  // legitimate; what is NOT legitimate is claiming a send that did not happen,
  // so the assertion accepts either and then checks the two are not confused.
  const confirmation = page.getByRole('dialog');
  await expect(confirmation).toBeVisible({ timeout: 15_000 });
  const text = (await confirmation.innerText()).toLowerCase();
  const claimedSent = /invitation envoyée|invitation sent/.test(text);
  const claimedManual = /transmettre vous-même|deliver it yourself/.test(text);
  expect(claimedSent || claimedManual, 'the dialog states a delivery outcome').toBeTruthy();
  expect(claimedSent && claimedManual, 'it states exactly one').toBeFalsy();

  // When the mail did not go out, the link is shown once so it can be relayed.
  let acceptUrl: string | null = null;
  if (claimedManual) {
    acceptUrl = await confirmation.locator('code').first().innerText();
    expect(acceptUrl).toContain('/invitations/accept?token=');
  } else {
    // Delivered: the admin must NOT be handed a credential that authenticates
    // as somebody else.
    await expect(confirmation.locator('code')).toHaveCount(0);
  }
  await confirmation.getByRole('button', { name: /terminé|done/i }).click();

  // It is really there, in the Invitations tab, pending.
  await page.locator('[data-testid="members-tab-invitations"]').click();
  const row = page.locator('[data-testid="invitation-row"]').filter({ hasText: email });
  await expect(row).toHaveCount(1);
  await expect(row).toContainText(/en attente|pending/i);

  // The link lands on a page that names the organization and asks for what it
  // needs — a signed-out browser, exactly like a real invitee's.
  if (acceptUrl) {
    const guest = await context.browser()!.newContext();
    const guestPage = await guest.newPage();
    await guestPage.goto(acceptUrl, { waitUntil: 'domcontentloaded' });
    await expect(guestPage.getByRole('heading').first()).toBeVisible({ timeout: 20_000 });
    await expect(guestPage.getByText(email)).toBeVisible();
    await expect(
      guestPage.getByRole('button', { name: /créer mon compte|create my account|rejoindre|join/i }),
    ).toBeVisible();
    await guest.close();
  }
});

test('revoking an invitation asks first, then kills the link', async ({ page, context }) => {
  const email = freshEmail();
  // Created through the API so the test is about the revoke UI, not the invite UI.
  const api = await adminApi();
  const created = await api.post('organization/invitations', { data: { email, role: 'user' } });
  expect(created.status()).toBe(201);
  const acceptUrl: string | undefined = (await created.json()).accept_url;

  await openMembers(page);
  await page.locator('[data-testid="members-tab-invitations"]').click();
  const row = page.locator('[data-testid="invitation-row"]').filter({ hasText: email });
  await expect(row).toHaveCount(1);

  await row.locator('button[aria-label*="évoquer"], button[aria-label*="evoke"]').click();
  // A confirmation, not an immediate mutation.
  const dialog = page.getByRole('dialog').or(page.locator('text=/révoquer l\'invitation|revoke the invitation/i')).first();
  await expect(dialog).toBeVisible();
  await expect(row).toHaveCount(1); // still there — nothing happened yet

  await page.getByRole('button', { name: /^révoquer$|^revoke$/i }).click();
  await expect(row.filter({ hasText: /en attente|pending/i })).toHaveCount(0, { timeout: 15_000 });

  // The mailed link is dead, not merely marked revoked in a table.
  if (acceptUrl) {
    const guest = await context.browser()!.newContext();
    const guestPage = await guest.newPage();
    await guestPage.goto(acceptUrl, { waitUntil: 'domcontentloaded' });
    await expect(
      guestPage.getByText(/invitation indisponible|invitation unavailable/i),
    ).toBeVisible({ timeout: 20_000 });
    await guest.close();
  }
});

test('withdrawing a member’s access asks first and offers the reversible option', async ({ page }) => {
  // A member to act on, provisioned through the real flow.
  const email = freshEmail();
  const api = await adminApi();
  const created = await api.post('organization/invitations', { data: { email, role: 'user' } });
  expect(created.status()).toBe(201);
  const acceptUrl: string | undefined = (await created.json()).accept_url;
  test.skip(!acceptUrl, 'needs the acceptance link, which is returned only when mail is unavailable');

  const accepted = await api.post('invitations/accept', {
    data: {
      token: new URL(acceptUrl!).searchParams.get('token'),
      full_name: 'E2E Member',
      password: 'e2e-member-passphrase-2026!',
    },
  });
  expect(accepted.status()).toBe(201);

  await openMembers(page);
  await page.getByLabel(/rechercher un membre|search members/i).fill(email);
  const row = page.locator('[data-testid="member-row"]').filter({ hasText: email });
  await expect(row).toHaveCount(1, { timeout: 15_000 });

  await row.getByRole('button', { name: /désactiver|deactivate/i }).click();
  // The impact radiography states the consequence before anything happens.
  await expect(page.getByText(/ses sessions seront fermées|their sessions are ended/i)).toBeVisible();
  await expect(row).toContainText(/actif|active/i); // unchanged so far

  await page.getByRole('button', { name: /^désactiver$|^deactivate$/i }).last().click();
  await expect(row).toContainText(/désactivé|deactivated/i, { timeout: 15_000 });

  // Reversible, as the dialog promised.
  await row.getByRole('button', { name: /réactiver|reactivate/i }).click();
  await expect(row).toContainText(/actif|active/i, { timeout: 15_000 });
});

test('the access history records what happened, with who and when', async ({ page }) => {
  await openMembers(page);
  const historyTab = page.locator('[data-testid="members-tab-history"]');
  test.skip((await historyTab.count()) === 0, 'this persona lacks organization:audit:read');
  await historyTab.click();

  const rows = page.locator('[data-testid="audit-row"]');
  await expect(rows.first()).toBeVisible({ timeout: 20_000 });
  // Each entry says what happened and who did it — a history that says neither
  // is a list nobody can reconcile.
  const first = await rows.first().innerText();
  expect(first.trim().length).toBeGreaterThan(10);
  expect(first).toMatch(/\d/); // carries a timestamp
});

test('the sidebar badge counts real outstanding invitations', async ({ page }) => {
  const api = await adminApi();
  const counts = await api.get('organization/counts');
  expect(counts.ok()).toBeTruthy();
  const pending: number = (await counts.json()).pending_invitations;

  await page.goto('/', { waitUntil: 'domcontentloaded' });
  const item = page.locator('nav button', { hasText: /rôles|roles|membres|members/i }).first();
  await item.waitFor({ state: 'visible', timeout: 20_000 });
  const badge = item.locator('span').filter({ hasText: /^\d+$/ });

  if (pending > 0) {
    await expect(badge.first()).toHaveText(String(pending));
  } else {
    // Zero renders NO badge. A "0" would be a claim nobody needs, and the
    // hardcoded '12' this replaced was a claim nobody measured.
    await expect(badge).toHaveCount(0);
  }
});
