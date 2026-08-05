import js from '@eslint/js'
import globals from 'globals'
import reactHooks from 'eslint-plugin-react-hooks'
import reactRefresh from 'eslint-plugin-react-refresh'
import tseslint from 'typescript-eslint'
import { defineConfig, globalIgnores } from 'eslint/config'
import noRawColors from './eslint-rules/no-raw-colors.js'

/**
 * Local plugin. Kept in-repo rather than published: the rule encodes this
 * project's token vocabulary and has no meaning outside it.
 */
const openrisk = { rules: { 'no-raw-colors': noRawColors } }

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
])
