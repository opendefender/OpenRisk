import React, { useState } from 'react';
import { LineChart, Line, AreaChart, Area, BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer, PieChart, Pie, Cell } from 'recharts';
import AnalyticsDashboardToolbar from '@/components/dashboard/AnalyticsDashboardToolbar';
import MetricCard from '@/components/dashboard/MetricCard';
import ChartWidget from '@/components/dashboard/ChartWidget';
import DashboardFilter from '@/components/dashboard/DashboardFilter';
import DataTableWidget from '@/components/dashboard/DataTableWidget';

// Sample data for charts
const riskTrendData = [
  { date: 'Mon', risks: 45, mitigations: 32, assets: 120 },
  { date: 'Tue', risks: 52, mitigations: 35, assets: 125 },
  { date: 'Wed', risks: 48, mitigations: 38, assets: 128 },
  { date: 'Thu', risks: 61, mitigations: 42, assets: 135 },
  { date: 'Fri', risks: 55, mitigations: 45, assets: 140 },
  { date: 'Sat', risks: 49, mitigations: 43, assets: 142 },
  { date: 'Sun', risks: 42, mitigations: 40, assets: 138 },
];

const riskSeverityData = [
  { name: 'Critical', value: 12, color: '#EF4444' },
  { name: 'High', value: 28, color: '#F97316' },
  { name: 'Medium', value: 45, color: '#FBBF24' },
  { name: 'Low', value: 65, color: '#22C55E' },
];

const mitigationStatusData = [
  { name: 'Completed', value: 42, color: '#22C55E' },
  { name: 'In Progress', value: 35, color: '#3B82F6' },
  { name: 'Not Started', value: 18, color: '#9CA3AF' },
  { name: 'Overdue', value: 5, color: '#EF4444' },
];

const topRisksData = [
  { id: '1', name: 'Data Breach Risk', value: 95, status: 'critical', trend: 12 },
  { id: '2', name: 'System Downtime', value: 78, status: 'warning', trend: -5 },
  { id: '3', name: 'Compliance Violation', value: 62, status: 'warning', trend: 8 },
  { id: '4', name: 'Unauthorized Access', value: 45, status: 'active', trend: -15 },
  { id: '5', name: 'Data Loss', value: 38, status: 'active', trend: 3 },
];

const mitigation Progress Data = [
  { id: '1', name: 'Implement MFA', value: 85, status: 'active', trend: 5 },
  { id: '2', name: 'Update Security Patches', value: 92, status: 'active', trend: 8 },
  { id: '3', name: 'Backup System Upgrade', value: 65, status: 'active', trend: 12 },
  { id: '4', name: 'Access Control Review', value: 78, status: 'active', trend: 3 },
];

const RealTimeAnalyticsDashboard: React.FC = () => {
  const [selectedPeriod, setSelectedPeriod] = useState('7d');
  const [dateRange, setDateRange] = useState({ start: '2026-02-15', end: '2026-02-22' });
  const [selectedMetrics, setSelectedMetrics] = useState(['risks', 'mitigations']);
  const [isLoading, setIsLoading] = useState(false);

  const handleRefresh = () => {
    setIsLoading(true);
    setTimeout(() => setIsLoading(false), 1500);
  };

  return (
    <div className="min-h-screen bg-gray-50">
      {/* Toolbar */}
      <AnalyticsDashboardToolbar
        isLoading={isLoading}
        onRefreshClick={handleRefresh}
        onFilterClick={() => console.log('Filter clicked')}
        onExportClick={() => console.log('Export clicked')}
      />

      <div className="p-6 space-y-6">
        {/* Top Section - Filters */}
        <div className="grid grid-cols-1 lg:grid-cols-4 gap-6">
          <div className="lg:col-span-1">
            <DashboardFilter
              selectedPeriod={selectedPeriod}
              onPeriodChange={setSelectedPeriod}
              selectedMetrics={selectedMetrics}
              onMetricsChange={setSelectedMetrics}
              dateRange={dateRange}
              onDateRangeChange={(start, end) => setDateRange({ start, end })}
            />
          </div>

          {/* KPI Cards Section */}
          <div className="lg:col-span-3 grid grid-cols-1 md:grid-cols-3 gap-4">
            <MetricCard
              title="Total Risks"
              value={287}
              unit="active"
              changePercent={12}
              isPositive={false}
              trend="up"
              description="Year-to-date changes"
            />
            <MetricCard
              title="Mitigation Progress"
              value={68}
              unit="%"
              changePercent={8}
              isPositive={true}
              trend="up"
              description="Overall completion rate"
            />
            <MetricCard
              title="Risk Coverage"
              value={92}
              unit="%"
              changePercent={5}
              isPositive={true}
              trend="up"
              status="normal"
              description="Risks with mitigations"
            />
          </div>
        </div>

        {/* Charts Section - 2 columns */}
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          {/* Risk Trends */}
          <ChartWidget
            title="Risk Trends"
            subtitle="7-day moving average"
            action={{ label: 'View Details', onClick: () => console.log('View details') }}
          >
            <ResponsiveContainer width="100%" height="100%">
              <AreaChart data={riskTrendData} margin={{ top: 10, right: 30, left: 0, bottom: 0 }}>
                <defs>
                  <linearGradient id="colorRisks" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="#3B82F6" stopOpacity={0.8} />
                    <stop offset="95%" stopColor="#3B82F6" stopOpacity={0} />
                  </linearGradient>
                </defs>
                <CartesianGrid strokeDasharray="3 3" stroke="#E5E7EB" />
                <XAxis dataKey="date" stroke="#9CA3AF" />
                <YAxis stroke="#9CA3AF" />
                <Tooltip
                  contentStyle={{ backgroundColor: '#fff', border: '1px solid #e5e7eb', borderRadius: '0.5rem' }}
                />
                <Area
                  type="monotone"
                  dataKey="risks"
                  stroke="#3B82F6"
                  fillOpacity={1}
                  fill="url(#colorRisks)"
                  name="Risks"
                />
              </AreaChart>
            </ResponsiveContainer>
          </ChartWidget>

          {/* Risk Severity Distribution */}
          <ChartWidget
            title="Risk Severity Distribution"
            subtitle="Current breakdown"
            action={{ label: 'View Details', onClick: () => console.log('View details') }}
          >
            <ResponsiveContainer width="100%" height="100%">
              <PieChart>
                <Pie
                  data={riskSeverityData}
                  cx="50%"
                  cy="50%"
                  labelLine={false}
                  label={({ name, value }) => `${name}: ${value}`}
                  outerRadius={80}
                  fill="#8884d8"
                  dataKey="value"
                >
                  {riskSeverityData.map((entry, index) => (
                    <Cell key={`cell-${index}`} fill={entry.color} />
                  ))}
                </Pie>
                <Tooltip />
              </PieChart>
            </ResponsiveContainer>
          </ChartWidget>
        </div>

        {/* Data Tables Section */}
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          {/* Top Risks */}
          <DataTableWidget
            title="Top Risks"
            columns={['Risk Name', 'Score', 'Trend']}
            data={topRisksData}
            onViewMore={() => console.log('View more risks')}
          />

          {/* Mitigation Progress */}
          <DataTableWidget
            title="Mitigation Progress"
            columns={['Mitigation', 'Progress', 'Trend']}
            data={mitigationProgressData}
            onViewMore={() => console.log('View more mitigations')}
          />
        </div>

        {/* Status Overview - Full Width */}
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          {/* Mitigation Status */}
          <ChartWidget
            title="Mitigation Status"
            subtitle="Breakdown by completion stage"
            height="medium"
          >
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={[
                { status: 'Completed', count: 42 },
                { status: 'In Progress', count: 35 },
                { status: 'Not Started', count: 18 },
                { status: 'Overdue', count: 5 },
              ]}>
                <CartesianGrid strokeDasharray="3 3" stroke="#E5E7EB" />
                <XAxis dataKey="status" stroke="#9CA3AF" />
                <YAxis stroke="#9CA3AF" />
                <Tooltip
                  contentStyle={{ backgroundColor: '#fff', border: '1px solid #e5e7eb', borderRadius: '0.5rem' }}
                />
                <Bar dataKey="count" fill="#3B82F6" />
              </BarChart>
            </ResponsiveContainer>
          </ChartWidget>

          {/* Key Metrics Summary */}
          <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
            <h3 className="text-lg font-semibold text-gray-900 mb-4">Key Metrics Summary</h3>
            <div className="space-y-4">
              <div className="flex items-justify-between">
                <span className="text-sm text-gray-600">Average Risk Score</span>
                <span className="text-2xl font-bold text-gray-900">6.8/10</span>
              </div>
              <div className="border-t border-gray-200 pt-4">
                <span className="text-sm text-gray-600">Risks Trending Up</span>
                <span className="text-2xl font-bold text-red-600">+12%</span>
              </div>
              <div className="border-t border-gray-200 pt-4">
                <span className="text-sm text-gray-600">Overdue Mitigations</span>
                <span className="text-2xl font-bold text-yellow-600">5</span>
              </div>
              <div className="border-t border-gray-200 pt-4">
                <span className="text-sm text-gray-600">SLA Compliance</span>
                <span className="text-2xl font-bold text-green-600">94%</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};

export default RealTimeAnalyticsDashboard;
