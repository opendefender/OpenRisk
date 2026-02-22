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
  columns: string[];
  data: DataItem[];
  onRowClick?: (item: DataItem) => void;
  onViewMore?: () => void;
}

const DataTableWidget: React.FC<DataTableWidgetProps> = ({
  title,
  columns,
  data,
  onRowClick,
  onViewMore,
}) => {
  const getStatusIcon = (status?: string) => {
    switch (status) {
      case 'active':
        return <CheckCircle size={16} className="text-green-600" />;
      case 'warning':
        return <AlertTriangle size={16} className="text-yellow-600" />;
      case 'critical':
        return <AlertTriangle size={16} className="text-red-600" />;
      default:
        return null;
    }
  };

  return (
    <div className="bg-white rounded-lg shadow-sm border border-gray-200 overflow-hidden">
      {/* Header */}
      <div className="flex items-center justify-between p-4 border-b border-gray-100">
        <h3 className="text-lg font-semibold text-gray-900">{title}</h3>
        <button
          onClick={onViewMore}
          className="p-2 text-gray-400 hover:text-gray-600 rounded-lg hover:bg-gray-100 transition"
        >
          <MoreVertical size={18} />
        </button>
      </div>

      {/* Table */}
      <div className="overflow-x-auto">
        <table className="w-full">
          <thead className="bg-gray-50 border-b border-gray-200">
            <tr>
              {columns.map((col) => (
                <th
                  key={col}
                  className="px-4 py-3 text-left text-xs font-semibold text-gray-700 uppercase tracking-wide"
                >
                  {col}
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-200">
            {data.map((item) => (
              <tr
                key={item.id}
                onClick={() => onRowClick?.(item)}
                className={`transition ${onRowClick ? 'hover:bg-gray-50 cursor-pointer' : ''}`}
              >
                <td className="px-4 py-3">
                  <div className="flex items-center gap-2">
                    {getStatusIcon(item.status)}
                    <span className="text-sm font-medium text-gray-900">{item.name}</span>
                  </div>
                </td>
                <td className="px-4 py-3">
                  <span className="text-sm text-gray-700">{item.value}</span>
                </td>
                {item.trend !== undefined && (
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-1">
                      <TrendingUp
                        size={14}
                        className={item.trend >= 0 ? 'text-green-600' : 'text-red-600'}
                      />
                      <span
                        className={`text-sm font-medium ${item.trend >= 0 ? 'text-green-600' : 'text-red-600'}`}
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
      {data.length === 0 && (
        <div className="p-8 text-center">
          <p className="text-gray-500">No data available</p>
        </div>
      )}
    </div>
  );
};

export default DataTableWidget;
