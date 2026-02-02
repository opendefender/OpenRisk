import { TrendingUp } from 'lucide-react';
import { motion } from 'framer-motion';
import { Card } from '../../../components/Card';

interface RiskMonitor {
  riskTitle: string;
  currentLevel: string;
  trend: 'up' | 'down' | 'stable';
  controlStatus: 'effective' | 'ineffective' | 'partial';
  lastReview: string;
}

export const RiskMonitoringPhase = () => {
  const monitors: RiskMonitor[] = [
    {
      riskTitle: 'Data Breach Risk',
      currentLevel: 'HIGH',
      trend: 'down',
      controlStatus: 'effective',
      lastReview: '2024-02-01',
    },
    {
      riskTitle: 'System Downtime',
      currentLevel: 'MEDIUM',
      trend: 'stable',
      controlStatus: 'partial',
      lastReview: '2024-02-02',
    },
  ];

  const getTrendIcon = (trend: string) => {
    switch (trend) {
      case 'up':
        return '📈 Increasing';
      case 'down':
        return '📉 Decreasing';
      case 'stable':
        return '➡️ Stable';
      default:
        return 'N/A';
    }
  };

  return (
    <div className="space-y-6">
      {/* Phase Info */}
      <Card>
        <div className="p-6">
          <div className="flex items-start gap-4">
            <TrendingUp size={32} className="text-blue-500 flex-shrink-0 mt-1" />
            <div>
              <h3 className="text-xl font-bold mb-2">Phase 4: Risk Monitoring</h3>
              <p className="text-zinc-400 mb-4">
                Continuously monitor risks and the effectiveness of treatment controls. Track trends, identify changes, and escalate issues as needed.
              </p>
              <div className="grid grid-cols-4 gap-4 text-sm">
                <div>
                  <p className="text-zinc-400">Total Monitored</p>
                  <p className="text-2xl font-bold">{monitors.length}</p>
                </div>
                <div>
                  <p className="text-zinc-400">Effective Controls</p>
                  <p className="text-2xl font-bold text-green-400">
                    {monitors.filter((m) => m.controlStatus === 'effective').length}
                  </p>
                </div>
                <div>
                  <p className="text-zinc-400">Partial Controls</p>
                  <p className="text-2xl font-bold text-yellow-400">
                    {monitors.filter((m) => m.controlStatus === 'partial').length}
                  </p>
                </div>
                <div>
                  <p className="text-zinc-400">Trending Down</p>
                  <p className="text-2xl font-bold text-green-400">
                    {monitors.filter((m) => m.trend === 'down').length}
                  </p>
                </div>
              </div>
            </div>
          </div>
        </div>
      </Card>

      {/* Monitoring Dashboard */}
      <div className="space-y-3">
        <h3 className="text-lg font-bold">Risk Monitoring Dashboard</h3>

        {monitors.length === 0 ? (
          <Card>
            <div className="p-12 text-center">
              <TrendingUp size={48} className="mx-auto mb-4 text-zinc-500" />
              <p className="text-zinc-400">No risks being monitored</p>
            </div>
          </Card>
        ) : (
          monitors.map((monitor, idx) => (
            <motion.div
              key={monitor.riskTitle}
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: idx * 0.1 }}
            >
              <Card>
                <div className="p-6">
                  <div className="flex items-center justify-between mb-4">
                    <h3 className="text-lg font-bold">{monitor.riskTitle}</h3>
                    <span className="text-2xl">{getTrendIcon(monitor.trend)}</span>
                  </div>

                  <div className="grid grid-cols-4 gap-4">
                    <div>
                      <p className="text-xs text-zinc-500">Current Level</p>
                      <p className="text-lg font-bold">{monitor.currentLevel}</p>
                    </div>
                    <div>
                      <p className="text-xs text-zinc-500">Trend</p>
                      <p className="text-lg font-bold">{monitor.trend}</p>
                    </div>
                    <div>
                      <p className="text-xs text-zinc-500">Control Status</p>
                      <p className={`text-lg font-bold ${
                        monitor.controlStatus === 'effective' ? 'text-green-400' :
                        monitor.controlStatus === 'partial' ? 'text-yellow-400' : 'text-red-400'
                      }`}>
                        {monitor.controlStatus.charAt(0).toUpperCase() + monitor.controlStatus.slice(1)}
                      </p>
                    </div>
                    <div>
                      <p className="text-xs text-zinc-500">Last Review</p>
                      <p className="text-lg font-bold">{monitor.lastReview}</p>
                    </div>
                  </div>
                </div>
              </Card>
            </motion.div>
          ))
        )}
      </div>
    </div>
  );
};
