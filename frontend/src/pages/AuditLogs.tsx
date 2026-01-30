import { useState, useEffect } from 'react';
import { useAuthStore } from '../hooks/useAuthStore';
import { format } from 'date-fns';
import { Clock, AlertCircle, CheckCircle, Filter } from 'lucide-react';

interface AuditLog {
  id: string;
  user_id?: string;
  action: string;
  resource?: string;
  resource_id?: string;
  result: string;
  error_message?: string;
  ip_address?: string;
  user_agent?: string;
  timestamp: string;
}

interface AuditLogsResponse {
  data: AuditLog[];
  page: number;
  limit: number;
  count: number;
}

export default function AuditLogs() {
  const { user } = useAuthStore();
  const [logs, setLogs] = useState<AuditLog[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [page, setPage] = useState();
  const [limit, setLimit] = useState();
  const [actionFilter, setActionFilter] = useState<string>('');
  const [resultFilter, setResultFilter] = useState<string>('');

  const fetchAuditLogs = async () => {
    try {
      setLoading(true);
      setError(null);

      const params = new URLSearchParams();
      params.append('page', page.toString());
      params.append('limit', limit.toString());
      if (actionFilter) params.append('action', actionFilter);
      if (resultFilter) params.append('result', resultFilter);

      const response = await fetch(/api/v/audit-logs?${params}, {
        headers: {
          Authorization: Bearer ${localStorage.getItem('token')},
        },
      });

      if (!response.ok) {
        if (response.status === ) {
          setError('You do not have permission to view audit logs');
        } else {
          setError("We're unable to load the audit logs. Please refresh and try again.");
        }
        return;
      }

      const data: AuditLogsResponse = await response.json();
      setLogs(data.data || []);
    } catch (err) {
      console.error('Error fetching audit logs:', err);
      setError("We're unable to load the audit logs. Please refresh and try again.");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    // Only admin users can view audit logs
    if (user && user.role !== 'admin') {
      setError('Only administrators can view audit logs');
      setLoading(false);
      return;
    }
    fetchAuditLogs();
  }, [page, limit, actionFilter, resultFilter, user]);

  const getResultIcon = (result: string) => {
    if (result === 'success') {
      return <CheckCircle className="w- h- text-green-" />;
    } else {
      return <AlertCircle className="w- h- text-red-" />;
    }
  };

  const getActionBadgeColor = (action: string) => {
    const colors: { [key: string]: string } = {
      login: 'bg-blue- text-blue-',
      login_failed: 'bg-red- text-red-',
      register: 'bg-green- text-green-',
      logout: 'bg-gray- text-gray-',
      token_refresh: 'bg-purple- text-purple-',
      role_change: 'bg-yellow- text-yellow-',
      user_delete: 'bg-red- text-red-',
      user_deactivate: 'bg-orange- text-orange-',
      user_activate: 'bg-green- text-green-',
      password_change: 'bg-indigo- text-indigo-',
    };
    return colors[action] || 'bg-gray- text-gray-';
  };

  if (loading && logs.length === ) {
    return (
      <div className="space-y-">
        <div className="flex justify-between items-center">
          <h className="text-xl font-bold text-gray-">Audit Logs</h>
        </div>
        <div className="flex justify-center py-">
          <div className="animate-spin rounded-full h- w- border-b- border-blue-"></div>
        </div>
      </div>
    );
  }

  if (error && !logs.length) {
    return (
      <div className="space-y-">
        <div className="flex justify-between items-center">
          <h className="text-xl font-bold text-gray-">Audit Logs</h>
        </div>
        <div className="bg-red- border border-red- rounded-lg p-">
          <div className="flex items-center">
            <AlertCircle className="w- h- text-red- mr-" />
            <p className="text-red-">{error}</p>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-">
      <div className="flex justify-between items-center">
        <h className="text-xl font-bold text-gray-">Audit Logs</h>
      </div>

      {error && (
        <div className="bg-yellow- border border-yellow- rounded-lg p-">
          <div className="flex items-center">
            <AlertCircle className="w- h- text-yellow- mr-" />
            <p className="text-yellow-">{error}</p>
          </div>
        </div>
      )}

      {/ Filters /}
      <div className="bg-white rounded-lg shadow-sm border border-gray- p-">
        <div className="flex items-center gap- mb-">
          <Filter className="w- h- text-gray-" />
          <h className="text-lg font-semibold text-gray-">Filters</h>
        </div>

        <div className="grid grid-cols- md:grid-cols- gap-">
          <div>
            <label className="block text-sm font-medium text-gray- mb-">
              Action
            </label>
            <select
              value={actionFilter}
              onChange={(e) => {
                setActionFilter(e.target.value);
                setPage();
              }}
              className="w-full px- py- border border-gray- rounded-lg focus:ring- focus:ring-blue- focus:border-transparent"
            >
              <option value="">All Actions</option>
              <option value="login">Login</option>
              <option value="login_failed">Login Failed</option>
              <option value="register">Register</option>
              <option value="logout">Logout</option>
              <option value="token_refresh">Token Refresh</option>
              <option value="role_change">Role Change</option>
              <option value="user_delete">User Delete</option>
              <option value="user_deactivate">User Deactivate</option>
              <option value="user_activate">User Activate</option>
            </select>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray- mb-">
              Result
            </label>
            <select
              value={resultFilter}
              onChange={(e) => {
                setResultFilter(e.target.value);
                setPage();
              }}
              className="w-full px- py- border border-gray- rounded-lg focus:ring- focus:ring-blue- focus:border-transparent"
            >
              <option value="">All Results</option>
              <option value="success">Success</option>
              <option value="failure">Failure</option>
            </select>
          </div>
        </div>
      </div>

      {/ Audit Logs Table /}
      <div className="bg-white rounded-lg shadow-sm border border-gray- overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead className="bg-gray- border-b border-gray-">
              <tr>
                <th className="px- py- text-left text-sm font-semibold text-gray-">
                  <Clock className="w- h- inline mr-" />
                  Timestamp
                </th>
                <th className="px- py- text-left text-sm font-semibold text-gray-">
                  Action
                </th>
                <th className="px- py- text-left text-sm font-semibold text-gray-">
                  Result
                </th>
                <th className="px- py- text-left text-sm font-semibold text-gray-">
                  IP Address
                </th>
                <th className="px- py- text-left text-sm font-semibold text-gray-">
                  Details
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-">
              {logs.map((log) => (
                <tr
                  key={log.id}
                  className="hover:bg-gray- transition-colors"
                >
                  <td className="px- py- text-sm text-gray- whitespace-nowrap">
                    {format(new Date(log.timestamp), 'MMM d, yyyy HH:mm:ss')}
                  </td>
                  <td className="px- py- text-sm">
                    <span
                      className={px- py- rounded-full text-xs font-medium ${getActionBadgeColor(
                        log.action
                      )}}
                    >
                      {log.action.replace(/_/g, ' ')}
                    </span>
                  </td>
                  <td className="px- py- text-sm">
                    <div className="flex items-center gap-">
                      {getResultIcon(log.result)}
                      <span className="capitalize">{log.result}</span>
                    </div>
                  </td>
                  <td className="px- py- text-sm text-gray- font-mono">
                    {log.ip_address || '-'}
                  </td>
                  <td className="px- py- text-sm text-gray-">
                    {log.error_message && (
                      <div className="text-xs text-red-">{log.error_message}</div>
                    )}
                    {log.resource && (
                      <div className="text-xs text-gray-">
                        Resource: {log.resource}
                      </div>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        {logs.length ===  && !loading && (
          <div className="px- py- text-center">
            <Clock className="w- h- text-gray- mx-auto mb-" />
            <p className="text-gray-">No audit logs found</p>
          </div>
        )}
      </div>

      {/ Pagination /}
      {logs.length >  && (
        <div className="flex justify-between items-center">
          <div className="flex items-center gap-">
            <label className="text-sm text-gray-">Items per page:</label>
            <select
              value={limit}
              onChange={(e) => {
                setLimit(Number(e.target.value));
                setPage();
              }}
              className="px- py- border border-gray- rounded-lg text-sm focus:ring- focus:ring-blue- focus:border-transparent"
            >
              <option value={}></option>
              <option value={}></option>
              <option value={}></option>
              <option value={}></option>
            </select>
          </div>

          <div className="flex gap-">
            <button
              onClick={() => setPage(Math.max(, page - ))}
              disabled={page === }
              className="px- py- border border-gray- rounded-lg text-sm font-medium text-gray- hover:bg-gray- disabled:opacity- disabled:cursor-not-allowed"
            >
              Previous
            </button>
            <span className="px- py- text-sm text-gray-">
              Page {page}
            </span>
            <button
              onClick={() => setPage(page + )}
              disabled={logs.length < limit}
              className="px- py- border border-gray- rounded-lg text-sm font-medium text-gray- hover:bg-gray- disabled:opacity- disabled:cursor-not-allowed"
            >
              Next
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
