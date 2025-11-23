import { useMemo, useState, useEffect } from 'react';
import { Responsive, WidthProvider } from 'react-grid-layout';
import { motion } from 'framer-motion';
import { GlobalScore } from './widgets/GlobalScore';
import { RiskHeatmap } from './widgets/RiskHeatmap';
import { useRiskStore } from '../../hooks/useRiskStore';
import Confetti from 'react-confetti';
import { useWindowSize } from 'react-use';

const ResponsiveGridLayout = WidthProvider(Responsive);

// Layout par défaut "Perfect UX"
const defaultLayouts = {
  lg: [
    { i: 'score', x: 0, y: 0, w: 3, h: 4 },
    { i: 'heatmap', x: 3, y: 0, w: 5, h: 4 },
    { i: 'stats', x: 8, y: 0, w: 4, h: 4 },
    { i: 'quick-actions', x: 0, y: 4, w: 12, h: 2 },
    { i: 'trends', x: 0, y: 6, w: 8, h: 4 },
    { i: 'top-risks', x: 8, y: 6, w: 4, h: 4 },
  ],
};

export const DashboardGrid = () => {
  const { width, height } = useWindowSize();
  const [showConfetti, setShowConfetti] = useState(false);
  
  // Persistance localStorage pour le Drag & Drop
  const [layouts, setLayouts] = useState(() => {
    const saved = localStorage.getItem('dashboard-layouts');
    return saved ? JSON.parse(saved) : defaultLayouts;
  });

  const onLayoutChange = (currentLayout: any, allLayouts: any) => {
    setLayouts(allLayouts);
    localStorage.setItem('dashboard-layouts', JSON.stringify(allLayouts));
  };

  // Demo Confetti effect
  useEffect(() => {
    // Simule une mitigation réussie au chargement
    setTimeout(() => setShowConfetti(true), 1000);
    setTimeout(() => setShowConfetti(false), 5000);
  }, []);

  // Wrapper de widget pour le style Glassmorphism unifié
  const WidgetWrapper = ({ children, title, ...props }: any) => (
    <div {...props} className="h-full w-full bg-surface/50 backdrop-blur-xl border border-border rounded-xl shadow-sm hover:shadow-glow hover:border-primary/30 transition-all duration-300 overflow-hidden flex flex-col relative group">
      {/* Drag Handle discret */}
      <div className="absolute top-2 right-2 opacity-0 group-hover:opacity-100 cursor-move text-zinc-600 hover:text-white transition-opacity z-10">
         :::
      </div>
      {children}
    </div>
  );

  return (
    <>
      {showConfetti && <Confetti width={width} height={height} numberOfPieces={200} recycle={false} />}
      
      <ResponsiveGridLayout
        className="layout"
        layouts={layouts}
        breakpoints={{ lg: 1200, md: 996, sm: 768, xs: 480, xxs: 0 }}
        cols={{ lg: 12, md: 10, sm: 6, xs: 4, xxs: 2 }}
        rowHeight={60}
        onLayoutChange={onLayoutChange}
        isDraggable
        isResizable
        draggableHandle=".cursor-move" 
        margin={[16, 16]}
      >
        <div key="score">
          <WidgetWrapper>
            <GlobalScore score={85} />
          </WidgetWrapper>
        </div>

        <div key="heatmap">
          <WidgetWrapper>
            <RiskHeatmap />
          </WidgetWrapper>
        </div>

        <div key="stats">
          <WidgetWrapper>
             <div className="p-6">
                <h3 className="text-zinc-400 text-sm uppercase font-bold mb-4">Statistiques Rapides</h3>
                <div className="space-y-4">
                    <div className="flex justify-between items-center">
                        <span>Total Assets</span>
                        <span className="font-mono text-primary">1,240</span>
                    </div>
                    <div className="flex justify-between items-center">
                        <span>Open Incidents</span>
                        <span className="font-mono text-red-500">3</span>
                    </div>
                    <div className="flex justify-between items-center">
                        <span>Mitigated Risks</span>
                        <span className="font-mono text-emerald-500">42</span>
                    </div>
                </div>
             </div>
          </WidgetWrapper>
        </div>

        <div key="quick-actions">
           <WidgetWrapper>
             <div className="h-full flex items-center justify-around px-8">
                <button className="flex flex-col items-center gap-2 group">
                    <div className="w-12 h-12 rounded-full bg-blue-500/10 text-blue-500 flex items-center justify-center group-hover:scale-110 transition-transform border border-blue-500/20">
                        +
                    </div>
                    <span className="text-xs font-medium text-zinc-400 group-hover:text-white">Créer Risque</span>
                </button>
                <button className="flex flex-col items-center gap-2 group">
                    <div className="w-12 h-12 rounded-full bg-purple-500/10 text-purple-500 flex items-center justify-center group-hover:scale-110 transition-transform border border-purple-500/20">
                        ⚡
                    </div>
                    <span className="text-xs font-medium text-zinc-400 group-hover:text-white">Lancer Scan</span>
                </button>
                <button className="flex flex-col items-center gap-2 group">
                    <div className="w-12 h-12 rounded-full bg-emerald-500/10 text-emerald-500 flex items-center justify-center group-hover:scale-110 transition-transform border border-emerald-500/20">
                        📄
                    </div>
                    <span className="text-xs font-medium text-zinc-400 group-hover:text-white">Rapport PDF</span>
                </button>
             </div>
           </WidgetWrapper>
        </div>

         {/* Placeholders pour les autres widgets demandés */}
        <div key="trends"><WidgetWrapper><div className="flex items-center justify-center h-full text-zinc-600">Widget Trends (Coming Soon)</div></WidgetWrapper></div>
        <div key="top-risks"><WidgetWrapper><div className="flex items-center justify-center h-full text-zinc-600">Widget Top Risks (Coming Soon)</div></WidgetWrapper></div>

      </ResponsiveGridLayout>
    </>
  );
};