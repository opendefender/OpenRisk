// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The live stream renews an expired session on its own (#701).
//
// EventSource exposes no status code, so once the 15-minute access cookie
// lapsed an idle page lost its live updates until the user clicked something.
// The realtime client now reads the 401 from its probe and renews through the
// same single-flight refresh as axios, or signs out like axios when the
// session is really over.
//
// How the stream is made to reconnect: the context goes offline and back
// online, which closes the stream and reopens it at once. Waiting for a real
// drop would tie the spec to the server's stream lifetime and to #962.
//
// Why the account signs in through the screen: the sign-in keeps the access
// token in memory, so axios stays authenticated by its bearer header after the
// cookie is gone. Any /auth/refresh seen here therefore comes from the stream,
// and the spec checks that no other API call was refused before it.

import { test, expect, type Page, type BrowserContext } from "@playwright/test";
import { API_URL } from "./support/env";
import { signUp, type Newcomer } from "./support/newcomer";

let account: Newcomer;

// A tenant of its own per test, past the wizard: a brand-new tenant is held on
// /onboarding, which never opens the live stream.
test.beforeEach(async ({ request }) => {
  account = await signUp(request, "rt");
  const done = await request.post(`${API_URL}/onboarding/complete`, {
    headers: { Authorization: `Bearer ${account.token}` },
    data: {},
  });
  expect(done.status(), await done.text()).toBe(200);
});

type Seen = { at: number; status: number; method: string; path: string };

/** Every API response the page receives, in order. */
function recordApi(page: Page): Seen[] {
  const seen: Seen[] = [];
  page.on("response", (r) => {
    const url = new URL(r.url());
    if (!url.pathname.startsWith("/api/v1/")) return;
    seen.push({
      at: Date.now(),
      status: r.status(),
      method: r.request().method(),
      path: url.pathname.replace("/api/v1", ""),
    });
  });
  return seen;
}

async function signIn(page: Page) {
  await page.goto("/login");
  await page.getByTestId("login-email").fill(account.email);
  await page.getByTestId("login-password").fill(account.password);
  await page.getByTestId("login-submit").click();
  await expect(page).not.toHaveURL(/\/login/);
}

/** Waits for the first live stream of the page to open. */
async function streamOpen(page: Page) {
  await page.waitForResponse(
    (r) => r.url().includes("/realtime/events") && r.status() === 200,
    { timeout: 15_000 },
  );
}

/** Drops the stream and lets it reconnect straight away. */
async function bounceNetwork(page: Page, context: BrowserContext) {
  await context.setOffline(true);
  await page.waitForFunction(() => !navigator.onLine);
  await context.setOffline(false);
  await page.waitForFunction(() => navigator.onLine);
}

async function sessionCookies(context: BrowserContext): Promise<string[]> {
  return (await context.cookies())
    .map((c) => c.name)
    .filter((n) => n.startsWith("or_"))
    .sort();
}

test.describe("live stream with an expired session", () => {
  test("renews the session once and the stream is live again, with no user action", async ({
    page,
    context,
  }) => {
    const seen = recordApi(page);
    const opened = streamOpen(page);
    await signIn(page);
    await opened;

    // What the browser does once the 15-minute access cookie lapses.
    await context.clearCookies({ name: "or_access" });
    const from = Date.now();
    await bounceNetwork(page, context);

    await expect
      .poll(
        () =>
          seen.filter((s) => s.at >= from && s.path === "/auth/refresh").length,
        {
          message: "the stream never renewed the session",
          timeout: 15_000,
        },
      )
      .toBeGreaterThan(0);
    const refresh = seen.find(
      (s) => s.at >= from && s.path === "/auth/refresh",
    )!;
    expect(refresh.status, "the refresh must succeed").toBe(200);

    // The stream comes back on the renewed cookie.
    await expect
      .poll(
        () =>
          seen.some(
            (s) =>
              s.at >= refresh.at &&
              s.path === "/realtime/events" &&
              s.status === 200,
          ),
        {
          message: "the stream did not reopen after the refresh",
          timeout: 15_000,
        },
      )
      .toBe(true);

    // The refresh came from the stream: its 401 is the only refusal before it.
    const before = seen.filter(
      (s) => s.at >= from && s.at <= refresh.at && s.status === 401,
    );
    expect(before.length, "the stream was never refused").toBeGreaterThan(0);
    expect(before.every((s) => s.path === "/realtime/events")).toBe(true);

    // One refresh, not one per reconnect attempt: give a retry loop time to show.
    await page.waitForTimeout(5_000);
    expect(
      seen.filter((s) => s.at >= from && s.path === "/auth/refresh"),
    ).toHaveLength(1);
    await expect(page).not.toHaveURL(/\/login/);
    expect(await sessionCookies(context)).toEqual([
      "or_access",
      "or_csrf",
      "or_refresh",
    ]);
  });

  test("signs out and stops reconnecting when the session cannot be renewed", async ({
    page,
    context,
  }) => {
    const seen = recordApi(page);
    const opened = streamOpen(page);
    await signIn(page);
    await opened;

    // The refresh cookie is gone too: expired, or revoked elsewhere.
    await context.clearCookies({ name: "or_access" });
    await context.clearCookies({ name: "or_refresh" });
    const from = Date.now();
    await bounceNetwork(page, context);

    await expect(page).toHaveURL(/\/login/, { timeout: 15_000 });
    const after = seen.filter((s) => s.at >= from);
    const refreshes = after.filter((s) => s.path === "/auth/refresh");
    expect(refreshes.map((s) => s.status)).toEqual([401]);
    const stream401 = after.findIndex(
      (s) => s.path === "/realtime/events" && s.status === 401,
    );
    expect(
      stream401,
      "the sign-out must follow the stream refusal",
    ).toBeGreaterThanOrEqual(0);
    expect(stream401).toBeLessThan(after.indexOf(refreshes[0]));

    // No retry loop once signed out.
    const landed = Date.now();
    await page.waitForTimeout(5_000);
    expect(
      seen.filter((s) => s.at >= landed && s.path === "/realtime/events"),
    ).toEqual([]);
  });
});
