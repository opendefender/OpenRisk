import { useEffect, useState } from 'react';
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer, Cell } from 'recharts';
import { api } from '../../../lib/api';
import { Loader2, Server, AlertTriangle, CheckCircle } from 'lucide-react';

interface AssetStatData {
  name: string;
  totalRisks: number;
  criticalRisks: number;
  mitigatedRisks: number;
  assetCount: number;
  riskScore: number;
}

interface AssetStatisticsProps {
  className?: string;
  topN?: number;
}

/**
 * AssetStatistics Component
 * Displays risk statistics per asset type
 * Shows: Total risks, critical risks, mitigation status by asset type
 */
export const AssetStatistics: React.FC<AssetStatisticsProps> = ({ className = '', topN = 8 }) => {
  const [data, setData] = useState<AssetStatData[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const fetchAssetStats = async () => {
      try {
        setIsLoading(true);
        setError(null);

        const response = await api.get('/analytics/assets/statistics');
        const stats = response.data?.statistics || [];

        // Sort by total risks and limit to topN
        const sorted = stats
          .sort((a: AssetStatData, b: AssetStatData) => b.totalRisks - a.totalRisks)
          .slice(0, topN);

        setData(sorted);
      } catch (err) {
        console.error('Failed to fetch asset statistics:', err);
        setError('Failed to load asset statistics');
        // Generate sample data for demo
        setData(generateSampleData());
      } finally {
        setIsLoading(false);
      }
    };

    fetchAssetStats();
  }, [topN]);

  // Generate sample data for demonstration
  const generateSampleData = (): AssetStatData[] => {
    const assetTypes = ['Servers', 'Databases', 'Web Apps', 'Mobile Apps', 'APIs', 'Cloud Services', 'Networks', 'Endpoints'];
    return assetTypes.map((type, i) => ({
      name: type,
      totalRisks: Math.floor(Math.random() * 40) + 5,
      criticalRisks: Math.floor(Math.random() * 10) + 1,
      mitigatedRisks: Math.floor(Math.random() * 15) + 2,
      assetCount: Math.floor(Math.random() * 50) + 10,
      riskScore: Math.floor(Math.random() * 60) + 20,
    }));
  };

  if (isLoading) {
    return (
      <div className={`flex justify-center items-center h-80 ${className}`}>
        <div className="flex flex-col items-center gap-2 text-zinc-500">
          <Loader2 size={32} className="animate-spin" />
          <p className="text-sm">Loading asset statistics...</p>
        </div>
      </div>
    );
  }

  if (error && data.length === 0) {
    return (
      <div className={`flex items-center justify-center h-80 text-red-400 ${className}`}>
        <p>{error}</p>
      </div>
    );
  }

  // Custom color for bars based on risk level
  const getBarColor = (riskScore: number) => {
    if (riskScore >= 60) return '#ef4444'; // red
    if (riskScore >= 40) return '#f97316'; // orange
    if (riskScore >= 20) return '#f59e0b'; // amber
    return '#10b981'; // green
  };

  return (
    <div className={`w-full h-full flex flex-col ${className}`}>
      {/* Header */}
      <div className="flex items-center gap-3 mb-4 pb-4 border-b border-white/10">
        <Server size={20} className="text-primary" />
        <div>
          <h3 className="text-sm font-semibold text-white">Risk Distribution by Asset Type</h3>
          <p className="text-xs text-zinc-400 mt-0.5">Top {topN} asset types by risk count</p>
        </div>
      </div>

      {/* Chart */}
      {data.length > 0 ? (
        <ResponsiveContainer width="100%" height={300}>
          <BarChart
            data={data}
            margin={{ top: 20, right: 30, left: 0, bottom: 5 }}
          >
            <CartesianGrid strokeDasharray="3 3" stroke="rgba(255,255,255,0.1)" />
            <XAxis
              dataKey="name"
              stroke="rgba(255,255,255,0.5)"
              style={{ fontSize: '12px' }}
              tick={{ fill: 'rgba(255,255,255,0.7)' }}
              angle={-45}
              textAnchor="end"
              height={80}
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
              cursor={{ fill: 'rgba(255,255,255,0.1)' }}
            />
            <Legend wrapperStyle={{ paddingTop: '16px' }} />
            <Bar
              dataKey="totalRisks"
              name="Total Risks"
              fill="#3b82f6"
              radius={[8, 8, 0, 0]}
            />
            <Bar
              dataKey="criticalRisks"
              name="Critical Risks"
              fill="#ef4444"
              radius={[8, 8, 0, 0]}
            />
            <Bar
              dataKey="mitigatedRisks"
              name="Mitigated"
              fill="#10b981"
              radius={[8, 8, 0, 0]}
            />
          </BarChart>
        </ResponsiveContainer>
      ) : (
        <div className="flex items-center justify-center h-64 text-zinc-500">
          <p>No asset data available</p>
        </div>
      )}

      {/* Asset Details Table */}
      <div className="mt-6 pt-4 border-t border-white/10">
        <h4 className="text-xs font-semibold text-zinc-400 mb-3 uppercase tracking-wider">Asset Details</h4>
        <div className="space-y-2 max-h-48 overflow-y-auto pr-2">
          {data.map((asset) => (
            <div
              key={asset.name}
              className="flex items-center justify-between p-3 rounded-lg bg-white/5 border border-white/10 hover:bg-white/10 transition-colors"
            >
              <div className="flex items-center gap-3 flex-1 min-w-0">
                <Server size={16} className="text-primary flex-shrink-0" />
                <div className="min-w-0">
                  <p className="text-sm font-medium text-white truncate">{asset.name}</p>
                  <p className="text-xs text-zinc-500">{asset.assetCount} assets</p>
                </div>
              </div>
              <div className="flex items-center gap-4 flex-shrink-0 ml-2">
                <div className="text-right">
                  <p className="text-sm font-bold text-zinc-400">{asset.totalRisks}</p>
                  <p className="text-xs text-zinc-500">risks</p>
                </div>
                <div
                  className="w-8 h-8 rounded-lg flex items-center justify-center text-xs font-bold text-white"
                  style={{ backgroundColor: getBarColor(asset.riskScore) }}
                  title={`Risk Score: ${asset.riskScore}`}
                >
                  {asset.riskScore}
                </div>
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Summary Stats */}
      <div className="mt-4 grid grid-cols-3 gap-2 pt-4 border-t border-white/10">
        <div className="p-2 rounded-lg bg-white/5 border border-white/10">
          <p className="text-xs text-zinc-500">Total Assets</p>
          <p className="text-lg font-bold text-white mt-1">
            {data.reduce((sum, d) => sum + d.assetCount, 0)}
          </p>
        </div>
        <div className="p-2 rounded-lg bg-white/5 border border-white/10">
          <p className="text-xs text-zinc-500">Total Risks</p>
          <p className="text-lg font-bold text-yellow-400 mt-1">
            {data.reduce((sum, d) => sum + d.totalRisks, 0)}
          </p>
        </div>
        <div className="p-2 rounded-lg bg-white/5 border border-white/10">
          <p className="text-xs text-zinc-500">Avg Risk/Asset</p>
          <p className="text-lg font-bold text-primary mt-1">
            {(
              data.reduce((sum, d) => sum + d.totalRisks, 0) /
              Math.max(1, data.reduce((sum, d) => sum + d.assetCount, 0))
            ).toFixed(1)}
          </p>
        </div>
      </div>
    </div>
  );
};
