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
  const { user } = useAuthStore();
  const [timeRange, setTimeRange] = useState<'d' | 'd' | 'd'>('d');

  // Mock data - in production, would come from API
  const permissionStats: PermissionStat[] = useMemo(() => [
    { permission: 'users:read', grantedCount: , usageCount: , deniedCount:  },
    { permission: 'users:create', grantedCount: , usageCount: , deniedCount:  },
    { permission: 'users:update', grantedCount: , usageCount: , deniedCount:  },
    { permission: 'roles:manage', grantedCount: , usageCount: , deniedCount:  },
    { permission: 'tenants:manage', grantedCount: , usageCount: , deniedCount:  },
    { permission: 'audit-logs:read', grantedCount: , usageCount: , deniedCount:  },
  ], []);

  const roleStats: RoleStatistic[] = useMemo(() => [
    { roleName: 'Administrator', permissionCount: , userCount: , lastModified: '--', usageRate:  },
    { roleName: 'Manager', permissionCount: , userCount: , lastModified: '--', usageRate:  },
    { roleName: 'Analyst', permissionCount: , userCount: , lastModified: '--', usageRate:  },
    { roleName: 'Viewer', permissionCount: , userCount: , lastModified: '--', usageRate:  },
  ], []);

  const trendData: TrendData[] = useMemo(() => [
    { date: 'Jan ', grants: , revokes: , denials:  },
    { date: 'Jan ', grants: , revokes: , denials:  },
    { date: 'Jan ', grants: , revokes: , denials:  },
    { date: 'Jan ', grants: , revokes: , denials:  },
    { date: 'Jan ', grants: , revokes: , denials:  },
    { date: 'Jan ', grants: , revokes: , denials:  },
  ], []);

  const stats = useMemo(() => ({
    totalPermissions: ,
    grantedCount: permissionStats.reduce((sum, p) => sum + p.grantedCount, ),
    denialRate: ((permissionStats.reduce((sum, p) => sum + p.deniedCount, ) / (permissionStats.reduce((sum, p) => sum + p.deniedCount + p.usageCount, )))  ).toFixed(),
    mostUsedPermission: permissionStats.reduce((max, p) => p.usageCount > max.usageCount ? p : max),
    activeRoles: roleStats.filter(r => r.usageRate > ).length,
  }), [permissionStats, roleStats]);

  return (
    <AdminOnly fallback={<div className="p- bg-red- rounded-lg">Permission Denied: Admin access required</div>}>
      <div className="w-full space-y- pb-">
        {/ Header /}
        <div className="flex items-center justify-between">
          <div>
            <h className="text-xl font-bold text-gray-">Permission Analytics</h>
            <p className="text-gray-">Monitor and analyze RBAC usage patterns</p>
          </div>
          <div className="flex gap-">
            {(['d', 'd', 'd'] as const).map((range) => (
              <button
                key={range}
                onClick={() => setTimeRange(range)}
                className={px- py- rounded-lg transition-colors ${
                  timeRange === range
                    ? 'bg-blue- text-white'
                    : 'bg-gray- text-gray- hover:bg-gray-'
                }}
              >
                {range}
              </button>
            ))}
          </div>
        </div>

        {/ Key Metrics /}
        <div className="grid grid-cols- md:grid-cols- lg:grid-cols- gap-">
          <motion.div
            initial={{ opacity: , y:  }}
            animate={{ opacity: , y:  }}
            className="bg-white rounded-lg shadow-sm border border-gray- p-"
          >
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm text-gray-">Total Permissions</p>
                <p className="text-xl font-bold text-gray-">{stats.totalPermissions}</p>
              </div>
              <Shield className="w- h- text-blue-" />
            </div>
          </motion.div>

          <motion.div
            initial={{ opacity: , y:  }}
            animate={{ opacity: , y:  }}
            transition={{ delay: . }}
            className="bg-white rounded-lg shadow-sm border border-gray- p-"
          >
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm text-gray-">Permissions Granted</p>
                <p className="text-xl font-bold text-green-">{stats.grantedCount}</p>
              </div>
              <Users className="w- h- text-green-" />
            </div>
          </motion.div>

          <motion.div
            initial={{ opacity: , y:  }}
            animate={{ opacity: , y:  }}
            transition={{ delay: . }}
            className="bg-white rounded-lg shadow-sm border border-gray- p-"
          >
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm text-gray-">Denial Rate</p>
                <p className="text-xl font-bold text-red-">{stats.denialRate}%</p>
              </div>
              <AlertCircle className="w- h- text-red-" />
            </div>
          </motion.div>

          <motion.div
            initial={{ opacity: , y:  }}
            animate={{ opacity: , y:  }}
            transition={{ delay: . }}
            className="bg-white rounded-lg shadow-sm border border-gray- p-"
          >
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm text-gray-">Active Roles</p>
                <p className="text-xl font-bold text-purple-">{stats.activeRoles}/</p>
              </div>
              <Lock className="w- h- text-purple-" />
            </div>
          </motion.div>
        </div>

        {/ Trends Chart /}
        <motion.div
          initial={{ opacity: , y:  }}
          animate={{ opacity: , y:  }}
          transition={{ delay: . }}
          className="bg-white rounded-lg shadow-sm border border-gray- p-"
        >
          <h className="text-lg font-semibold mb- flex items-center gap-">
            <TrendingUp className="w- h- text-blue-" />
            Permission Activity Trends
          </h>
          <ResponsiveContainer width="%" height={}>
            <LineChart data={trendData}>
              <CartesianGrid strokeDasharray=" " />
              <XAxis dataKey="date" />
              <YAxis />
              <Tooltip />
              <Legend />
              <Line type="monotone" dataKey="grants" stroke="b" strokeWidth={} />
              <Line type="monotone" dataKey="revokes" stroke="feb" strokeWidth={} />
              <Line type="monotone" dataKey="denials" stroke="ef" strokeWidth={} />
            </LineChart>
          </ResponsiveContainer>
        </motion.div>

        {/ Permission Usage /}
        <div className="grid grid-cols- lg:grid-cols- gap-">
          <motion.div
            initial={{ opacity: , y:  }}
            animate={{ opacity: , y:  }}
            transition={{ delay: . }}
            className="bg-white rounded-lg shadow-sm border border-gray- p-"
          >
            <h className="text-lg font-semibold mb-">Top Permissions</h>
            <ResponsiveContainer width="%" height={}>
              <BarChart data={permissionStats.slice(, )}>
                <CartesianGrid strokeDasharray=" " />
                <XAxis dataKey="permission" angle={-} textAnchor="end" height={} />
                <YAxis />
                <Tooltip />
                <Bar dataKey="usageCount" fill="bf" />
              </BarChart>
            </ResponsiveContainer>
          </motion.div>

          {/ Role Statistics Table /}
          <motion.div
            initial={{ opacity: , y:  }}
            animate={{ opacity: , y:  }}
            transition={{ delay: . }}
            className="bg-white rounded-lg shadow-sm border border-gray- p-"
          >
            <h className="text-lg font-semibold mb-">Role Statistics</h>
            <div className="space-y-">
              {roleStats.map((role) => (
                <div key={role.roleName} className="flex items-center justify-between p- bg-gray- rounded-lg">
                  <div>
                    <p className="font-medium text-sm">{role.roleName}</p>
                    <p className="text-xs text-gray-">
                      {role.permissionCount} perms • {role.userCount} users
                    </p>
                  </div>
                  <div className="text-right">
                    <div className="w- h- bg-gray- rounded-full overflow-hidden">
                      <div
                        className="h-full bg-blue- transition-all"
                        style={{ width: ${role.usageRate}% }}
                      />
                    </div>
                    <p className="text-xs text-gray- mt-">{role.usageRate}%</p>
                  </div>
                </div>
              ))}
            </div>
          </motion.div>
        </div>

        {/ Permission Distribution /}
        <motion.div
          initial={{ opacity: , y:  }}
          animate={{ opacity: , y:  }}
          transition={{ delay: . }}
          className="bg-white rounded-lg shadow-sm border border-gray- p-"
        >
          <h className="text-lg font-semibold mb- flex items-center gap-">
            <Activity className="w- h- text-purple-" />
            Permission Distribution Matrix
          </h>
          
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-gray-">
                  <th className="text-left py- px- font-medium text-gray-">Permission</th>
                  <th className="text-right py- px- font-medium text-gray-">Granted</th>
                  <th className="text-right py- px- font-medium text-gray-">Used</th>
                  <th className="text-right py- px- font-medium text-gray-">Denied</th>
                  <th className="text-right py- px- font-medium text-gray-">Coverage %</th>
                </tr>
              </thead>
              <tbody>
                {permissionStats.map((stat) => (
                  <tr key={stat.permission} className="border-b border-gray- hover:bg-gray-">
                    <td className="py- px- font-medium text-gray-">{stat.permission}</td>
                    <td className="py- px- text-right text-green-">{stat.grantedCount}</td>
                    <td className="py- px- text-right text-blue-">{stat.usageCount}</td>
                    <td className="py- px- text-right text-red-">{stat.deniedCount}</td>
                    <td className="py- px- text-right">
                      <span className="inline-block px- py- bg-blue- text-blue- rounded">
                        {((stat.usageCount / (stat.usageCount + stat.deniedCount))  ).toFixed()}%
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </motion.div>

        {/ Insights /}
        <motion.div
          initial={{ opacity: , y:  }}
          animate={{ opacity: , y:  }}
          transition={{ delay: . }}
          className="bg-blue- border border-blue- rounded-lg p-"
        >
          <h className="font-semibold text-blue- mb-"> Insights</h>
          <ul className="space-y- text-sm text-blue-">
            <li>• Most used permission: {stats.mostUsedPermission.permission} ({stats.mostUsedPermission.usageCount} uses)</li>
            <li>• {stats.activeRoles} out of  roles show high activity (&gt;% usage rate)</li>
            <li>• Denial rate is {stats.denialRate}% - monitor for access issues</li>
            <li>• Administrator role has % permission coverage</li>
          </ul>
        </motion.div>
      </div>
    </AdminOnly>
  );
};

export default PermissionAnalyticsPage;
