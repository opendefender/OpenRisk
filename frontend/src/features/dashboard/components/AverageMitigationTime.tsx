import { useEffect, useState } from 'react';
import { PieChart, Pie, Cell, ResponsiveContainer } from 'recharts';
import { api } from '../../../lib/api';
import { Loader2 } from 'lucide-react';

interface MitigationMetrics {
  averageTimeHours: number;
  averageTimeDays: number;
  completedCount: number;
  pendingCount: number;
  completionRate: number;
}

export const AverageMitigationTime = () => {
  const [metrics, setMetrics] = useState<MitigationMetrics | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    api.get('/stats/mitigation-metrics')
      .then(res => setMetrics(res.data))
      .catch(() => {
        // Fallback demo data
        setMetrics({
          averageTimeHours: 96,
          averageTimeDays: 4,
          completedCount: 28,
          pendingCount: 12,
          completionRate: 70,
        });
      })
      .finally(() => setIsLoading(false));
  }, []);

  if (isLoading) {
    return (
      <div className="flex justify-center items-center h-full text-zinc-500">
        <Loader2 className="animate-spin mr-2" size={20} />
        Loading Metrics...
      </div>
    );
  }

  if (!metrics) return null;

  const gaugeLevels = [
    { name: 'Completed', value: metrics.completedCount, color: '#10b981' },
    { name: 'Pending', value: metrics.pendingCount, color: '#ef4444' },
  ];

  const hours = metrics.averageTimeHours;
  const minutes = (metrics.averageTimeHours % 1) * 60;

  return (
    <div className="h-full w-full flex flex-col">
      {/* Gauge Chart */}
      <div className="flex-1 flex flex-col items-center justify-center">
        <div className="relative w-32 h-32 flex items-center justify-center">
          <ResponsiveContainer width={150} height={150}>
            <PieChart>
              <Pie
                data={gaugeLevels}
                cx="50%"
                cy="50%"
                innerRadius={50}
                outerRadius={75}
                paddingAngle={2}
                dataKey="value"
                startAngle={180}
                endAngle={0}
              >
                {gaugeLevels.map((entry, index) => (
                  <Cell key={`cell-${index}`} fill={entry.color} />
                ))}
              </Pie>
            </PieChart>
          </ResponsiveContainer>

          {/* Center Text */}
          <div className="absolute inset-0 flex flex-col items-center justify-center">
            <span className="text-2xl font-bold text-white">
              {Math.floor(hours)}h {Math.floor(minutes)}m
            </span>
            <span className="text-xs text-zinc-400 mt-1">Avg Time</span>
          </div>
        </div>
      </div>

      {/* Stats Grid */}
      <div className="grid grid-cols-2 gap-3 mt-4">
        <div className="p-3 rounded-lg bg-gradient-to-br from-emerald-500/10 to-emerald-600/5 border border-emerald-500/20">
          <p className="text-xs text-zinc-400 uppercase tracking-wider">Completed</p>
          <p className="text-lg font-bold text-emerald-400 mt-1">{metrics.completedCount}</p>
        </div>

        <div className="p-3 rounded-lg bg-gradient-to-br from-red-500/10 to-red-600/5 border border-red-500/20">
          <p className="text-xs text-zinc-400 uppercase tracking-wider">Pending</p>
          <p className="text-lg font-bold text-red-400 mt-1">{metrics.pendingCount}</p>
        </div>
      </div>

      {/* Progress Bar */}
      <div className="mt-4 space-y-2">
        <div className="flex items-center justify-between">
          <p className="text-xs text-zinc-400 uppercase tracking-wider">Completion Rate</p>
          <p className="text-sm font-bold text-white">{metrics.completionRate}%</p>
        </div>

        <div className="w-full h-2 rounded-full bg-zinc-800 border border-zinc-700 overflow-hidden">
          <div
            className="h-full bg-gradient-to-r from-blue-500 to-cyan-500 rounded-full transition-all duration-500"
            style={{ width: `${metrics.completionRate}%` }}
          />
        </div>
      </div>
    </div>
  );
};
