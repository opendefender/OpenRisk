// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import React from 'react';
import { TrendingUp, TrendingDown, AlertCircle } from 'lucide-react';

interface MetricCardProps {
  title: string;
  value: string | number;
  unit?: string;
  change?: number;
  changePercent?: number;
  isPositive?: boolean;
  trend?: 'up' | 'down' | 'stable';
  status?: 'normal' | 'warning' | 'critical';
  description?: string;
  onClick?: () => void;
}

const MetricCard: React.FC<MetricCardProps> = ({
  title,
  value,
  unit,
  change,
  changePercent,
  isPositive = true,
  trend,
  status = 'normal',
  description,
  onClick,
}) => {
  const getStatusColor = () => {
    switch (status) {
      case 'critical':
        return 'border-l-4 border-danger bg-danger-surface';
      case 'warning':
        return 'border-l-4 border-warning bg-warning-surface';
      default:
        return 'border-l-4 border-accent bg-info-surface';
    }
  };

  const getTrendColor = () => {
    if (trend === 'up') return isPositive ? 'text-success-text' : 'text-danger-text';
    if (trend === 'down') return isPositive ? 'text-danger-text' : 'text-success-text';
    return 'text-text-muted';
  };

  return (
    <div
      onClick={onClick}
      className={`p-4 rounded-lg shadow-sm hover:shadow-md transition ${getStatusColor()} ${
        onClick ? 'cursor-pointer' : ''
      }`}
    >
      {/* Header */}
      <div className="flex items-start justify-between mb-3">
        <div className="flex-1">
          <p className="text-sm font-medium text-text-muted">{title}</p>
          {description && <p className="text-xs text-text-muted mt-1">{description}</p>}
        </div>
        {status === 'critical' && <AlertCircle size={16} className="text-danger-text flex-shrink-0" />}
      </div>

      {/* Value */}
      <div className="flex items-baseline gap-2 mb-2">
        <span className="text-3xl font-bold text-text-primary">{value}</span>
        {unit && <span className="text-sm text-text-muted">{unit}</span>}
      </div>

      {/* Change */}
      {(change !== undefined || changePercent !== undefined) && (
        <div className="flex items-center gap-2">
          {trend === 'up' && <TrendingUp size={14} className={getTrendColor()} />}
          {trend === 'down' && <TrendingDown size={14} className={getTrendColor()} />}
          <span className={`text-sm font-medium ${getTrendColor()}`}>
            {isPositive ? '+' : ''}{changePercent ?? change}
            {changePercent ? '%' : ''} vs last period
          </span>
        </div>
      )}
    </div>
  );
};

export default MetricCard;
