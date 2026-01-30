import React, { useState, useEffect } from 'react';
import {
  LineChart,
  Line,
  AreaChart,
  Area,
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ResponsiveContainer,
  ComposedChart,
} from 'recharts';
import { TrendingUp, TrendingDown, BarChart, Activity } from 'lucide-react';

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
        const response = await fetch(/api/analytics/timeseries?metric=${selectedMetric}&period=${selectedPeriod});
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
    if (timeSeriesData.length === ) return null;

    const values = timeSeriesData.map((d) => d.value);
    const average = values.reduce((a, b) => a + b, ) / values.length;
    const min = Math.min(...values);
    const max = Math.max(...values);
    const stdDev = Math.sqrt(values.reduce((sq, n) => sq + Math.pow(n - average, ), ) / values.length);

    return { average, min, max, stdDev };
  };

  const metrics = calculateMetrics();

  // Render metric card
  const renderMetricCard = (card: MetricCard) => (
    <div key={card.title} className="bg-white rounded-lg shadow p-">
      <div className="flex items-center justify-between">
        <div>
          <p className="text-gray- text-sm font-medium">{card.title}</p>
          <p className="text-xl font-bold mt-">
            {card.value}
            {card.unit && <span className="text-lg ml-">{card.unit}</span>}
          </p>
          <p
            className={text-sm mt- flex items-center gap- ${
              card.isPositive ? 'text-red-' : 'text-green-'
            }}
          >
            {card.isPositive ? <TrendingUp size={} /> : <TrendingDown size={} />}
            {Math.abs(card.change)}%
          </p>
        </div>
        <div className="text-blue-">{card.icon}</div>
      </div>
    </div>
  );

  return (
    <div className="min-h-screen bg-gray- p-">
      <div className="max-w-xl mx-auto">
        {/ Header /}
        <div className="mb-">
          <h className="text-xl font-bold text-gray-">Analytics Dashboard</h>
          <p className="text-gray- mt-">Real-time performance metrics and trend analysis</p>
        </div>

        {/ Controls /}
        <div className="bg-white rounded-lg shadow p- mb-">
          <div className="flex gap- flex-wrap items-center">
            <div>
              <label className="block text-sm font-medium text-gray- mb-">
                Metric
              </label>
              <select
                value={selectedMetric}
                onChange={(e) => setSelectedMetric(e.target.value)}
                className="rounded border-gray- border px- py- focus:outline-none focus:ring- focus:ring-blue-"
              >
                <option value="latency_ms">Latency (ms)</option>
                <option value="throughput_rps">Throughput (RPS)</option>
                <option value="error_rate">Error Rate (%)</option>
                <option value="cpu_usage">CPU Usage (%)</option>
                <option value="memory_usage">Memory Usage (%)</option>
              </select>
            </div>

            <div>
              <label className="block text-sm font-medium text-gray- mb-">
                Period
              </label>
              <select
                value={selectedPeriod}
                onChange={(e) => setSelectedPeriod(e.target.value as any)}
                className="rounded border-gray- border px- py- focus:outline-none focus:ring- focus:ring-blue-"
              >
                <option value="hourly">Hourly</option>
                <option value="daily">Daily</option>
                <option value="weekly">Weekly</option>
                <option value="monthly">Monthly</option>
              </select>
            </div>

            {loading && (
              <div className="flex items-center gap- text-blue-">
                <div className="animate-spin">
                  <Activity size={} />
                </div>
                <span>Loading...</span>
              </div>
            )}
          </div>
        </div>

        {/ Metric Cards /}
        {metricCards.length >  && (
          <div className="grid grid-cols- md:grid-cols- lg:grid-cols- gap- mb-">
            {metricCards.map(renderMetricCard)}
          </div>
        )}

        {/ Statistics Cards /}
        {metrics && (
          <div className="grid grid-cols- md:grid-cols- gap- mb-">
            <div className="bg-white rounded-lg shadow p-">
              <p className="text-gray- text-sm font-medium">Average</p>
              <p className="text-xl font-bold mt-">{metrics.average.toFixed()}</p>
            </div>
            <div className="bg-white rounded-lg shadow p-">
              <p className="text-gray- text-sm font-medium">Minimum</p>
              <p className="text-xl font-bold mt-">{metrics.min.toFixed()}</p>
            </div>
            <div className="bg-white rounded-lg shadow p-">
              <p className="text-gray- text-sm font-medium">Maximum</p>
              <p className="text-xl font-bold mt-">{metrics.max.toFixed()}</p>
            </div>
            <div className="bg-white rounded-lg shadow p-">
              <p className="text-gray- text-sm font-medium">Std Dev</p>
              <p className="text-xl font-bold mt-">{metrics.stdDev.toFixed()}</p>
            </div>
          </div>
        )}

        {/ Trend Analysis /}
        {trendData && (
          <div className="bg-white rounded-lg shadow p- mb-">
            <h className="text-xl font-bold text-gray- mb-">Trend Analysis</h>
            <div className="grid grid-cols- md:grid-cols- gap-">
              <div className="border-l- border-blue- pl-">
                <p className="text-gray- text-sm">Direction</p>
                <p className="text-xl font-bold text-gray-">{trendData.direction}</p>
              </div>
              <div className="border-l- border-purple- pl-">
                <p className="text-gray- text-sm">Magnitude</p>
                <p className="text-xl font-bold text-gray-">{trendData.magnitude.toFixed()}</p>
              </div>
              <div className="border-l- border-green- pl-">
                <p className="text-gray- text-sm">Confidence</p>
                <p className="text-xl font-bold text-gray-">{(trendData.confidence  ).toFixed()}%</p>
              </div>
              <div className="border-l- border-orange- pl-">
                <p className="text-gray- text-sm">Forecast</p>
                <p className="text-xl font-bold text-gray-">{trendData.forecast.toFixed()}</p>
              </div>
            </div>
          </div>
        )}

        {/ Charts /}
        <div className="grid grid-cols- lg:grid-cols- gap- mb-">
          {/ Time Series Chart /}
          <div className="bg-white rounded-lg shadow p-">
            <h className="text-xl font-bold text-gray- mb-">Time Series Data</h>
            {timeSeriesData.length >  ? (
              <ResponsiveContainer width="%" height={}>
                <LineChart data={timeSeriesData}>
                  <CartesianGrid strokeDasharray=" " />
                  <XAxis dataKey="timestamp" />
                  <YAxis />
                  <Tooltip />
                  <Legend />
                  <Line
                    type="monotone"
                    dataKey="value"
                    stroke="bf"
                    dot={false}
                    isAnimationActive={false}
                  />
                </LineChart>
              </ResponsiveContainer>
            ) : (
              <p className="text-gray- text-center py-">No data available</p>
            )}
          </div>

          {/ Aggregated Data Chart /}
          <div className="bg-white rounded-lg shadow p-">
            <h className="text-xl font-bold text-gray- mb-">Aggregated Data</h>
            {aggregatedData.length >  ? (
              <ResponsiveContainer width="%" height={}>
                <AreaChart data={aggregatedData}>
                  <CartesianGrid strokeDasharray=" " />
                  <XAxis dataKey="timestamp" />
                  <YAxis />
                  <Tooltip />
                  <Legend />
                  <Area
                    type="monotone"
                    dataKey="average"
                    fill="cfd"
                    stroke="bf"
                    isAnimationActive={false}
                  />
                </AreaChart>
              </ResponsiveContainer>
            ) : (
              <p className="text-gray- text-center py-">No data available</p>
            )}
          </div>
        </div>

        {/ Distribution Chart /}
        {aggregatedData.length >  && (
          <div className="bg-white rounded-lg shadow p-">
            <h className="text-xl font-bold text-gray- mb-">Min/Max Distribution</h>
            <ResponsiveContainer width="%" height={}>
              <ComposedChart data={aggregatedData}>
                <CartesianGrid strokeDasharray=" " />
                <XAxis dataKey="timestamp" />
                <YAxis />
                <Tooltip />
                <Legend />
                <Bar dataKey="min" fill="ef" opacity={.} />
                <Bar dataKey="max" fill="b" opacity={.} />
                <Line type="monotone" dataKey="average" stroke="feb" />
              </ComposedChart>
            </ResponsiveContainer>
          </div>
        )}
      </div>
    </div>
  );
};

export default AnalyticsDashboard;
