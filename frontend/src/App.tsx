// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

import { useState } from 'react';
import { BrowserRouter, Routes, Route, Navigate, Outlet, useLocation } from 'react-router-dom';
import { motion, AnimatePresence } from 'framer-motion';
import { Plus, Search, Menu, Zap } from 'lucide-react';

// --- Imports des Stores & Hooks ---
import { useAuthStore } from './hooks/useAuthStore';
import { useRiskStore, type Risk } from './hooks/useRiskStore';

// --- Imports des Pages & Features ---
import { Login } from './pages/Login';
import { Register } from './pages/Register';
import { Users } from './pages/Users';
import { RoleManagement } from './pages/RoleManagement';
import { TenantManagement } from './pages/TenantManagement';
import { Settings } from './pages/Settings';
import AuditLogs from './pages/AuditLogs';
import { Sidebar } from './components/layout/Sidebar';
import { NotificationCenter } from './components/layout/NotificationCenter';
import { DashboardGrid } from './features/dashboard/components/DashboardGrid';
import { CreateRiskModal } from './features/risks/components/CreateRiskModal';
import { RiskDetails } from './features/risks/components/RiskDetails';
import { EditRiskModal } from './features/risks/components/EditRiskModal';
import { ImportRisksPage } from './features/risks/ImportRisksPage';
import { RiskListPage } from './features/risks/RiskListPage';
import { Mitigations } from './pages/Mitigations';
import { Compliance } from './pages/Compliance';
import { Assets } from './pages/Assets';
import { Risks } from './pages/Risks';
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


// --- Imports UI Components ---
import { Button } from './components/ui/Button';
import { Drawer } from './components/ui/Drawer';

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
 * switching pages never feels like an abrupt jump-cut — matches the app-wide
 * "every route change fades ~200ms" rule (see docs/MASTER_PROMPT_V4.md, Part C).
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
 * COMPOSANT 2: LAYOUT GLOBAL
 * Sidebar responsive (statique ≥ lg, tiroir off-canvas < lg) + zone de contenu dynamique.
 * Une barre supérieure mobile porte le bouton d'ouverture du tiroir ; elle disparaît sur
 * grand écran, où la Sidebar est déjà dans le flux, de sorte que la mise en page desktop
 * reste identique.
 */
const DashboardLayout = () => {
  const [mobileNavOpen, setMobileNavOpen] = useState(false);
  return (
    <div className="flex h-screen bg-background text-white overflow-hidden font-sans selection:bg-primary/30">
      <Sidebar mobileOpen={mobileNavOpen} onMobileClose={() => setMobileNavOpen(false)} />
      <div className="flex-1 flex flex-col h-screen overflow-hidden relative min-w-0">
        {/* Mobile top bar — visible only < lg. */}
        <div className="lg:hidden h-14 shrink-0 border-b border-border bg-surface/90 backdrop-blur-md flex items-center gap-3 px-4 z-30">
          <button
            onClick={() => setMobileNavOpen(true)}
            className="w-9 h-9 rounded-lg border border-border flex items-center justify-center text-zinc-300 hover:text-white hover:border-primary transition-colors"
            aria-label="Open navigation menu"
          >
            <Menu size={18} />
          </button>
          <div className="flex items-center gap-2">
            <div className="w-6 h-6 rounded-md bg-gradient-to-br from-blue-500 to-purple-600 flex items-center justify-center shrink-0">
              <Zap size={13} className="text-white" fill="currentColor" />
            </div>
            <span className="font-bold tracking-tight bg-gradient-to-r from-white to-zinc-400 bg-clip-text text-transparent">OpenRisk</span>
          </div>
        </div>
        <main className="flex-1 overflow-hidden relative flex flex-col">
          <AnimatedOutlet />
        </main>
      </div>
    </div>
  );
};

/**
 * COMPOSANT 3: VUE DASHBOARD (La page d'accueil)
 * Contient le Header spécifique, la Grille, et les Modals.
 */
const DashboardView = () => {
  const [isModalOpen, setIsModalOpen] = useState(false);
  const selectedRisk = useRiskStore((s) => s.selectedRisk);
  const setSelectedRisk = useRiskStore((s) => s.setSelectedRisk);
  const [editRisk, setEditRisk] = useState<Risk | null>(null);
  
  // Pour la démo : On récupère les risques pour la liste du bas
  const { risks } = useRiskStore();

  return (
    <>
      {/* --- HEADER FLOTTANT (Spécifique Dashboard) --- */}
      <header className="h-16 shrink-0 border-b border-border bg-background/80 backdrop-blur-md flex items-center justify-between gap-3 px-4 sm:px-6 z-10 sticky top-0">

        {/* Search Bar (Linear style) — grows to fill on mobile, fixed width on desktop. */}
        <div className="flex items-center gap-2 text-zinc-500 bg-surface border border-white/5 px-3 py-1.5 rounded-md min-w-0 flex-1 sm:flex-none sm:w-64 focus-within:border-primary/50 focus-within:text-white transition-colors group">
          <Search size={14} className="shrink-0 group-focus-within:text-primary transition-colors" />
          <input
              type="text"
              placeholder="Search risks, assets..."
              className="bg-transparent border-none outline-none text-sm w-full min-w-0 placeholder:text-zinc-600"
          />
          <kbd className="hidden sm:inline-block px-1.5 py-0.5 text-[10px] font-bold text-zinc-500 bg-zinc-800 rounded border border-zinc-700">⌘K</kbd>
        </div>

        {/* Actions Droite */}
        <div className="flex items-center gap-2 sm:gap-4 shrink-0">
           <NotificationCenter />

           <Button onClick={() => setIsModalOpen(true)} className="shadow-lg shadow-blue-500/20 whitespace-nowrap">
              <Plus size={16} className="sm:mr-2" /> <span className="hidden sm:inline">New Risk</span>
           </Button>
        </div>
      </header>

      {/* --- CONTENU SCROLLABLE --- */}
      <div className="flex-1 overflow-y-auto overflow-x-hidden p-6 scrollbar-thin scrollbar-thumb-zinc-800 scrollbar-track-transparent">
         <motion.div
           initial={{ opacity: 0, y: 10 }}
           animate={{ opacity: 1, y: 0 }}
           transition={{ duration: 0.4 }}
           className="pb-20"
         >
            {/* 1. La Grille de Widgets */}
            <DashboardGrid />

            {/* 2. Liste Rapide (Pour tester l'ouverture du Drawer) */}
            <div className="mt-12 max-w-7xl mx-auto">
              <h3 className="text-sm font-bold text-zinc-500 uppercase tracking-widest mb-4">
                Active Risks Overview
              </h3>
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                {risks.map((risk) => (
                  <div 
                    key={risk.id} 
                    className="bg-surface border border-border p-4 rounded-xl hover:border-primary cursor-pointer transition-all hover:scale-[1.01] hover:shadow-lg group"
                  >
                    <div className="flex justify-between mb-3">
                        <div className="flex gap-2">
                           {risk.tags?.[0] && (
                             <span className="text-[10px] font-bold bg-zinc-800 px-2 py-0.5 rounded text-zinc-400 border border-white/5">
                               {risk.tags[0]}
                             </span>
                           )}
                           {risk.source !== 'MANUAL' && (
                              <span className="text-[10px] font-bold bg-blue-500/10 px-2 py-0.5 rounded text-blue-400 border border-blue-500/20">
                                {risk.source}
                              </span>
                           )}
                        </div>
                        <div className="flex items-center gap-2">
                          <span className={`font-mono font-bold ${
                            risk.score >= 15 ? 'text-red-500' : 'text-emerald-500'
                          }`}>
                            {risk.score}
                          </span>
                          <Button onClick={() => setEditRisk(risk)} variant="ghost">Edit</Button>
                        </div>
                    </div>
                    <h4 onClick={() => setSelectedRisk(risk)} className="font-medium text-zinc-200 truncate group-hover:text-white transition-colors cursor-pointer">
                      {risk.title}
                    </h4>
                    <div className="mt-2 flex items-center justify-between text-xs text-zinc-500">
                       <span>{risk.mitigations?.length || 0} mitigations</span>
                       <span>{new Date(risk.created_at || Date.now()).toLocaleDateString()}</span>
                    </div>
                  </div>
                ))}
              </div>
            </div>
         </motion.div>
      </div>

      {/* --- MODALS & DRAWERS --- */}
        <CreateRiskModal isOpen={isModalOpen} onClose={() => setIsModalOpen(false)} />

        <EditRiskModal isOpen={!!editRisk} onClose={() => setEditRisk(null)} risk={editRisk} />

        <Drawer 
          isOpen={!!selectedRisk} 
          onClose={() => setSelectedRisk(null)}
          title={selectedRisk?.title || "Détails du Risque"}
        >
          {selectedRisk && <RiskDetails risk={selectedRisk} onClose={() => setSelectedRisk(null)} />}
        </Drawer>
    </>
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
          <Route index element={<DashboardView />} />
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
        </Route>
        
        {/* Redirection par défaut */}
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </BrowserRouter>
  );
}

export default App;