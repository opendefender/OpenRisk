// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { useState, useEffect } from 'react';
import { useAuthStore } from '../hooks/useAuthStore';
import { format } from 'date-fns';
import { Clock, AlertCircle, CheckCircle, Filter } from 'lucide-react';
import { getAccessToken } from '../lib/session';

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
  const [page, setPage] = useState(1);
  const [limit, setLimit] = useState(20);
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

      const response = await fetch(`/api/v1/audit-logs?${params}`, {
        headers: {
          Authorization: `Bearer ${getAccessToken()}`,
        },
      });

      if (!response.ok) {
        if (response.status === 403) {
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
      return <CheckCircle className="w-5 h-5 text-success-text" />;
    } else {
      return <AlertCircle className="w-5 h-5 text-danger-text" />;
    }
  };

  const getActionBadgeColor = (action: string) => {
    const colors: { [key: string]: string } = {
      login: 'bg-info-surface text-info-text',
      login_failed: 'bg-danger-surface text-danger-text',
      register: 'bg-success-surface text-success-text',
      logout: 'bg-surface-sunken text-text-primary',
      token_refresh: 'bg-purple-100 text-purple-800',
      role_change: 'bg-warning-surface text-warning-text',
      user_delete: 'bg-danger-surface text-danger-text',
      user_deactivate: 'bg-warning-surface text-warning-text',
      user_activate: 'bg-success-surface text-success-text',
      password_change: 'bg-info-surface text-info-text',
    };
    return colors[action] || 'bg-surface-sunken text-text-primary';
  };

  if (loading && logs.length === 0) {
    return (
      <div className="space-y-4">
        <div className="flex justify-between items-center">
          <h1 className="text-3xl font-bold text-text-primary">Audit Logs</h1>
        </div>
        <div className="flex justify-center py-12">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-accent-400"></div>
        </div>
      </div>
    );
  }

  if (error && !logs.length) {
    return (
      <div className="space-y-4">
        <div className="flex justify-between items-center">
          <h1 className="text-3xl font-bold text-text-primary">Audit Logs</h1>
        </div>
        <div className="bg-danger-surface border border-red-200 rounded-lg p-4">
          <div className="flex items-center">
            <AlertCircle className="w-5 h-5 text-danger-text mr-2" />
            <p className="text-danger-text">{error}</p>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <h1 className="text-3xl font-bold text-text-primary">Audit Logs</h1>
      </div>

      {error && (
        <div className="bg-warning-surface border border-yellow-200 rounded-lg p-4">
          <div className="flex items-center">
            <AlertCircle className="w-5 h-5 text-warning-text mr-2" />
            <p className="text-warning-text">{error}</p>
          </div>
        </div>
      )}

      {/* Filters */}
      <div className="bg-surface-1 rounded-lg shadow-sm border border-border-subtle p-4">
        <div className="flex items-center gap-2 mb-4">
          <Filter className="w-5 h-5 text-text-muted" />
          <h2 className="text-lg font-semibold text-text-primary">Filters</h2>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div>
            <label className="block text-sm font-medium text-text-primary mb-2">
              Action
            </label>
            <select
              value={actionFilter}
              onChange={(e) => {
                setActionFilter(e.target.value);
                setPage(1);
              }}
              className="w-full px-4 py-2 border border-border-subtle rounded-lg focus:ring-2 focus:ring-accent-400 focus:border-transparent"
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
            <label className="block text-sm font-medium text-text-primary mb-2">
              Result
            </label>
            <select
              value={resultFilter}
              onChange={(e) => {
                setResultFilter(e.target.value);
                setPage(1);
              }}
              className="w-full px-4 py-2 border border-border-subtle rounded-lg focus:ring-2 focus:ring-accent-400 focus:border-transparent"
            >
              <option value="">All Results</option>
              <option value="success">Success</option>
              <option value="failure">Failure</option>
            </select>
          </div>
        </div>
      </div>

      {/* Audit Logs Table */}
      <div className="bg-surface-1 rounded-lg shadow-sm border border-border-subtle overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead className="bg-surface-sunken border-b border-border-subtle">
              <tr>
                <th className="px-6 py-3 text-left text-sm font-semibold text-text-primary">
                  <Clock className="w-4 h-4 inline mr-2" />
                  Timestamp
                </th>
                <th className="px-6 py-3 text-left text-sm font-semibold text-text-primary">
                  Action
                </th>
                <th className="px-6 py-3 text-left text-sm font-semibold text-text-primary">
                  Result
                </th>
                <th className="px-6 py-3 text-left text-sm font-semibold text-text-primary">
                  IP Address
                </th>
                <th className="px-6 py-3 text-left text-sm font-semibold text-text-primary">
                  Details
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border-subtle">
              {logs.map((log) => (
                <tr
                  key={log.id}
                  className="hover:bg-surface-sunken transition-colors"
                >
                  <td className="px-6 py-4 text-sm text-text-muted whitespace-nowrap">
                    {format(new Date(log.timestamp), 'MMM d, yyyy HH:mm:ss')}
                  </td>
                  <td className="px-6 py-4 text-sm">
                    <span
                      className={`px-3 py-1 rounded-full text-xs font-medium ${getActionBadgeColor(
                        log.action
                      )}`}
                    >
                      {log.action.replace(/_/g, ' ')}
                    </span>
                  </td>
                  <td className="px-6 py-4 text-sm">
                    <div className="flex items-center gap-2">
                      {getResultIcon(log.result)}
                      <span className="capitalize">{log.result}</span>
                    </div>
                  </td>
                  <td className="px-6 py-4 text-sm text-text-muted font-mono">
                    {log.ip_address || '-'}
                  </td>
                  <td className="px-6 py-4 text-sm text-text-muted">
                    {log.error_message && (
                      <div className="text-xs text-danger-text">{log.error_message}</div>
                    )}
                    {log.resource && (
                      <div className="text-xs text-text-muted">
                        Resource: {log.resource}
                      </div>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        {logs.length === 0 && !loading && (
          <div className="px-6 py-12 text-center">
            <Clock className="w-12 h-12 text-text-secondary mx-auto mb-4" />
            <p className="text-text-muted">No audit logs found</p>
          </div>
        )}
      </div>

      {/* Pagination */}
      {logs.length > 0 && (
        <div className="flex justify-between items-center">
          <div className="flex items-center gap-2">
            <label className="text-sm text-text-muted">Items per page:</label>
            <select
              value={limit}
              onChange={(e) => {
                setLimit(Number(e.target.value));
                setPage(1);
              }}
              className="px-3 py-1 border border-border-subtle rounded-lg text-sm focus:ring-2 focus:ring-accent-400 focus:border-transparent"
            >
              <option value={10}>10</option>
              <option value={20}>20</option>
              <option value={50}>50</option>
              <option value={100}>100</option>
            </select>
          </div>

          <div className="flex gap-2">
            <button
              onClick={() => setPage(Math.max(1, page - 1))}
              disabled={page === 1}
              className="px-4 py-2 border border-border-subtle rounded-lg text-sm font-medium text-text-primary hover:bg-surface-sunken disabled:opacity-50 disabled:cursor-not-allowed"
            >
              Previous
            </button>
            <span className="px-4 py-2 text-sm text-text-muted">
              Page {page}
            </span>
            <button
              onClick={() => setPage(page + 1)}
              disabled={logs.length < limit}
              className="px-4 py-2 border border-border-subtle rounded-lg text-sm font-medium text-text-primary hover:bg-surface-sunken disabled:opacity-50 disabled:cursor-not-allowed"
            >
              Next
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
