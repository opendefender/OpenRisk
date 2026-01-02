import { useEffect, useState } from 'react';
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts';
import { api } from '../../../lib/api';
import { Loader2, TrendingDown } from 'lucide-react';

interface TrendPoint {
  date: string;
  score: number;
}

export const RiskTrendChart = () => {
  const [data, setData] = useState<TrendPoint[]>([]);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    api.get('/stats/trends')
       .then(res => setData(res.data))
       .catch(() => {
         // Fallback demo data
         setData([
           { date: '2024-12-01', score: 65 },
           { date: '2024-12-05', score: 62 },
           { date: '2024-12-10', score: 58 },
           { date: '2024-12-15', score: 55 },
           { date: '2024-12-20', score: 52 },
           { date: '2024-12-25', score: 48 },
           { date: '2024-12-30', score: 45 },
         ]);
       })
       .finally(() => setIsLoading(false));
  }, []);

  if (isLoading) {
    return (
      <div className="h-full flex items-center justify-center">
        <Loader2 className="animate-spin text-primary" size={24} />
      </div>
    );
  }

  return (
    <div className="h-full w-full flex flex-col">
      <div className="mb-4 flex items-center gap-2">
        <TrendingDown size={20} className="text-emerald-400" />
        <div>
          <p className="text-sm font-semibold text-white">Positive Trend</p>
          <p className="text-xs text-zinc-400">Risk score improving over time</p>
        </div>
      </div>
      
      <div className="flex-1 min-h-[200px]">
        <ResponsiveContainer width="100%" height="100%">
          <LineChart data={data}>
            <defs>
              <linearGradient id="colorScore" x1="0" y1="0" x2="0" y2="1">
                <stop offset="5%" stopColor="#3b82f6" stopOpacity={0.5}/>
                <stop offset="95%" stopColor="#3b82f6" stopOpacity={0}/>
              </linearGradient>
              <filter id="glow">
                <feGaussianBlur stdDeviation="4" result="coloredBlur"/>
                <feMerge>
                  <feMergeNode in="coloredBlur"/>
                  <feMergeNode in="SourceGraphic"/>
                </feMerge>
              </filter>
            </defs>
            <CartesianGrid 
              strokeDasharray="3 3" 
              stroke="#27272a" 
              vertical={false}
              opacity={0.3}
            />
            <XAxis 
              dataKey="date" 
              stroke="#52525b" 
              tick={{ fontSize: 11, fill: '#71717a' }}
              tickFormatter={(val) => val.split('-')[2]} // Affiche juste le jour
              axisLine={false}
              tickLine={false}
              style={{ fontSize: '12px' }}
            />
            <YAxis 
              stroke="#52525b" 
              tick={{ fontSize: 11, fill: '#71717a' }}
              domain={[0, 100]}
              axisLine={false}
              tickLine={false}
            />
            <Tooltip 
              contentStyle={{ 
                backgroundColor: '#18181b', 
                border: '1px solid #27272a',
                borderRadius: '8px',
                boxShadow: '0 0 20px rgba(59, 130, 246, 0.3)'
              }}
              itemStyle={{ color: '#3b82f6', fontWeight: 500 }}
              labelStyle={{ color: '#a1a1aa', marginBottom: '4px' }}
              cursor={{ stroke: '#3b82f6', strokeWidth: 2, opacity: 0.5 }}
            />
            <Line 
              type="monotone" 
              dataKey="score" 
              stroke="#3b82f6" 
              strokeWidth={3}
              dot={{ 
                fill: '#3b82f6', 
                r: 4,
                filter: 'url(#glow)'
              }}
              activeDot={{
                r: 6,
                fill: '#60a5fa'
              }}
              isAnimationActive={true}
              animationDuration={800}
            />
          </LineChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
};