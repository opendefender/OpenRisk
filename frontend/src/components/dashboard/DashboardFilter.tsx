// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import React from 'react';
import { Calendar, ChevronLeft, ChevronRight } from 'lucide-react';

interface PeriodFilter {
  label: string;
  value: 'today' | '7d' | '30d' | '90d' | 'ytd' | 'custom';
}

interface DashboardFilterProps {
  selectedPeriod: string;
  onPeriodChange: (period: string) => void;
  selectedMetrics?: string[];
  onMetricsChange?: (metrics: string[]) => void;
  dateRange?: {
    start: string;
    end: string;
  };
  onDateRangeChange?: (start: string, end: string) => void;
}

const PERIOD_OPTIONS: PeriodFilter[] = [
  { label: 'Today', value: 'today' },
  { label: 'Last 7 Days', value: '7d' },
  { label: 'Last 30 Days', value: '30d' },
  { label: 'Last 90 Days', value: '90d' },
  { label: 'Year to Date', value: 'ytd' },
  { label: 'Custom', value: 'custom' },
];

const DashboardFilter: React.FC<DashboardFilterProps> = ({
  selectedPeriod,
  onPeriodChange,
  selectedMetrics = [],
  onMetricsChange,
  dateRange,
  onDateRangeChange,
}) => {
  return (
    <div className="bg-surface-1 border border-border-subtle rounded-lg p-4">
      <div className="space-y-4">
        {/* Period Selection */}
        <div>
          <label className="block text-sm font-medium text-text-primary mb-3">Time Period</label>
          <div className="grid grid-cols-3 md:grid-cols-6 gap-2">
            {PERIOD_OPTIONS.map((option) => (
              <button
                key={option.value}
                onClick={() => onPeriodChange(option.value)}
                className={`px-3 py-2 rounded-lg text-sm font-medium transition ${
                  selectedPeriod === option.value
                    ? 'bg-accent-soft text-accent-strong'
                    : 'bg-surface-sunken text-text-primary hover:bg-surface-sunken'
                }`}
              >
                {option.label}
              </button>
            ))}
          </div>
        </div>

        {/* Date Range for Custom */}
        {selectedPeriod === 'custom' && dateRange && onDateRangeChange && (
          <div className="space-y-3">
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="block text-sm font-medium text-text-primary mb-1">Start Date</label>
                <div className="flex items-center gap-2">
                  <Calendar size={16} className="text-text-secondary" />
                  <input
                    type="date"
                    value={dateRange.start}
                    onChange={(e) => onDateRangeChange(e.target.value, dateRange.end)}
                    className="w-full px-3 py-2 border border-border-subtle rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-focus"
                  />
                </div>
              </div>
              <div>
                <label className="block text-sm font-medium text-text-primary mb-1">End Date</label>
                <div className="flex items-center gap-2">
                  <Calendar size={16} className="text-text-secondary" />
                  <input
                    type="date"
                    value={dateRange.end}
                    onChange={(e) => onDateRangeChange(dateRange.start, e.target.value)}
                    className="w-full px-3 py-2 border border-border-subtle rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-focus"
                  />
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Metric Selection */}
        {onMetricsChange && (
          <div>
            <label className="block text-sm font-medium text-text-primary mb-2">Metrics</label>
            <div className="space-y-2">
              {[
                { id: 'risks', label: 'Risks' },
                { id: 'mitigations', label: 'Mitigations' },
                { id: 'assets', label: 'Assets' },
                { id: 'users', label: 'Active Users' },
              ].map((metric) => (
                <label key={metric.id} className="flex items-center gap-2 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={selectedMetrics.includes(metric.id)}
                    onChange={(e) => {
                      if (e.target.checked) {
                        onMetricsChange([...selectedMetrics, metric.id]);
                      } else {
                        onMetricsChange(selectedMetrics.filter((m) => m !== metric.id));
                      }
                    }}
                    className="w-4 h-4 rounded border-border-subtle text-info-text focus:ring-focus"
                  />
                  <span className="text-sm text-text-primary">{metric.label}</span>
                </label>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  );
};

export default DashboardFilter;
