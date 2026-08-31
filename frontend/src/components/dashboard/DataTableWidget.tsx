// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import React from 'react';
import { MoreVertical, TrendingUp, Users, AlertTriangle, CheckCircle } from 'lucide-react';

interface DataItem {
  id: string;
  name: string;
  value: number;
  trend?: number;
  status?: 'active' | 'warning' | 'critical';
}

interface DataTableWidgetProps {
  title: string;
  columns?: string[];
  data?: DataItem[];
  rowCount?: number;
  children?: React.ReactNode;
  onRowClick?: (item: DataItem) => void;
  onViewMore?: () => void;
}

const DataTableWidget: React.FC<DataTableWidgetProps> = ({
  title,
  columns,
  data,
  rowCount,
  children,
  onRowClick,
  onViewMore,
}) => {
  const getStatusIcon = (status?: string) => {
    switch (status) {
      case 'active':
        return <CheckCircle size={16} className="text-success-text" />;
      case 'warning':
        return <AlertTriangle size={16} className="text-warning-text" />;
      case 'critical':
        return <AlertTriangle size={16} className="text-danger-text" />;
      default:
        return null;
    }
  };

  return (
    <div className="bg-surface-1 rounded-lg shadow-sm border border-border-subtle overflow-hidden">
      {/* Header */}
      <div className="flex items-center justify-between p-4 border-b border-border-subtle">
        <h3 className="text-lg font-semibold text-fg-primary">
          {title}
          {rowCount !== undefined && (
            <span className="ml-2 text-sm font-normal text-fg-secondary">({rowCount})</span>
          )}
        </h3>
        <button
          onClick={onViewMore}
          className="p-2 text-fg-secondary hover:text-fg-muted rounded-lg hover:bg-surface-sunken transition"
        >
          <MoreVertical size={18} />
        </button>
      </div>

      {/* Table */}
      {children ? (
        <div className="overflow-x-auto">{children}</div>
      ) : (
        <>
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead className="bg-surface-sunken border-b border-border-subtle">
                <tr>
                  {(columns ?? []).map((col) => (
                    <th
                      key={col}
                      className="px-4 py-3 text-left text-xs font-semibold text-fg-primary uppercase tracking-wide"
                    >
                      {col}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody className="divide-y divide-border-subtle">
                {(data ?? []).map((item) => (
                  <tr
                    key={item.id}
                    onClick={() => onRowClick?.(item)}
                    className={`transition ${onRowClick ? 'hover:bg-surface-sunken cursor-pointer' : ''}`}
                  >
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-2">
                        {getStatusIcon(item.status)}
                        <span className="text-sm font-medium text-fg-primary">{item.name}</span>
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <span className="text-sm text-fg-primary">{item.value}</span>
                    </td>
                    {item.trend !== undefined && (
                      <td className="px-4 py-3">
                        <div className="flex items-center gap-1">
                          <TrendingUp
                            size={14}
                            className={item.trend >= 0 ? 'text-success-text' : 'text-danger-text'}
                          />
                          <span
                            className={`text-sm font-medium ${item.trend >= 0 ? 'text-success-text' : 'text-danger-text'}`}
                          >
                            {item.trend >= 0 ? '+' : ''}{item.trend}%
                          </span>
                        </div>
                      </td>
                    )}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {/* Empty state */}
          {(data ?? []).length === 0 && (
            <div className="p-8 text-center">
              <p className="text-fg-muted">No data available</p>
            </div>
          )}
        </>
      )}
    </div>
  );
};

export default DataTableWidget;
