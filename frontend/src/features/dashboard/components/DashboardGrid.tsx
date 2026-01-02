import React, { useEffect, useState } from 'react';
import { ShieldAlert, CheckCircle2, Server, TrendingUp, AlertTriangle, ChevronRight, Loader2, FileDown, GripVertical, Clock, TrendingDown } from 'lucide-react';
import { motion } from 'framer-motion';
import { Link } from 'react-router-dom';
import type { Layout } from 'react-grid-layout';
import GridLayout from 'react-grid-layout';
import 'react-grid-layout/css/styles.css';
import 'react-resizable/css/styles.css';

// Stores & Components
import { useRiskStore } from '../../../hooks/useRiskStore';
import { useAssetStore } from '../../../hooks/useAssetStore';
import { useAuthStore } from '../../../hooks/useAuthStore';
import { RiskDistribution } from './RiskDistribution';
import { TopVulnerabilities } from './TopVulnerabilities';
import { AverageMitigationTime } from './AverageMitigationTime';
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
  icon?: React.ElementType;
}

// Enhanced Glassmorphic Widget Component
const GlassmorphicWidget: React.FC<WidgetProps> = ({ 
  title, 
  children, 
  className = '', 
  padding = 'p-6', 
  isDragging = false,
  icon: Icon 
}) => (
  <div className={`rounded-2xl border border-white/10 bg-gradient-to-br from-white/5 to-white/0 backdrop-blur-xl shadow-2xl 
                  ${isDragging ? 'opacity-50' : ''} hover:border-white/20 transition-all duration-300 ${className}`}>
    <div className={`text-lg font-semibold text-white mb-4 flex items-center gap-2 ${padding} react-grid-dragHandleExampleStyle cursor-grab active:cursor-grabbing`}>
      <GripVertical size={16} className="text-zinc-600" />
      {Icon && <Icon size={20} className="text-primary" />}
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
  <div className="flex items-center justify-between p-4 bg-gradient-to-br from-white/5 to-white/0 rounded-lg border border-white/10 
                  transition-all duration-200 hover:bg-white/10 hover:border-white/20">
    <div className="flex items-center">
      <div className={`p-2 rounded-full ${color}/20 mr-3 bg-gradient-to-br from-${color}/20 to-transparent`}>
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

// =================================================================
// Le Composant Principal : DashboardGrid avec Drag-and-Drop
// =================================================================

const defaultLayout: Layout[] = [
  { i: 'risk-distribution', x: 0, y: 0, w: 6, h: 4 },
  { i: 'risk-trend', x: 6, y: 0, w: 6, h: 4 },
  { i: 'top-vulnerabilities', x: 0, y: 4, w: 6, h: 4 },
  { i: 'mitigation-time', x: 6, y: 4, w: 6, h: 4 },
  { i: 'key-indicators', x: 0, y: 8, w: 12, h: 3 },
  { i: 'top-risks', x: 0, y: 11, w: 12, h: 4 },
];

export const DashboardGrid: React.FC = () => {
  const { risks, fetchRisks, isLoading: isRisksLoading } = useRiskStore();
  const { assets, fetchAssets, isLoading: isAssetsLoading } = useAssetStore();
  const { user } = useAuthStore();
  const [layout, setLayout] = useState<Layout[]>(defaultLayout);
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
        className="p-8 space-y-8 h-full overflow-y-auto bg-gradient-to-br from-background via-background to-blue-950/10"
    >
        {/* Enhanced Header with Gradient */}
        <div className="flex flex-col md:flex-row justify-between items-start md:items-center pb-6 border-b border-white/10 gap-4">
            <div>
                <h1 className="text-4xl font-bold text-white flex items-center gap-3 bg-gradient-to-r from-white to-blue-200 bg-clip-text text-transparent">
                    <ShieldAlert className="text-primary drop-shadow-lg" size={32} /> {welcomeMessage}
                </h1>
                <p className="text-zinc-400 text-sm mt-2 ml-12">Real-time security risk assessment and monitoring</p>
            </div>
            
            <div className="flex items-center gap-3 flex-wrap">
                <Link to="/assets">
                    <Button variant="ghost" className="text-zinc-400 hover:text-white border-white/20 hover:bg-white/5">
                        <Server size={16} className="mr-2" /> Inventory
                    </Button>
                </Link>
                <Button onClick={resetLayout} variant="ghost" className="text-zinc-400 hover:text-white border-white/20 hover:bg-white/5">
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
          onDragStart={() => {}}
          onDragStop={() => {}}
          containerPadding={[0, 0]}
          margin={[24, 24]}
          draggableHandle=".react-grid-dragHandleExampleStyle"
        >
          {/* 1. Risk Distribution Donut Chart */}
          <div key="risk-distribution">
            <GlassmorphicWidget 
              title="Risk Distribution" 
              icon={TrendingUp}
              className="rounded-2xl overflow-hidden h-full"
            >
              <RiskDistribution />
            </GlassmorphicWidget>
          </div>

          {/* 2. Risk Score Trends Line Chart */}
          <div key="risk-trend">
            <GlassmorphicWidget 
              title="Risk Score Trends" 
              icon={TrendingDown}
              className="rounded-2xl overflow-hidden h-full"
            >
              <RiskTrendChart />
            </GlassmorphicWidget>
          </div>

          {/* 3. Top Vulnerabilities List */}
          <div key="top-vulnerabilities">
            <GlassmorphicWidget 
              title="Top Vulnerabilities" 
              icon={AlertTriangle}
              padding="p-6"
              className="rounded-2xl overflow-hidden h-full"
            >
              <TopVulnerabilities />
            </GlassmorphicWidget>
          </div>

          {/* 4. Average Mitigation Time Gauge */}
          <div key="mitigation-time">
            <GlassmorphicWidget 
              title="Average Mitigation Time" 
              icon={Clock}
              className="rounded-2xl overflow-hidden h-full"
            >
              <AverageMitigationTime />
            </GlassmorphicWidget>
          </div>

          {/* 5. Key Indicators Stats */}
          <div key="key-indicators">
            <GlassmorphicWidget 
              title="Key Indicators" 
              icon={ShieldAlert}
              className="rounded-2xl overflow-hidden h-full"
              padding="p-6"
            >
              <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                <StatCard 
                    label="Critical Risks" 
                    value={criticalRisks} 
                    icon={AlertTriangle} 
                    color="text-red-400" 
                />
                <StatCard 
                    label="Total Active Risks" 
                    value={totalRisks} 
                    icon={ShieldAlert} 
                    color="text-yellow-400" 
                />
                <StatCard 
                    label="Mitigated Risks" 
                    value={`${mitigatedCount} / ${totalRisks}`} 
                    icon={CheckCircle2} 
                    color="text-emerald-400" 
                />
                <StatCard 
                    label="Total Assets" 
                    value={assets.length} 
                    icon={Server} 
                    color="text-blue-400" 
                />
              </div>
            </GlassmorphicWidget>
          </div>

          {/* 6. Top Unmitigated Risks */}
          <div key="top-risks">
            <GlassmorphicWidget 
              title="Top Unmitigated Risks" 
              icon={AlertTriangle}
              padding="p-6"
              className="rounded-2xl overflow-hidden h-full"
            >
              {topRisks.length > 0 ? (
                  <div className="space-y-3 overflow-y-auto max-h-[300px] pr-2">
                      {topRisks.map((risk, index) => (
                          <Link 
                              to={`/?riskId=${risk.id}`} 
                              key={risk.id} 
                              className="flex justify-between items-center p-3 rounded-lg border border-white/10 
                                        bg-gradient-to-r from-white/5 to-white/0 hover:bg-white/10 
                                        hover:border-white/20 transition-all duration-200 cursor-pointer group"
                          >
                              <div className="flex items-center gap-3 flex-1">
                                  <span className="text-sm font-bold text-primary w-6">{index + 1}</span>
                                  <TrendingUp size={16} className="text-red-500 group-hover:scale-110 transition-transform flex-shrink-0" />
                                  <div className="min-w-0">
                                      <div className="font-medium text-white group-hover:text-primary transition-colors truncate">{risk.title}</div>
                                      <div className="text-xs text-zinc-500 truncate">{risk.description}</div>
                                  </div>
                              </div>
                              <div className="flex items-center gap-4 flex-shrink-0 ml-2">
                                  <span className={`text-xs font-bold px-3 py-1.5 rounded-lg border whitespace-nowrap transition-all ${
                                      risk.score >= 15 
                                      ? 'bg-red-500/20 text-red-400 border-red-500/30' 
                                      : 'bg-yellow-500/20 text-yellow-400 border-yellow-500/30'
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
            </GlassmorphicWidget>
          </div>
        </GridLayout>
    </motion.div>
  );
};