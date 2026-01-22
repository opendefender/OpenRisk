import { useState } from 'react';
import { BrowserRouter, Routes, Route, Navigate, Outlet } from 'react-router-dom';
import { Toaster } from 'sonner';
import { motion } from 'framer-motion';
import { Plus, Search } from 'lucide-react';

// --- Imports des Stores & Hooks ---
import { useAuthStore } from './hooks/useAuthStore';
import { useRiskStore, type Risk } from './hooks/useRiskStore';

// --- Imports des Pages & Features ---
import { Login } from './pages/Login';
import { Register } from './pages/Register';
import { Users } from './pages/Users';
import { Settings } from './pages/Settings';
import AuditLogs from './pages/AuditLogs';
import { Sidebar } from './components/layout/Sidebar';
import { NotificationCenter } from './components/layout/NotificationCenter';
import { DashboardGrid } from './features/dashboard/components/DashboardGrid';
import { CreateRiskModal } from './features/risks/components/CreateRiskModal';
import { RiskDetails } from './features/risks/components/RiskDetails';
import { EditRiskModal } from './features/risks/components/EditRiskModal';
import { Assets } from './pages/Assets';
import { Risks } from './pages/Risks';
import { TokenManagement } from './pages/TokenManagement';
import { Recommendations } from './pages/Recommendations';
import Analytics from './pages/Analytics';
import { Incidents } from './pages/Incidents';
import { ThreatMap } from './pages/ThreatMap';
import { Reports } from './pages/Reports';
import Marketplace from './pages/Marketplace';


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
 * COMPOSANT 2: LAYOUT GLOBAL
 * Contient la Sidebar fixe et la zone de contenu dynamique.
 */
const DashboardLayout = () => (
  <div className="flex h-screen bg-background text-white overflow-hidden font-sans selection:bg-primary/30">
    <Sidebar />
    <div className="flex-1 flex flex-col h-screen overflow-hidden relative">
      <main className="flex-1 overflow-hidden relative flex flex-col">
        <Outlet />
      </main>
    </div>
  </div>
);

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
      <header className="h-16 shrink-0 border-b border-border bg-background/80 backdrop-blur-md flex items-center justify-between px-6 z-10 sticky top-0">
        
        {/* Search Bar (Linear style) */}
        <div className="flex items-center gap-2 text-zinc-500 bg-surface border border-white/5 px-3 py-1.5 rounded-md w-64 focus-within:border-primary/50 focus-within:text-white transition-colors group">
          <Search size={14} className="group-focus-within:text-primary transition-colors" />
          <input 
              type="text" 
              placeholder="Search risks, assets..." 
              className="bg-transparent border-none outline-none text-sm w-full placeholder:text-zinc-600"
          />
          <kbd className="hidden sm:inline-block px-1.5 py-0.5 text-[10px] font-bold text-zinc-500 bg-zinc-800 rounded border border-zinc-700">⌘K</kbd>
        </div>

        {/* Actions Droite */}
        <div className="flex items-center gap-4">
           <NotificationCenter />
           
           <Button onClick={() => setIsModalOpen(true)} className="shadow-lg shadow-blue-500/20">
              <Plus size={16} className="mr-2" /> New Risk
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
          <Route path="risks" element={<Risks />} />
          <Route path="analytics" element={<Analytics />} />
          <Route path="incidents" element={<Incidents />} />
          <Route path="threat-map" element={<ThreatMap />} />
          <Route path="reports" element={<Reports />} />
          <Route path="marketplace" element={<Marketplace />} />
          <Route path="settings" element={<Settings />} />
          <Route path="users" element={<Users />} />
          <Route path="audit-logs" element={<AuditLogs />} />
          <Route path="tokens" element={<TokenManagement />} />
          <Route path="assets" element={<Assets />} />
          <Route path="recommendations" element={<Recommendations />} />
        </Route>
        
        {/* Redirection par défaut */}
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>

      {/* Toast Notifications Global */}
      <Toaster position="top-left" theme="dark" richColors closeButton />
    </BrowserRouter>
  );
}

export default App;