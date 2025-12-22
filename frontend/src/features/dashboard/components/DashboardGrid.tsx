import React, { useEffect, useState } from 'react';
import { ShieldAlert, CheckCircle2, Server, TrendingUp, AlertTriangle, ChevronRight, Loader2, Zap, FileDown, GripVertical } from 'lucide-react';
import { motion } from 'framer-motion';
import { Link } from 'react-router-dom';
import GridLayout, { Layout } from 'react-grid-layout';
import 'react-grid-layout/css/styles.css';
import 'react-resizable/css/styles.css';

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
  isDragging?: boolean;
}

const Widget: React.FC<WidgetProps> = ({ title, children, className = '', padding = 'p-6', isDragging = false }) => (
  <div className={`rounded-xl border border-border bg-surface shadow-lg ${isDragging ? 'opacity-50' : ''} ${className}`}>
    <div className={`text-lg font-semibold text-white mb-4 flex items-center gap-2 ${padding}`}>
      <GripVertical size={16} className="text-zinc-600 cursor-grab active:cursor-grabbing" />
      {title}
    </div>
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
// Le Composant Principal : DashboardGrid avec Drag-and-Drop
// =================================================================

const defaultLayout: Layout[] = [
  { i: 'risk-matrix', x: 0, y: 0, w: 7, h: 4 },
  { i: 'key-indicators', x: 7, y: 0, w: 5, h: 4 },
  { i: 'top-risks', x: 0, y: 4, w: 12, h: 4 },
  { i: 'mitigation-progress', x: 0, y: 8, w: 6, h: 3 },
  { i: 'risk-trend', x: 6, y: 8, w: 6, h: 3 },
  { i: 'asset-distribution', x: 0, y: 11, w: 6, h: 3 },
];

export const DashboardGrid: React.FC = () => {
  const { risks, fetchRisks, isLoading: isRisksLoading } = useRiskStore();
  const { assets, fetchAssets, isLoading: isAssetsLoading } = useAssetStore();
  const { user } = useAuthStore();
  const [layout, setLayout] = useState<Layout[]>(defaultLayout);
  const [isDragging, setIsDragging] = useState(false);
  const [containerWidth, setContainerWidth] = useState(1200);
  
  // Track container width for responsive grid
  useEffect(() => {
    const handleResize = () => {
      const mainElement = document.querySelector('main');
      if (mainElement) {
        // Account for padding (p-6 = 24px on each side)
        setContainerWidth(Math.max(mainElement.clientWidth - 48, 300));
      }
    };
    
    handleResize();
    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, []);
  
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

  // Handler pour l'export PDF
  const handleExport = () => {
    const apiUrl = import.meta.env.VITE_API_URL || 'http://localhost:8080/api/v1';
    window.open(`${apiUrl}/export/pdf`, '_blank');
  };

  const handleLayoutChange = (newLayout: Layout[]) => {
    setLayout(newLayout);
    // Optionnel : sauvegarder la mise en page dans localStorage
    localStorage.setItem('dashboardLayout', JSON.stringify(newLayout));
  };

  const resetLayout = () => {
    setLayout(defaultLayout);
    localStorage.removeItem('dashboardLayout');
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
        {/* Header Dashboard */}
        <div className="flex flex-col md:flex-row justify-between items-start md:items-center pb-6 border-b border-white/5 gap-4">
            <div>
                <h1 className="text-3xl font-bold text-white flex items-center gap-3">
                    <Zap className="text-primary" size={28} /> {welcomeMessage}
                </h1>
                <p className="text-zinc-400 text-sm mt-1 ml-10">Drag and drop widgets to customize your dashboard.</p>
            </div>
            
            <div className="flex items-center gap-3">
                <Link to="/assets">
                    <Button variant="ghost" className="text-zinc-400 hover:text-white border-zinc-700">
                        <Server size={16} className="mr-2" /> Inventory
                    </Button>
                </Link>
                <Button onClick={resetLayout} variant="ghost" className="text-zinc-400 hover:text-white border-zinc-700">
                    Reset Layout
                </Button>
                <Button onClick={handleExport} variant="secondary">
                    <FileDown size={16} className="mr-2" /> Export Report
                </Button>
            </div>
        </div>

        {/* Grille Draggable */}
        <GridLayout 
          className="bg-transparent w-full"
          layout={layout}
          onLayoutChange={handleLayoutChange}
          cols={12}
          rowHeight={80}
          width={containerWidth}
          isDraggable={true}
          isResizable={true}
          compactType="vertical"
          preventCollision={false}
          useCSSTransforms={true}
          onDragStart={() => setIsDragging(true)}
          onDragStop={() => setIsDragging(false)}
          containerPadding={[0, 0]}
          margin={[24, 24]}
          draggableHandle=".react-grid-dragHandleExampleStyle"
        >
          {/* 1. Risk Matrix */}
          <div key="risk-matrix" className="rounded-xl border border-border bg-surface shadow-lg overflow-hidden">
            <div className="text-lg font-semibold text-white mb-4 flex items-center gap-2 p-6 react-grid-dragHandleExampleStyle cursor-grab active:cursor-grabbing">
              <GripVertical size={16} className="text-zinc-600" />
              Risk Matrix
            </div>
            <div className="p-6 pt-0">
              <RiskMatrix />
            </div>
          </div>

          {/* 2. Key Indicators */}
          <div key="key-indicators" className="rounded-xl border border-border bg-surface shadow-lg p-6 space-y-4 overflow-y-auto">
            <div className="text-lg font-semibold text-white flex items-center gap-2 react-grid-dragHandleExampleStyle cursor-grab active:cursor-grabbing">
              <GripVertical size={16} className="text-zinc-600" />
              Key Indicators
            </div>
            <div className="space-y-4">
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
            </div>
          </div>

          {/* 3. Top Risks */}
          <div key="top-risks" className="rounded-xl border border-border bg-surface shadow-lg p-6 overflow-y-auto">
            <div className="text-lg font-semibold text-white flex items-center gap-2 mb-4">
              <GripVertical size={16} className="text-zinc-600 cursor-grab active:cursor-grabbing" />
              Top 5 Unmitigated Risks
            </div>
            {topRisks.length > 0 ? (
                <div className="space-y-3">
                    {topRisks.map((risk) => (
                        <Link 
                            to={`/?riskId=${risk.id}`} 
                            key={risk.id} 
                            className="flex justify-between items-center p-3 rounded-lg border border-white/5 hover:bg-white/5 transition-colors cursor-pointer group"
                        >
                            <div className="flex items-center gap-3 flex-1">
                                <TrendingUp size={16} className="text-red-500 group-hover:scale-110 transition-transform flex-shrink-0" />
                                <div className="min-w-0">
                                    <div className="font-medium text-white group-hover:text-primary transition-colors truncate">{risk.title}</div>
                                    <div className="text-xs text-zinc-500 truncate">{risk.description}</div>
                                </div>
                            </div>
                            <div className="flex items-center gap-4 flex-shrink-0 ml-2">
                                <span className={`text-xs font-bold px-2 py-1 rounded border whitespace-nowrap ${
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
                <div className="flex flex-col items-center justify-center py-8 text-zinc-500">
                    <CheckCircle2 size={32} className="mb-2 text-emerald-500/50" />
                    <p>No high priority risks found. Excellent work!</p>
                </div>
            )}
          </div>

          {/* 4. Mitigation Progress */}
          <div key="mitigation-progress" className="rounded-xl border border-border bg-surface shadow-lg p-6">
            <div className="text-lg font-semibold text-white flex items-center gap-2 mb-4">
              <GripVertical size={16} className="text-zinc-600 cursor-grab active:cursor-grabbing" />
              Mitigation Progress Overview
            </div>
            <div className="flex flex-col justify-center items-center h-full text-zinc-600 border border-dashed border-zinc-800 rounded-lg p-4">
               <TrendingUp size={24} className="mb-2 opacity-50" />
               <span className="text-xs uppercase tracking-widest">Chart Coming Soon</span>
            </div>
          </div>

          {/* 5. Risk Trend */}
          <div key="risk-trend" className="rounded-xl border border-border bg-surface shadow-lg p-6 overflow-hidden">
            <div className="text-lg font-semibold text-white flex items-center gap-2 mb-4">
              <GripVertical size={16} className="text-zinc-600 cursor-grab active:cursor-grabbing" />
              Global Risk Trend
            </div>
            <RiskTrendChart />
          </div>

          {/* 6. Asset Distribution */}
          <div key="asset-distribution" className="rounded-xl border border-border bg-surface shadow-lg p-6">
            <div className="text-lg font-semibold text-white flex items-center gap-2 mb-4">
              <GripVertical size={16} className="text-zinc-600 cursor-grab active:cursor-grabbing" />
              Asset Criticality Distribution
            </div>
            <div className="flex flex-col justify-center items-center h-full text-zinc-600 border border-dashed border-zinc-800 rounded-lg p-4">
                <Server size={24} className="mb-2 opacity-50" />
                <span className="text-xs uppercase tracking-widest">Distribution Chart Coming Soon</span>
            </div>
          </div>
        </GridLayout>
    </motion.div>
  );
};