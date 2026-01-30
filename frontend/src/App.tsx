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
import { Assets } from './pages/Assets';
import { Risks } from './pages/Risks';
import { TokenManagement } from './pages/TokenManagement';
import { Recommendations } from './pages/Recommendations';
import Analytics from './pages/Analytics';
import { Incidents } from './pages/Incidents';
import { ThreatMap } from './pages/ThreatMap';
import { Reports } from './pages/Reports';
import Marketplace from './pages/Marketplace';
import PermissionAnalyticsPage from './pages/PermissionAnalytics';


// --- Imports UI Components ---
import { Button } from './components/ui/Button';
import { Drawer } from './components/ui/Drawer';

/
  COMPOSANT : PROTECTION DE ROUTE
  V√rifie si le token existe, sinon redirige vers Login.
 /
const ProtectedRoute = ({ children }: { children: React.ReactNode }) => {
  const token = useAuthStore((state) => state.token);
  if (!token) return <Navigate to="/login" replace />;
  return <>{children}</>;
};

/
  COMPOSANT : LAYOUT GLOBAL
  Contient la Sidebar fixe et la zone de contenu dynamique.
 /
const DashboardLayout = () => (
  <div className="flex h-screen bg-background text-white overflow-hidden font-sans selection:bg-primary/">
    <Sidebar />
    <div className="flex- flex flex-col h-screen overflow-hidden relative">
      <main className="flex- overflow-hidden relative flex flex-col">
        <Outlet />
      </main>
    </div>
  </div>
);

/
  COMPOSANT : VUE DASHBOARD (La page d'accueil)
  Contient le Header sp√cifique, la Grille, et les Modals.
 /
const DashboardView = () => {
  const [isModalOpen, setIsModalOpen] = useState(false);
  const selectedRisk = useRiskStore((s) => s.selectedRisk);
  const setSelectedRisk = useRiskStore((s) => s.setSelectedRisk);
  const [editRisk, setEditRisk] = useState<Risk | null>(null);
  
  // Pour la d√mo : On r√cup√re les risques pour la liste du bas
  const { risks } = useRiskStore();

  return (
    <>
      {/ --- HEADER FLOTTANT (Sp√cifique Dashboard) --- /}
      <header className="h- shrink- border-b border-border bg-background/ backdrop-blur-md flex items-center justify-between px- z- sticky top-">
        
        {/ Search Bar (Linear style) /}
        <div className="flex items-center gap- text-zinc- bg-surface border border-white/ px- py-. rounded-md w- focus-within:border-primary/ focus-within:text-white transition-colors group">
          <Search size={} className="group-focus-within:text-primary transition-colors" />
          <input 
              type="text" 
              placeholder="Search risks, assets..." 
              className="bg-transparent border-none outline-none text-sm w-full placeholder:text-zinc-"
          />
          <kbd className="hidden sm:inline-block px-. py-. text-[px] font-bold text-zinc- bg-zinc- rounded border border-zinc-">‚åòK</kbd>
        </div>

        {/ Actions Droite /}
        <div className="flex items-center gap-">
           <NotificationCenter />
           
           <Button onClick={() => setIsModalOpen(true)} className="shadow-lg shadow-blue-/">
              <Plus size={} className="mr-" /> New Risk
           </Button>
        </div>
      </header>

      {/ --- CONTENU SCROLLABLE --- /}
      <div className="flex- overflow-y-auto overflow-x-hidden p- scrollbar-thin scrollbar-thumb-zinc- scrollbar-track-transparent">
         <motion.div
           initial={{ opacity: , y:  }}
           animate={{ opacity: , y:  }}
           transition={{ duration: . }}
           className="pb-"
         >
            {/ . La Grille de Widgets /}
            <DashboardGrid />

            {/ . Liste Rapide (Pour tester l'ouverture du Drawer) /}
            <div className="mt- max-w-xl mx-auto">
              <h className="text-sm font-bold text-zinc- uppercase tracking-widest mb-">
                Active Risks Overview
              </h>
              <div className="grid grid-cols- md:grid-cols- lg:grid-cols- gap-">
                {risks.map((risk) => (
                  <div 
                    key={risk.id} 
                    className="bg-surface border border-border p- rounded-xl hover:border-primary cursor-pointer transition-all hover:scale-[.] hover:shadow-lg group"
                  >
                    <div className="flex justify-between mb-">
                        <div className="flex gap-">
                           {risk.tags?.[] && (
                             <span className="text-[px] font-bold bg-zinc- px- py-. rounded text-zinc- border border-white/">
                               {risk.tags[]}
                             </span>
                           )}
                           {risk.source !== 'MANUAL' && (
                              <span className="text-[px] font-bold bg-blue-/ px- py-. rounded text-blue- border border-blue-/">
                                {risk.source}
                              </span>
                           )}
                        </div>
                        <div className="flex items-center gap-">
                          <span className={font-mono font-bold ${
                            risk.score >=  ? 'text-red-' : 'text-emerald-'
                          }}>
                            {risk.score}
                          </span>
                          <Button onClick={() => setEditRisk(risk)} variant="ghost">Edit</Button>
                        </div>
                    </div>
                    <h onClick={() => setSelectedRisk(risk)} className="font-medium text-zinc- truncate group-hover:text-white transition-colors cursor-pointer">
                      {risk.title}
                    </h>
                    <div className="mt- flex items-center justify-between text-xs text-zinc-">
                       <span>{risk.mitigations?.length || } mitigations</span>
                       <span>{new Date(risk.created_at || Date.now()).toLocaleDateString()}</span>
                    </div>
                  </div>
                ))}
              </div>
            </div>
         </motion.div>
      </div>

      {/ --- MODALS & DRAWERS --- /}
        <CreateRiskModal isOpen={isModalOpen} onClose={() => setIsModalOpen(false)} />

        <EditRiskModal isOpen={!!editRisk} onClose={() => setEditRisk(null)} risk={editRisk} />

        <Drawer 
          isOpen={!!selectedRisk} 
          onClose={() => setSelectedRisk(null)}
          title={selectedRisk?.title || "D√tails du Risque"}
        >
          {selectedRisk && <RiskDetails risk={selectedRisk} onClose={() => setSelectedRisk(null)} />}
        </Drawer>
    </>
  );
};

/
  COMPOSANT PRINCIPAL : APP ROUTER
 /
function App() {
  return (
    <BrowserRouter>
      <Routes>
        {/ Routes Publiques /}
        <Route path="/login" element={<Login />} />
        <Route path="/register" element={<Register />} />

        {/ Routes Prot√g√es (Layout Global) /}
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
          <Route path="roles" element={<RoleManagement />} />
          <Route path="tenants" element={<TenantManagement />} />
          <Route path="audit-logs" element={<AuditLogs />} />
          <Route path="analytics/permissions" element={<PermissionAnalyticsPage />} />
          <Route path="tokens" element={<TokenManagement />} />
          <Route path="assets" element={<Assets />} />
          <Route path="recommendations" element={<Recommendations />} />
        </Route>
        
        {/ Redirection par d√faut /}
        <Route path="" element={<Navigate to="/" replace />} />
      </Routes>

      {/ Toast Notifications Global /}
      <Toaster position="top-left" theme="dark" richColors closeButton />
    </BrowserRouter>
  );
}

export default App;