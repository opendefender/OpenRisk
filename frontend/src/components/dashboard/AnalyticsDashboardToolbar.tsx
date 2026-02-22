import React from 'react';
import { Filter, Download, RefreshCw, Settings } from 'lucide-react';

interface AnalyticsDashboardToolbarProps {
  onFilterClick?: () => void;
  onExportClick?: () => void;
  onRefreshClick?: () => void;
  onSettingsClick?: () => void;
  isLoading?: boolean;
}

const AnalyticsDashboardToolbar: React.FC<AnalyticsDashboardToolbarProps> = ({
  onFilterClick,
  onExportClick,
  onRefreshClick,
  onSettingsClick,
  isLoading = false,
}) => {
  return (
    <div className="flex items-center justify-between gap-4 p-4 bg-white border-b border-gray-200 rounded-t-lg">
      {/* Left side - Title and description */}
      <div className="flex-1">
        <h2 className="text-xl font-bold text-gray-900">Analytics Dashboard</h2>
        <p className="text-sm text-gray-500">Real-time metrics and insights</p>
      </div>

      {/* Right side - Action buttons */}
      <div className="flex items-center gap-2">
        {/* Refresh button */}
        <button
          onClick={onRefreshClick}
          disabled={isLoading}
          className="p-2 text-gray-600 hover:bg-gray-100 rounded-lg transition disabled:opacity-50 disabled:cursor-not-allowed"
          title="Refresh data"
        >
          <RefreshCw size={18} className={isLoading ? 'animate-spin' : ''} />
        </button>

        {/* Settings button */}
        <button
          onClick={onSettingsClick}
          className="p-2 text-gray-600 hover:bg-gray-100 rounded-lg transition"
          title="Dashboard settings"
        >
          <Settings size={18} />
        </button>

        {/* Filter button */}
        <button
          onClick={onFilterClick}
          className="px-4 py-2 text-sm font-medium text-gray-700 bg-gray-100 hover:bg-gray-200 rounded-lg transition flex items-center gap-2"
          title="Filter data"
        >
          <Filter size={16} />
          Filter
        </button>

        {/* Export button */}
        <button
          onClick={onExportClick}
          className="px-4 py-2 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 rounded-lg transition flex items-center gap-2"
          title="Export dashboard"
        >
          <Download size={16} />
          Export
        </button>
      </div>
    </div>
  );
};

export default AnalyticsDashboardToolbar;
