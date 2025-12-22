import { useEffect, useState } from 'react';
import { motion } from 'framer-motion';
import { Globe, MapPin, AlertTriangle, TrendingUp, RefreshCw } from 'lucide-react';
import { Button } from '../components/ui/Button';
import { useThreatStore, type Threat } from '../hooks/useThreatStore';

export const ThreatMap = () => {
  const { threats, stats, isLoading, error, fetchThreats, fetchThreatStats } = useThreatStore();
  const [selectedThreat, setSelectedThreat] = useState<Threat | null>(null);
  const [threatFilter, setThreatFilter] = useState<string>('all');

  useEffect(() => {
    const severity = threatFilter === 'all' ? undefined : threatFilter;
    fetchThreats({ severity });
    fetchThreatStats();
  }, [threatFilter, fetchThreats, fetchThreatStats]);

  const filteredThreats = threats;

  const getSeverityColor = (severity: string) => {
    switch (severity) {
      case 'critical':
        return 'from-red-500 to-red-600 text-red-100';
      case 'high':
        return 'from-orange-500 to-orange-600 text-orange-100';
      case 'medium':
        return 'from-yellow-500 to-yellow-600 text-yellow-100';
      case 'low':
        return 'from-blue-500 to-blue-600 text-blue-100';
      default:
        return 'from-zinc-500 to-zinc-600 text-zinc-100';
    }
  };

  const totalThreats = filteredThreats.reduce((acc, t) => acc + t.threats, 0);

  return (
    <div className="max-w-7xl mx-auto p-6">
      {/* Header */}
      <div className="mb-6">
        <h2 className="text-2xl font-bold mb-2">Threat Intelligence Map</h2>
        <p className="text-zinc-400">Global view of cyber threats and attacks targeting your organization</p>
      </div>

      {/* Map Section */}
      <div className="bg-surface border border-border rounded-lg p-6 mb-6 h-96 flex items-center justify-center relative overflow-hidden">
        <div className="absolute inset-0 bg-gradient-to-br from-blue-500/5 to-purple-500/5" />
        <div className="relative z-10 text-center">
          <Globe size={64} className="mx-auto text-zinc-600 mb-4" />
          <p className="text-zinc-400 mb-4">Interactive threat map visualization</p>
          <p className="text-sm text-zinc-500">Map view would be rendered here with a mapping library</p>
          <Button className="mt-4">View Full Map</Button>
        </div>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4 mb-6">
        <div className="bg-surface border border-border rounded-lg p-4">
          <div className="text-zinc-400 text-sm mb-2">Total Threats</div>
          <div className="text-3xl font-bold">{stats?.total_threats || threats.reduce((acc, t) => acc + t.threats, 0)}</div>
        </div>
        <div className="bg-surface border border-border rounded-lg p-4">
          <div className="text-zinc-400 text-sm mb-2">Active Countries</div>
          <div className="text-3xl font-bold">{filteredThreats.length}</div>
        </div>
        <div className="bg-surface border border-border rounded-lg p-4">
          <div className="text-zinc-400 text-sm mb-2">Critical</div>
          <div className="text-3xl font-bold text-red-400">
            {stats?.critical || 0}
          </div>
        </div>
        <div className="bg-surface border border-border rounded-lg p-4">
          <div className="text-zinc-400 text-sm mb-2">Trend</div>
          <div className="flex items-center gap-2">
            <TrendingUp size={20} className="text-red-400" />
            <span className="text-lg font-bold text-red-400">↑ {stats?.trend_percent || 0}%</span>
          </div>
        </div>
      </div>

      {/* Filter & Threats List */}
      <div>
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-lg font-bold">Threat Sources</h3>
          <div className="flex items-center gap-2">
            <select
              value={threatFilter}
              onChange={(e) => setThreatFilter(e.target.value)}
              className="bg-surface border border-border px-3 py-2 rounded-lg text-sm"
            >
              <option value="all">All Severities</option>
              <option value="critical">Critical</option>
              <option value="high">High</option>
              <option value="medium">Medium</option>
              <option value="low">Low</option>
            </select>
            <Button variant="ghost" size="sm">
              <RefreshCw size={16} />
            </Button>
          </div>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {filteredThreats.map((threat, index) => (
            <motion.div
              key={threat.code}
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.3, delay: index * 0.05 }}
              onClick={() => setSelectedThreat(threat)}
              className={`bg-gradient-to-br ${getSeverityColor(threat.severity)} rounded-lg p-4 cursor-pointer transform transition-transform hover:scale-105 ${
                selectedThreat?.code === threat.code ? 'ring-2 ring-white' : ''
              }`}
            >
              <div className="flex items-start justify-between mb-3">
                <div>
                  <h4 className="font-bold text-sm">{threat.country}</h4>
                  <p className="text-xs opacity-80">{threat.code}</p>
                </div>
                <MapPin size={20} />
              </div>
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <AlertTriangle size={16} />
                  <span className="font-bold">{threat.threats}</span>
                  <span className="text-xs opacity-80">threats</span>
                </div>
                <span className="text-xs font-bold opacity-80 uppercase">{threat.severity}</span>
              </div>
            </motion.div>
          ))}
        </div>
      </div>

      {/* Selected Threat Details */}
      {selectedThreat && (
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          className="mt-6 bg-surface border border-border rounded-lg p-6"
        >
          <h4 className="text-lg font-bold mb-4">Threat Details: {selectedThreat.country}</h4>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div>
              <p className="text-zinc-400 text-sm mb-2">Location</p>
              <p className="font-semibold">{selectedThreat.country}</p>
              <p className="text-sm text-zinc-500">{selectedThreat.lat.toFixed(4)}°, {selectedThreat.lon.toFixed(4)}°</p>
            </div>
            <div>
              <p className="text-zinc-400 text-sm mb-2">Active Threats</p>
              <p className="font-semibold text-2xl">{selectedThreat.threats}</p>
            </div>
            <div>
              <p className="text-zinc-400 text-sm mb-2">Severity Level</p>
              <p className={`font-semibold capitalize px-3 py-1 rounded inline-block`}>{selectedThreat.severity}</p>
            </div>
          </div>
        </motion.div>
      )}
    </div>
  );
};
