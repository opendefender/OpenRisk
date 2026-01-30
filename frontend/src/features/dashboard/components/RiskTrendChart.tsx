import { useEffect, useState } from 'react';
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts';
import { api } from '../../../lib/api';
import { Loader, TrendingDown, AlertTriangle } from 'lucide-react';

interface TrendPoint {
  date: string;
  score: number;
}

export const RiskTrendChart = () => {
  const [data, setData] = useState<TrendPoint[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const fetchData = async () => {
      try {
        const res = await api.get('/stats/trends');
        const trendData = res.data || [];
        setData(trendData);
        setError(null);
      } catch (err) {
        console.error('Failed to fetch risk trends:', err);
        setError('Failed to load trend data');
        setData([]);
      } finally {
        setIsLoading(false);
      }
    };

    fetchData();
  }, []);

  if (isLoading) {
    return (
      <div className="h-full flex items-center justify-center">
        <Loader className="animate-spin text-primary" size={} />
      </div>
    );
  }

  if (error || data.length === ) {
    return (
      <div className="h-full flex flex-col items-center justify-center text-zinc-">
        <AlertTriangle size={} className="mb- text-orange-/" />
        <p className="text-sm">{error || 'No trend data available'}</p>
      </div>
    );
  }

  return (
    <div className="h-full w-full flex flex-col">
      <div className="mb- flex items-center gap-">
        <TrendingDown size={} className="text-emerald-" />
        <div>
          <p className="text-sm font-semibold text-white">Risk Score Timeline</p>
          <p className="text-xs text-zinc-">Historical trend data from backend</p>
        </div>
      </div>
      
      <div className="flex- min-h-[px]">
        <ResponsiveContainer width="%" height="%">
          <LineChart data={data}>
            <defs>
              <linearGradient id="colorScore" x="" y="" x="" y="">
                <stop offset="%" stopColor="bf" stopOpacity={.}/>
                <stop offset="%" stopColor="bf" stopOpacity={}/>
              </linearGradient>
              <filter id="glow">
                <feGaussianBlur stdDeviation="" result="coloredBlur"/>
                <feMerge>
                  <feMergeNode in="coloredBlur"/>
                  <feMergeNode in="SourceGraphic"/>
                </feMerge>
              </filter>
            </defs>
            <CartesianGrid 
              strokeDasharray=" " 
              stroke="a" 
              vertical={false}
              opacity={.}
            />
            <XAxis 
              dataKey="date" 
              stroke="b" 
              tick={{ fontSize: , fill: 'a' }}
              tickFormatter={(val) => val.split('-')[]} // Affiche juste le jour
              axisLine={false}
              tickLine={false}
              style={{ fontSize: 'px' }}
            />
            <YAxis 
              stroke="b" 
              tick={{ fontSize: , fill: 'a' }}
              domain={[, ]}
              axisLine={false}
              tickLine={false}
            />
            <Tooltip 
              contentStyle={{ 
                backgroundColor: 'b', 
                border: 'px solid a',
                borderRadius: 'px',
                boxShadow: '  px rgba(, , , .)'
              }}
              itemStyle={{ color: 'bf', fontWeight:  }}
              labelStyle={{ color: 'aaaa', marginBottom: 'px' }}
              cursor={{ stroke: 'bf', strokeWidth: , opacity: . }}
            />
            <Line 
              type="monotone" 
              dataKey="score" 
              stroke="bf" 
              strokeWidth={}
              dot={{ 
                fill: 'bf', 
                r: ,
                filter: 'url(glow)'
              }}
              activeDot={{
                r: ,
                fill: 'afa'
              }}
              isAnimationActive={true}
              animationDuration={}
            />
          </LineChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
};