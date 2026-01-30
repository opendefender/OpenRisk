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
        value: 'ms',
        status: 'healthy',
      },
      {
        label: 'Cache Hit Rate',
        value: '.%',
        status: 'healthy',
      },
      {
        label: 'Error Rate',
        value: '.%',
        status: 'healthy',
      },
      {
        label: 'Active Requests',
        value: ',',
        status: 'healthy',
      },
      {
        label: 'Permission Denials',
        value: '',
        unit: '/hour',
        status: 'healthy',
      },
      {
        label: 'System Security Score',
        value: ,
        unit: '/',
        status: 'healthy',
      },
    ];

    const mockAlerts: AlertData[] = [
      {
        id: 'ALERT-',
        title: 'High Memory Usage Detected',
        severity: 'WARNING',
        timestamp: new Date(Date.now() -   ).toISOString(),
        resolved: false,
      },
      {
        id: 'ALERT-',
        title: 'Successful deployment completed',
        severity: 'INFO',
        timestamp: new Date(Date.now() -   ).toISOString(),
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
        <div className="text-gray-">Loading monitoring dashboard...</div>
      </div>
    );
  }

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'healthy':
        return 'bg-green- border-green- text-green-';
      case 'warning':
        return 'bg-yellow- border-yellow- text-yellow-';
      case 'critical':
        return 'bg-red- border-red- text-red-';
      default:
        return 'bg-gray- border-gray- text-gray-';
    }
  };

  const getSeverityColor = (severity: string) => {
    switch (severity) {
      case 'CRITICAL':
        return 'bg-red- border-l- border-red-';
      case 'WARNING':
        return 'bg-yellow- border-l- border-yellow-';
      case 'INFO':
        return 'bg-blue- border-l- border-blue-';
      default:
        return 'bg-gray- border-l- border-gray-';
    }
  };

  const getSeverityIcon = (severity: string) => {
    switch (severity) {
      case 'CRITICAL':
        return '';
      case 'WARNING':
        return '';
      case 'INFO':
        return 'ℹ';
      default:
        return '';
    }
  };

  return (
    <div className="min-h-screen bg-gradient-to-br from-gray- to-gray- p-">
      <div className="max-w-xl mx-auto">
        {/ Header /}
        <div className="mb-">
          <h className="text-xl font-bold text-gray- mb-">System Monitoring</h>
          <p className="text-gray-">Real-time metrics and alerts for OpenRisk</p>
        </div>

        {/ System Health Status /}
        <div className="bg-white rounded-lg shadow-md p- mb-">
          <div className="flex items-center justify-between">
            <div>
              <h className="text-xl font-semibold text-gray- mb-">System Health</h>
              <p className="text-gray-">Overall system status</p>
            </div>
            <div className={px- py- rounded-full font-semibold ${
              dashboardData.systemHealth === 'HEALTHY'
                ? 'bg-green- text-green-'
                : dashboardData.systemHealth === 'WARNING'
                ? 'bg-yellow- text-yellow-'
                : 'bg-red- text-red-'
            }}>
              {dashboardData.systemHealth === 'HEALTHY' ? ' HEALTHY' :
               dashboardData.systemHealth === 'WARNING' ? ' WARNING' :
               ' CRITICAL'}
            </div>
          </div>
        </div>

        {/ Metrics Grid /}
        <div className="mb-">
          <h className="text-xl font-bold text-gray- mb-">Performance Metrics</h>
          <div className="grid grid-cols- md:grid-cols- lg:grid-cols- gap-">
            {dashboardData.metrics.map((metric, index) => (
              <div
                key={index}
                className={rounded-lg border- p- transition-all hover:shadow-lg ${getStatusColor(metric.status)}}
              >
                <div className="text-sm font-medium opacity-">{metric.label}</div>
                <div className="text-xl font-bold mt-">
                  {metric.value}
                  {metric.unit && <span className="text-lg ml-">{metric.unit}</span>}
                </div>
              </div>
            ))}
          </div>
        </div>

        {/ Alerts /}
        <div>
          <h className="text-xl font-bold text-gray- mb-">Recent Alerts</h>
          <div className="space-y-">
            {dashboardData.alerts.length >  ? (
              dashboardData.alerts.map((alert) => (
                <div
                  key={alert.id}
                  className={rounded-lg p- flex items-start gap- ${getSeverityColor(alert.severity)}}
                >
                  <div className="text-xl mt-">{getSeverityIcon(alert.severity)}</div>
                  <div className="flex-">
                    <div className="font-semibold text-gray-">{alert.title}</div>
                    <div className="text-sm text-gray- mt-">
                      {new Date(alert.timestamp).toLocaleString()}
                    </div>
                  </div>
                  {!alert.resolved && (
                    <span className="px- py- bg-red- text-red- rounded-full text-sm font-medium">
                      Active
                    </span>
                  )}
                </div>
              ))
            ) : (
              <div className="text-center py- text-gray-">
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
