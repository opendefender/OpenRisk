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
    <div className="bg-white border border-gray-200 rounded-lg p-4">
      <div className="space-y-4">
        {/* Period Selection */}
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-3">Time Period</label>
          <div className="grid grid-cols-3 md:grid-cols-6 gap-2">
            {PERIOD_OPTIONS.map((option) => (
              <button
                key={option.value}
                onClick={() => onPeriodChange(option.value)}
                className={`px-3 py-2 rounded-lg text-sm font-medium transition ${
                  selectedPeriod === option.value
                    ? 'bg-blue-600 text-white'
                    : 'bg-gray-100 text-gray-700 hover:bg-gray-200'
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
                <label className="block text-sm font-medium text-gray-700 mb-1">Start Date</label>
                <div className="flex items-center gap-2">
                  <Calendar size={16} className="text-gray-400" />
                  <input
                    type="date"
                    value={dateRange.start}
                    onChange={(e) => onDateRangeChange(e.target.value, dateRange.end)}
                    className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                  />
                </div>
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">End Date</label>
                <div className="flex items-center gap-2">
                  <Calendar size={16} className="text-gray-400" />
                  <input
                    type="date"
                    value={dateRange.end}
                    onChange={(e) => onDateRangeChange(dateRange.start, e.target.value)}
                    className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                  />
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Metric Selection */}
        {onMetricsChange && (
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">Metrics</label>
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
                    className="w-4 h-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                  />
                  <span className="text-sm text-gray-700">{metric.label}</span>
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
