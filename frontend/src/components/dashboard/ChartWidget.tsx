import React from 'react';
import { ChevronDown } from 'lucide-react';

interface ChartWidgetProps {
  title: string;
  subtitle?: string;
  children: React.ReactNode;
  action?: {
    label: string;
    onClick: () => void;
  };
  fullWidth?: boolean;
  height?: 'small' | 'medium' | 'large';
}

const ChartWidget: React.FC<ChartWidgetProps> = ({
  title,
  subtitle,
  children,
  action,
  fullWidth = false,
  height = 'medium',
}) => {
  const heightClass = {
    small: 'h-64',
    medium: 'h-80',
    large: 'h-96',
  }[height];

  const widthClass = fullWidth ? 'col-span-full' : '';

  return (
    <div className={`bg-white rounded-lg shadow-sm border border-gray-200 overflow-hidden ${widthClass}`}>
      {/* Header */}
      <div className="flex items-start justify-between p-4 border-b border-gray-100">
        <div className="flex-1">
          <h3 className="text-lg font-semibold text-gray-900">{title}</h3>
          {subtitle && <p className="text-sm text-gray-500 mt-1">{subtitle}</p>}
        </div>
        {action && (
          <button
            onClick={action.onClick}
            className="px-3 py-1 text-sm text-blue-600 hover:text-blue-700 hover:bg-blue-50 rounded transition flex items-center gap-1"
          >
            {action.label}
            <ChevronDown size={14} />
          </button>
        )}
      </div>

      {/* Content */}
      <div className={`p-4 ${heightClass} overflow-auto`}>{children}</div>
    </div>
  );
};

export default ChartWidget;
