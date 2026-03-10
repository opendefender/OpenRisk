import { useEffect, useState } from 'react';
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer } from 'recharts';
import { api } from '../../../lib/api';
import { Loader2, TrendingUp, Calendar } from 'lucide-react';

interface TrendDataPoint {
  date: string;
  trend30: number;
  trend60: number;
  trend90: number;
  average: number;
}

interface RiskTrendMultiPeriodProps {
  className?: string;
}

/**
 * RiskTrendMultiPeriod Component
 * Displays risk trends over 30, 60, and 90-day periods
 * Helps identify patterns and predict future risk trajectories
 */
export const RiskTrendMultiPeriod: React.FC<RiskTrendMultiPeriodProps> = ({ className = '' }) => {
  const [data, setData] = useState<TrendDataPoint[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedPeriod, setSelectedPeriod] = useState<'30' | '60' | '90'>('30');

  useEffect(() => {
    const fetchTrendData = async () => {
      try {
        setIsLoading(true);
        setError(null);

        // Fetch trends for all periods
        const [trend30, trend60, trend90] = await Promise.all([
          api.get('/analytics/risks/trends?days=30'),
          api.get('/analytics/risks/trends?days=60'),
          api.get('/analytics/risks/trends?days=90'),
        ]);

        // Transform and merge data
        const trends30 = trend30.data?.trends || [];
        const trends60 = trend60.data?.trends || [];
        const trends90 = trend90.data?.trends || [];

        // Create unified dataset
        const mergedData: TrendDataPoint[] = [];
        const maxLength = Math.max(trends30.length, trends60.length, trends90.length);

        for (let i = 0; i < maxLength; i++) {
          mergedData.push({
            date: trends30[i]?.date || trends60[i]?.date || trends90[i]?.date || `Day ${i + 1}`,
            trend30: trends30[i]?.value || 0,
            trend60: trends60[i]?.value || 0,
            trend90: trends90[i]?.value || 0,
            average: (
              (trends30[i]?.value || 0) +
              (trends60[i]?.value || 0) +
              (trends90[i]?.value || 0)
            ) / 3,
          });
        }

        setData(mergedData);
      } catch (err) {
        console.error('Failed to fetch trend data:', err);
        setError('Failed to load trend data');
        // Generate sample data for demo
        setData(generateSampleData());
      } finally {
        setIsLoading(false);
      }
    };

    fetchTrendData();
  }, []);

  // Generate sample data for demonstration
  const generateSampleData = (): TrendDataPoint[] => {
    const data: TrendDataPoint[] = [];
    for (let i = 0; i < 30; i++) {
      const date = new Date();
      date.setDate(date.getDate() - (30 - i));
      const baseValue = 45 + Math.sin(i / 5) * 15;
      data.push({
        date: date.toISOString().split('T')[0],
        trend30: baseValue + Math.random() * 10,
        trend60: baseValue + Math.random() * 8,
        trend90: baseValue + Math.random() * 6,
        average: baseValue + Math.random() * 8,
      });
    }
    return data;
  };

  if (isLoading) {
    return (
      <div className={`flex justify-center items-center h-80 ${className}`}>
        <div className="flex flex-col items-center gap-2 text-zinc-500">
          <Loader2 size={32} className="animate-spin" />
          <p className="text-sm">Loading trend data...</p>
        </div>
      </div>
    );
  }

  // Filter data based on selected period
  const filteredData = data.slice(-parseInt(selectedPeriod));

  return (
    <div className={`w-full h-full flex flex-col ${className}`}>
      {/* Period Selector */}
      <div className="flex items-center gap-2 mb-4 pb-4 border-b border-white/10">
        <Calendar size={18} className="text-primary" />
        <span className="text-sm font-medium text-zinc-400">Period:</span>
        <div className="flex gap-2 ml-auto">
          {(['30', '60', '90'] as const).map((period) => (
            <button
              key={period}
              onClick={() => setSelectedPeriod(period)}
              className={`px-3 py-1 rounded-lg text-xs font-semibold transition-all ${
                selectedPeriod === period
                  ? 'bg-primary text-white'
                  : 'bg-white/5 text-zinc-400 hover:bg-white/10'
              }`}
            >
              {period} days
            </button>
          ))}
        </div>
      </div>

      {/* Chart */}
      {error ? (
        <div className="flex items-center justify-center h-64 text-red-400">
          <p>{error}</p>
        </div>
      ) : (
        <ResponsiveContainer width="100%" height={280}>
          <LineChart data={filteredData} margin={{ top: 5, right: 30, left: 0, bottom: 5 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="rgba(255,255,255,0.1)" />
            <XAxis
              dataKey="date"
              stroke="rgba(255,255,255,0.5)"
              style={{ fontSize: '12px' }}
              tick={{ fill: 'rgba(255,255,255,0.7)' }}
            />
            <YAxis
              stroke="rgba(255,255,255,0.5)"
              style={{ fontSize: '12px' }}
              tick={{ fill: 'rgba(255,255,255,0.7)' }}
            />
            <Tooltip
              contentStyle={{
                backgroundColor: 'rgba(0,0,0,0.8)',
                border: '1px solid rgba(255,255,255,0.2)',
                borderRadius: '8px',
              }}
              cursor={{ stroke: 'rgba(255,255,255,0.1)' }}
            />
            <Legend
              wrapperStyle={{ paddingTop: '16px' }}
              iconType="line"
            />
            <Line
              type="monotone"
              dataKey="trend30"
              stroke="#3b82f6"
              dot={false}
              strokeWidth={2}
              name="30-Day Trend"
              isAnimationActive={true}
            />
            <Line
              type="monotone"
              dataKey="trend60"
              stroke="#f59e0b"
              dot={false}
              strokeWidth={2}
              name="60-Day Trend"
              isAnimationActive={true}
            />
            <Line
              type="monotone"
              dataKey="trend90"
              stroke="#ef4444"
              dot={false}
              strokeWidth={2}
              name="90-Day Trend"
              isAnimationActive={true}
            />
            <Line
              type="monotone"
              dataKey="average"
              stroke="#10b981"
              dot={false}
              strokeWidth={2}
              strokeDasharray="5 5"
              name="Average"
              isAnimationActive={true}
            />
          </LineChart>
        </ResponsiveContainer>
      )}

      {/* Info Footer */}
      <div className="mt-4 pt-4 border-t border-white/10">
        <div className="grid grid-cols-2 gap-2 text-xs text-zinc-400">
          <div className="flex items-center gap-2">
            <div className="w-2 h-2 rounded-full bg-blue-400"></div>
            <span>30-day moving average</span>
          </div>
          <div className="flex items-center gap-2">
            <div className="w-2 h-2 rounded-full bg-amber-400"></div>
            <span>60-day moving average</span>
          </div>
          <div className="flex items-center gap-2">
            <div className="w-2 h-2 rounded-full bg-red-400"></div>
            <span>90-day moving average</span>
          </div>
          <div className="flex items-center gap-2">
            <div className="w-2 h-2 rounded-full bg-emerald-400"></div>
            <span>Overall average</span>
          </div>
        </div>
      </div>
    </div>
  );
};
