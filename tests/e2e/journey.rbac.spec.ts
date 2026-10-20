// RBAC journey (UX-24): an admin changes a member's business role in the real
// members screen, and the member sees the change in their own browser.
//
// The member is the seeded analyst (dev/fixtures/e2e-dataset.json): POST /users
// creates it as a member of the admin's organization, carrying its business
// role on the membership (#590). analystUsable is false only when that seed
// step failed, and the global-setup log then names the cause.
//
// How a role change reaches the member, which is what the second half asserts:
//   - The business role lives on the membership. The SPA reads it from the
//     sign-in response and keeps it in its cached profile; a token refresh does
//     not re-read it (useAuthStore.refreshToken).
//   - So the change does not repaint an open session. Instead the role change
//     revokes the member's refresh tokens in that organization
//     (membership/members.go SetRole -> RevokeUserTokensInTenant, #831). The
//     open session lives until its 15-minute access cookie lapses; the renewal
//     after that is refused and the member is sent back to sign in.
//   - A fresh sign-in returns the new business role, which picks the landing
//     route (navModel.ts landingForBusinessRole) and the dashboard view
//     (dashboardVariant.ts).
// The lapse is reproduced by dropping the access cookie, which is exactly what
// the browser does when it expires: waiting 15 minutes is not a test.
//
// Both projects (chromium, Mobile Chrome) change the SAME membership, so the
// test holds a cross-worker lock and each project moves the analyst to a
// different role. The original role is restored in `finally`.

import fs from 'node:fs';
import path from 'node:path';
import {
  test,
  expect,
  request as pwRequest,
  type APIResponse,
  type Browser,
  type BrowserContext,
  type Page,
  type TestInfo,
} from '@playwright/test';
import { authFileFor, API_URL, API_BASE, AUTH_DIR, E2E_DIR } from './support/env';
import { apiLogin, decodeJwt, storageStateFor } from './support/auth';
import { readSeedIds } from './support/routes';

const seed = readSeedIds();
const analystUsable = !!seed.personas?.analyst?.usable;

test.describe('rbac — admin surface', () => {
  test.use({ storageState: authFileFor('admin') });

  test('UX-24: roles & access screen renders the member/role matrix', async ({ page }) => {
    await page.goto('/settings/roles', { waitUntil: 'domcontentloaded' });
    await page.waitForLoadState('networkidle', { timeout: 15_000 }).catch(() => {});
    const text = (await page.locator('main').innerText().catch(() => '')).trim();
    expect(text.length, 'roles screen renders content').toBeGreaterThan(20);
  });
});

// ---------------------------------------------------------------------------
// Fixtures for the propagation journey
// ---------------------------------------------------------------------------

interface PersonaSpec {
  email: string;
  password: string;
  business_role: string;
}
interface Dataset {
  personas: { analyst: PersonaSpec };
}
const dataset = JSON.parse(
  fs.readFileSync(path.resolve(E2E_DIR, '../../dev/fixtures/e2e-dataset.json'), 'utf8'),
) as Dataset;
const ANALYST = dataset.personas.analyst;

/** The role each project moves the analyst to. Different on purpose: a
 *  leftover from the other project can never pass for this one's change. */
const TARGET_ROLE: Record<string, string> = {
  chromium: 'risk_manager',
  'Mobile Chrome': 'auditor',
};

/** Where a fresh sign-in lands, per business role (frontend/src/shared/navModel.ts
 *  BUSINESS_ROLE_LANDING). Only the roles this journey uses. */
const LANDING: Record<string, RegExp> = {
  security_analyst: /\/vulnerabilities(\?|$)/,
  risk_manager: /\/risks(\?|$)/,
  auditor: /\/compliance(\?|$)/,
};

/** Dashboard view per business role (frontend/src/features/dashboard/dashboardVariant.ts). */
const VARIANT: Record<string, 'rssi' | 'rm' | 'audit'> = {
  security_analyst: 'rssi',
  risk_manager: 'rm',
  auditor: 'audit',
};

/** The heading of the dashboard's priority panel, which names the view, in
 *  both languages, read from the shipped catalogues rather than copied here. */
type Catalogue = { dashboard: { priorities: Record<string, string> } };
const readLocale = (lang: 'fr' | 'en'): Catalogue =>
  JSON.parse(
    fs.readFileSync(path.resolve(E2E_DIR, `../../frontend/src/locales/${lang}.json`), 'utf8'),
  ) as Catalogue;
const LOCALES = [readLocale('fr'), readLocale('en')];
const escapeRe = (s: string) => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
const exactAny = (labels: string[]) => new RegExp(`^(${labels.map(escapeRe).join('|')})$`);
const priorityHeading = (variant: string) =>
  exactAny(LOCALES.map((l) => l.dashboard.priorities[variant]));

interface BusinessRole {
  key: string;
  label_fr: string;
  label_en: string;
}
interface Member {
  member_id: string;
  email: string;
  org_role: string;
  business_role?: string;
}

/**
 * Serialises the journey across workers. The two projects act on one
 * membership; run in parallel, one project's BEFORE would read the other's
 * change. An exclusive-create lock file names its holder's pid, so a lock left
 * by a crashed worker is recognised and broken instead of stalling the run.
 */
async function withAnalystLock<T>(fn: () => Promise<T>): Promise<T> {
  const lock = path.join(AUTH_DIR, 'rbac-analyst.lock');
  const deadline = Date.now() + 150_000;
  for (;;) {
    try {
      fs.writeFileSync(lock, String(process.pid), { flag: 'wx' });
      break;
    } catch (e) {
      if ((e as NodeJS.ErrnoException).code !== 'EEXIST') throw e;
      let holder = 0;
      try {
        holder = Number(fs.readFileSync(lock, 'utf8').trim() || '0');
      } catch {
        continue; // released between the two calls
      }
      let alive = false;
      try {
        if (holder > 0) {
          process.kill(holder, 0);
          alive = true;
        }
      } catch {
        alive = false;
      }
      if (!alive) {
        fs.rmSync(lock, { force: true });
        continue;
      }
      if (Date.now() > deadline) throw new Error(`rbac lock still held by pid ${holder}`);
      await new Promise((r) => setTimeout(r, 250));
    }
  }
  try {
    return await fn();
  } finally {
    fs.rmSync(lock, { force: true });
  }
}

/**
 * The admin's API calls ride the admin page's own session: the same cookies
 * the screen uses, with the CSRF double-submit header a cookie-authenticated
 * write needs. No extra sign-in (/auth/login allows 15 per IP per 5 minutes).
 */
async function adminCall(
  page: Page,
  method: 'GET' | 'PUT',
  route: string,
  data?: Record<string, string>,
): Promise<APIResponse> {
  const csrf = (await page.context().cookies(API_URL)).find((c) => c.name === 'or_csrf')?.value;
  return page.request.fetch(`${API_URL}/${route}`, {
    method,
    data,
    headers: csrf ? { 'X-CSRF-Token': csrf } : {},
  });
}

async function findAnalyst(page: Page): Promise<Member> {
  const res = await adminCall(page, 'GET', 'organization/members?limit=100');
  expect(res.ok(), `GET /organization/members: ${res.status()}`).toBeTruthy();
  const body = (await res.json()) as { items: Member[] };
  const m = body.items.find((x) => x.email.toLowerCase() === ANALYST.email.toLowerCase());
  expect(m, `${ANALYST.email} is a member of the admin's organization`).toBeTruthy();
  return m as Member;
}

async function setAnalystRole(page: Page, memberId: string, businessRole: string) {
  const res = await adminCall(page, 'PUT', `organization/members/${memberId}/role`, {
    role: 'user',
    business_role: businessRole,
  });
  expect(res.ok(), `set role: ${res.status()} ${await res.text()}`).toBeTruthy();
}

type StorageState = Awaited<ReturnType<typeof storageStateFor>>;

/**
 * The analyst's browser session for the BEFORE half: the storageState
 * global-setup minted, while its access cookie has a few minutes left. A
 * previous run of this journey revoked its refresh token, so once the access
 * cookie lapses it can no longer be renewed; only then is a fresh session
 * minted, which costs one sign-in.
 */
async function analystState(): Promise<string | StorageState> {
  const file = authFileFor('analyst');
  const state = JSON.parse(fs.readFileSync(file, 'utf8')) as StorageState;
  const access = state.cookies.find((c) => c.name === 'or_access')?.value;
  const exp = access ? Number(decodeJwt(access).exp ?? 0) : 0;
  if (exp - Date.now() / 1000 > 180) return file;
  const raw = await pwRequest.newContext({ baseURL: API_BASE });
  try {
    const login = await apiLogin(raw, ANALYST.email, ANALYST.password, undefined);
    return await storageStateFor(raw, login);
  } finally {
    await raw.dispose();
  }
}

/** A second browser context for the member, with this project's device. */
async function memberContext(browser: Browser, testInfo: TestInfo): Promise<BrowserContext> {
  const use = testInfo.project.use;
  return browser.newContext({
    baseURL: use.baseURL,
    viewport: use.viewport,
    userAgent: use.userAgent,
    deviceScaleFactor: use.deviceScaleFactor,
    isMobile: use.isMobile,
    hasTouch: use.hasTouch,
    locale: use.locale,
    storageState: await analystState(),
  });
}

/**
 * Opens the member's dashboard and asserts which view it is.
 *
 * A member who joins an organization that is already set up is shown the
 * recognition screen once, in place of the signup wizard (#438 criterion 9,
 * OnboardingGuard), and enters the app from its button. On a fresh database
 * the seeded analyst has not done that yet, so the member does it here, the way
 * a real member would; on later runs the screen no longer appears.
 */
async function expectDashboardView(page: Page, variant: string) {
  await page.goto('/', { waitUntil: 'domcontentloaded' });
  const heading = page.getByRole('heading', { level: 2, name: priorityHeading(variant) });
  const enter = page.getByRole('button', { name: /^(Voir ma posture|See my posture)$/ });
  await expect(heading.or(enter).first()).toBeVisible({ timeout: 20_000 });
  if (await enter.isVisible()) {
    await enter.click();
    await expect(page).toHaveURL(/\/posture/, { timeout: 15_000 });
    await page.goto('/', { waitUntil: 'domcontentloaded' });
  }
  await expect(page).not.toHaveURL(/\/(login|onboarding)/);
  await expect(heading).toBeVisible({ timeout: 20_000 });
}

// ---------------------------------------------------------------------------

test.describe('rbac — role change propagation', () => {
  test.use({ storageState: authFileFor('admin') });

  test.fixme(
    !analystUsable,
    'analyst persona was not provisioned: the seed (POST /users) failed, see the [seed] personas line',
  );

  test('UX-24: admin changes a member business role; member sees it', async ({
    page,
    browser,
  }, testInfo) => {
    test.setTimeout(240_000); // includes up to 150 s waiting for the other project
    const original = ANALYST.business_role;
    const target = TARGET_ROLE[testInfo.project.name] ?? 'risk_manager';
    expect(target).not.toBe(original);

    await withAnalystLock(async () => {
      let member: BrowserContext | undefined;
      let memberId: string | undefined;
      try {
        // ---- Setup: the analyst starts from its seeded role. ---------------
        await page.goto('/settings/members', { waitUntil: 'domcontentloaded' });
        await page.getByTestId('members-tab-members').waitFor({ state: 'visible', timeout: 20_000 });
        const before = await findAnalyst(page);
        memberId = before.member_id;
        if (before.business_role !== original) {
          // A run that died before its cleanup; put the fixture back.
          await setAnalystRole(page, memberId, original);
          await page.reload({ waitUntil: 'domcontentloaded' });
        }

        const catRes = await adminCall(page, 'GET', 'rbac/business-roles');
        expect(catRes.ok(), `GET /rbac/business-roles: ${catRes.status()}`).toBeTruthy();
        const catalog = ((await catRes.json()) as { business_roles: BusinessRole[] }).business_roles;
        const labelOf = (key: string) => {
          const r = catalog.find((b) => b.key === key);
          expect(r, `business role ${key} is in the catalogue`).toBeTruthy();
          return exactAny([(r as BusinessRole).label_fr, (r as BusinessRole).label_en]);
        };

        // ---- BEFORE, member side: the seeded role's view. ------------------
        member = await memberContext(browser, testInfo);
        const memberPage = await member.newPage();
        await expectDashboardView(memberPage, VARIANT[original]);
        await expect(
          memberPage.getByRole('heading', { level: 2, name: priorityHeading(VARIANT[target]) }),
        ).toHaveCount(0);

        // ---- The change, through the admin's members screen. --------------
        const row = page.getByTestId('member-row').filter({ hasText: ANALYST.email });
        await expect(row).toHaveCount(1, { timeout: 20_000 });
        const control = row.getByRole('combobox', {
          name: new RegExp(`^(Rôle de|Role for) ${escapeRe(ANALYST.email)}$`),
        });
        await expect(control).toHaveText(labelOf(original));
        await control.click();
        await page.getByRole('option', { name: labelOf(target) }).click();

        // The screen confirms it, and the control now shows the new role…
        await expect(page.getByText(/^(Rôle mis à jour|Role updated)$/)).toBeVisible({
          timeout: 15_000,
        });
        await expect(control).toHaveText(labelOf(target));
        // …and so does the server the screen claims to have written to.
        const after = await findAnalyst(page);
        expect(after.member_id).toBe(memberId);
        expect(after.org_role).toBe('user');
        expect(after.business_role).toBe(target);

        // ---- AFTER, member side. ------------------------------------------
        // The open session's access cookie lapses (reproduced: see header).
        // Its renewal must be refused, because the role change revoked it, and
        // the member is sent to sign in rather than left on the old view.
        await member.clearCookies({ name: 'or_access' });
        await memberPage.goto('/', { waitUntil: 'domcontentloaded' });
        await expect(memberPage, 'the old session is not renewed after a role change').toHaveURL(
          /\/login/,
          { timeout: 20_000 },
        );

        // The member signs in again, through the sign-in screen.
        await memberPage.getByTestId('login-email').fill(ANALYST.email);
        await memberPage.getByTestId('login-password').fill(ANALYST.password);
        await memberPage.getByTestId('login-submit').click();

        // They land where the NEW role lands, and their dashboard is the new
        // role's view, not the analyst's.
        await expect(memberPage).toHaveURL(LANDING[target], { timeout: 20_000 });
        await expectDashboardView(memberPage, VARIANT[target]);
        await expect(
          memberPage.getByRole('heading', { level: 2, name: priorityHeading(VARIANT[original]) }),
        ).toHaveCount(0);
      } finally {
        await member?.close();
        // Cleanup through the API: the seeded role back, so a rerun and every
        // other spec start from the fixture.
        // Only when it moved: the server refuses a no-op change, and a refusal
        // here would hide the assertion that actually failed.
        if (memberId && (await findAnalyst(page)).business_role !== original) {
          await setAnalystRole(page, memberId, original);
          expect((await findAnalyst(page)).business_role).toBe(original);
        }
      }
    });
  });
});
