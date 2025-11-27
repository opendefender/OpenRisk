import React, { useEffect } from 'react';
import { ShieldAlert, CheckCircle2, Server, TrendingUp, AlertTriangle, ChevronRight, Loader2, Zap, FileDown } from 'lucide-react';
import { motion } from 'framer-motion';
import { Link } from 'react-router-dom';

// Stores & Components
import { useRiskStore } from '../../../hooks/useRiskStore';
import { useAssetStore } from '../../../hooks/useAssetStore';
import { useAuthStore } from '../../../hooks/useAuthStore';
import { RiskMatrix } from './RiskMatrix';
import { Button } from '../../../components/ui/Button';
import { RiskTrendChart } from './RiskTrendChart';

// =================================================================
// Composants UI Internes (Widgets)
// =================================================================

interface WidgetProps {
  title: string;
  children: React.ReactNode;
  className?: string;
  padding?: string;
}

const Widget: React.FC<WidgetProps> = ({ title, children, className = '', padding = 'p-6' }) => (
  <div className={`rounded-xl border border-border bg-surface shadow-lg ${className}`}>
    <div className={`text-lg font-semibold text-white mb-4 ${padding}`}>{title}</div>
    <div className={padding}>
        {children}
    </div>
  </div>
);

interface StatCardProps {
  label: string;
  value: string | number;
  icon: React.ElementType;
  color?: string;
}

const StatCard: React.FC<StatCardProps> = ({ label, value, icon: Icon, color = 'text-blue-400' }) => (
  <div className="flex items-center justify-between p-4 bg-zinc-900/50 rounded-lg border border-white/5 transition-colors hover:bg-zinc-900">
    <div className="flex items-center">
      <div className={`p-2 rounded-full ${color}/20 mr-3`}>
        <Icon size={18} className={color} />
      </div>
      <div>
        <div className="text-zinc-400 text-xs uppercase tracking-wider">{label}</div>
        <div className="text-white text-xl font-bold">{value}</div>
      </div>
    </div>
    <ChevronRight size={16} className="text-zinc-600" />
  </div>
);

// =================================================================
// Le Composant Principal : DashboardGrid
// =================================================================

export const DashboardGrid: React.FC = () => {
  const { risks, fetchRisks, isLoading: isRisksLoading } = useRiskStore();
  const { assets, fetchAssets, isLoading: isAssetsLoading } = useAssetStore();
  const { user } = useAuthStore();
  
  // Calcul des Stats Rapides
  const totalRisks = risks.length;
  const criticalRisks = risks.filter(r => r.score >= 15).length;
  const mitigatedCount = risks.filter(r => r.status === 'MITIGATED').length;
  
  // Top 5 des risques non mitigés (Triés par score décroissant)
  const topRisks = [...risks]
    .filter(r => r.status !== 'MITIGATED' && r.status !== 'CLOSED')
    .sort((a, b) => b.score - a.score)
    .slice(0, 5);

  // Chargement initial des données
  useEffect(() => {
    fetchRisks();
    fetchAssets();
    // La matrice gère son propre fetch via /stats/risk-matrix
  }, [fetchRisks, fetchAssets]);

  // Handler pour l'export PDF (Commit #15)
  const handleExport = () => {
    // Utilise la variable d'env VITE_API_URL ou fallback sur localhost
    const apiUrl = import.meta.env.VITE_API_URL || 'http://localhost:8080/api/v1';
    window.open(`${apiUrl}/export/pdf`, '_blank');
  };

  const welcomeMessage = `Welcome back, ${user?.full_name || user?.email || 'Admin'}.`;
  
  // Loader global
  if (isRisksLoading || isAssetsLoading) {
      return (
          <div className="flex justify-center items-center h-[50vh] text-zinc-500">
              <Loader2 className="animate-spin mr-3" size={32} /> Loading OpenRisk data...
          </div>
      );
  }

  return (
    <motion.div 
        initial={{ opacity: 0 }} 
        animate={{ opacity: 1 }} 
        className="p-8 space-y-8 h-full overflow-y-auto"
    >
        {/* Header Dashboard : Titre + Actions (Export & Assets) */}
        <div className="flex flex-col md:flex-row justify-between items-start md:items-center pb-6 border-b border-white/5 gap-4">
            <div>
                <h1 className="text-3xl font-bold text-white flex items-center gap-3">
                    <Zap className="text-primary" size={28} /> {welcomeMessage}
                </h1>
                <p className="text-zinc-400 text-sm mt-1 ml-10">Overview of your security posture.</p>
            </div>
            
            <div className="flex items-center gap-3">
                {/* Lien rapide vers Assets */}
                <Link to="/assets">
                    <Button variant="ghost" className="text-zinc-400 hover:text-white border-zinc-700">
                        <Server size={16} className="mr-2" /> Inventory
                    </Button>
                </Link>

                {/* Bouton Export PDF */}
                <Button onClick={handleExport} variant="secondary">
                    <FileDown size={16} className="mr-2" /> Export Report
                </Button>
            </div>
        </div>

        {/* Grille des Widgets */}
        <div className="grid grid-cols-12 gap-6">

            {/* 1. Matrice des Risques (Grande zone d'affichage) */}
            <Widget title="Risk Matrix" className="col-span-12 lg:col-span-7 bg-surface p-0">
               <RiskMatrix />
            </Widget>

            {/* 2. Indicateurs Clés (Stats Rapides) */}
            <Widget title="Key Indicators" className="col-span-12 lg:col-span-5 space-y-4">
                <StatCard 
                    label="Risques Critiques (Score >= 15)" 
                    value={criticalRisks} 
                    icon={AlertTriangle} 
                    color="text-red-400" 
                />
                <StatCard 
                    label="Total Risques Actifs" 
                    value={totalRisks} 
                    icon={ShieldAlert} 
                    color="text-yellow-400" 
                />
                <StatCard 
                    label="Risques Mitigés" 
                    value={`${mitigatedCount} / ${totalRisks}`} 
                    icon={CheckCircle2} 
                    color="text-emerald-400" 
                />
                <StatCard 
                    label="Total Assets Inventoriés" 
                    value={assets.length} 
                    icon={Server} 
                    color="text-blue-400" 
                />
            </Widget>

            {/* 3. Top Risques (Liste détaillée) */}
            <Widget title="Top 5 Unmitigated Risks" className="col-span-12 lg:col-span-12">
                {topRisks.length > 0 ? (
                    <div className="space-y-3">
                        {topRisks.map((risk) => (
                            <Link 
                                to={`/?riskId=${risk.id}`} 
                                key={risk.id} 
                                className="flex justify-between items-center p-3 rounded-lg border border-white/5 hover:bg-white/5 transition-colors cursor-pointer group"
                            >
                                <div className="flex items-center gap-3">
                                    <TrendingUp size={16} className="text-red-500 group-hover:scale-110 transition-transform" />
                                    <div>
                                        <div className="font-medium text-white group-hover:text-primary transition-colors">{risk.title}</div>
                                        <div className="text-xs text-zinc-500 truncate max-w-md">{risk.description}</div>
                                    </div>
                                </div>
                                <div className="flex items-center gap-4">
                                    <span className={`text-xs font-bold px-2 py-1 rounded border ${
                                        risk.score >= 15 
                                        ? 'bg-red-500/10 text-red-400 border-red-500/20' 
                                        : 'bg-yellow-500/10 text-yellow-400 border-yellow-500/20'
                                    }`}>
                                        SCORE: {risk.score}
                                    </span>
                                    <ChevronRight size={16} className="text-zinc-600 group-hover:translate-x-1 transition-transform" />
                                </div>
                            </Link>
                        ))}
                    </div>
                ) : (
                    <div className="flex flex-col items-center justify-center py-8 text-zinc-500 border-t border-white/5 mt-4">
                        <CheckCircle2 size={32} className="mb-2 text-emerald-500/50" />
                        <p>No high priority risks found. Excellent work!</p>
                    </div>
                )}
            </Widget>
            
            {/* 4. Placeholder pour l'évolution des Mitigations */}
            <Widget title="Mitigation Progress Overview" className="col-span-12 lg:col-span-6 h-64">
                <div className="flex flex-col justify-center items-center h-full text-zinc-600 border border-dashed border-zinc-800 rounded-lg m-2">
                   <TrendingUp size={24} className="mb-2 opacity-50" />
                   <span className="text-xs uppercase tracking-widest">Chart Coming Soon</span>
                </div>
            </Widget>

            {/* 5. TENDANCES GLOBALES (Remplacement du Placeholder) */}
            <Widget title="Global Risk Trend" className="col-span-12 lg:col-span-6 h-96 p-0">
                <RiskTrendChart />
            </Widget>

            {/* 5. Placeholder pour la distribution des Assets */}
            <Widget title="Asset Criticality Distribution" className="col-span-12 lg:col-span-6 h-64">
                <div className="flex flex-col justify-center items-center h-full text-zinc-600 border border-dashed border-zinc-800 rounded-lg m-2">
                    <Server size={24} className="mb-2 opacity-50" />
                    <span className="text-xs uppercase tracking-widest">Distribution Chart Coming Soon</span>
                </div>
            </Widget>

        </div>
    </motion.div>
  );
};