import js from '@eslint/js'
import globals from 'globals'
import reactHooks from 'eslint-plugin-react-hooks'
import reactRefresh from 'eslint-plugin-react-refresh'
import tseslint from 'typescript-eslint'
import { defineConfig, globalIgnores } from 'eslint/config'
import noRawColors from './eslint-rules/no-raw-colors.js'
import noMockData from './eslint-rules/no-mock-data.js'
import noAdHocDesignValues from './eslint-rules/no-ad-hoc-design-values.js'
import noDesignCliches from './eslint-rules/no-design-cliches.js'

/**
 * Local plugin. Kept in-repo rather than published: the rule encodes this
 * project's token vocabulary and has no meaning outside it.
 */
const openrisk = {
  rules: {
    'no-raw-colors': noRawColors,
    'no-mock-data': noMockData,
    'no-ad-hoc-design-values': noAdHocDesignValues,
    'no-design-cliches': noDesignCliches,
  },
}

export default defineConfig([
  globalIgnores(['dist']),
  {
    files: ['**/*.{ts,tsx}'],
    extends: [
      js.configs.recommended,
      tseslint.configs.recommended,
      reactHooks.configs.flat.recommended,
      reactRefresh.configs.vite,
    ],
    languageOptions: {
      ecmaVersion: 2020,
      globals: globals.browser,
    },
  },
  {
    // The Playwright harness and the design-system gallery are test fixtures,
    // not application components: Vite never fast-refreshes them, so the rule
    // about exporting constants beside a component does not apply.
    files: ['e2e/**/*.{ts,tsx}'],
    rules: { 'react-refresh/only-export-components': 'off' },
  },
  {
    // The theme guard, at error level: a raw colour fails the build.
    //
    // Every overlay and every shared UI primitive is covered, which is what
    // matters — an overlay is where a hardcoded dark panel is most tempting and
    // where the reported bug lived.
    files: [
      'src/components/**/*.{ts,tsx}',
      'src/pages/**/*.{ts,tsx}',
      'src/shared/**/*.{ts,tsx}',
      // features/ holds most of the modals, so leaving it out would have made
      // the guard miss the very files the bug was reported against. Live
      // verification found a governance modal reading var(--surface, #fff) —
      // an undefined variable whose fallback rendered white in both themes.
      'src/features/**/*.{ts,tsx}',
    ],
    // Legacy screens not yet migrated. Their remaining colours are chart series
    // hues and decorative gradients, which the colour codemod deliberately
    // refuses to guess at because a wrong guess silently changes a design.
    //
    // This list may only shrink. Listing them explicitly keeps the rule at
    // error for everything else; the alternative — dropping the whole rule to
    // 'warn' — would let new violations in everywhere to accommodate old ones
    // here, which is how the 1600 got there in the first place.
    ignores: [
      'src/features/ai/AiAuditReportButton.tsx',
      'src/features/ai/AiEvidenceAnalysis.tsx',
      'src/features/ai/EmergingRisksPage.tsx',
      'src/features/assets/AssetsPage.tsx',
      'src/features/auth/AuthScreen.tsx',
      'src/features/automation/AutomationPage.tsx',
      'src/features/automation/RuleEditorModal.tsx',
      'src/features/compliance/AuditsPage.tsx',
      'src/features/compliance/ComplianceModals.tsx',
      'src/features/compliance/CompliancePage.tsx',
      'src/features/compliance/ControlMappingsSection.tsx',
      'src/features/compliance/CreateControlModal.tsx',
      'src/features/compliance/CreateFrameworkModal.tsx',
      'src/features/compliance/FrameworkDetail.tsx',
      'src/features/compliance/GapAnalysisPage.tsx',
      'src/features/compliance/ImportCatalogModal.tsx',
      'src/features/compliance/RemediationPage.tsx',
      'src/features/cti/ThreatIntel.tsx',
      'src/features/dashboard/DashboardPage.tsx',
      'src/features/financial/FinancialDashboard.tsx',
      'src/features/gamification/LeaderboardPage.tsx',
      'src/features/gamification/UserLevelCard.tsx',
      'src/features/governance/GovernancePage.tsx',
      'src/features/incidents/IncidentDrawer.tsx',
      'src/features/incidents/IncidentsScreen.tsx',
      'src/features/incidents/WarRoom.tsx',
      'src/features/infrastructure/AgentDeployModal.tsx',
      'src/features/infrastructure/ScanConfigDrawer.tsx',
      'src/features/mitigations/MitigationCard.tsx',
      'src/features/mitigations/MitigationsBoard.tsx',
      'src/features/notifications/NotificationCategoryPrefs.tsx',
      'src/features/onboarding/OnboardingChecklist.tsx',
      'src/features/onboarding/PersonalizeCard.tsx',
      'src/features/risks/RiskRegisterPage.tsx',
      'src/features/risks/components/RiskCard.tsx',
      'src/features/settings/GeneralTab.tsx',
      'src/features/settings/RBACTab.tsx',
      'src/features/settings/SettingsScreen.tsx',
      'src/features/settings/TeamTab.tsx',
      'src/features/simulations/SimulationsPage.tsx',
      'src/features/vulnerabilities/IngestModal.tsx',
      'src/features/vulnerabilities/IntegrationsPanel.tsx',
      'src/features/vulnerabilities/VulnerabilitiesPage.tsx',
      'src/components/dashboard/RBACDashboardWidget.tsx',
      'src/components/gamification/AchievementTrackingUI.tsx',
      'src/components/gamification/EnhancedNotificationCenter.tsx',
      'src/components/gamification/GamificationDashboard.tsx',
      'src/components/layout/PageHeader.tsx',
      'src/components/shared/FloatingBulkBar.tsx',
      'src/components/shared/ScoreMeter.tsx',
      'src/components/shared/StatusDot.tsx',
      'src/components/shared/UserAvatar.tsx',
      'src/pages/Analytics.tsx',
      'src/pages/AnalyticsDashboard.tsx',
      'src/pages/AuditLogs.tsx',
      'src/pages/BulkOperations.tsx',
      'src/pages/ComplianceReportDashboard.tsx',
      'src/pages/CustomFields.tsx',
      'src/pages/ImportRisks.tsx',
      'src/pages/Login.tsx',
      'src/pages/Marketplace.tsx',
      'src/pages/Register.tsx',
      'src/pages/Reports.tsx',
      'src/pages/RoleManagement.tsx',
      'src/pages/ThreatMap.tsx',
      'src/pages/TokenManagement.tsx',
      'src/pages/Users.tsx',
      'src/shared/ui.tsx',
    ],
    plugins: { openrisk },
    rules: {
      'openrisk/no-raw-colors': 'error',
    },
  },
  {
    // The mock guard, at error level: fabricated data fails the build.
    //
    // Scoped to application source only. Tests are excluded because mocking is
    // what a unit test is for — the point of the rule is that a *screen* must
    // never invent what it displays, not that the word "mock" is banned.
    files: ['src/**/*.{ts,tsx}'],
    ignores: [
      'src/**/__tests__/**',
      'src/**/*.test.{ts,tsx}',
      'src/**/*.spec.{ts,tsx}',
      'src/test/**',
      'src/utils/rbacTestUtils.ts',
    ],
    plugins: { openrisk },
    rules: {
      'openrisk/no-mock-data': 'error',
    },
  },
  {
    // The token guard, at error level: an ad-hoc font size, radius, z-index or
    // duration fails the build.
    //
    // An ALLOWLIST, not an ignore list. The migration off 1559 arbitrary values
    // is partial and this records exactly how far it has got: a file joins the
    // list when it has been migrated, and the list only grows. The alternative
    // — enable it everywhere and drop the level to 'warn' to accommodate the
    // backlog — is what let the backlog happen.
    files: [
      'src/shared/ds/**/*.{ts,tsx}',
      'src/shared/DangerConfirm.tsx',
      'src/shared/ImpactDialog.tsx',
      'src/shared/EmptyState.tsx',
      'src/shared/AccessDenied.tsx',
    ],
    plugins: { openrisk },
    rules: {
      'openrisk/no-ad-hoc-design-values': 'error',
    },
  },
  {
    // The anti-cliché guard, at error level. The design guide's own
    // contribution rule 1: "Lint rule, not a code review preference."
    //
    // Scoped to src/** and NOT to src/components/**: the primitive layer is
    // src/shared/ds/ (src/components/ui/ was deleted on 2026-08-25 by 96a4a57
    // and #443 is forbidden to recreate it), so a components-only glob would
    // cover none of the ~50 components #443 vendors — the exact reason #443 is
    // blocked behind this issue.
    files: ['src/**/*.{ts,tsx}'],
    // Screens carrying the cliché today. This list may ONLY shrink; a file
    // leaves it by being fixed, never by being added.
    //
    // src/shared/ds/** is deliberately absent and must stay absent: it is
    // clean today, and it is where the vendored primitives land. Acceptance
    // criterion 1 of this issue is exactly that absence.
    ignores: [
      '**/__tests__/**',
      '**/*.test.{ts,tsx}',
      '**/*.spec.{ts,tsx}',
      'src/components/gamification/AchievementTrackingUI.tsx',
      'src/components/gamification/EnhancedNotificationCenter.tsx',
      'src/components/gamification/GamificationDashboard.tsx',
      'src/components/layout/NotificationCenter.tsx',
      'src/components/layout/PageHeader.tsx',
      'src/components/shared/FloatingBulkBar.tsx',
      'src/components/shared/ScoreMeter.tsx',
      'src/components/shared/UserAvatar.tsx',
      'src/features/assets/AssetHistoryDrawer.tsx',
      'src/features/assets/AssetsPage.tsx',
      'src/features/attackSurface/AssetSchemaSettings.tsx',
      'src/features/billing/BillingPanel.tsx',
      'src/features/compliance/ComplianceModals.tsx',
      'src/features/compliance/CompliancePage.tsx',
      'src/features/compliance/ControlDrawer.tsx',
      'src/features/compliance/ImportCatalogModal.tsx',
      'src/features/gamification/UserLevelCard.tsx',
      'src/features/mitigations/MitigationKanbanPage.tsx',
      'src/features/reports/BoardReportPage.tsx',
      'src/features/risks/ComplianceMappingField.tsx',
      'src/features/risks/LifecycleStepper.tsx',
      'src/features/settings/GeneralTab.tsx',
      'src/features/users/CreateUserModal.tsx',
      'src/pages/ComplianceReportDashboard.tsx',
      'src/pages/Login.tsx',
      'src/pages/Marketplace.tsx',
      'src/pages/Register.tsx',
      'src/pages/RoleManagement.tsx',
      'src/pages/ThreatMap.tsx',
      'src/pages/TokenManagement.tsx',
      'src/pages/Users.tsx',
    ],
    plugins: { openrisk },
    rules: {
      'openrisk/no-design-cliches': 'error',
    },
  },
  {
    // Import deny-list from the design guide. Purely preventive: none of these
    // is imported anywhere today, so the rule needs no exemption and starts
    // life at error with a clean tree.
    //
    // The chart bans are about honesty rather than looks — a gauge, a ring and
    // a sunburst all encode a magnitude as an angle, which people read far less
    // accurately than a length. A GRC console reports numbers a regulator will
    // ask about; it uses bars.
    files: ['src/**/*.{ts,tsx}'],
    rules: {
      'no-restricted-imports': [
        'error',
        {
          paths: [
            {
              name: '@number-flow/react',
              message:
                'Animated number tickers make a figure unreadable while it settles. Render the value; if it must change visibly, change it once.',
            },
            {
              name: 'motion',
              message:
                "Use 'framer-motion', already a dependency. Two animation runtimes in one bundle is the size budget spent twice.",
            },
          ],
          patterns: [
            {
              group: ['**/GaugeChart', '**/RingChart', '**/SunburstChart', '**/LiveLineChart'],
              message:
                'Angle-encoded and self-updating charts are banned by the design guide: they read a magnitude less accurately than a bar, and a live line implies data the console does not stream. Use the bar/area primitives in src/shared/ds.',
            },
          ],
        },
      ],
    },
  },
])
