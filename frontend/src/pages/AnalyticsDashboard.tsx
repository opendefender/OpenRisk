import React, { useState, useEffect } from 'react';
import {
  LineChart,
  Line,
  AreaChart,
  Area,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ResponsiveContainer,
  ComposedChart,
} from 'recharts';
import { TrendingUp, TrendingDown, Activity } from 'lucide-react';

interface TimeSeriesData {
  timestamp: string;
  value: number;
  average?: number;
  min?: number;
  max?: number;
}

interface MetricCard {
  title: string;
  value: number;
  change: number;
  isPositive: boolean;
  icon: React.ReactNode;
  unit?: string;
}

interface TrendData {
  direction: 'UP' | 'DOWN' | 'STABLE';
  magnitude: number;
  confidence: number;
  forecast: number;
}

const AnalyticsDashboard: React.FC = () => {
  const [timeSeriesData, setTimeSeriesData] = useState<TimeSeriesData[]>([]);
  const [aggregatedData, setAggregatedData] = useState<TimeSeriesData[]>([]);
  const [metricCards, setMetricCards] = useState<MetricCard[]>([]);
  const [trendData, setTrendData] = useState<TrendData | null>(null);
  const [selectedPeriod, setSelectedPeriod] = useState<'hourly' | 'daily' | 'weekly' | 'monthly'>('daily');
  const [loading, setLoading] = useState(false);
  const [selectedMetric, setSelectedMetric] = useState('latency_ms');

  // Fetch analytics data
  useEffect(() => {
    const fetchData = async () => {
      setLoading(true);
      try {
        // Fetch time series data
        const response = await fetch(`/api/analytics/timeseries?metric=${selectedMetric}&period=${selectedPeriod}`);
        if (response.ok) {
          const data = await response.json();
          setTimeSeriesData(data.points || []);
          setAggregatedData(data.aggregated || []);
          setTrendData(data.trend || null);
          setMetricCards(data.cards || []);
        }
      } catch (error) {
        console.error('Failed to fetch analytics data:', error);
      } finally {
        setLoading(false);
      }
    };

    fetchData();
  }, [selectedMetric, selectedPeriod]);

  // Calculate performance metrics
  const calculateMetrics = () => {
    if (timeSeriesData.length === 0) return null;

    const values = timeSeriesData.map((d) => d.value);
    const average = values.reduce((a, b) => a + b, 0) / values.length;
    const min = Math.min(...values);
    const max = Math.max(...values);
    const stdDev = Math.sqrt(values.reduce((sq, n) => sq + Math.pow(n - average, 2), 0) / values.length);

    return { average, min, max, stdDev };
  };

  const metrics = calculateMetrics();

  // Render metric card
  const renderMetricCard = (card: MetricCard) => (
    <div key={card.title} className="bg-white rounded-lg shadow p-6">
      <div className="flex items-center justify-between">
        <div>
          <p className="text-gray-600 text-sm font-medium">{card.title}</p>
          <p className="text-3xl font-bold mt-2">
            {card.value}
            {card.unit && <span className="text-lg ml-1">{card.unit}</span>}
          </p>
          <p
            className={`text-sm mt-2 flex items-center gap-1 ${
              card.isPositive ? 'text-red-600' : 'text-green-600'
            }`}
          >
            {card.isPositive ? <TrendingUp size={16} /> : <TrendingDown size={16} />}
            {Math.abs(card.change)}%
          </p>
        </div>
        <div className="text-blue-600">{card.icon}</div>
      </div>
    </div>
  );

  return (
    <div className="min-h-screen bg-gray-50 p-8">
      <div className="max-w-7xl mx-auto">
        {/* Header */}
        <div className="mb-8">
          <h1 className="text-4xl font-bold text-gray-900">Analytics Dashboard</h1>
          <p className="text-gray-600 mt-2">Real-time performance metrics and trend analysis</p>
        </div>

        {/* Controls */}
        <div className="bg-white rounded-lg shadow p-6 mb-8">
          <div className="flex gap-4 flex-wrap items-center">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                Metric
              </label>
              <select
                value={selectedMetric}
                onChange={(e) => setSelectedMetric(e.target.value)}
                className="rounded border-gray-300 border px-3 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500"
              >
                <option value="latency_ms">Latency (ms)</option>
                <option value="throughput_rps">Throughput (RPS)</option>
                <option value="error_rate">Error Rate (%)</option>
                <option value="cpu_usage">CPU Usage (%)</option>
                <option value="memory_usage">Memory Usage (%)</option>
              </select>
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                Period
              </label>
              <select
                value={selectedPeriod}
                onChange={(e) => setSelectedPeriod(e.target.value as any)}
                className="rounded border-gray-300 border px-3 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500"
              >
                <option value="hourly">Hourly</option>
                <option value="daily">Daily</option>
                <option value="weekly">Weekly</option>
                <option value="monthly">Monthly</option>
              </select>
            </div>

            {loading && (
              <div className="flex items-center gap-2 text-blue-600">
                <div className="animate-spin">
                  <Activity size={20} />
                </div>
                <span>Loading...</span>
              </div>
            )}
          </div>
        </div>

        {/* Metric Cards */}
        {metricCards.length > 0 && (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-8">
            {metricCards.map(renderMetricCard)}
          </div>
        )}

        {/* Statistics Cards */}
        {metrics && (
          <div className="grid grid-cols-1 md:grid-cols-4 gap-6 mb-8">
            <div className="bg-white rounded-lg shadow p-6">
              <p className="text-gray-600 text-sm font-medium">Average</p>
              <p className="text-3xl font-bold mt-2">{metrics.average.toFixed(2)}</p>
            </div>
            <div className="bg-white rounded-lg shadow p-6">
              <p className="text-gray-600 text-sm font-medium">Minimum</p>
              <p className="text-3xl font-bold mt-2">{metrics.min.toFixed(2)}</p>
            </div>
            <div className="bg-white rounded-lg shadow p-6">
              <p className="text-gray-600 text-sm font-medium">Maximum</p>
              <p className="text-3xl font-bold mt-2">{metrics.max.toFixed(2)}</p>
            </div>
            <div className="bg-white rounded-lg shadow p-6">
              <p className="text-gray-600 text-sm font-medium">Std Dev</p>
              <p className="text-3xl font-bold mt-2">{metrics.stdDev.toFixed(2)}</p>
            </div>
          </div>
        )}

        {/* Trend Analysis */}
        {trendData && (
          <div className="bg-white rounded-lg shadow p-6 mb-8">
            <h2 className="text-2xl font-bold text-gray-900 mb-4">Trend Analysis</h2>
            <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
              <div className="border-l-4 border-blue-500 pl-4">
                <p className="text-gray-600 text-sm">Direction</p>
                <p className="text-xl font-bold text-gray-900">{trendData.direction}</p>
              </div>
              <div className="border-l-4 border-purple-500 pl-4">
                <p className="text-gray-600 text-sm">Magnitude</p>
                <p className="text-xl font-bold text-gray-900">{trendData.magnitude.toFixed(2)}</p>
              </div>
              <div className="border-l-4 border-green-500 pl-4">
                <p className="text-gray-600 text-sm">Confidence</p>
                <p className="text-xl font-bold text-gray-900">{(trendData.confidence * 100).toFixed(1)}%</p>
              </div>
              <div className="border-l-4 border-orange-500 pl-4">
                <p className="text-gray-600 text-sm">Forecast</p>
                <p className="text-xl font-bold text-gray-900">{trendData.forecast.toFixed(2)}</p>
              </div>
            </div>
          </div>
        )}

        {/* Charts */}
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-8 mb-8">
          {/* Time Series Chart */}
          <div className="bg-white rounded-lg shadow p-6">
            <h2 className="text-xl font-bold text-gray-900 mb-4">Time Series Data</h2>
            {timeSeriesData.length > 0 ? (
              <ResponsiveContainer width="100%" height={300}>
                <LineChart data={timeSeriesData}>
                  <CartesianGrid strokeDasharray="3 3" />
                  <XAxis dataKey="timestamp" />
                  <YAxis />
                  <Tooltip />
                  <Legend />
                  <Line
                    type="monotone"
                    dataKey="value"
                    stroke="#3b82f6"
                    dot={false}
                    isAnimationActive={false}
                  />
                </LineChart>
              </ResponsiveContainer>
            ) : (
              <p className="text-gray-500 text-center py-8">No data available</p>
            )}
          </div>

          {/* Aggregated Data Chart */}
          <div className="bg-white rounded-lg shadow p-6">
            <h2 className="text-xl font-bold text-gray-900 mb-4">Aggregated Data</h2>
            {aggregatedData.length > 0 ? (
              <ResponsiveContainer width="100%" height={300}>
                <AreaChart data={aggregatedData}>
                  <CartesianGrid strokeDasharray="3 3" />
                  <XAxis dataKey="timestamp" />
                  <YAxis />
                  <Tooltip />
                  <Legend />
                  <Area
                    type="monotone"
                    dataKey="average"
                    fill="#93c5fd"
                    stroke="#3b82f6"
                    isAnimationActive={false}
                  />
                </AreaChart>
              </ResponsiveContainer>
            ) : (
              <p className="text-gray-500 text-center py-8">No data available</p>
            )}
          </div>
        </div>

        {/* Distribution Chart */}
        {aggregatedData.length > 0 && (
          <div className="bg-white rounded-lg shadow p-6">
            <h2 className="text-xl font-bold text-gray-900 mb-4">Min/Max Distribution</h2>
            <ResponsiveContainer width="100%" height={300}>
              <ComposedChart data={aggregatedData}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="timestamp" />
                <YAxis />
                <Tooltip />
                <Legend />
                <Bar dataKey="min" fill="#ef4444" opacity={0.7} />
                <Bar dataKey="max" fill="#10b981" opacity={0.7} />
                <Line type="monotone" dataKey="average" stroke="#f59e0b" />
              </ComposedChart>
            </ResponsiveContainer>
          </div>
        )}
      </div>
    </div>
  );
};

export default AnalyticsDashboard;
