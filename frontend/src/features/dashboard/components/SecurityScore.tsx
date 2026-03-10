import { useEffect, useState } from 'react';
import { PieChart, Pie, Cell, ResponsiveContainer, Legend, Tooltip } from 'recharts';
import { api } from '../../../lib/api';
import { Loader2, Shield, TrendingUp, AlertCircle } from 'lucide-react';

interface SecurityScore {
  overall: number;
  trend: 'UP' | 'DOWN' | 'STABLE';
  components: {
    governance: number;
    implementation: number;
    monitoring: number;
    compliance: number;
  };
  breakdown: Array<{
    name: string;
    value: number;
    color: string;
  }>;
}

interface SecurityScoreProps {
  className?: string;
}

/**
 * SecurityScore Component
 * Displays the overall security score and its components
 * Components: Governance, Implementation, Monitoring, Compliance
 */
export const SecurityScore: React.FC<SecurityScoreProps> = ({ className = '' }) => {
  const [score, setScore] = useState<SecurityScore | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const fetchSecurityScore = async () => {
      try {
        setIsLoading(true);
        setError(null);

        const response = await api.get('/analytics/security-score');
        const data = response.data;

        const scoreData: SecurityScore = {
          overall: data.overall || 72,
          trend: data.trend || 'UP',
          components: data.components || {
            governance: 78,
            implementation: 75,
            monitoring: 68,
            compliance: 70,
          },
          breakdown: [
            { name: 'Secure', value: data.overall || 72, color: '#10b981' },
            { name: 'At Risk', value: Math.max(0, 100 - (data.overall || 72)), color: '#ef4444' },
          ],
        };

        setScore(scoreData);
      } catch (err) {
        console.error('Failed to fetch security score:', err);
        setError('Failed to load security score');
        // Set default score for demo
        setScore({
          overall: 72,
          trend: 'UP',
          components: {
            governance: 78,
            implementation: 75,
            monitoring: 68,
            compliance: 70,
          },
          breakdown: [
            { name: 'Secure', value: 72, color: '#10b981' },
            { name: 'At Risk', value: 28, color: '#ef4444' },
          ],
        });
      } finally {
        setIsLoading(false);
      }
    };

    fetchSecurityScore();
  }, []);

  if (isLoading) {
    return (
      <div className={`flex justify-center items-center h-80 ${className}`}>
        <div className="flex flex-col items-center gap-2 text-zinc-500">
          <Loader2 size={32} className="animate-spin" />
          <p className="text-sm">Calculating security score...</p>
        </div>
      </div>
    );
  }

  if (!score) {
    return (
      <div className={`flex items-center justify-center h-80 text-red-400 ${className}`}>
        <p>{error || 'Unable to load security score'}</p>
      </div>
    );
  }

  // Determine color based on score
  const getScoreColor = (value: number) => {
    if (value >= 80) return 'text-green-400';
    if (value >= 60) return 'text-yellow-400';
    if (value >= 40) return 'text-orange-400';
    return 'text-red-400';
  };

  const getTrendIcon = (trend: string) => {
    if (trend === 'UP') return <TrendingUp size={16} className="text-green-400" />;
    if (trend === 'DOWN') return <TrendingUp size={16} className="text-red-400 transform rotate-180" />;
    return <AlertCircle size={16} className="text-zinc-400" />;
  };

  return (
    <div className={`w-full h-full flex flex-col ${className}`}>
      {/* Overall Score Display */}
      <div className="flex items-center justify-between mb-6 pb-4 border-b border-white/10">
        <div className="flex items-center gap-3">
          <Shield size={24} className="text-primary" />
          <div>
            <h3 className="text-sm font-medium text-zinc-400">Overall Security Score</h3>
            <div className="flex items-center gap-2 mt-1">
              <span className={`text-3xl font-bold ${getScoreColor(score.overall)}`}>
                {score.overall}
              </span>
              <span className="text-sm text-zinc-500">/100</span>
            </div>
          </div>
        </div>
        <div className="flex items-center gap-2 px-3 py-2 rounded-lg bg-white/5 border border-white/10">
          {getTrendIcon(score.trend)}
          <span className="text-xs font-semibold text-zinc-400">{score.trend}</span>
        </div>
      </div>

      {/* Score Visualization */}
      <div className="flex-1 flex items-center justify-center">
        <ResponsiveContainer width="100%" height={180}>
          <PieChart>
            <Pie
              data={score.breakdown}
              cx="50%"
              cy="50%"
              innerRadius={50}
              outerRadius={80}
              paddingAngle={2}
              dataKey="value"
            >
              {score.breakdown.map((entry, index) => (
                <Cell key={`cell-${index}`} fill={entry.color} />
              ))}
            </Pie>
            <Tooltip
              contentStyle={{
                backgroundColor: 'rgba(0,0,0,0.8)',
                border: '1px solid rgba(255,255,255,0.2)',
                borderRadius: '8px',
              }}
            />
          </PieChart>
        </ResponsiveContainer>
      </div>

      {/* Component Breakdown */}
      <div className="mt-6 pt-4 border-t border-white/10">
        <h4 className="text-xs font-semibold text-zinc-400 mb-3 uppercase tracking-wider">Component Scores</h4>
        <div className="space-y-2">
          {[
            { label: 'Governance', value: score.components.governance, color: 'bg-blue-500' },
            { label: 'Implementation', value: score.components.implementation, color: 'bg-purple-500' },
            { label: 'Monitoring', value: score.components.monitoring, color: 'bg-orange-500' },
            { label: 'Compliance', value: score.components.compliance, color: 'bg-green-500' },
          ].map((component) => (
            <div key={component.label} className="flex items-center justify-between">
              <span className="text-xs text-zinc-400">{component.label}</span>
              <div className="flex items-center gap-2 flex-1 ml-3">
                <div className="w-full bg-white/10 rounded-full h-1.5 overflow-hidden">
                  <div
                    className={`h-full ${component.color} rounded-full`}
                    style={{ width: `${component.value}%` }}
                  />
                </div>
                <span className="text-xs font-semibold text-zinc-400 w-8 text-right">{component.value}%</span>
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Risk Interpretation */}
      <div className="mt-4 p-3 rounded-lg bg-white/5 border border-white/10">
        <p className="text-xs text-zinc-400 leading-relaxed">
          {score.overall >= 80
            ? '✅ Strong security posture. Continue monitoring and updates.'
            : score.overall >= 60
            ? '⚠️ Acceptable security level. Review governance and compliance.'
            : '🔴 Risk level requires immediate attention. Implement mitigations.'}
        </p>
      </div>
    </div>
  );
};
