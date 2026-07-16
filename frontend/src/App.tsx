// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

import { useEffect, useState } from 'react';
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

// --- Imports des Pages & Features ---
import { AuthScreen } from './features/auth/AuthScreen';
import { SettingsScreen } from './features/settings/SettingsScreen';
import { DashboardPage } from './features/dashboard/DashboardPage';
import { ImportRisksPage } from './features/risks/ImportRisksPage';
import { RiskRegisterPage } from './features/risks/RiskRegisterPage';
import { VulnerabilitiesPage } from './features/vulnerabilities/VulnerabilitiesPage';
import { MitigationsBoard } from './features/mitigations/MitigationsBoard';
import { ComplianceScreen } from './features/compliance/ComplianceScreen';
import { FrameworkDetail } from './features/compliance/FrameworkDetail';
import { GapAnalysisPage } from './features/compliance/GapAnalysisPage';
import { AuditsPage } from './features/compliance/AuditsPage';
import { RemediationPage } from './features/compliance/RemediationPage';
import { InventoryPage } from './features/assets/InventoryPage';
import { AssetUniverse } from './features/universe/AssetUniverse';
import { AnalyticsCiso } from './features/analytics/AnalyticsCiso';
import { LeaderboardPage } from './features/gamification/LeaderboardPage';
import { WarRoom } from './features/incidents/WarRoom';
import { IncidentsScreen } from './features/incidents/IncidentsScreen';
import { ThreatIntel } from './features/cti/ThreatIntel';
import { InfrastructurePage } from './features/infrastructure/InfrastructurePage';
import { ScanPreviewPage } from './features/infrastructure/ScanPreviewPage';
import { SimulationsPage } from './features/simulations/SimulationsPage';
import { ReportsScreen } from './features/reports/ReportsScreen';
import { AiAdvisor } from './features/ai/AiAdvisor';
import { BoardReportPage } from './features/reports/BoardReportPage';
import RiskTimeline from './pages/RiskTimeline';

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
          <AnimatedOutlet />
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
          <Route path="risks/:riskId/timeline" element={<RiskTimeline />} />
          <Route path="analytics" element={<AnalyticsCiso />} />
          <Route path="leaderboard" element={<LeaderboardPage />} />
          <Route path="incidents" element={<IncidentsScreen />} />
          <Route path="incidents/:id/war-room" element={<WarRoom />} />
          <Route path="infrastructure" element={<InfrastructurePage />} />
          <Route path="infrastructure/scans/:jobId" element={<ScanPreviewPage />} />
          <Route path="threat-map" element={<ThreatIntel />} />
          <Route path="simulations" element={<SimulationsPage />} />
          <Route path="assets" element={<InventoryPage />} />
          <Route path="assets/universe" element={<AssetUniverse />} />
          <Route path="reports" element={<ReportsScreen />} />
          <Route path="reports/board" element={<BoardReportPage />} />
          <Route path="recommendations" element={<AiAdvisor />} />
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
