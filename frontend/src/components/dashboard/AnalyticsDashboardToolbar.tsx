// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

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
    <div className="flex items-center justify-between gap-4 p-4 bg-surface-1 border-b border-border-subtle rounded-t-lg">
      {/* Left side - Title and description */}
      <div className="flex-1">
        <h2 className="text-xl font-bold text-fg-primary">Analytics Dashboard</h2>
        <p className="text-sm text-fg-muted">Real-time metrics and insights</p>
      </div>

      {/* Right side - Action buttons */}
      <div className="flex items-center gap-2">
        {/* Refresh button */}
        <button
          onClick={onRefreshClick}
          disabled={isLoading}
          className="p-2 text-fg-muted hover:bg-surface-sunken rounded-lg transition disabled:opacity-50 disabled:cursor-not-allowed"
          title="Refresh data"
        >
          <RefreshCw size={18} className={isLoading ? 'animate-spin' : ''} />
        </button>

        {/* Settings button */}
        <button
          onClick={onSettingsClick}
          className="p-2 text-fg-muted hover:bg-surface-sunken rounded-lg transition"
          title="Dashboard settings"
        >
          <Settings size={18} />
        </button>

        {/* Filter button */}
        <button
          onClick={onFilterClick}
          className="px-4 py-2 text-sm font-medium text-fg-primary bg-surface-sunken hover:bg-surface-sunken rounded-lg transition flex items-center gap-2"
          title="Filter data"
        >
          <Filter size={16} />
          Filter
        </button>

        {/* Export button */}
        <button
          onClick={onExportClick}
          className="px-4 py-2 text-sm font-medium text-fg-on-solid bg-accent-solid hover:brightness-110 rounded-lg transition flex items-center gap-2"
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
