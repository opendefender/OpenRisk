import React, { useMemo, useState } from 'react';
import { BarChart, Bar, LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer } from 'recharts';
import { Activity, TrendingUp, Users, Shield, Lock, AlertCircle } from 'lucide-react';
import { useAuthStore } from "../hooks/useAuthStore";
import { AdminOnly } from "../components/rbac/PermissionGates";
import { motion } from 'framer-motion';

interface PermissionStat {
  permission: string;
  grantedCount: number;
  usageCount: number;
  deniedCount: number;
}

interface RoleStatistic {
  roleName: string;
  permissionCount: number;
  userCount: number;
  lastModified: string;
  usageRate: number;
}

interface TrendData {
  date: string;
  grants: number;
  revokes: number;
  denials: number;
}

const PermissionAnalyticsPage: React.FC = () => {
  const { } = useAuthStore();
  const [timeRange, setTimeRange] = useState<'7d' | '30d' | '90d'>('30d');

  // Mock data - in production, would come from API
  const permissionStats: PermissionStat[] = useMemo(() => [
    { permission: 'users:read', grantedCount: 45, usageCount: 892, deniedCount: 3 },
    { permission: 'users:create', grantedCount: 12, usageCount: 234, deniedCount: 8 },
    { permission: 'users:update', grantedCount: 12, usageCount: 567, deniedCount: 5 },
    { permission: 'roles:manage', grantedCount: 3, usageCount: 89, deniedCount: 12 },
    { permission: 'tenants:manage', grantedCount: 2, usageCount: 45, deniedCount: 18 },
    { permission: 'audit-logs:read', grantedCount: 28, usageCount: 654, deniedCount: 2 },
  ], []);

  const roleStats: RoleStatistic[] = useMemo(() => [
    { roleName: 'Administrator', permissionCount: 44, userCount: 3, lastModified: '2026-01-15', usageRate: 98 },
    { roleName: 'Manager', permissionCount: 18, userCount: 12, lastModified: '2026-01-18', usageRate: 75 },
    { roleName: 'Analyst', permissionCount: 12, userCount: 28, lastModified: '2026-01-10', usageRate: 62 },
    { roleName: 'Viewer', permissionCount: 2, userCount: 45, lastModified: '2026-01-05', usageRate: 89 },
  ], []);

  const trendData: TrendData[] = useMemo(() => [
    { date: 'Jan 1', grants: 12, revokes: 2, denials: 3 },
    { date: 'Jan 5', grants: 18, revokes: 4, denials: 2 },
    { date: 'Jan 10', grants: 15, revokes: 3, denials: 5 },
    { date: 'Jan 15', grants: 22, revokes: 5, denials: 4 },
    { date: 'Jan 20', grants: 19, revokes: 2, denials: 6 },
    { date: 'Jan 23', grants: 25, revokes: 6, denials: 3 },
  ], []);

  const stats = useMemo(() => ({
    totalPermissions: 44,
    grantedCount: permissionStats.reduce((sum, p) => sum + p.grantedCount, 0),
    denialRate: ((permissionStats.reduce((sum, p) => sum + p.deniedCount, 0) / (permissionStats.reduce((sum, p) => sum + p.deniedCount + p.usageCount, 0))) * 100).toFixed(2),
    mostUsedPermission: permissionStats.reduce((max, p) => p.usageCount > max.usageCount ? p : max),
    activeRoles: roleStats.filter(r => r.usageRate > 50).length,
  }), [permissionStats, roleStats]);

  return (
    <AdminOnly fallback={<div className="p-6 bg-red-50 rounded-lg">Permission Denied: Admin access required</div>}>
      <div className="w-full space-y-6 pb-6">
        {/* Header */}
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-3xl font-bold text-gray-900">Permission Analytics</h1>
            <p className="text-gray-600">Monitor and analyze RBAC usage patterns</p>
          </div>
          <div className="flex gap-2">
            {(['7d', '30d', '90d'] as const).map((range) => (
              <button
                key={range}
                onClick={() => setTimeRange(range)}
                className={`px-4 py-2 rounded-lg transition-colors ${
                  timeRange === range
                    ? 'bg-blue-600 text-white'
                    : 'bg-gray-200 text-gray-700 hover:bg-gray-300'
                }`}
              >
                {range}
              </button>
            ))}
          </div>
        </div>

        {/* Key Metrics */}
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
          <motion.div
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            className="bg-white rounded-lg shadow-sm border border-gray-200 p-6"
          >
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm text-gray-600">Total Permissions</p>
                <p className="text-2xl font-bold text-gray-900">{stats.totalPermissions}</p>
              </div>
              <Shield className="w-8 h-8 text-blue-600" />
            </div>
          </motion.div>

          <motion.div
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.1 }}
            className="bg-white rounded-lg shadow-sm border border-gray-200 p-6"
          >
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm text-gray-600">Permissions Granted</p>
                <p className="text-2xl font-bold text-green-600">{stats.grantedCount}</p>
              </div>
              <Users className="w-8 h-8 text-green-600" />
            </div>
          </motion.div>

          <motion.div
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.2 }}
            className="bg-white rounded-lg shadow-sm border border-gray-200 p-6"
          >
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm text-gray-600">Denial Rate</p>
                <p className="text-2xl font-bold text-red-600">{stats.denialRate}%</p>
              </div>
              <AlertCircle className="w-8 h-8 text-red-600" />
            </div>
          </motion.div>

          <motion.div
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.3 }}
            className="bg-white rounded-lg shadow-sm border border-gray-200 p-6"
          >
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm text-gray-600">Active Roles</p>
                <p className="text-2xl font-bold text-purple-600">{stats.activeRoles}/4</p>
              </div>
              <Lock className="w-8 h-8 text-purple-600" />
            </div>
          </motion.div>
        </div>

        {/* Trends Chart */}
        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.4 }}
          className="bg-white rounded-lg shadow-sm border border-gray-200 p-6"
        >
          <h2 className="text-lg font-semibold mb-4 flex items-center gap-2">
            <TrendingUp className="w-5 h-5 text-blue-600" />
            Permission Activity Trends
          </h2>
          <ResponsiveContainer width="100%" height={300}>
            <LineChart data={trendData}>
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis dataKey="date" />
              <YAxis />
              <Tooltip />
              <Legend />
              <Line type="monotone" dataKey="grants" stroke="#10b981" strokeWidth={2} />
              <Line type="monotone" dataKey="revokes" stroke="#f59e0b" strokeWidth={2} />
              <Line type="monotone" dataKey="denials" stroke="#ef4444" strokeWidth={2} />
            </LineChart>
          </ResponsiveContainer>
        </motion.div>

        {/* Permission Usage */}
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          <motion.div
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.5 }}
            className="bg-white rounded-lg shadow-sm border border-gray-200 p-6"
          >
            <h2 className="text-lg font-semibold mb-4">Top Permissions</h2>
            <ResponsiveContainer width="100%" height={300}>
              <BarChart data={permissionStats.slice(0, 6)}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="permission" angle={-45} textAnchor="end" height={80} />
                <YAxis />
                <Tooltip />
                <Bar dataKey="usageCount" fill="#3b82f6" />
              </BarChart>
            </ResponsiveContainer>
          </motion.div>

          {/* Role Statistics Table */}
          <motion.div
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.6 }}
            className="bg-white rounded-lg shadow-sm border border-gray-200 p-6"
          >
            <h2 className="text-lg font-semibold mb-4">Role Statistics</h2>
            <div className="space-y-3">
              {roleStats.map((role) => (
                <div key={role.roleName} className="flex items-center justify-between p-3 bg-gray-50 rounded-lg">
                  <div>
                    <p className="font-medium text-sm">{role.roleName}</p>
                    <p className="text-xs text-gray-600">
                      {role.permissionCount} perms • {role.userCount} users
                    </p>
                  </div>
                  <div className="text-right">
                    <div className="w-16 h-8 bg-gray-200 rounded-full overflow-hidden">
                      <div
                        className="h-full bg-blue-600 transition-all"
                        style={{ width: `${role.usageRate}%` }}
                      />
                    </div>
                    <p className="text-xs text-gray-600 mt-1">{role.usageRate}%</p>
                  </div>
                </div>
              ))}
            </div>
          </motion.div>
        </div>

        {/* Permission Distribution */}
        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.7 }}
          className="bg-white rounded-lg shadow-sm border border-gray-200 p-6"
        >
          <h2 className="text-lg font-semibold mb-4 flex items-center gap-2">
            <Activity className="w-5 h-5 text-purple-600" />
            Permission Distribution Matrix
          </h2>
          
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-gray-200">
                  <th className="text-left py-3 px-4 font-medium text-gray-700">Permission</th>
                  <th className="text-right py-3 px-4 font-medium text-gray-700">Granted</th>
                  <th className="text-right py-3 px-4 font-medium text-gray-700">Used</th>
                  <th className="text-right py-3 px-4 font-medium text-gray-700">Denied</th>
                  <th className="text-right py-3 px-4 font-medium text-gray-700">Coverage %</th>
                </tr>
              </thead>
              <tbody>
                {permissionStats.map((stat) => (
                  <tr key={stat.permission} className="border-b border-gray-200 hover:bg-gray-50">
                    <td className="py-3 px-4 font-medium text-gray-900">{stat.permission}</td>
                    <td className="py-3 px-4 text-right text-green-600">{stat.grantedCount}</td>
                    <td className="py-3 px-4 text-right text-blue-600">{stat.usageCount}</td>
                    <td className="py-3 px-4 text-right text-red-600">{stat.deniedCount}</td>
                    <td className="py-3 px-4 text-right">
                      <span className="inline-block px-2 py-1 bg-blue-100 text-blue-800 rounded">
                        {((stat.usageCount / (stat.usageCount + stat.deniedCount)) * 100).toFixed(0)}%
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </motion.div>

        {/* Insights */}
        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.8 }}
          className="bg-blue-50 border border-blue-200 rounded-lg p-6"
        >
          <h3 className="font-semibold text-blue-900 mb-3">📊 Insights</h3>
          <ul className="space-y-2 text-sm text-blue-800">
            <li>• Most used permission: {stats.mostUsedPermission.permission} ({stats.mostUsedPermission.usageCount} uses)</li>
            <li>• {stats.activeRoles} out of 4 roles show high activity (&gt;50% usage rate)</li>
            <li>• Denial rate is {stats.denialRate}% - monitor for access issues</li>
            <li>• Administrator role has 100% permission coverage</li>
          </ul>
        </motion.div>
      </div>
    </AdminOnly>
  );
};

export default PermissionAnalyticsPage;
