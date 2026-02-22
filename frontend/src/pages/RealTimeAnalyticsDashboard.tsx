import React, { useState } from 'react';
import {
  AreaChart,
  Area,
  PieChart,
  Pie,
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ResponsiveContainer,
  Cell,
} from 'recharts';
import { AlertCircle, RefreshCw, Wifi, WifiOff } from 'lucide-react';
import AnalyticsDashboardToolbar from '@/components/dashboard/AnalyticsDashboardToolbar';
import MetricCard from '@/components/dashboard/MetricCard';
import ChartWidget from '@/components/dashboard/ChartWidget';
import DashboardFilter from '@/components/dashboard/DashboardFilter';
import DataTableWidget from '@/components/dashboard/DataTableWidget';
import {
  useCompleteDashboard,
  useDashboardPoller,
} from '@/hooks/useDashboard';
import { useDashboardWithWebSocket } from '@/hooks/useWebSocket';

const RealTimeAnalyticsDashboard: React.FC = () => {
  const [filterPeriod, setFilterPeriod] = useState<'today' | '7d' | '30d' | '90d' | 'ytd' | 'custom'>('7d');
  const [selectedMetrics, setSelectedMetrics] = useState<string[]>(['risk_score', 'mitigation_rate']);
  const [autoRefresh, setAutoRefresh] = useState(true);

  // Fetch data with WebSocket (real-time) with fallback to polling
  const dashboard = useDashboardWithWebSocket(true);
  const { data: analyticsData, loading, error, connected, source, refresh } = dashboard;

  // Handle manual refresh
  const handleRefresh = () => {
    refresh();
  };

  // Handle filter changes
  const handleFilterChange = (period: 'today' | '7d' | '30d' | '90d' | 'ytd' | 'custom') => {
    setFilterPeriod(period);
    // In a real app, you would fetch new data based on the period
  };

  const handleMetricFilterChange = (metrics: string[]) => {
    setSelectedMetrics(metrics);
  };

  const handleExport = () => {
    if (!analyticsData) return;

    const dataStr = JSON.stringify(analyticsData, null, 2);
    const dataBlob = new Blob([dataStr], { type: 'application/json' });
    const url = URL.createObjectURL(dataBlob);
    const link = document.createElement('a');
    link.href = url;
    link.download = `dashboard-export-${new Date().toISOString()}.json`;
    link.click();
  };

  const handleSettings = () => {
    // Settings dialog would open here
    console.log('Settings clicked');
  };

  // Color scheme
  const COLORS = {
    critical: '#ef4444',
    high: '#f97316',
    medium: '#eab308',
    low: '#22c55e',
  };

  const MITIGATION_COLORS = {
    completed: '#22c55e',
    in_progress: '#3b82f6',
    not_started: '#94a3b8',
    overdue: '#ef4444',
  };

  // Error state
  if (error) {
    return (
      <div className="flex items-center justify-center min-h-screen bg-gray-50 dark:bg-slate-900">
        <div className="bg-white dark:bg-slate-800 p-8 rounded-lg shadow-md max-w-md w-full">
          <div className="flex items-center gap-3 mb-4">
            <AlertCircle className="w-6 h-6 text-red-500" />
            <h2 className="text-lg font-semibold text-red-600">Error Loading Dashboard</h2>
          </div>
          <p className="text-gray-600 dark:text-gray-300 mb-4">{error}</p>
          <button
            onClick={handleRefresh}
            className="w-full bg-blue-600 hover:bg-blue-700 text-white py-2 rounded-lg font-medium flex items-center justify-center gap-2"
          >
            <RefreshCw className="w-4 h-4" />
            Try Again
          </button>
        </div>
      </div>
    );
  }

  // Loading state
  if (loading) {
    return (
      <div className="min-h-screen bg-gray-50 dark:bg-slate-900 p-6">
        <div className="animate-pulse space-y-6">
          <div className="h-12 bg-gray-200 dark:bg-slate-700 rounded w-full" />
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
            {[1, 2, 3, 4].map((i) => (
              <div key={i} className="h-32 bg-gray-200 dark:bg-slate-700 rounded" />
            ))}
          </div>
          <div className="h-64 bg-gray-200 dark:bg-slate-700 rounded" />
        </div>
      </div>
    );
  }

  if (!analyticsData) {
    return (
      <div className="flex items-center justify-center min-h-screen bg-gray-50 dark:bg-slate-900">
        <div className="text-center">
          <p className="text-gray-600 dark:text-gray-300">No data available</p>
        </div>
      </div>
    );
  }

  const metrics = analyticsData.metrics;
  const riskTrends = analyticsData.risk_trends || [];
  const severityDist = analyticsData.severity_distribution || {};
  const mitigationStat = analyticsData.mitigation_status || {};
  const topRisks = analyticsData.top_risks || [];
  const mitigationProgress = analyticsData.mitigation_progress || [];

  // Prepare data for charts
  const severityChartData = [
    { name: 'Critical', value: severityDist.critical || 0, fill: COLORS.critical },
    { name: 'High', value: severityDist.high || 0, fill: COLORS.high },
    { name: 'Medium', value: severityDist.medium || 0, fill: COLORS.medium },
    { name: 'Low', value: severityDist.low || 0, fill: COLORS.low },
  ];

  const mitigationChartData = [
    { name: 'Completed', value: mitigationStat.completed || 0, fill: MITIGATION_COLORS.completed },
    { name: 'In Progress', value: mitigationStat.in_progress || 0, fill: MITIGATION_COLORS.in_progress },
    { name: 'Not Started', value: mitigationStat.not_started || 0, fill: MITIGATION_COLORS.not_started },
    { name: 'Overdue', value: mitigationStat.overdue || 0, fill: MITIGATION_COLORS.overdue },
  ];

  // Format table data
  const riskTableData = topRisks.map((risk) => ({
    ...risk,
    statusColor:
      risk.severity === 'critical'
        ? 'red'
        : risk.severity === 'high'
          ? 'orange'
          : risk.severity === 'medium'
            ? 'yellow'
            : 'green',
  }));

  const mitigationTableData = mitigationProgress.map((m) => ({
    ...m,
    statusColor: m.status === 'completed' ? 'green' : m.status === 'in_progress' ? 'blue' : m.days_remaining < 0 ? 'red' : 'gray',
  }));

  return (
    <div className="min-h-screen bg-gray-50 dark:bg-slate-900">
      {/* Header with toolbar */}
      <div className="flex items-center justify-between">
        <AnalyticsDashboardToolbar
          onRefresh={handleRefresh}
          onExport={handleExport}
          onSettingsClick={handleSettings}
        />
        
        {/* WebSocket Status Indicator */}
        <div className="flex items-center gap-2 px-6 py-4">
          <div className="flex items-center gap-2">
            {connected ? (
              <>
                <Wifi className="w-4 h-4 text-green-500" />
                <span className="text-xs font-medium text-green-600 dark:text-green-400">
                  Live ({source})
                </span>
              </>
            ) : (
              <>
                <WifiOff className="w-4 h-4 text-amber-500" />
                <span className="text-xs font-medium text-amber-600 dark:text-amber-400">
                  {loading ? 'Connecting...' : 'Polling'}
                </span>
              </>
            )}
          </div>
        </div>
      </div>

      <div className="p-6 max-w-7xl mx-auto">
        {/* Filter Section */}
        <DashboardFilter
          selectedPeriod={filterPeriod}
          selectedMetrics={selectedMetrics}
          onPeriodChange={handleFilterChange}
          onMetricsChange={handleMetricFilterChange}
          onApply={() => handleRefresh()}
        />

        {/* KPI Cards - Row 1 */}
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-6">
          <MetricCard
            title="Avg Risk Score"
            value={metrics?.average_risk_score.toFixed(1) || '0'}
            unit="points"
            changePercent={metrics?.trending_up_percent || 0}
            trend={metrics && metrics.trending_up_percent > 0 ? 'up' : 'down'}
            status={
              metrics && metrics.average_risk_score > 15
                ? 'critical'
                : metrics && metrics.average_risk_score > 10
                  ? 'warning'
                  : 'normal'
            }
          />
          <MetricCard
            title="Trending Up"
            value={metrics?.trending_up_percent.toFixed(1) || '0'}
            unit="%"
            changePercent={0}
            trend="neutral"
            status="normal"
          />
          <MetricCard
            title="Overdue Items"
            value={metrics?.overdue_count.toString() || '0'}
            unit="tasks"
            changePercent={0}
            trend="neutral"
            status={metrics && metrics.overdue_count > 0 ? 'warning' : 'normal'}
          />
          <MetricCard
            title="SLA Compliance"
            value={metrics?.sla_compliance_rate.toFixed(1) || '0'}
            unit="%"
            changePercent={0}
            trend="neutral"
            status={metrics && metrics.sla_compliance_rate > 80 ? 'normal' : 'warning'}
          />
        </div>

        {/* Charts Row */}
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-6">
          {/* Risk Trends Chart */}
          <ChartWidget title="Risk Trends (7 Days)" height="h-80">
            {riskTrends.length > 0 ? (
              <ResponsiveContainer width="100%" height={300}>
                <AreaChart data={riskTrends} margin={{ top: 10, right: 30, left: 0, bottom: 0 }}>
                  <defs>
                    <linearGradient id="colorScore" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="5%" stopColor="#3b82f6" stopOpacity={0.8} />
                      <stop offset="95%" stopColor="#3b82f6" stopOpacity={0} />
                    </linearGradient>
                  </defs>
                  <CartesianGrid strokeDasharray="3 3" />
                  <XAxis dataKey="date" stroke="#64748b" />
                  <YAxis stroke="#64748b" />
                  <Tooltip
                    contentStyle={{ backgroundColor: '#1e293b', border: 'none', borderRadius: '8px' }}
                    labelStyle={{ color: '#e2e8f0' }}
                  />
                  <Area
                    type="monotone"
                    dataKey="score"
                    stroke="#3b82f6"
                    fillOpacity={1}
                    fill="url(#colorScore)"
                  />
                </AreaChart>
              </ResponsiveContainer>
            ) : (
              <div className="flex items-center justify-center h-80 text-gray-400">No data available</div>
            )}
          </ChartWidget>

          {/* Severity Distribution Chart */}
          <ChartWidget title="Risk Severity Distribution" height="h-80">
            {severityChartData.some((d) => d.value > 0) ? (
              <ResponsiveContainer width="100%" height={300}>
                <PieChart>
                  <Pie
                    data={severityChartData}
                    cx="50%"
                    cy="50%"
                    labelLine={false}
                    label={({ name, value }) => `${name}: ${value}`}
                    outerRadius={100}
                    fill="#8884d8"
                    dataKey="value"
                  >
                    {severityChartData.map((entry, index) => (
                      <Cell key={`cell-${index}`} fill={entry.fill} />
                    ))}
                  </Pie>
                  <Tooltip />
                </PieChart>
              </ResponsiveContainer>
            ) : (
              <div className="flex items-center justify-center h-80 text-gray-400">No risks recorded</div>
            )}
          </ChartWidget>
        </div>

        {/* Secondary Charts */}
        <div className="grid grid-cols-1 gap-6 mb-6">
          {/* Mitigation Status Chart */}
          <ChartWidget title="Mitigation Status Overview" height="h-72">
            {mitigationChartData.some((d) => d.value > 0) ? (
              <ResponsiveContainer width="100%" height={280}>
                <BarChart data={mitigationChartData} margin={{ top: 10, right: 30, left: 0, bottom: 0 }}>
                  <CartesianGrid strokeDasharray="3 3" />
                  <XAxis dataKey="name" stroke="#64748b" />
                  <YAxis stroke="#64748b" />
                  <Tooltip
                    contentStyle={{ backgroundColor: '#1e293b', border: 'none', borderRadius: '8px' }}
                    labelStyle={{ color: '#e2e8f0' }}
                  />
                  <Bar dataKey="value" fill="#3b82f6">
                    {mitigationChartData.map((entry, index) => (
                      <Cell key={`cell-${index}`} fill={entry.fill} />
                    ))}
                  </Bar>
                </BarChart>
              </ResponsiveContainer>
            ) : (
              <div className="flex items-center justify-center h-72 text-gray-400">No mitigations recorded</div>
            )}
          </ChartWidget>
        </div>

        {/* Data Tables */}
        <div className="grid grid-cols-1 gap-6">
          {/* Top Risks Table */}
          <DataTableWidget title="Top Risks" rowCount={riskTableData.length}>
            <table className="w-full">
              <thead>
                <tr className="border-b border-gray-200 dark:border-slate-700">
                  <th className="px-6 py-3 text-left text-sm font-semibold text-gray-900 dark:text-white">Risk Name</th>
                  <th className="px-6 py-3 text-left text-sm font-semibold text-gray-900 dark:text-white">Score</th>
                  <th className="px-6 py-3 text-left text-sm font-semibold text-gray-900 dark:text-white">Severity</th>
                  <th className="px-6 py-3 text-left text-sm font-semibold text-gray-900 dark:text-white">Status</th>
                  <th className="px-6 py-3 text-left text-sm font-semibold text-gray-900 dark:text-white">Trend</th>
                  <th className="px-6 py-3 text-left text-sm font-semibold text-gray-900 dark:text-white">Mitigations</th>
                </tr>
              </thead>
              <tbody>
                {riskTableData.map((risk) => (
                  <tr key={risk.id} className="border-b border-gray-100 dark:border-slate-800 hover:bg-gray-50 dark:hover:bg-slate-800">
                    <td className="px-6 py-4 text-sm text-gray-900 dark:text-gray-100">{risk.name}</td>
                    <td className="px-6 py-4 text-sm font-medium text-gray-900 dark:text-white">{risk.score.toFixed(1)}</td>
                    <td className="px-6 py-4 text-sm">
                      <span
                        className={`inline-block px-3 py-1 rounded-full text-white text-xs font-medium ${
                          risk.severity === 'critical'
                            ? 'bg-red-500'
                            : risk.severity === 'high'
                              ? 'bg-orange-500'
                              : risk.severity === 'medium'
                                ? 'bg-yellow-500'
                                : 'bg-green-500'
                        }`}
                      >
                        {risk.severity}
                      </span>
                    </td>
                    <td className="px-6 py-4 text-sm text-gray-600 dark:text-gray-400">{risk.status}</td>
                    <td className="px-6 py-4 text-sm">
                      <span className={risk.trend_percent > 0 ? 'text-red-600' : 'text-green-600'}>
                        {risk.trend_percent > 0 ? '↑' : '↓'} {Math.abs(risk.trend_percent).toFixed(1)}%
                      </span>
                    </td>
                    <td className="px-6 py-4 text-sm text-gray-600 dark:text-gray-400">{risk.mitigation_count}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </DataTableWidget>

          {/* Mitigation Progress Table */}
          <DataTableWidget title="Mitigation Progress" rowCount={mitigationTableData.length}>
            <table className="w-full">
              <thead>
                <tr className="border-b border-gray-200 dark:border-slate-700">
                  <th className="px-6 py-3 text-left text-sm font-semibold text-gray-900 dark:text-white">Mitigation</th>
                  <th className="px-6 py-3 text-left text-sm font-semibold text-gray-900 dark:text-white">Risk</th>
                  <th className="px-6 py-3 text-left text-sm font-semibold text-gray-900 dark:text-white">Status</th>
                  <th className="px-6 py-3 text-left text-sm font-semibold text-gray-900 dark:text-white">Progress</th>
                  <th className="px-6 py-3 text-left text-sm font-semibold text-gray-900 dark:text-white">Due Date</th>
                  <th className="px-6 py-3 text-left text-sm font-semibold text-gray-900 dark:text-white">Days Left</th>
                </tr>
              </thead>
              <tbody>
                {mitigationTableData.map((mitigation) => (
                  <tr key={mitigation.id} className="border-b border-gray-100 dark:border-slate-800 hover:bg-gray-50 dark:hover:bg-slate-800">
                    <td className="px-6 py-4 text-sm text-gray-900 dark:text-gray-100">{mitigation.name}</td>
                    <td className="px-6 py-4 text-sm text-gray-600 dark:text-gray-400">{mitigation.risk_name}</td>
                    <td className="px-6 py-4 text-sm">
                      <span
                        className={`inline-block px-3 py-1 rounded-full text-white text-xs font-medium ${
                          mitigation.status === 'completed'
                            ? 'bg-green-500'
                            : mitigation.status === 'in_progress'
                              ? 'bg-blue-500'
                              : mitigation.days_remaining < 0
                                ? 'bg-red-500'
                                : 'bg-gray-500'
                        }`}
                      >
                        {mitigation.status}
                      </span>
                    </td>
                    <td className="px-6 py-4 text-sm">
                      <div className="w-full bg-gray-200 dark:bg-slate-700 rounded-full h-2">
                        <div
                          className="bg-blue-600 h-2 rounded-full"
                          style={{ width: `${mitigation.progress}%` }}
                        />
                      </div>
                      <span className="text-xs text-gray-600 dark:text-gray-400">{mitigation.progress}%</span>
                    </td>
                    <td className="px-6 py-4 text-sm text-gray-600 dark:text-gray-400">
                      {new Date(mitigation.due_date).toLocaleDateString()}
                    </td>
                    <td className="px-6 py-4 text-sm">
                      <span className={mitigation.days_remaining < 0 ? 'text-red-600 font-medium' : 'text-gray-600'}>
                        {mitigation.days_remaining < 0
                          ? `Overdue ${Math.abs(mitigation.days_remaining)}d`
                          : `${mitigation.days_remaining}d`}
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </DataTableWidget>
        </div>
      </div>
    </div>
  );
};

export default RealTimeAnalyticsDashboard;
