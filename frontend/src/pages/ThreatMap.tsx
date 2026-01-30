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
        return 'from-red- to-red- text-red-';
      case 'high':
        return 'from-orange- to-orange- text-orange-';
      case 'medium':
        return 'from-yellow- to-yellow- text-yellow-';
      case 'low':
        return 'from-blue- to-blue- text-blue-';
      default:
        return 'from-zinc- to-zinc- text-zinc-';
    }
  };

  const totalThreats = filteredThreats.reduce((acc, t) => acc + t.threats, );

  return (
    <div className="max-w-xl mx-auto p-">
      {/ Header /}
      <div className="mb-">
        <h className="text-xl font-bold mb-">Threat Intelligence Map</h>
        <p className="text-zinc-">Global view of cyber threats and attacks targeting your organization</p>
      </div>

      {/ Map Section /}
      <div className="bg-surface border border-border rounded-lg p- mb- h- flex items-center justify-center relative overflow-hidden">
        <div className="absolute inset- bg-gradient-to-br from-blue-/ to-purple-/" />
        <div className="relative z- text-center">
          <Globe size={} className="mx-auto text-zinc- mb-" />
          <p className="text-zinc- mb-">Interactive threat map visualization</p>
          <p className="text-sm text-zinc-">Map view would be rendered here with a mapping library</p>
          <Button className="mt-">View Full Map</Button>
        </div>
      </div>

      {/ Stats /}
      <div className="grid grid-cols- md:grid-cols- gap- mb-">
        <div className="bg-surface border border-border rounded-lg p-">
          <div className="text-zinc- text-sm mb-">Total Threats</div>
          <div className="text-xl font-bold">{stats?.total_threats || threats.reduce((acc, t) => acc + t.threats, )}</div>
        </div>
        <div className="bg-surface border border-border rounded-lg p-">
          <div className="text-zinc- text-sm mb-">Active Countries</div>
          <div className="text-xl font-bold">{filteredThreats.length}</div>
        </div>
        <div className="bg-surface border border-border rounded-lg p-">
          <div className="text-zinc- text-sm mb-">Critical</div>
          <div className="text-xl font-bold text-red-">
            {stats?.critical || }
          </div>
        </div>
        <div className="bg-surface border border-border rounded-lg p-">
          <div className="text-zinc- text-sm mb-">Trend</div>
          <div className="flex items-center gap-">
            <TrendingUp size={} className="text-red-" />
            <span className="text-lg font-bold text-red-">↑ {stats?.trend_percent || }%</span>
          </div>
        </div>
      </div>

      {/ Filter & Threats List /}
      <div>
        <div className="flex items-center justify-between mb-">
          <h className="text-lg font-bold">Threat Sources</h>
          <div className="flex items-center gap-">
            <select
              value={threatFilter}
              onChange={(e) => setThreatFilter(e.target.value)}
              className="bg-surface border border-border px- py- rounded-lg text-sm"
            >
              <option value="all">All Severities</option>
              <option value="critical">Critical</option>
              <option value="high">High</option>
              <option value="medium">Medium</option>
              <option value="low">Low</option>
            </select>
            <Button variant="ghost" size="sm">
              <RefreshCw size={} />
            </Button>
          </div>
        </div>

        <div className="grid grid-cols- md:grid-cols- lg:grid-cols- gap-">
          {filteredThreats.map((threat, index) => (
            <motion.div
              key={threat.code}
              initial={{ opacity: , y:  }}
              animate={{ opacity: , y:  }}
              transition={{ duration: ., delay: index  . }}
              onClick={() => setSelectedThreat(threat)}
              className={bg-gradient-to-br ${getSeverityColor(threat.severity)} rounded-lg p- cursor-pointer transform transition-transform hover:scale- ${
                selectedThreat?.code === threat.code ? 'ring- ring-white' : ''
              }}
            >
              <div className="flex items-start justify-between mb-">
                <div>
                  <h className="font-bold text-sm">{threat.country}</h>
                  <p className="text-xs opacity-">{threat.code}</p>
                </div>
                <MapPin size={} />
              </div>
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-">
                  <AlertTriangle size={} />
                  <span className="font-bold">{threat.threats}</span>
                  <span className="text-xs opacity-">threats</span>
                </div>
                <span className="text-xs font-bold opacity- uppercase">{threat.severity}</span>
              </div>
            </motion.div>
          ))}
        </div>
      </div>

      {/ Selected Threat Details /}
      {selectedThreat && (
        <motion.div
          initial={{ opacity: , y:  }}
          animate={{ opacity: , y:  }}
          className="mt- bg-surface border border-border rounded-lg p-"
        >
          <h className="text-lg font-bold mb-">Threat Details: {selectedThreat.country}</h>
          <div className="grid grid-cols- md:grid-cols- gap-">
            <div>
              <p className="text-zinc- text-sm mb-">Location</p>
              <p className="font-semibold">{selectedThreat.country}</p>
              <p className="text-sm text-zinc-">{selectedThreat.lat.toFixed()}, {selectedThreat.lon.toFixed()}</p>
            </div>
            <div>
              <p className="text-zinc- text-sm mb-">Active Threats</p>
              <p className="font-semibold text-xl">{selectedThreat.threats}</p>
            </div>
            <div>
              <p className="text-zinc- text-sm mb-">Severity Level</p>
              <p className={font-semibold capitalize px- py- rounded inline-block}>{selectedThreat.severity}</p>
            </div>
          </div>
        </motion.div>
      )}
    </div>
  );
};
