import React, { useState, useEffect } from 'react';
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ResponsiveContainer,
  PieChart,
  Pie,
  Cell,
  LineChart,
  Line,
} from 'recharts';
import { CheckCircle, AlertCircle, Clock, Shield } from 'lucide-react';

interface ComplianceFramework {
  name: string;
  score: number;
  status: 'compliant' | 'warning' | 'non-compliant';
  issues: string[];
  recommendations: string[];
}

interface AuditEvent {
  id: string;
  user: string;
  action: string;
  resource: string;
  timestamp: string;
  status: 'success' | 'failure';
}

interface ComplianceReport {
  frameworks: ComplianceFramework[];
  overallScore: number;
  auditEvents: AuditEvent[];
  trend: Array<{ date: string; score: number }>;
}

const ComplianceReportDashboard: React.FC = () => {
  const [report, setReport] = useState<ComplianceReport | null>(null);
  const [loading, setLoading] = useState(false);
  const [selectedFramework, setSelectedFramework] = useState<string | null>(null);
  const [timeRange, setTimeRange] = useState('30d');

  // Fetch compliance report
  useEffect(() => {
    const fetchReport = async () => {
      setLoading(true);
      try {
        const response = await fetch(`/api/compliance/report?range=${timeRange}`);
        if (response.ok) {
          const data = await response.json();
          setReport(data);
          if (data.frameworks.length > 0 && !selectedFramework) {
            setSelectedFramework(data.frameworks[0].name);
          }
        }
      } catch (error) {
        console.error('Failed to fetch compliance report:', error);
      } finally {
        setLoading(false);
      }
    };

    fetchReport();
  }, [timeRange]);

  if (loading) {
    return (
      <div className="min-h-screen bg-gray-50 p-8 flex items-center justify-center">
        <p className="text-gray-600">Loading compliance report...</p>
      </div>
    );
  }

  if (!report) {
    return (
      <div className="min-h-screen bg-gray-50 p-8">
        <p className="text-gray-600">Failed to load compliance report</p>
      </div>
    );
  }

  const currentFramework = report.frameworks.find((f) => f.name === selectedFramework);

  // Status colors
  const getStatusColor = (status: string) => {
    switch (status) {
      case 'compliant':
        return 'bg-green-100 text-green-800 border-green-300';
      case 'warning':
        return 'bg-yellow-100 text-yellow-800 border-yellow-300';
      case 'non-compliant':
        return 'bg-red-100 text-red-800 border-red-300';
      default:
        return 'bg-gray-100 text-gray-800 border-gray-300';
    }
  };

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'compliant':
        return <CheckCircle size={20} className="text-green-600" />;
      case 'warning':
        return <AlertCircle size={20} className="text-yellow-600" />;
      case 'non-compliant':
        return <AlertCircle size={20} className="text-red-600" />;
      default:
        return <Shield size={20} className="text-gray-600" />;
    }
  };

  // Prepare chart data
  const frameworkScores = report.frameworks.map((f) => ({
    name: f.name,
    score: f.score,
  }));

  const complianceStatusData = [
    {
      name: 'Compliant',
      value: report.frameworks.filter((f) => f.status === 'compliant').length,
      color: '#10b981',
    },
    {
      name: 'Warning',
      value: report.frameworks.filter((f) => f.status === 'warning').length,
      color: '#f59e0b',
    },
    {
      name: 'Non-Compliant',
      value: report.frameworks.filter((f) => f.status === 'non-compliant').length,
      color: '#ef4444',
    },
  ];

  return (
    <div className="min-h-screen bg-gray-50 p-8">
      <div className="max-w-7xl mx-auto">
        {/* Header */}
        <div className="mb-8">
          <h1 className="text-4xl font-bold text-gray-900">Compliance Report</h1>
          <p className="text-gray-600 mt-2">Multi-framework compliance dashboard</p>
        </div>

        {/* Overall Score Card */}
        <div className="bg-gradient-to-r from-blue-500 to-purple-600 rounded-lg shadow p-8 mb-8 text-white">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-blue-100 text-sm font-medium">Overall Compliance Score</p>
              <p className="text-6xl font-bold mt-2">{report.overallScore}</p>
              <p className="text-blue-100 text-sm mt-2">out of 100</p>
            </div>
            <div className="text-blue-100 opacity-50">
              <Shield size={80} />
            </div>
          </div>
        </div>

        {/* Controls */}
        <div className="bg-white rounded-lg shadow p-6 mb-8">
          <div className="flex gap-4 flex-wrap items-center">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                Time Range
              </label>
              <select
                value={timeRange}
                onChange={(e) => setTimeRange(e.target.value)}
                className="rounded border-gray-300 border px-3 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500"
              >
                <option value="7d">Last 7 Days</option>
                <option value="30d">Last 30 Days</option>
                <option value="90d">Last 90 Days</option>
                <option value="1y">Last Year</option>
              </select>
            </div>
          </div>
        </div>

        {/* Framework Cards */}
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-8">
          {report.frameworks.map((framework) => (
            <button
              key={framework.name}
              onClick={() => setSelectedFramework(framework.name)}
              className={`rounded-lg shadow p-6 text-left transition-all hover:shadow-lg ${
                selectedFramework === framework.name
                  ? 'ring-2 ring-blue-500 bg-blue-50'
                  : 'bg-white hover:bg-gray-50'
              }`}
            >
              <div className="flex items-start justify-between mb-4">
                <h3 className="text-lg font-bold text-gray-900">{framework.name}</h3>
                {getStatusIcon(framework.status)}
              </div>
              <div className="mb-4">
                <div className="flex items-baseline gap-1">
                  <p className="text-3xl font-bold text-gray-900">{framework.score}</p>
                  <p className="text-gray-600">/100</p>
                </div>
              </div>
              <div className="w-full bg-gray-200 rounded-full h-2">
                <div
                  className={`h-2 rounded-full transition-all ${
                    framework.score >= 80
                      ? 'bg-green-500'
                      : framework.score >= 60
                        ? 'bg-yellow-500'
                        : 'bg-red-500'
                  }`}
                  style={{ width: `${framework.score}%` }}
                />
              </div>
              <p
                className={`text-xs mt-2 px-2 py-1 rounded inline-block font-semibold border ${getStatusColor(
                  framework.status
                )}`}
              >
                {framework.status.replace('-', ' ').toUpperCase()}
              </p>
            </button>
          ))}
        </div>

        {/* Charts */}
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-8 mb-8">
          {/* Framework Scores */}
          <div className="bg-white rounded-lg shadow p-6">
            <h2 className="text-xl font-bold text-gray-900 mb-4">Framework Scores</h2>
            <ResponsiveContainer width="100%" height={300}>
              <BarChart data={frameworkScores}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="name" />
                <YAxis domain={[0, 100]} />
                <Tooltip />
                <Bar dataKey="score" fill="#3b82f6" radius={[8, 8, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          </div>

          {/* Compliance Status Distribution */}
          <div className="bg-white rounded-lg shadow p-6">
            <h2 className="text-xl font-bold text-gray-900 mb-4">Compliance Status</h2>
            <ResponsiveContainer width="100%" height={300}>
              <PieChart>
                <Pie
                  data={complianceStatusData}
                  cx="50%"
                  cy="50%"
                  labelLine={false}
                  label={({ name, value }) => `${name}: ${value}`}
                  outerRadius={80}
                  fill="#8884d8"
                  dataKey="value"
                >
                  {complianceStatusData.map((entry, index) => (
                    <Cell key={`cell-${index}`} fill={entry.color} />
                  ))}
                </Pie>
                <Tooltip />
              </PieChart>
            </ResponsiveContainer>
          </div>
        </div>

        {/* Compliance Trend */}
        {report.trend.length > 0 && (
          <div className="bg-white rounded-lg shadow p-6 mb-8">
            <h2 className="text-xl font-bold text-gray-900 mb-4">Compliance Trend</h2>
            <ResponsiveContainer width="100%" height={300}>
              <LineChart data={report.trend}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="date" />
                <YAxis domain={[0, 100]} />
                <Tooltip />
                <Legend />
                <Line
                  type="monotone"
                  dataKey="score"
                  stroke="#3b82f6"
                  dot={{ fill: '#3b82f6' }}
                  isAnimationActive={false}
                />
              </LineChart>
            </ResponsiveContainer>
          </div>
        )}

        {/* Selected Framework Details */}
        {currentFramework && (
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-8 mb-8">
            {/* Issues */}
            <div className="bg-white rounded-lg shadow p-6">
              <h2 className="text-xl font-bold text-gray-900 mb-4 flex items-center gap-2">
                <AlertCircle size={24} className="text-red-600" />
                Issues Found
              </h2>
              {currentFramework.issues.length > 0 ? (
                <ul className="space-y-2">
                  {currentFramework.issues.map((issue, idx) => (
                    <li
                      key={idx}
                      className="flex gap-3 p-3 bg-red-50 border border-red-200 rounded text-red-700 text-sm"
                    >
                      <span className="flex-shrink-0">⚠</span>
                      <span>{issue}</span>
                    </li>
                  ))}
                </ul>
              ) : (
                <p className="text-gray-500 text-center py-4">No issues found</p>
              )}
            </div>

            {/* Recommendations */}
            <div className="bg-white rounded-lg shadow p-6">
              <h2 className="text-xl font-bold text-gray-900 mb-4 flex items-center gap-2">
                <CheckCircle size={24} className="text-green-600" />
                Recommendations
              </h2>
              {currentFramework.recommendations.length > 0 ? (
                <ul className="space-y-2">
                  {currentFramework.recommendations.map((rec, idx) => (
                    <li
                      key={idx}
                      className="flex gap-3 p-3 bg-green-50 border border-green-200 rounded text-green-700 text-sm"
                    >
                      <span className="flex-shrink-0">✓</span>
                      <span>{rec}</span>
                    </li>
                  ))}
                </ul>
              ) : (
                <p className="text-gray-500 text-center py-4">No recommendations</p>
              )}
            </div>
          </div>
        )}

        {/* Recent Audit Events */}
        <div className="bg-white rounded-lg shadow p-6">
          <h2 className="text-xl font-bold text-gray-900 mb-4 flex items-center gap-2">
            <Clock size={24} />
            Recent Audit Events
          </h2>
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead>
                <tr className="border-b border-gray-200">
                  <th className="text-left px-4 py-3 font-semibold text-gray-700">User</th>
                  <th className="text-left px-4 py-3 font-semibold text-gray-700">Action</th>
                  <th className="text-left px-4 py-3 font-semibold text-gray-700">Resource</th>
                  <th className="text-left px-4 py-3 font-semibold text-gray-700">Timestamp</th>
                  <th className="text-left px-4 py-3 font-semibold text-gray-700">Status</th>
                </tr>
              </thead>
              <tbody>
                {report.auditEvents.slice(0, 10).map((event) => (
                  <tr key={event.id} className="border-b border-gray-100 hover:bg-gray-50">
                    <td className="px-4 py-3 text-gray-900">{event.user}</td>
                    <td className="px-4 py-3 text-gray-600">{event.action}</td>
                    <td className="px-4 py-3 text-gray-600">{event.resource}</td>
                    <td className="px-4 py-3 text-gray-600 text-sm">
                      {new Date(event.timestamp).toLocaleString()}
                    </td>
                    <td className="px-4 py-3">
                      <span
                        className={`px-3 py-1 rounded-full text-xs font-semibold ${
                          event.status === 'success'
                            ? 'bg-green-100 text-green-800'
                            : 'bg-red-100 text-red-800'
                        }`}
                      >
                        {event.status}
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>
  );
};

export default ComplianceReportDashboard;
