import js from '@eslint/js'
import globals from 'globals'
import reactHooks from 'eslint-plugin-react-hooks'
import reactRefresh from 'eslint-plugin-react-refresh'
import tseslint from 'typescript-eslint'
import { defineConfig, globalIgnores } from 'eslint/config'
import noRawColors from './eslint-rules/no-raw-colors.js'
import noMockData from './eslint-rules/no-mock-data.js'

/**
 * Local plugin. Kept in-repo rather than published: the rule encodes this
 * project's token vocabulary and has no meaning outside it.
 */
const openrisk = { rules: { 'no-raw-colors': noRawColors, 'no-mock-data': noMockData } }

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
      'src/features/dashboard/widgets/GlobalScore.tsx',
      'src/features/dashboard/widgets/RiskHeatmap.tsx',
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
      'src/features/rbac/RolesAccessPage.tsx',
      'src/features/risks/RiskDrawer.tsx',
      'src/features/risks/RiskListPage.tsx',
      'src/features/risks/RiskRegisterPage.tsx',
      'src/features/risks/components/RiskCard.tsx',
      'src/features/settings/GeneralTab.tsx',
      'src/features/settings/RBACTab.tsx',
      'src/features/settings/SettingsScreen.tsx',
      'src/features/settings/TeamTab.tsx',
      'src/features/simulations/SimulationsPage.tsx',
      'src/features/universe/AssetUniverse.tsx',
      'src/features/vulnerabilities/IngestModal.tsx',
      'src/features/vulnerabilities/IntegrationsPanel.tsx',
      'src/features/vulnerabilities/VulnerabilitiesPage.tsx',
      'src/components/dashboard/RBACDashboardWidget.tsx',
      'src/components/gamification/AchievementTrackingUI.tsx',
      'src/components/gamification/EnhancedNotificationCenter.tsx',
      'src/components/gamification/GamificationDashboard.tsx',
      'src/components/layout/PageHeader.tsx',
      'src/components/rbac/RoleTemplateBuilder.tsx',
      'src/components/shared/FloatingBulkBar.tsx',
      'src/components/shared/ScoreMeter.tsx',
      'src/components/shared/StatusDot.tsx',
      'src/components/shared/UserAvatar.tsx',
      'src/components/ui/Input.tsx',
      'src/pages/AIRiskInsights.tsx',
      'src/pages/Analytics.tsx',
      'src/pages/AnalyticsDashboard.tsx',
      'src/pages/AuditLogs.tsx',
      'src/pages/BulkOperations.tsx',
      'src/pages/ComplianceReportDashboard.tsx',
      'src/pages/CustomFields.tsx',
      'src/pages/ImportRisks.tsx',
      'src/pages/Login.tsx',
      'src/pages/Marketplace.tsx',
      'src/pages/MonitoringDashboard.tsx',
      'src/pages/PermissionAnalytics.tsx',
      'src/pages/RealTimeAnalyticsDashboard.tsx',
      'src/pages/Register.tsx',
      'src/pages/Reports.tsx',
      'src/pages/RiskTimeline.tsx',
      'src/pages/RoleManagement.tsx',
      'src/pages/TenantManagement.tsx',
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
])
