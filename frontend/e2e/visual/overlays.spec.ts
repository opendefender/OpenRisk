// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1

import { test, expect } from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, resolve } from 'node:path';

/**
 * Visual regression and accessibility for overlays, in both themes.
 *
 * Two guarantees, deliberately separate:
 *
 *  1. Every registered overlay renders identically to its reference in light
 *     and dark. This catches a token regression — the "modal is dark while the
 *     app is light" bug — at the pixel level rather than by inspection.
 *
 *  2. Every overlay listed in docs/ui/overlays.md is registered in the harness.
 *     A new modal added to the app fails CI until someone gives it a snapshot,
 *     which is what stops coverage silently falling behind the codebase.
 *
 * The second guarantee is the one that has to hold over time. The first only
 * covers what is registered.
 */

const here = dirname(fileURLToPath(import.meta.url));
const INVENTORY = resolve(here, '../../../docs/ui/overlays.md');

const THEMES = ['light', 'dark'] as const;

/** Overlays mounted in the harness. Keep in sync with harness.tsx. */
const REGISTERED = ['danger-confirm', 'impact-dialog'] as const;

/**
 * Overlays present in the app but not yet mounted in the harness.
 *
 * This list is the honest record of what the visual suite does NOT cover. It
 * exists so the coverage guard can be strict about *new* overlays without
 * pretending the existing backlog is done. It may only shrink — an entry is
 * removed by registering the overlay in harness.tsx, never by adding to it.
 */
const NOT_YET_REGISTERED = new Set<string>([
  'frontend/src/components/gamification/EnhancedNotificationCenter.tsx',
  'frontend/src/components/layout/AppHeader.tsx',
  'frontend/src/components/layout/CommandPalette.tsx',
  'frontend/src/components/layout/GlobalShortcuts.tsx',
  'frontend/src/components/layout/NotificationCenter.tsx',
  'frontend/src/components/layout/Sidebar.tsx',
  'frontend/src/components/search/AdvancedSearch.tsx',
  'frontend/src/features/ai/AiAuditReportButton.tsx',
  'frontend/src/features/assets/CreateAssetModal.tsx',
  'frontend/src/features/assets/EditAssetModal.tsx',
  'frontend/src/features/auth/MFAEnrollmentDialog.tsx',
  'frontend/src/features/auth/MFAPostAhaPrompt.tsx',
  'frontend/src/features/automation/AutomationPage.tsx',
  'frontend/src/features/automation/DryRunPanel.tsx',
  'frontend/src/features/automation/RuleEditorModal.tsx',
  'frontend/src/features/compliance/ComplianceModals.tsx',
  'frontend/src/features/compliance/CreateControlModal.tsx',
  'frontend/src/features/compliance/CreateFrameworkModal.tsx',
  'frontend/src/features/compliance/FrameworkDetail.tsx',
  'frontend/src/features/compliance/ImportCatalogModal.tsx',
  'frontend/src/features/cti/CveDetailDrawer.tsx',
  'frontend/src/features/evidence/CreateEvidenceModal.tsx',
  'frontend/src/features/evidence/EvidenceDrawer.tsx',
  'frontend/src/features/financial/FinancialDashboard.tsx',
  'frontend/src/features/governance/GovernancePage.tsx',
  'frontend/src/features/incidents/DeclareIncidentModal.tsx',
  'frontend/src/features/incidents/IncidentDrawer.tsx',
  'frontend/src/features/incidents/PostMortemPanel.tsx',
  'frontend/src/features/incidents/WarRoom.tsx',
  'frontend/src/features/infrastructure/AgentDeployModal.tsx',
  'frontend/src/features/infrastructure/ScanConfigDrawer.tsx',
  'frontend/src/features/mitigations/CreateMitigationModal.tsx',
  'frontend/src/features/mitigations/MitigationDetailDrawer.tsx',
  'frontend/src/features/mitigations/MitigationEditModal.tsx',
  'frontend/src/features/mitigations/MitigationsBoard.tsx',
  'frontend/src/features/onboarding/ProductTour.tsx',
  'frontend/src/features/organization/MembersView.tsx',
  'frontend/src/features/reports/FrameworkPickerDialog.tsx',
  'frontend/src/features/reports/ReportConfigurator.tsx',
  'frontend/src/features/risks/CreateRiskModal.tsx',
  'frontend/src/features/risks/RiskRegisterPage.tsx',
  'frontend/src/features/risks/components/EditRiskModal.tsx',
  'frontend/src/features/users/CreateUserModal.tsx',
  'frontend/src/features/vulnerabilities/IngestModal.tsx',
  'frontend/src/features/vulnerabilities/IntegrationsPanel.tsx',
  'frontend/src/features/vulnerabilities/VulnerabilitiesPage.tsx',
  'frontend/src/pages/CustomFields.tsx',
  'frontend/src/pages/Marketplace.tsx',
  'frontend/src/pages/RoleManagement.tsx',
  'frontend/src/shared/FieldHelp.tsx',
  'frontend/src/shared/Hint.tsx',
  'frontend/src/shared/InfoHint.tsx',
  'frontend/src/shared/ScoreExplainer.tsx',
  'frontend/src/shared/ShortcutsOverlay.tsx',
  'frontend/src/shared/UserPicker.tsx',
  'frontend/src/shared/ds/Drawer.tsx',
  'frontend/src/shared/ds/Modal.tsx',
]);


/**
 * Web fonts are fetched at runtime (Inter / DM Sans / JetBrains Mono). A
 * screenshot taken before they land captures the fallback face, so the same
 * page yields two different images depending on cache state — a diff that looks
 * like a typography regression and is not. Waiting for document.fonts is what
 * makes the suite deterministic.
 */
async function waitForFonts(page: import('@playwright/test').Page) {
  await page.evaluate(() => document.fonts.ready);
}

function harnessURL(overlay: string, theme: string) {
  return `/e2e/visual/harness.html?overlay=${overlay}&theme=${theme}`;
}

for (const overlay of REGISTERED) {
  for (const theme of THEMES) {
    test(`${overlay} — ${theme}`, async ({ page }) => {
      await page.goto(harnessURL(overlay, theme));

      // Wait for the overlay itself, not just load: screenshotting during mount
      // produces a diff that looks like a colour regression but is a race.
      await page.waitForSelector('[role="dialog"], .fixed.inset-0', { state: 'visible' });
      await waitForFonts(page);

      // Confirms the harness honoured the requested theme. Without this a bug
      // that ignores ?theme would make both snapshots identical and pass.
      await expect(page.locator('html')).toHaveAttribute('data-theme', theme);

      await expect(page).toHaveScreenshot(`${overlay}-${theme}.png`, { fullPage: true });
    });

    test(`${overlay} — ${theme} — accessibility`, async ({ page }) => {
      await page.goto(harnessURL(overlay, theme));
      await page.waitForSelector('[role="dialog"], .fixed.inset-0', { state: 'visible' });

      const results = await new AxeBuilder({ page })
        // colour-contrast is the rule that matters here: it is the automated
        // check that the tokens actually produce readable text once composited,
        // rather than only on paper in check-contrast.mjs.
        .withTags(['wcag2a', 'wcag2aa'])
        .analyze();

      const serious = results.violations.filter(
        (v) => v.impact === 'critical' || v.impact === 'serious',
      );

      expect(
        serious,
        `axe found ${serious.length} serious/critical violations in ${overlay} (${theme}):\n` +
          serious.map((v) => `  ${v.id}: ${v.help}`).join('\n'),
      ).toEqual([]);
    });
  }
}

/**
 * The coverage guard. Parses the generated inventory and asserts every overlay
 * is either registered or explicitly acknowledged as unregistered.
 */
test('every overlay in the inventory has a snapshot or an explicit exemption', () => {
  const inventory = readFileSync(INVENTORY, 'utf8');

  const listed = [...inventory.matchAll(/^\| `([^`]+)` \|/gm)].map((m) => m[1]);
  expect(listed.length, 'inventory looks empty — regenerate it').toBeGreaterThan(10);

  // Map the two registered overlays back to their source files.
  const registeredFiles = new Set([
    'frontend/src/shared/DangerConfirm.tsx',
    'frontend/src/shared/ImpactDialog.tsx',
  ]);

  const unaccounted = listed.filter(
    (file) => !registeredFiles.has(file) && !NOT_YET_REGISTERED.has(file),
  );

  expect(
    unaccounted,
    'These overlays have no visual snapshot and are not listed as exempt.\n' +
      'Register them in e2e/visual/harness.tsx, or add them to NOT_YET_REGISTERED\n' +
      'with a reason:\n' +
      unaccounted.map((f) => `  ${f}`).join('\n'),
  ).toEqual([]);
});

/** Guards the exemption list against rotting as files are renamed or deleted. */
test('no stale exemptions', () => {
  const inventory = readFileSync(INVENTORY, 'utf8');
  const listed = new Set([...inventory.matchAll(/^\| `([^`]+)` \|/gm)].map((m) => m[1]));

  const stale = [...NOT_YET_REGISTERED].filter((f) => !listed.has(f));

  expect(
    stale,
    'These exemptions no longer match any overlay — remove them:\n' +
      stale.map((f) => `  ${f}`).join('\n'),
  ).toEqual([]);
});
