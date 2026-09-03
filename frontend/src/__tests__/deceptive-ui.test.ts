// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// W0-05 — the deceptive-UI guards, asserted.
//
// `openrisk/no-mock-data` (ESLint) already forbids fabricated data by NAME:
// MOCK_RISKS, fakeUsers, sampleRows, placeholderData. That is the reliable
// signal for a fixture array, and it is why this wave found no `const people =
// [...]` in a production component.
//
// It could not see the three shapes this wave DID find, because none of them
// involves the word "mock":
//
//   1. a route that exists only as a placeholder,
//   2. a fixture module genuinely imported by production code,
//   3. a primary action rendered as available with no handler behind it.
//
// scripts/audit-surfaces.mjs derives all three from the import graph and the
// route table. These tests run it, so a regression fails the suite rather than
// waiting for someone to run the script by hand.

import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { audit } from '../../scripts/audit-surfaces.mjs';

// Derived from this file, not from cwd: vitest may be invoked from the repo
// root or from frontend/, and a path that depends on which would make the guard
// pass or fail for the wrong reason.
const SRC = join(dirname(fileURLToPath(import.meta.url)), '..');

/**
 * Reads a module with its comments blanked out.
 *
 * Without this, a guard that forbids a pattern also forbids EXPLAINING why the
 * pattern was removed — and the explanation is the most valuable line in the
 * diff. Newlines are preserved so reported line numbers stay true. Same
 * treatment the audit script gives before parsing button tags.
 */
const read = (p: string) =>
  readFileSync(join(SRC, p), 'utf8')
    .replace(/\/\*[\s\S]*?\*\//g, (m) => m.replace(/[^\n]/g, ' '))
    .replace(/\/\/[^\n]*/g, '');

describe('placeholder routes', () => {
  it('no route path reads as scaffolding', () => {
    // A `/demo`, `/mock` or `/coming-soon` route is a promise the product does
    // not keep. Legitimate exceptions are declared in the script with a reason,
    // so an exception is a decision recorded in review.
    expect(audit().placeholderRoutes).toEqual([]);
  });

  it('every legacy path is a redirect to a real destination, not an empty page', () => {
    const app = read('App.tsx');
    // These paths all predate a reorganisation. Each must still resolve — a
    // bookmark that 404s and a bookmark that renders an empty screen are both
    // dead ends, and the second one lies about why.
    const legacy = [
      'mitigations',
      'roles',
      'settings/roles',
      'settings/audit-log',
      'audit-logs',
      'users',
      'tenants',
      'tokens',
      'marketplace',
      'custom-fields',
      'risk-management',
      'bulk-operations',
      'assets/universe',
      'compliance/gap-analysis',
      'compliance/remediations',
    ];
    for (const path of legacy) {
      const route = new RegExp(
        `<Route\\s+path="${path.replace(/\//g, '\\/')}"[^>]*element=\\{<Navigate`,
      );
      expect(route.test(app), `${path} must redirect, not render`).toBe(true);
    }
  });

  it('the broken standalone risk-timeline page is gone, and its URL redirects', () => {
    // It hand-rolled a fetch without the session cookie, mis-read the response
    // envelope (so any risk WITH history white-screened), and rendered an auth
    // failure as "Total changes: 0" (W0-05 / D8).
    const app = read('App.tsx');
    expect(app).not.toMatch(/pages\/RiskTimeline/);
    expect(app).toMatch(/risks\/:riskId\/timeline"\s+element=\{<LegacyRiskTimelineRedirect/);
    expect(app).toMatch(/focus=\$\{riskId \?\? ''\}&tab=timeline/);
  });
});

describe('fixture leakage', () => {
  it('no test or demonstration module is reachable from main.tsx', () => {
    // Demonstration data lives in dev/fixtures/*.json, outside src/ and outside
    // the bundle, loaded by the Go seeder under DEMO_MODE only — which is why
    // this can be an absolute rule rather than an allowlist.
    expect(audit().fixtureLeakage).toEqual([]);
  });

  it('the orphan count does not grow', () => {
    // 110 modules are unreachable duplicates — old pages no route renders, and
    // the reservoir this wave's findings were drawn from (a settings
    // IntegrationsTab that simulates a connection with a 2s setTimeout, a
    // scoreEngineService with nine `catch → return fixture` fallbacks). They are
    // not user-facing, so they are not this wave's to delete; pinning the number
    // means it can only shrink.
    expect(audit().orphans).toBeLessThanOrEqual(110);
  });

  it('the settings screen no longer keeps preferences in localStorage', () => {
    // The prefs store followed the BROWSER rather than the person, so a switch
    // set by one user greeted the next one to sign in (W0-05 / D2).
    const settings = read('features/settings/SettingsScreen.tsx');
    expect(settings).not.toMatch(/settingsPrefs|useSettingsPrefs/);
    expect(settings).toMatch(/useNotificationPreferences/);
  });
});

describe('inert primary actions', () => {
  it('no button in a reachable component lacks a handler', () => {
    expect(audit().inertActions).toEqual([]);
  });

  it('no primary CTA has a toast as its entire effect', () => {
    // "New custom field" fired toast('Field editor — coming soon') and nothing
    // else. An empty state whose CTA does nothing reads as "you have not done
    // this yet" when the truth is "you cannot do this here" (W0-05 / D4).
    const settings = read('features/settings/SettingsScreen.tsx');
    expect(settings).not.toMatch(/toast\([^)]*coming soon/i);
    expect(settings).not.toMatch(/toast\([^)]*bient[oô]t/i);
  });
});

describe('no fake integrations', () => {
  const settings = read('features/settings/SettingsScreen.tsx');

  it('integration state is read from an API, never from a literal', () => {
    // The tab listed six providers with Slack, Teams and Splunk hardcoded to
    // enabled, on every tenant. A user who believes Slack is connected stops
    // watching Slack — the interface created a monitoring gap and hid it
    // (W0-05 / D1).
    expect(settings).toMatch(/useChannelConfig/);
    expect(settings).toMatch(/useVulnIntegrations/);
    expect(settings).toMatch(/useVulnTicketing/);
    // The literal, in the exact shape it had.
    expect(settings).not.toMatch(/\['Splunk',/);
    expect(settings).not.toMatch(/\['Microsoft Teams',[^\]]*true\]/);
  });

  it('a failed lookup renders "unknown", never "not connected"', () => {
    // Falling back to "not connected" is a guess dressed as a fact, and in this
    // tab a wrong guess in either direction is the bug.
    expect(settings).toMatch(/State unknown/);
  });

  it('the panel never reads a secret back out', () => {
    // The channel API exposes has_slack-style booleans precisely so a UI can say
    // "configured" without ever handling a webhook URL (RULE #6).
    expect(settings).not.toMatch(/slack_webhook_url|webhook_secret|sms_api_key/);
  });
});

describe('no simulated success', () => {
  it('no reachable component fakes an async operation with a timer', () => {
    // `await new Promise(r => setTimeout(r, 2000))` followed by a success state
    // is a spinner that resolves to a lie. (One such call survives in the
    // ORPHANED settings/IntegrationsTab, which no route renders — hence the
    // reachability filter rather than a repo-wide grep.)
    const reachable = [
      'features/settings/SettingsScreen.tsx',
      'features/incidents/WarRoom.tsx',
      'features/reports/ReportsScreen.tsx',
    ];
    for (const file of reachable) {
      expect(read(file), `${file} simulates work with a timer`).not.toMatch(
        /await new Promise\([^)]*setTimeout/,
      );
    }
  });

  it('the War Room composer that delivered nowhere is gone', () => {
    // It appended to useState and sent nothing. In an incident console that is
    // the worst place in the product to fake a send: somebody writes "@alice
    // take the DB offline", sees it appear, and believes it was communicated
    // (W0-05 / D6).
    const warRoom = read('features/incidents/WarRoom.tsx');
    expect(warRoom).not.toMatch(/ephemeral|éphémère/);
    expect(warRoom).not.toMatch(/setNotes/);
    // Replaced by the real, persisted action board.
    expect(warRoom).toMatch(/useIncidentActions/);
  });
});

describe('cross-identity isolation', () => {
  it('the auth store clears the session scope at both ends of a transition', () => {
    // Logout and login are both soft navigations, so the tab is never torn
    // down. Clearing on one side only leaves the gap the other way (W0-05 / D9).
    const store = read('hooks/useAuthStore.ts');
    const calls = store.match(/clearSessionScope\(\)/g) ?? [];
    expect(calls.length, 'expected login, adoptSession and logout to clear').toBeGreaterThanOrEqual(
      3,
    );
  });

  it('the query client is handed to the session scope at start-up', () => {
    // Without the registration the cache is unreachable from the auth store and
    // the clear silently does nothing.
    expect(read('main.tsx')).toMatch(/registerQueryClient\(queryClient\)/);
  });
});
