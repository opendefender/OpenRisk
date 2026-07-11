// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

import { useEffect, useState } from 'react';
import { BrowserRouter, Routes, Route, Navigate, Outlet, useLocation } from 'react-router-dom';
import { motion, AnimatePresence } from 'framer-motion';

// --- Imports des Stores & Hooks ---
import { useAuthStore } from './hooks/useAuthStore';

// --- App shell ---
import { Sidebar } from './components/layout/Sidebar';
import { AppHeader } from './components/layout/AppHeader';
import { CommandPalette } from './components/layout/CommandPalette';
import { CreateRiskModal } from './features/risks/components/CreateRiskModal';

// --- Imports des Pages & Features ---
import { Login } from './pages/Login';
import { Register } from './pages/Register';
import { Users } from './pages/Users';
import { RoleManagement } from './pages/RoleManagement';
import { TenantManagement } from './pages/TenantManagement';
import { Settings } from './pages/Settings';
import AuditLogs from './pages/AuditLogs';
import ComingSoon from './pages/ComingSoon';
import { DashboardPage } from './features/dashboard/DashboardPage';
import { ImportRisksPage } from './features/risks/ImportRisksPage';
import { RiskListPage } from './features/risks/RiskListPage';
import { Mitigations } from './pages/Mitigations';
import { Compliance } from './pages/Compliance';
import { Assets } from './pages/Assets';
import { RiskManagement } from './pages/RiskManagement';
import { TokenManagement } from './pages/TokenManagement';
import { Recommendations } from './pages/Recommendations';
import Analytics from './pages/Analytics';
import { Incidents } from './pages/Incidents';
import { ThreatMap } from './pages/ThreatMap';
import { Reports } from './pages/Reports';
import { BoardReportPage } from './features/reports/BoardReportPage';
import Marketplace from './pages/Marketplace';
import PermissionAnalyticsPage from './pages/PermissionAnalytics';
import CustomFields from './pages/CustomFields';
import BulkOperations from './pages/BulkOperations';
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
      <CreateRiskModal isOpen={newRiskOpen} onClose={() => setNewRiskOpen(false)} />
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
        <Route path="/login" element={<Login />} />
        <Route path="/register" element={<Register />} />

        {/* Routes Protégées (Layout Global) */}
        <Route
          element={
            <ProtectedRoute>
              <DashboardLayout />
            </ProtectedRoute>
          }
        >
          <Route index element={<DashboardPage />} />
          <Route path="risks" element={<RiskListPage />} />
          <Route path="mitigations" element={<Mitigations />} />
          <Route path="compliance" element={<Compliance />} />
          <Route path="risks/import" element={<ImportRisksPage />} />
          <Route path="risks/:riskId/timeline" element={<RiskTimeline />} />
          <Route path="risk-management" element={<RiskManagement />} />
          <Route path="analytics" element={<Analytics />} />
          <Route path="incidents" element={<Incidents />} />
          <Route path="threat-map" element={<ThreatMap />} />
          <Route path="reports" element={<Reports />} />
          <Route path="reports/board" element={<BoardReportPage />} />
          <Route path="marketplace" element={<Marketplace />} />
          <Route path="custom-fields" element={<CustomFields />} />
          <Route path="bulk-operations" element={<BulkOperations />} />
          <Route path="settings" element={<Settings />} />
          <Route path="users" element={<Users />} />
          <Route path="roles" element={<RoleManagement />} />
          <Route path="tenants" element={<TenantManagement />} />
          <Route path="audit-logs" element={<AuditLogs />} />
          <Route path="analytics/permissions" element={<PermissionAnalyticsPage />} />
          <Route path="tokens" element={<TokenManagement />} />
          <Route path="assets" element={<Assets />} />
          <Route path="recommendations" element={<Recommendations />} />

          {/* Design-language screens without a backend yet — graceful placeholder. */}
          <Route path="leaderboard" element={<ComingSoon />} />
          <Route path="infrastructure" element={<ComingSoon />} />
          <Route path="simulations" element={<ComingSoon />} />
          <Route path="assets/universe" element={<ComingSoon />} />
        </Route>

        {/* Redirection par défaut */}
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </BrowserRouter>
  );
}

export default App;
