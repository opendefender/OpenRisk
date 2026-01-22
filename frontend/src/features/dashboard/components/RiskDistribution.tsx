import { useEffect, useState } from 'react';
import { PieChart, Pie, Cell, ResponsiveContainer, Tooltip } from 'recharts';
import { api } from '../../../lib/api';
import { Loader2, AlertTriangle } from 'lucide-react';

interface RiskDistributionRecord {
  level: string;
  count: number;
}

interface RiskDistributionData {
  critical: number;
  high: number;
  medium: number;
  low: number;
}

export const RiskDistribution = () => {
  const [data, setData] = useState<RiskDistributionData | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const fetchData = async () => {
      try {
        const res = await api.get('/stats/risk-distribution');
        const records: RiskDistributionRecord[] = res.data || [];
        
        // Transform backend response format to match UI expectations
        const transformed: RiskDistributionData = {
          critical: 0,
          high: 0,
          medium: 0,
          low: 0
        };
        
        records.forEach((record: RiskDistributionRecord) => {
          const level = record.level?.toUpperCase() || 'LOW';
          if (level in transformed) {
            transformed[level as keyof RiskDistributionData] = record.count || 0;
          }
        });
        
        setData(transformed);
        setError(null);
      } catch (err) {
        console.error('Failed to fetch risk distribution:', err);
        setError('Failed to load risk distribution data');
        setData(null);
      } finally {
        setIsLoading(false);
      }
    };

    fetchData();
  }, []);

  if (isLoading) {
    return (
      <div className="flex justify-center items-center h-full text-zinc-500">
        <Loader2 className="animate-spin mr-2" size={20} />
        Loading Distribution...
      </div>
    );
  }

  if (error || !data) {
    return (
      <div className="flex flex-col items-center justify-center h-full text-zinc-500">
        <AlertTriangle size={32} className="mb-2 text-orange-500/50" />
        <p className="text-sm">{error || 'No distribution data available'}</p>
      </div>
    );
  }

  const chartData = [
    { name: 'Critical', value: data.critical, color: '#ef4444' },
    { name: 'High', value: data.high, color: '#f97316' },
    { name: 'Medium', value: data.medium, color: '#eab308' },
    { name: 'Low', value: data.low, color: '#3b82f6' },
  ];

  const total = data.critical + data.high + data.medium + data.low;

  return (
    <div className="h-full w-full flex flex-col">
      <div className="flex-1 flex items-center justify-center">
        <ResponsiveContainer width="100%" height="100%">
          <PieChart>
            <Pie
              data={chartData}
              cx="50%"
              cy="50%"
              innerRadius={60}
              outerRadius={100}
              paddingAngle={2}
              dataKey="value"
            >
              {chartData.map((entry, index) => (
                <Cell key={`cell-${index}`} fill={entry.color} />
              ))}
            </Pie>
            <Tooltip
              contentStyle={{
                backgroundColor: '#18181b',
                border: '1px solid #27272a',
                borderRadius: '8px',
              }}
              formatter={(value) => [value, 'Count']}
            />
          </PieChart>
        </ResponsiveContainer>
      </div>

      {/* Legend */}
      <div className="grid grid-cols-2 gap-3 mt-4">
        {chartData.map((item) => (
          <div key={item.name} className="flex items-center gap-2 p-2 rounded-lg bg-white/5 border border-white/10">
            <div
              className="w-3 h-3 rounded-full"
              style={{ backgroundColor: item.color }}
            />
            <span className="text-xs text-zinc-300">{item.name}</span>
            <span className="ml-auto text-xs font-bold text-white">{item.value}</span>
          </div>
        ))}
      </div>

      {/* Stats Summary */}
      <div className="mt-4 p-3 rounded-lg bg-gradient-to-r from-blue-500/10 to-purple-500/10 border border-blue-500/20">
        <p className="text-xs text-zinc-400">Total Risks: <span className="text-white font-bold">{total}</span></p>
      </div>
    </div>
  );
};
