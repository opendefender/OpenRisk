import React, { useEffect, useState } from 'react';
import { ShieldAlert, CheckCircle, Server, TrendingUp, AlertTriangle, ChevronRight, Loader, FileDown, GripVertical, Clock, TrendingDown } from 'lucide-react';
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
  padding = 'p-', 
  isDragging = false,
  icon: Icon 
}) => (
  <div className={rounded-xl border border-white/ bg-gradient-to-br from-white/ to-white/ backdrop-blur-xl shadow-xl 
                  ${isDragging ? 'opacity-' : ''} hover:border-white/ transition-all duration- ${className}}>
    <div className={text-lg font-semibold text-white mb- flex items-center gap- ${padding} react-grid-dragHandleExampleStyle cursor-grab active:cursor-grabbing}>
      <GripVertical size={} className="text-zinc-" />
      {Icon && <Icon size={} className="text-primary" />}
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

const StatCard: React.FC<StatCardProps> = ({ label, value, icon: Icon, color = 'text-blue-' }) => (
  <div className="flex items-center justify-between p- bg-gradient-to-br from-white/ to-white/ rounded-lg border border-white/ 
                  transition-all duration- hover:bg-white/ hover:border-white/">
    <div className="flex items-center">
      <div className={p- rounded-full ${color}/ mr- bg-gradient-to-br from-${color}/ to-transparent}>
        <Icon size={} className={color} />
      </div>
      <div>
        <div className="text-zinc- text-xs uppercase tracking-wider">{label}</div>
        <div className="text-white text-xl font-bold">{value}</div>
      </div>
    </div>
    <ChevronRight size={} className="text-zinc-" />
  </div>
);

// =================================================================
// Le Composant Principal : DashboardGrid avec Drag-and-Drop
// =================================================================

// =================================================================
// Le Composant Principal : DashboardGrid avec Drag-and-Drop
// =================================================================

const defaultLayout: Layout[] = [
  { i: 'risk-distribution', x: , y: , w: , h:  },
  { i: 'risk-trend', x: , y: , w: , h:  },
  { i: 'top-vulnerabilities', x: , y: , w: , h:  },
  { i: 'mitigation-time', x: , y: , w: , h:  },
  { i: 'key-indicators', x: , y: , w: , h:  },
  { i: 'top-risks', x: , y: , w: , h:  },
];

export const DashboardGrid: React.FC = () => {
  const { risks, fetchRisks, isLoading: isRisksLoading } = useRiskStore();
  const { assets, fetchAssets, isLoading: isAssetsLoading } = useAssetStore();
  const { user } = useAuthStore();
  const [layout, setLayout] = useState<Layout[]>(defaultLayout);
  const [containerWidth, setContainerWidth] = useState();
  
  // Track container width for responsive grid
  useEffect(() => {
    const handleResize = () => {
      const mainElement = document.querySelector('main');
      if (mainElement) {
        // Account for padding (p- = px on each side)
        setContainerWidth(Math.max(mainElement.clientWidth - , ));
      }
    };
    
    handleResize();
    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, []);
  
  // Calcul des Stats Rapides
  const totalRisks = risks.length;
  const criticalRisks = risks.filter(r => r.score >= ).length;
  const mitigatedCount = risks.filter(r => r.status === 'MITIGATED').length;
  
  // Top  des risques non mitigs (Tris par score dcroissant)
  const topRisks = [...risks]
    .filter(r => r.status !== 'MITIGATED' && r.status !== 'CLOSED')
    .sort((a, b) => b.score - a.score)
    .slice(, );

  // Chargement initial des donnes
  useEffect(() => {
    fetchRisks();
    fetchAssets();
    // La matrice gre son propre fetch via /stats/risk-matrix
  }, [fetchRisks, fetchAssets]);

  // Handler pour l'export PDF
  const handleExport = () => {
    const apiUrl = import.meta.env.VITE_API_URL || 'http://localhost:/api/v';
    window.open(${apiUrl}/export/pdf, '_blank');
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

  const welcomeMessage = Welcome back, ${user?.full_name || user?.email || 'Admin'}.;
  
  // Loader global
  if (isRisksLoading || isAssetsLoading) {
      return (
          <div className="flex justify-center items-center h-[vh] text-zinc-">
              <Loader className="animate-spin mr-" size={} /> Loading OpenRisk data...
          </div>
      );
  }

  return (
    <motion.div 
        initial={{ opacity:  }} 
        animate={{ opacity:  }} 
        className="p- space-y- h-full overflow-y-auto bg-gradient-to-br from-background via-background to-blue-/"
    >
        {/ Enhanced Header with Gradient /}
        <div className="flex flex-col md:flex-row justify-between items-start md:items-center pb- border-b border-white/ gap-">
            <div>
                <h className="text-xl font-bold text-white flex items-center gap- bg-gradient-to-r from-white to-blue- bg-clip-text text-transparent">
                    <ShieldAlert className="text-primary drop-shadow-lg" size={} /> {welcomeMessage}
                </h>
                <p className="text-zinc- text-sm mt- ml-">Real-time security risk assessment and monitoring</p>
            </div>
            
            <div className="flex items-center gap- flex-wrap">
                <Link to="/assets">
                    <Button variant="ghost" className="text-zinc- hover:text-white border-white/ hover:bg-white/">
                        <Server size={} className="mr-" /> Inventory
                    </Button>
                </Link>
                <Button onClick={resetLayout} variant="ghost" className="text-zinc- hover:text-white border-white/ hover:bg-white/">
                    Reset Layout
                </Button>
                <Button onClick={handleExport} variant="secondary">
                    <FileDown size={} className="mr-" /> Export Report
                </Button>
            </div>
        </div>

        {/ Grille Draggable /}
        <GridLayout 
          className="bg-transparent w-full"
          layout={layout}
          onLayoutChange={handleLayoutChange}
          cols={}
          rowHeight={}
          width={containerWidth}
          isDraggable={true}
          isResizable={true}
          compactType="vertical"
          preventCollision={false}
          useCSSTransforms={true}
          onDragStart={() => {}}
          onDragStop={() => {}}
          containerPadding={[, ]}
          margin={[, ]}
          draggableHandle=".react-grid-dragHandleExampleStyle"
        >
          {/ . Risk Distribution Donut Chart /}
          <div key="risk-distribution">
            <GlassmorphicWidget 
              title="Risk Distribution" 
              icon={TrendingUp}
              className="rounded-xl overflow-hidden h-full"
            >
              <RiskDistribution />
            </GlassmorphicWidget>
          </div>

          {/ . Risk Score Trends Line Chart /}
          <div key="risk-trend">
            <GlassmorphicWidget 
              title="Risk Score Trends" 
              icon={TrendingDown}
              className="rounded-xl overflow-hidden h-full"
            >
              <RiskTrendChart />
            </GlassmorphicWidget>
          </div>

          {/ . Top Vulnerabilities List /}
          <div key="top-vulnerabilities">
            <GlassmorphicWidget 
              title="Top Vulnerabilities" 
              icon={AlertTriangle}
              padding="p-"
              className="rounded-xl overflow-hidden h-full"
            >
              <TopVulnerabilities />
            </GlassmorphicWidget>
          </div>

          {/ . Average Mitigation Time Gauge /}
          <div key="mitigation-time">
            <GlassmorphicWidget 
              title="Average Mitigation Time" 
              icon={Clock}
              className="rounded-xl overflow-hidden h-full"
            >
              <AverageMitigationTime />
            </GlassmorphicWidget>
          </div>

          {/ . Key Indicators Stats /}
          <div key="key-indicators">
            <GlassmorphicWidget 
              title="Key Indicators" 
              icon={ShieldAlert}
              className="rounded-xl overflow-hidden h-full"
              padding="p-"
            >
              <div className="grid grid-cols- md:grid-cols- gap-">
                <StatCard 
                    label="Critical Risks" 
                    value={criticalRisks} 
                    icon={AlertTriangle} 
                    color="text-red-" 
                />
                <StatCard 
                    label="Total Active Risks" 
                    value={totalRisks} 
                    icon={ShieldAlert} 
                    color="text-yellow-" 
                />
                <StatCard 
                    label="Mitigated Risks" 
                    value={${mitigatedCount} / ${totalRisks}} 
                    icon={CheckCircle} 
                    color="text-emerald-" 
                />
                <StatCard 
                    label="Total Assets" 
                    value={assets.length} 
                    icon={Server} 
                    color="text-blue-" 
                />
              </div>
            </GlassmorphicWidget>
          </div>

          {/ . Top Unmitigated Risks /}
          <div key="top-risks">
            <GlassmorphicWidget 
              title="Top Unmitigated Risks" 
              icon={AlertTriangle}
              padding="p-"
              className="rounded-xl overflow-hidden h-full"
            >
              {topRisks.length >  ? (
                  <div className="space-y- overflow-y-auto max-h-[px] pr-">
                      {topRisks.map((risk, index) => (
                          <Link 
                              to={/?riskId=${risk.id}} 
                              key={risk.id} 
                              className="flex justify-between items-center p- rounded-lg border border-white/ 
                                        bg-gradient-to-r from-white/ to-white/ hover:bg-white/ 
                                        hover:border-white/ transition-all duration- cursor-pointer group"
                          >
                              <div className="flex items-center gap- flex-">
                                  <span className="text-sm font-bold text-primary w-">{index + }</span>
                                  <TrendingUp size={} className="text-red- group-hover:scale- transition-transform flex-shrink-" />
                                  <div className="min-w-">
                                      <div className="font-medium text-white group-hover:text-primary transition-colors truncate">{risk.title}</div>
                                      <div className="text-xs text-zinc- truncate">{risk.description}</div>
                                  </div>
                              </div>
                              <div className="flex items-center gap- flex-shrink- ml-">
                                  <span className={text-xs font-bold px- py-. rounded-lg border whitespace-nowrap transition-all ${
                                      risk.score >=  
                                      ? 'bg-red-/ text-red- border-red-/' 
                                      : 'bg-yellow-/ text-yellow- border-yellow-/'
                                  }}>
                                      SCORE: {risk.score}
                                  </span>
                                  <ChevronRight size={} className="text-zinc- group-hover:translate-x- transition-transform" />
                              </div>
                          </Link>
                      ))}
                  </div>
              ) : (
                  <div className="flex flex-col items-center justify-center py- text-zinc-">
                      <CheckCircle size={} className="mb- text-emerald-/" />
                      <p>No high priority risks found. Excellent work!</p>
                  </div>
              )}
            </GlassmorphicWidget>
          </div>
        </GridLayout>
    </motion.div>
  );
};