// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { useEffect, useState, lazy, Suspense } from 'react';
import { BrowserRouter, Routes, Route, Navigate, Outlet, useLocation } from 'react-router-dom';
import { motion, AnimatePresence } from 'framer-motion';

// --- Imports des Stores & Hooks ---
import { useAuthStore } from './hooks/useAuthStore';
import { useRiskStore } from './hooks/useRiskStore';

// --- App shell ---
import { Sidebar } from './components/layout/Sidebar';
import { AppHeader } from './components/layout/AppHeader';
import { CommandPalette } from './components/layout/CommandPalette';
// The dc.html-redesign Create-Risk modal (crash-free, correct P×I×AC score scale).
// The older features/risks/components/CreateRiskModal embedded ScoreEngineVisualizer,
// which white-screened the whole app on a null risk_stats/matrix response.
import { CreateRiskModal } from './features/risks/CreateRiskModal';

// --- Public auth screen stays eager (first paint / login path) ---
import { AuthScreen } from './features/auth/AuthScreen';

// --- Feature pages are route-split with React.lazy so the initial bundle only
//     carries the shell + auth; each screen's chunk loads on navigation. This
//     cut the single ~1.5 MB bundle into per-route chunks. ---
const SettingsScreen = lazy(() => import('./features/settings/SettingsScreen').then(m => ({ default: m.SettingsScreen })));
const DashboardPage = lazy(() => import('./features/dashboard/DashboardPage').then(m => ({ default: m.DashboardPage })));
const ImportRisksPage = lazy(() => import('./features/risks/ImportRisksPage').then(m => ({ default: m.ImportRisksPage })));
const RiskRegisterPage = lazy(() => import('./features/risks/RiskRegisterPage').then(m => ({ default: m.RiskRegisterPage })));
const RiskWeightsSettings = lazy(() => import('./features/risks/RiskWeightsSettings').then(m => ({ default: m.RiskWeightsSettings })));
const VulnerabilitiesPage = lazy(() => import('./features/vulnerabilities/VulnerabilitiesPage').then(m => ({ default: m.VulnerabilitiesPage })));
const MitigationsBoard = lazy(() => import('./features/mitigations/MitigationsBoard').then(m => ({ default: m.MitigationsBoard })));
const ComplianceScreen = lazy(() => import('./features/compliance/ComplianceScreen').then(m => ({ default: m.ComplianceScreen })));
const FrameworkDetail = lazy(() => import('./features/compliance/FrameworkDetail').then(m => ({ default: m.FrameworkDetail })));
const GapAnalysisPage = lazy(() => import('./features/compliance/GapAnalysisPage').then(m => ({ default: m.GapAnalysisPage })));
const AuditsPage = lazy(() => import('./features/compliance/AuditsPage').then(m => ({ default: m.AuditsPage })));
const RemediationPage = lazy(() => import('./features/compliance/RemediationPage').then(m => ({ default: m.RemediationPage })));
const InventoryPage = lazy(() => import('./features/assets/InventoryPage').then(m => ({ default: m.InventoryPage })));
const AssetUniverse = lazy(() => import('./features/universe/AssetUniverse').then(m => ({ default: m.AssetUniverse })));
const ExecutiveDashboard = lazy(() => import('./features/analytics/ExecutiveDashboard').then(m => ({ default: m.ExecutiveDashboard })));
const FinancialDashboard = lazy(() => import('./features/financial/FinancialDashboard').then(m => ({ default: m.FinancialDashboard })));
const AutomationPage = lazy(() => import('./features/automation/AutomationPage').then(m => ({ default: m.AutomationPage })));
const GovernancePage = lazy(() => import('./features/governance/GovernancePage').then(m => ({ default: m.GovernancePage })));
const RolesAccessPage = lazy(() => import('./features/rbac/RolesAccessPage').then(m => ({ default: m.RolesAccessPage })));
const LeaderboardPage = lazy(() => import('./features/gamification/LeaderboardPage').then(m => ({ default: m.LeaderboardPage })));
const WarRoom = lazy(() => import('./features/incidents/WarRoom').then(m => ({ default: m.WarRoom })));
const IncidentsScreen = lazy(() => import('./features/incidents/IncidentsScreen').then(m => ({ default: m.IncidentsScreen })));
const ThreatIntel = lazy(() => import('./features/cti/ThreatIntel').then(m => ({ default: m.ThreatIntel })));
const InfrastructurePage = lazy(() => import('./features/infrastructure/InfrastructurePage').then(m => ({ default: m.InfrastructurePage })));
const ScanPreviewPage = lazy(() => import('./features/infrastructure/ScanPreviewPage').then(m => ({ default: m.ScanPreviewPage })));
const SimulationsPage = lazy(() => import('./features/simulations/SimulationsPage').then(m => ({ default: m.SimulationsPage })));
const ReportsScreen = lazy(() => import('./features/reports/ReportsScreen').then(m => ({ default: m.ReportsScreen })));
const AiAdvisor = lazy(() => import('./features/ai/AiAdvisor').then(m => ({ default: m.AiAdvisor })));
const EmergingRisksPage = lazy(() => import('./features/ai/EmergingRisksPage').then(m => ({ default: m.EmergingRisksPage })));
const BoardReportPage = lazy(() => import('./features/reports/BoardReportPage').then(m => ({ default: m.BoardReportPage })));
const RiskTimeline = lazy(() => import('./pages/RiskTimeline'));

/**
 * COMPOSANT 1: PROTECTION DE ROUTE
 * Vérifie si le token existe, sinon redirige vers Login.
 */
const ProtectedRoute = ({ children }: { children: React.ReactNode }) => {
  const token = useAuthStore((state) => state.token);
  if (!token) return <Navigate to="/login" replace />;
  return <>{children}</>;
};

/**
 * Fades/slides the route content on every navigation, keyed by pathname, so
 * switching pages never feels like an abrupt jump-cut.
 */
const AnimatedOutlet = () => {
  const location = useLocation();
  return (
    <AnimatePresence mode="wait">
      <motion.div
        key={location.pathname}
        initial={{ opacity: 0, y: 6 }}
        animate={{ opacity: 1, y: 0 }}
        exit={{ opacity: 0 }}
        transition={{ duration: 0.18, ease: 'easeOut' }}
        className="flex-1 flex flex-col min-h-0 h-full"
      >
        <Outlet />
      </motion.div>
    </AnimatePresence>
  );
};

/**
 * COMPOSANT 2: LAYOUT GLOBAL — App Shell (OpenRisk.dc.html §5)
 * Grouped Sidebar (static ≥ lg, off-canvas drawer < lg) + glass AppHeader +
 * scrollable body. The ⌘K command palette and a global "New risk" modal live
 * here so they're reachable from anywhere (sidebar quick action, palette).
 */
const DashboardLayout = () => {
  const [mobileNavOpen, setMobileNavOpen] = useState(false);
  const [newRiskOpen, setNewRiskOpen] = useState(false);

  // The sidebar quick action and command palette dispatch this to open the modal.
  useEffect(() => {
    const open = () => setNewRiskOpen(true);
    window.addEventListener('openrisk:new-risk', open);
    return () => window.removeEventListener('openrisk:new-risk', open);
  }, []);

  return (
    <div className="flex h-screen bg-app text-ink overflow-hidden font-sans selection:bg-accent-soft">
      <Sidebar mobileOpen={mobileNavOpen} onMobileClose={() => setMobileNavOpen(false)} />
      <div className="flex-1 flex flex-col h-screen overflow-hidden relative min-w-0" style={{ background: 'var(--bg-primary)' }}>
        <AppHeader onOpenMobileNav={() => setMobileNavOpen(true)} />
        <main className="flex-1 overflow-hidden relative flex flex-col">
          <Suspense fallback={<RouteFallback />}>
            <AnimatedOutlet />
          </Suspense>
        </main>
      </div>

      {/* Global shell overlays */}
      <CommandPalette />
      <CreateRiskModal
        isOpen={newRiskOpen}
        onClose={() => setNewRiskOpen(false)}
        onCreated={() => { void useRiskStore.getState().fetchRisks(); }}
      />
    </div>
  );
};

/**
 * Fallback shown while a route's lazy chunk is being fetched. Kept minimal and
 * theme-aware — route chunks are small so this is only visible briefly.
 */
function RouteFallback() {
  return (
    <div className="flex-1 flex items-center justify-center" style={{ background: 'var(--bg-primary)' }}>
      <div
        className="h-8 w-8 rounded-full animate-spin"
        style={{ border: '3px solid var(--border-subtle)', borderTopColor: 'var(--accent, #2e6be6)' }}
        role="status"
        aria-label="Chargement…"
      />
    </div>
  );
}

/**
 * COMPOSANT PRINCIPAL : APP ROUTER
 */
function App() {
  return (
    <BrowserRouter>
      <Routes>
        {/* Routes Publiques */}
        <Route path="/login" element={<AuthScreen initialView="login" />} />
        <Route path="/register" element={<AuthScreen initialView="register" />} />

        {/* Routes Protégées (Layout Global) */}
        <Route
          element={
            <ProtectedRoute>
              <DashboardLayout />
            </ProtectedRoute>
          }
        >
          <Route index element={<DashboardPage />} />
          <Route path="risks" element={<RiskRegisterPage />} />
          <Route path="vulnerabilities" element={<VulnerabilitiesPage />} />
          <Route path="mitigations" element={<MitigationsBoard />} />
          <Route path="compliance" element={<ComplianceScreen />} />
          <Route path="compliance/gap-analysis" element={<GapAnalysisPage />} />
          <Route path="compliance/audits" element={<AuditsPage />} />
          <Route path="compliance/remediations" element={<RemediationPage />} />
          <Route path="compliance/:frameworkId" element={<FrameworkDetail />} />
          <Route path="risks/import" element={<ImportRisksPage />} />
          <Route path="risks/weighting" element={<RiskWeightsSettings />} />
          <Route path="risks/:riskId/timeline" element={<RiskTimeline />} />
          <Route path="analytics" element={<ExecutiveDashboard />} />
          <Route path="analytics/financial" element={<FinancialDashboard />} />
          <Route path="leaderboard" element={<LeaderboardPage />} />
          <Route path="incidents" element={<IncidentsScreen />} />
          <Route path="incidents/:id/war-room" element={<WarRoom />} />
          <Route path="automation" element={<AutomationPage />} />
          <Route path="infrastructure" element={<InfrastructurePage />} />
          <Route path="infrastructure/scans/:jobId" element={<ScanPreviewPage />} />
          <Route path="threat-map" element={<ThreatIntel />} />
          <Route path="simulations" element={<SimulationsPage />} />
          <Route path="assets" element={<InventoryPage />} />
          <Route path="assets/universe" element={<AssetUniverse />} />
          <Route path="reports" element={<ReportsScreen />} />
          <Route path="reports/board" element={<BoardReportPage />} />
          <Route path="recommendations" element={<AiAdvisor />} />
          <Route path="ai/emerging-risks" element={<EmergingRisksPage />} />
          <Route path="governance" element={<GovernancePage />} />
          <Route path="settings/roles" element={<RolesAccessPage />} />
          <Route path="settings" element={<SettingsScreen />} />

          {/* Admin features consolidated into Settings — old routes redirect there
              so existing deep links keep working. Risk-management / bulk-ops fold
              into the Risk Register (which now carries the bulk action bar). */}
          <Route path="users" element={<Navigate to="/settings" replace />} />
          <Route path="roles" element={<Navigate to="/settings" replace />} />
          <Route path="tenants" element={<Navigate to="/settings" replace />} />
          <Route path="audit-logs" element={<Navigate to="/settings" replace />} />
          <Route path="tokens" element={<Navigate to="/settings" replace />} />
          <Route path="marketplace" element={<Navigate to="/settings" replace />} />
          <Route path="custom-fields" element={<Navigate to="/settings" replace />} />
          <Route path="analytics/permissions" element={<Navigate to="/settings" replace />} />
          <Route path="risk-management" element={<Navigate to="/risks" replace />} />
          <Route path="bulk-operations" element={<Navigate to="/risks" replace />} />
        </Route>

        {/* Redirection par défaut */}
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </BrowserRouter>
  );
}

export default App;
