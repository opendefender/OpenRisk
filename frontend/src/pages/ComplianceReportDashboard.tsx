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
  const [timeRange, setTimeRange] = useState('d');

  // Fetch compliance report
  useEffect(() => {
    const fetchReport = async () => {
      setLoading(true);
      try {
        const response = await fetch(/api/compliance/report?range=${timeRange});
        if (response.ok) {
          const data = await response.json();
          setReport(data);
          if (data.frameworks.length >  && !selectedFramework) {
            setSelectedFramework(data.frameworks[].name);
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
      <div className="min-h-screen bg-gray- p- flex items-center justify-center">
        <p className="text-gray-">Loading compliance report...</p>
      </div>
    );
  }

  if (!report) {
    return (
      <div className="min-h-screen bg-gray- p-">
        <p className="text-gray-">Failed to load compliance report</p>
      </div>
    );
  }

  const currentFramework = report.frameworks.find((f) => f.name === selectedFramework);

  // Status colors
  const getStatusColor = (status: string) => {
    switch (status) {
      case 'compliant':
        return 'bg-green- text-green- border-green-';
      case 'warning':
        return 'bg-yellow- text-yellow- border-yellow-';
      case 'non-compliant':
        return 'bg-red- text-red- border-red-';
      default:
        return 'bg-gray- text-gray- border-gray-';
    }
  };

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'compliant':
        return <CheckCircle size={} className="text-green-" />;
      case 'warning':
        return <AlertCircle size={} className="text-yellow-" />;
      case 'non-compliant':
        return <AlertCircle size={} className="text-red-" />;
      default:
        return <Shield size={} className="text-gray-" />;
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
      color: 'b',
    },
    {
      name: 'Warning',
      value: report.frameworks.filter((f) => f.status === 'warning').length,
      color: 'feb',
    },
    {
      name: 'Non-Compliant',
      value: report.frameworks.filter((f) => f.status === 'non-compliant').length,
      color: 'ef',
    },
  ];

  return (
    <div className="min-h-screen bg-gray- p-">
      <div className="max-w-xl mx-auto">
        {/ Header /}
        <div className="mb-">
          <h className="text-xl font-bold text-gray-">Compliance Report</h>
          <p className="text-gray- mt-">Multi-framework compliance dashboard</p>
        </div>

        {/ Overall Score Card /}
        <div className="bg-gradient-to-r from-blue- to-purple- rounded-lg shadow p- mb- text-white">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-blue- text-sm font-medium">Overall Compliance Score</p>
              <p className="text-xl font-bold mt-">{report.overallScore}</p>
              <p className="text-blue- text-sm mt-">out of </p>
            </div>
            <div className="text-blue- opacity-">
              <Shield size={} />
            </div>
          </div>
        </div>

        {/ Controls /}
        <div className="bg-white rounded-lg shadow p- mb-">
          <div className="flex gap- flex-wrap items-center">
            <div>
              <label className="block text-sm font-medium text-gray- mb-">
                Time Range
              </label>
              <select
                value={timeRange}
                onChange={(e) => setTimeRange(e.target.value)}
                className="rounded border-gray- border px- py- focus:outline-none focus:ring- focus:ring-blue-"
              >
                <option value="d">Last  Days</option>
                <option value="d">Last  Days</option>
                <option value="d">Last  Days</option>
                <option value="y">Last Year</option>
              </select>
            </div>
          </div>
        </div>

        {/ Framework Cards /}
        <div className="grid grid-cols- md:grid-cols- lg:grid-cols- gap- mb-">
          {report.frameworks.map((framework) => (
            <button
              key={framework.name}
              onClick={() => setSelectedFramework(framework.name)}
              className={rounded-lg shadow p- text-left transition-all hover:shadow-lg ${
                selectedFramework === framework.name
                  ? 'ring- ring-blue- bg-blue-'
                  : 'bg-white hover:bg-gray-'
              }}
            >
              <div className="flex items-start justify-between mb-">
                <h className="text-lg font-bold text-gray-">{framework.name}</h>
                {getStatusIcon(framework.status)}
              </div>
              <div className="mb-">
                <div className="flex items-baseline gap-">
                  <p className="text-xl font-bold text-gray-">{framework.score}</p>
                  <p className="text-gray-">/</p>
                </div>
              </div>
              <div className="w-full bg-gray- rounded-full h-">
                <div
                  className={h- rounded-full transition-all ${
                    framework.score >= 
                      ? 'bg-green-'
                      : framework.score >= 
                        ? 'bg-yellow-'
                        : 'bg-red-'
                  }}
                  style={{ width: ${framework.score}% }}
                />
              </div>
              <p
                className={text-xs mt- px- py- rounded inline-block font-semibold border ${getStatusColor(
                  framework.status
                )}}
              >
                {framework.status.replace('-', ' ').toUpperCase()}
              </p>
            </button>
          ))}
        </div>

        {/ Charts /}
        <div className="grid grid-cols- lg:grid-cols- gap- mb-">
          {/ Framework Scores /}
          <div className="bg-white rounded-lg shadow p-">
            <h className="text-xl font-bold text-gray- mb-">Framework Scores</h>
            <ResponsiveContainer width="%" height={}>
              <BarChart data={frameworkScores}>
                <CartesianGrid strokeDasharray=" " />
                <XAxis dataKey="name" />
                <YAxis domain={[, ]} />
                <Tooltip />
                <Bar dataKey="score" fill="bf" radius={[, , , ]} />
              </BarChart>
            </ResponsiveContainer>
          </div>

          {/ Compliance Status Distribution /}
          <div className="bg-white rounded-lg shadow p-">
            <h className="text-xl font-bold text-gray- mb-">Compliance Status</h>
            <ResponsiveContainer width="%" height={}>
              <PieChart>
                <Pie
                  data={complianceStatusData}
                  cx="%"
                  cy="%"
                  labelLine={false}
                  label={({ name, value }) => ${name}: ${value}}
                  outerRadius={}
                  fill="d"
                  dataKey="value"
                >
                  {complianceStatusData.map((entry, index) => (
                    <Cell key={cell-${index}} fill={entry.color} />
                  ))}
                </Pie>
                <Tooltip />
              </PieChart>
            </ResponsiveContainer>
          </div>
        </div>

        {/ Compliance Trend /}
        {report.trend.length >  && (
          <div className="bg-white rounded-lg shadow p- mb-">
            <h className="text-xl font-bold text-gray- mb-">Compliance Trend</h>
            <ResponsiveContainer width="%" height={}>
              <LineChart data={report.trend}>
                <CartesianGrid strokeDasharray=" " />
                <XAxis dataKey="date" />
                <YAxis domain={[, ]} />
                <Tooltip />
                <Legend />
                <Line
                  type="monotone"
                  dataKey="score"
                  stroke="bf"
                  dot={{ fill: 'bf' }}
                  isAnimationActive={false}
                />
              </LineChart>
            </ResponsiveContainer>
          </div>
        )}

        {/ Selected Framework Details /}
        {currentFramework && (
          <div className="grid grid-cols- lg:grid-cols- gap- mb-">
            {/ Issues /}
            <div className="bg-white rounded-lg shadow p-">
              <h className="text-xl font-bold text-gray- mb- flex items-center gap-">
                <AlertCircle size={} className="text-red-" />
                Issues Found
              </h>
              {currentFramework.issues.length >  ? (
                <ul className="space-y-">
                  {currentFramework.issues.map((issue, idx) => (
                    <li
                      key={idx}
                      className="flex gap- p- bg-red- border border-red- rounded text-red- text-sm"
                    >
                      <span className="flex-shrink-"></span>
                      <span>{issue}</span>
                    </li>
                  ))}
                </ul>
              ) : (
                <p className="text-gray- text-center py-">No issues found</p>
              )}
            </div>

            {/ Recommendations /}
            <div className="bg-white rounded-lg shadow p-">
              <h className="text-xl font-bold text-gray- mb- flex items-center gap-">
                <CheckCircle size={} className="text-green-" />
                Recommendations
              </h>
              {currentFramework.recommendations.length >  ? (
                <ul className="space-y-">
                  {currentFramework.recommendations.map((rec, idx) => (
                    <li
                      key={idx}
                      className="flex gap- p- bg-green- border border-green- rounded text-green- text-sm"
                    >
                      <span className="flex-shrink-"></span>
                      <span>{rec}</span>
                    </li>
                  ))}
                </ul>
              ) : (
                <p className="text-gray- text-center py-">No recommendations</p>
              )}
            </div>
          </div>
        )}

        {/ Recent Audit Events /}
        <div className="bg-white rounded-lg shadow p-">
          <h className="text-xl font-bold text-gray- mb- flex items-center gap-">
            <Clock size={} />
            Recent Audit Events
          </h>
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead>
                <tr className="border-b border-gray-">
                  <th className="text-left px- py- font-semibold text-gray-">User</th>
                  <th className="text-left px- py- font-semibold text-gray-">Action</th>
                  <th className="text-left px- py- font-semibold text-gray-">Resource</th>
                  <th className="text-left px- py- font-semibold text-gray-">Timestamp</th>
                  <th className="text-left px- py- font-semibold text-gray-">Status</th>
                </tr>
              </thead>
              <tbody>
                {report.auditEvents.slice(, ).map((event) => (
                  <tr key={event.id} className="border-b border-gray- hover:bg-gray-">
                    <td className="px- py- text-gray-">{event.user}</td>
                    <td className="px- py- text-gray-">{event.action}</td>
                    <td className="px- py- text-gray-">{event.resource}</td>
                    <td className="px- py- text-gray- text-sm">
                      {new Date(event.timestamp).toLocaleString()}
                    </td>
                    <td className="px- py-">
                      <span
                        className={px- py- rounded-full text-xs font-semibold ${
                          event.status === 'success'
                            ? 'bg-green- text-green-'
                            : 'bg-red- text-red-'
                        }}
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
