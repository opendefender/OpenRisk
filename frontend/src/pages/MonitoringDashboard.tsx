import React, { useEffect, useState } from 'react';

interface MetricData {
  label: string;
  value: number | string;
  unit?: string;
  status: 'healthy' | 'warning' | 'critical';
}

interface AlertData {
  id: string;
  title: string;
  severity: 'INFO' | 'WARNING' | 'CRITICAL';
  timestamp: string;
  resolved: boolean;
}

interface DashboardData {
  metrics: MetricData[];
  alerts: AlertData[];
  systemHealth: 'HEALTHY' | 'WARNING' | 'CRITICAL';
}

export const MonitoringDashboard: React.FC = () => {
  const [dashboardData, setDashboardData] = useState<DashboardData>({
    metrics: [],
    alerts: [],
    systemHealth: 'HEALTHY',
  });

  const [loading, setLoading] = useState(true);

  useEffect(() => {
    // Simulate fetching metrics
    const mockMetrics: MetricData[] = [
      {
        label: 'Average Latency',
        value: '45ms',
        status: 'healthy',
      },
      {
        label: 'Cache Hit Rate',
        value: '92.5%',
        status: 'healthy',
      },
      {
        label: 'Error Rate',
        value: '0.2%',
        status: 'healthy',
      },
      {
        label: 'Active Requests',
        value: '1,234',
        status: 'healthy',
      },
      {
        label: 'Permission Denials',
        value: '5',
        unit: '/hour',
        status: 'healthy',
      },
      {
        label: 'System Security Score',
        value: 98,
        unit: '/100',
        status: 'healthy',
      },
    ];

    const mockAlerts: AlertData[] = [
      {
        id: 'ALERT-001',
        title: 'High Memory Usage Detected',
        severity: 'WARNING',
        timestamp: new Date(Date.now() - 5 * 60000).toISOString(),
        resolved: false,
      },
      {
        id: 'ALERT-002',
        title: 'Successful deployment completed',
        severity: 'INFO',
        timestamp: new Date(Date.now() - 15 * 60000).toISOString(),
        resolved: false,
      },
    ];

    setDashboardData({
      metrics: mockMetrics,
      alerts: mockAlerts,
      systemHealth: 'HEALTHY',
    });

    setLoading(false);
  }, []);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-screen">
        <div className="text-gray-500">Loading monitoring dashboard...</div>
      </div>
    );
  }

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'healthy':
        return 'bg-green-100 border-green-300 text-green-800';
      case 'warning':
        return 'bg-yellow-100 border-yellow-300 text-yellow-800';
      case 'critical':
        return 'bg-red-100 border-red-300 text-red-800';
      default:
        return 'bg-gray-100 border-gray-300 text-gray-800';
    }
  };

  const getSeverityColor = (severity: string) => {
    switch (severity) {
      case 'CRITICAL':
        return 'bg-red-50 border-l-4 border-red-500';
      case 'WARNING':
        return 'bg-yellow-50 border-l-4 border-yellow-500';
      case 'INFO':
        return 'bg-blue-50 border-l-4 border-blue-500';
      default:
        return 'bg-gray-50 border-l-4 border-gray-500';
    }
  };

  const getSeverityIcon = (severity: string) => {
    switch (severity) {
      case 'CRITICAL':
        return '🔴';
      case 'WARNING':
        return '🟠';
      case 'INFO':
        return 'ℹ️';
      default:
        return '⚪';
    }
  };

  return (
    <div className="min-h-screen bg-gradient-to-br from-gray-50 to-gray-100 p-8">
      <div className="max-w-7xl mx-auto">
        {/* Header */}
        <div className="mb-8">
          <h1 className="text-4xl font-bold text-gray-900 mb-2">System Monitoring</h1>
          <p className="text-gray-600">Real-time metrics and alerts for OpenRisk</p>
        </div>

        {/* System Health Status */}
        <div className="bg-white rounded-lg shadow-md p-6 mb-8">
          <div className="flex items-center justify-between">
            <div>
              <h2 className="text-xl font-semibold text-gray-800 mb-1">System Health</h2>
              <p className="text-gray-600">Overall system status</p>
            </div>
            <div className={`px-6 py-3 rounded-full font-semibold ${
              dashboardData.systemHealth === 'HEALTHY'
                ? 'bg-green-100 text-green-800'
                : dashboardData.systemHealth === 'WARNING'
                ? 'bg-yellow-100 text-yellow-800'
                : 'bg-red-100 text-red-800'
            }`}>
              {dashboardData.systemHealth === 'HEALTHY' ? '✅ HEALTHY' :
               dashboardData.systemHealth === 'WARNING' ? '⚠️ WARNING' :
               '❌ CRITICAL'}
            </div>
          </div>
        </div>

        {/* Metrics Grid */}
        <div className="mb-8">
          <h2 className="text-2xl font-bold text-gray-800 mb-4">Performance Metrics</h2>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {dashboardData.metrics.map((metric, index) => (
              <div
                key={index}
                className={`rounded-lg border-2 p-6 transition-all hover:shadow-lg ${getStatusColor(metric.status)}`}
              >
                <div className="text-sm font-medium opacity-75">{metric.label}</div>
                <div className="text-3xl font-bold mt-2">
                  {metric.value}
                  {metric.unit && <span className="text-lg ml-1">{metric.unit}</span>}
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* Alerts */}
        <div>
          <h2 className="text-2xl font-bold text-gray-800 mb-4">Recent Alerts</h2>
          <div className="space-y-3">
            {dashboardData.alerts.length > 0 ? (
              dashboardData.alerts.map((alert) => (
                <div
                  key={alert.id}
                  className={`rounded-lg p-4 flex items-start gap-4 ${getSeverityColor(alert.severity)}`}
                >
                  <div className="text-2xl mt-1">{getSeverityIcon(alert.severity)}</div>
                  <div className="flex-1">
                    <div className="font-semibold text-gray-900">{alert.title}</div>
                    <div className="text-sm text-gray-600 mt-1">
                      {new Date(alert.timestamp).toLocaleString()}
                    </div>
                  </div>
                  {!alert.resolved && (
                    <span className="px-3 py-1 bg-red-200 text-red-800 rounded-full text-sm font-medium">
                      Active
                    </span>
                  )}
                </div>
              ))
            ) : (
              <div className="text-center py-8 text-gray-500">
                No alerts to display
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
};

export default MonitoringDashboard;
