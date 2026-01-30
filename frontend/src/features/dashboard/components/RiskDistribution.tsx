import { useEffect, useState } from 'react';
import { PieChart, Pie, Cell, ResponsiveContainer, Tooltip } from 'recharts';
import { api } from '../../../lib/api';
import { Loader, AlertTriangle } from 'lucide-react';

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
          critical: ,
          high: ,
          medium: ,
          low: 
        };
        
        records.forEach((record: RiskDistributionRecord) => {
          const level = record.level?.toUpperCase() || 'LOW';
          if (level in transformed) {
            transformed[level as keyof RiskDistributionData] = record.count || ;
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
      <div className="flex justify-center items-center h-full text-zinc-">
        <Loader className="animate-spin mr-" size={} />
        Loading Distribution...
      </div>
    );
  }

  if (error || !data) {
    return (
      <div className="flex flex-col items-center justify-center h-full text-zinc-">
        <AlertTriangle size={} className="mb- text-orange-/" />
        <p className="text-sm">{error || 'No distribution data available'}</p>
      </div>
    );
  }

  const chartData = [
    { name: 'Critical', value: data.critical, color: 'ef' },
    { name: 'High', value: data.high, color: 'f' },
    { name: 'Medium', value: data.medium, color: 'eab' },
    { name: 'Low', value: data.low, color: 'bf' },
  ];

  const total = data.critical + data.high + data.medium + data.low;

  return (
    <div className="h-full w-full flex flex-col">
      <div className="flex- flex items-center justify-center">
        <ResponsiveContainer width="%" height="%">
          <PieChart>
            <Pie
              data={chartData}
              cx="%"
              cy="%"
              innerRadius={}
              outerRadius={}
              paddingAngle={}
              dataKey="value"
            >
              {chartData.map((entry, index) => (
                <Cell key={cell-${index}} fill={entry.color} />
              ))}
            </Pie>
            <Tooltip
              contentStyle={{
                backgroundColor: 'b',
                border: 'px solid a',
                borderRadius: 'px',
              }}
              formatter={(value) => [value, 'Count']}
            />
          </PieChart>
        </ResponsiveContainer>
      </div>

      {/ Legend /}
      <div className="grid grid-cols- gap- mt-">
        {chartData.map((item) => (
          <div key={item.name} className="flex items-center gap- p- rounded-lg bg-white/ border border-white/">
            <div
              className="w- h- rounded-full"
              style={{ backgroundColor: item.color }}
            />
            <span className="text-xs text-zinc-">{item.name}</span>
            <span className="ml-auto text-xs font-bold text-white">{item.value}</span>
          </div>
        ))}
      </div>

      {/ Stats Summary /}
      <div className="mt- p- rounded-lg bg-gradient-to-r from-blue-/ to-purple-/ border border-blue-/">
        <p className="text-xs text-zinc-">Total Risks: <span className="text-white font-bold">{total}</span></p>
      </div>
    </div>
  );
};
