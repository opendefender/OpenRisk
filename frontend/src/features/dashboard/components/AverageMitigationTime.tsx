import { useEffect, useState } from 'react';
import { PieChart, Pie, Cell, ResponsiveContainer } from 'recharts';
import { api } from '../../../lib/api';
import { Loader, AlertTriangle } from 'lucide-react';

interface MitigationMetricsData {
  total_mitigations: number;
  completed_mitigations: number;
  in_progress_mitigations: number;
  planned_mitigations: number;
  average_time_days: number;
  completion_rate: number;
}

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
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const fetchData = async () => {
      try {
        const res = await api.get('/stats/mitigation-metrics');
        const data: MitigationMetricsData = res.data || {};
        
        // Transform backend response format to match UI expectations
        const transformed: MitigationMetrics = {
          averageTimeHours: (data.average_time_days || )  ,
          averageTimeDays: data.average_time_days || ,
          completedCount: data.completed_mitigations || ,
          pendingCount: data.in_progress_mitigations || data.planned_mitigations || ,
          completionRate: data.completion_rate || ,
        };
        
        setMetrics(transformed);
        setError(null);
      } catch (err) {
        console.error('Failed to fetch mitigation metrics:', err);
        setError('Failed to load mitigation metrics');
        setMetrics(null);
      } finally {
        setIsLoading(false);
      }
    };

    fetchData();
  }, []);

  if (isLoading) {
    return (
      <div className="flex justify-center items-center h-full text-zinc-">
        <Loader className="animate-spin mr-" size={} />
        Loading Metrics...
      </div>
    );
  }

  if (error || !metrics) {
    return (
      <div className="flex flex-col items-center justify-center h-full text-zinc-">
        <AlertTriangle size={} className="mb- text-orange-/" />
        <p className="text-sm">{error || 'No metrics available'}</p>
      </div>
    );
  }

  const gaugeLevels = [
    { name: 'Completed', value: metrics.completedCount, color: 'b' },
    { name: 'Pending', value: metrics.pendingCount, color: 'ef' },
  ];

  const hours = metrics.averageTimeHours;
  const minutes = (metrics.averageTimeHours % )  ;

  return (
    <div className="h-full w-full flex flex-col">
      {/ Gauge Chart /}
      <div className="flex- flex flex-col items-center justify-center">
        <div className="relative w- h- flex items-center justify-center">
          <ResponsiveContainer width={} height={}>
            <PieChart>
              <Pie
                data={gaugeLevels}
                cx="%"
                cy="%"
                innerRadius={}
                outerRadius={}
                paddingAngle={}
                dataKey="value"
                startAngle={}
                endAngle={}
              >
                {gaugeLevels.map((entry, index) => (
                  <Cell key={cell-${index}} fill={entry.color} />
                ))}
              </Pie>
            </PieChart>
          </ResponsiveContainer>

          {/ Center Text /}
          <div className="absolute inset- flex flex-col items-center justify-center">
            <span className="text-xl font-bold text-white">
              {Math.floor(hours)}h {Math.floor(minutes)}m
            </span>
            <span className="text-xs text-zinc- mt-">Avg Time</span>
          </div>
        </div>
      </div>

      {/ Stats Grid /}
      <div className="grid grid-cols- gap- mt-">
        <div className="p- rounded-lg bg-gradient-to-br from-emerald-/ to-emerald-/ border border-emerald-/">
          <p className="text-xs text-zinc- uppercase tracking-wider">Completed</p>
          <p className="text-lg font-bold text-emerald- mt-">{metrics.completedCount}</p>
        </div>

        <div className="p- rounded-lg bg-gradient-to-br from-red-/ to-red-/ border border-red-/">
          <p className="text-xs text-zinc- uppercase tracking-wider">Pending</p>
          <p className="text-lg font-bold text-red- mt-">{metrics.pendingCount}</p>
        </div>
      </div>

      {/ Progress Bar /}
      <div className="mt- space-y-">
        <div className="flex items-center justify-between">
          <p className="text-xs text-zinc- uppercase tracking-wider">Completion Rate</p>
          <p className="text-sm font-bold text-white">{metrics.completionRate}%</p>
        </div>

        <div className="w-full h- rounded-full bg-zinc- border border-zinc- overflow-hidden">
          <div
            className="h-full bg-gradient-to-r from-blue- to-cyan- rounded-full transition-all duration-"
            style={{ width: ${metrics.completionRate}% }}
          />
        </div>
      </div>
    </div>
  );
};
