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
        return 'border-l-4 border-red-500 bg-red-50';
      case 'warning':
        return 'border-l-4 border-yellow-500 bg-yellow-50';
      default:
        return 'border-l-4 border-blue-500 bg-blue-50';
    }
  };

  const getTrendColor = () => {
    if (trend === 'up') return isPositive ? 'text-green-600' : 'text-red-600';
    if (trend === 'down') return isPositive ? 'text-red-600' : 'text-green-600';
    return 'text-gray-600';
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
          <p className="text-sm font-medium text-gray-600">{title}</p>
          {description && <p className="text-xs text-gray-500 mt-1">{description}</p>}
        </div>
        {status === 'critical' && <AlertCircle size={16} className="text-red-600 flex-shrink-0" />}
      </div>

      {/* Value */}
      <div className="flex items-baseline gap-2 mb-2">
        <span className="text-3xl font-bold text-gray-900">{value}</span>
        {unit && <span className="text-sm text-gray-600">{unit}</span>}
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
