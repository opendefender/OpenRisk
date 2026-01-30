import React, { useState, useEffect } from 'react';
import { BarChart, Bar, LineChart, Line, PieChart, Pie, Cell, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer } from 'recharts';
import { Download, RefreshCw, TrendingUp, TrendingDown } from 'lucide-react';

interface RiskMetrics {
  total_risks: number;
  active_risks: number;
  mitigated_risks: number;
  average_score: number;
  high_risks: number;
  medium_risks: number;
  low_risks: number;
  risks_by_framework: Record<string, number>;
  risks_by_status: Record<string, number>;
  created_this_month: number;
  updated_this_month: number;
}

interface MitigationMetrics {
  total_mitigations: number;
  completed_mitigations: number;
  pending_mitigations: number;
  overdue_mitigations: number;
  completion_rate: number;
  avg_completion_days: number;
  risks_with_mitigation: number;
}

interface TrendPoint {
  date: string;
  count: number;
  avg_score: number;
  new_risks: number;
  mitigated: number;
}

interface FrameworkAnalytic {
  framework: string;
  associated_risks: number;
  average_risk_score: number;
  compliance_percentage: number;
}

interface DashboardSnapshot {
  timestamp: string;
  risk_metrics: RiskMetrics;
  mitigation_metrics: MitigationMetrics;
  framework_analytics: FrameworkAnalytic[];
  trends: TrendPoint[];
}

export default function Analytics() {
  const [dashboard, setDashboard] = useState<DashboardSnapshot | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [refreshing, setRefreshing] = useState(false);

  const fetchDashboard = async () => {
    try {
      setRefreshing(true);
      const response = await fetch('/api/v/analytics/dashboard');
      if (!response.ok) throw new Error("We couldn't load your dashboard. Please refresh the page.");
      const data = await response.json();
      setDashboard(data);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error');
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  };

  useEffect(() => {
    fetchDashboard();
    // Refresh every  minutes
    const interval = setInterval(fetchDashboard,     );
    return () => clearInterval(interval);
  }, []);

  const handleExport = async (format: 'json' | 'csv') => {
    try {
      const response = await fetch(/api/v/analytics/export?format=${format});
      if (!response.ok) throw new Error("We couldn't export the data. Please try again.");
      
      const blob = await response.blob();
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = analytics-${format}.${format === 'json' ? 'json' : 'csv'};
      document.body.appendChild(a);
      a.click();
      window.URL.revokeObjectURL(url);
      document.body.removeChild(a);
    } catch (err) {
      setError(err instanceof Error ? err.message : "We couldn't export the data. Please try again.");
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="text-center">
          <div className="inline-block animate-spin">
            <div className="h- w- border- border-blue- border-t-blue- rounded-full"></div>
          </div>
          <p className="mt- text-gray-">Loading analytics...</p>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="p-">
        <div className="bg-red-/ border border-red- rounded-lg p-">
          <p className="text-red-">Error: {error}</p>
        </div>
      </div>
    );
  }

  if (!dashboard) {
    return (
      <div className="p-">
        <div className="text-gray-">No data available</div>
      </div>
    );
  }

  const riskMetrics = dashboard.risk_metrics;
  const mitMetrics = dashboard.mitigation_metrics;

  // Prepare chart data
  const riskLevelData = [
    { name: 'High', value: riskMetrics.high_risks, color: 'ef' },
    { name: 'Medium', value: riskMetrics.medium_risks, color: 'f' },
    { name: 'Low', value: riskMetrics.low_risks, color: 'eab' },
  ];

  const statusData = Object.entries(riskMetrics.risks_by_status).map(([status, count]) => ({
    name: status.charAt().toUpperCase() + status.slice(),
    value: count,
  }));

  const frameworkData = Object.entries(riskMetrics.risks_by_framework).map(([framework, count]) => ({
    name: framework,
    risks: count,
  }));

  return (
    <div className="space-y- pb-">
      {/ Header /}
      <div className="flex justify-between items-center">
        <div>
          <h className="text-xl font-bold text-white">Analytics Dashboard</h>
          <p className="text-gray- mt-">
            Last updated: {new Date(dashboard.timestamp).toLocaleString()}
          </p>
        </div>
        <div className="flex gap-">
          <button
            onClick={fetchDashboard}
            disabled={refreshing}
            className="flex items-center gap- px- py- bg-blue- hover:bg-blue- text-white rounded-lg transition disabled:opacity-"
          >
            <RefreshCw size={} className={refreshing ? 'animate-spin' : ''} />
            Refresh
          </button>
          <div className="flex gap-">
            <button
              onClick={() => handleExport('json')}
              className="flex items-center gap- px- py- bg-green- hover:bg-green- text-white rounded-lg transition"
            >
              <Download size={} />
              JSON
            </button>
            <button
              onClick={() => handleExport('csv')}
              className="flex items-center gap- px- py- bg-green- hover:bg-green- text-white rounded-lg transition"
            >
              <Download size={} />
              CSV
            </button>
          </div>
        </div>
      </div>

      {/ Key Metrics /}
      <div className="grid grid-cols- md:grid-cols- lg:grid-cols- gap-">
        <MetricCard
          title="Total Risks"
          value={riskMetrics.total_risks}
          change={riskMetrics.created_this_month}
          changeLabel="this month"
          icon={TrendingUp}
        />
        <MetricCard
          title="Active Risks"
          value={riskMetrics.active_risks}
          percentage={
            riskMetrics.total_risks > 
              ? ((riskMetrics.active_risks / riskMetrics.total_risks)  ).toFixed()
              : ''
          }
          icon={TrendingUp}
        />
        <MetricCard
          title="Avg Risk Score"
          value={riskMetrics.average_score.toFixed()}
          maxValue=""
          icon={TrendingDown}
        />
        <MetricCard
          title="Mitigation Rate"
          value={mitMetrics.completion_rate.toFixed()}
          suffix="%"
          icon={TrendingUp}
        />
      </div>

      {/ Risk Distribution /}
      <div className="grid grid-cols- lg:grid-cols- gap-">
        {/ Risk Levels Pie Chart /}
        <div className="bg-zinc- rounded-lg p-">
          <h className="text-xl font-bold text-white mb-">Risk Distribution by Level</h>
          <ResponsiveContainer width="%" height={}>
            <PieChart>
              <Pie
                data={riskLevelData}
                cx="%"
                cy="%"
                labelLine={false}
                label={({ name, value }) => ${name}: ${value}}
                outerRadius={}
                fill="d"
                dataKey="value"
              >
                {riskLevelData.map((entry, index) => (
                  <Cell key={cell-${index}} fill={entry.color} />
                ))}
              </Pie>
              <Tooltip />
            </PieChart>
          </ResponsiveContainer>
        </div>

        {/ Risk Status Distribution /}
        <div className="bg-zinc- rounded-lg p-">
          <h className="text-xl font-bold text-white mb-">Risk Status Distribution</h>
          <ResponsiveContainer width="%" height={}>
            <BarChart data={statusData}>
              <CartesianGrid strokeDasharray=" " stroke="" />
              <XAxis dataKey="name" stroke="" />
              <YAxis stroke="" />
              <Tooltip contentStyle={{ backgroundColor: 'fff', border: 'px solid ' }} />
              <Bar dataKey="value" fill="bf" />
            </BarChart>
          </ResponsiveContainer>
        </div>
      </div>

      {/ Trends /}
      <div className="bg-zinc- rounded-lg p-">
        <h className="text-xl font-bold text-white mb-">Risk Trends ( days)</h>
        <ResponsiveContainer width="%" height={}>
          <LineChart data={dashboard.trends}>
            <CartesianGrid strokeDasharray=" " stroke="" />
            <XAxis
              dataKey="date"
              stroke=""
              tickFormatter={(date) => new Date(date).toLocaleDateString('en-US', { month: 'short', day: 'numeric' })}
            />
            <YAxis stroke="" />
            <Tooltip
              contentStyle={{ backgroundColor: 'fff', border: 'px solid ' }}
              labelFormatter={(date) => new Date(date).toLocaleDateString()}
            />
            <Legend />
            <Line type="monotone" dataKey="count" stroke="bf" name="Total Risks" />
            <Line type="monotone" dataKey="avg_score" stroke="f" name="Avg Score" />
            <Line type="monotone" dataKey="new_risks" stroke="b" name="New Risks" />
          </LineChart>
        </ResponsiveContainer>
      </div>

      {/ Framework Analysis /}
      <div className="bg-zinc- rounded-lg p-">
        <h className="text-xl font-bold text-white mb-">Risks by Framework</h>
        <ResponsiveContainer width="%" height={}>
          <BarChart data={frameworkData}>
            <CartesianGrid strokeDasharray=" " stroke="" />
            <XAxis dataKey="name" stroke="" />
            <YAxis stroke="" />
            <Tooltip contentStyle={{ backgroundColor: 'fff', border: 'px solid ' }} />
            <Bar dataKey="risks" fill="bcf" />
          </BarChart>
        </ResponsiveContainer>
      </div>

      {/ Mitigation Metrics /}
      <div className="grid grid-cols- md:grid-cols- lg:grid-cols- gap-">
        <MetricCard
          title="Total Mitigations"
          value={mitMetrics.total_mitigations}
          icon={TrendingUp}
        />
        <MetricCard
          title="Completed"
          value={mitMetrics.completed_mitigations}
          percentage={
            mitMetrics.total_mitigations > 
              ? ((mitMetrics.completed_mitigations / mitMetrics.total_mitigations)  ).toFixed()
              : ''
          }
          icon={TrendingUp}
        />
        <MetricCard
          title="Overdue"
          value={mitMetrics.overdue_mitigations}
          alert={mitMetrics.overdue_mitigations > }
          icon={TrendingDown}
        />
      </div>
    </div>
  );
}

interface MetricCardProps {
  title: string;
  value: string | number;
  change?: number;
  changeLabel?: string;
  percentage?: string;
  suffix?: string;
  maxValue?: string;
  alert?: boolean;
  icon?: React.ComponentType<{ size: number }>;
}

function MetricCard({
  title,
  value,
  change,
  changeLabel,
  percentage,
  suffix = '',
  maxValue,
  alert = false,
  icon: Icon,
}: MetricCardProps) {
  return (
    <div className={bg-zinc- rounded-lg p- border ${alert ? 'border-red-' : 'border-zinc-'}}>
      <div className="flex justify-between items-start">
        <div>
          <p className="text-gray- text-sm font-medium">{title}</p>
          <p className={text-xl font-bold mt- ${alert ? 'text-red-' : 'text-white'}}>
            {value}{suffix}
            {maxValue && <span className="text-sm text-gray-">/{maxValue}</span>}
          </p>
          {change !== undefined && (
            <p className="text-sm text-green- mt-">
              +{change} {changeLabel}
            </p>
          )}
          {percentage && (
            <p className="text-sm text-blue- mt-">
              {percentage}%
            </p>
          )}
        </div>
        {Icon && <Icon size={} className="text-gray-" />}
      </div>
    </div>
  );
}
