import { useEffect, useState } from 'react';
import { motion } from 'framer-motion';
import { AlertTriangle, Clock, User, Filter, Search } from 'lucide-react';
import { Button } from '../components/ui/Button';
import { useIncidentStore, type Incident } from '../hooks/useIncidentStore';

export const Incidents = () => {
  const { incidents, total, page, pageSize, isLoading, error, fetchIncidents } = useIncidentStore();
  const [filterSeverity, setFilterSeverity] = useState<string>('all');
  const [filterStatus, setFilterStatus] = useState<string>('all');
  const [searchTerm, setSearchTerm] = useState('');

  useEffect(() => {
    const severity = filterSeverity === 'all' ? undefined : filterSeverity;
    const status = filterStatus === 'all' ? undefined : filterStatus;
    fetchIncidents({ page, limit: pageSize, severity, status });
  }, [page, pageSize, filterSeverity, filterStatus, fetchIncidents]);

  const filteredIncidents = incidents.filter((incident) =>
    incident.title.toLowerCase().includes(searchTerm.toLowerCase())
  );

  const getSeverityColor = (severity: string) => {
    switch (severity) {
      case 'critical':
        return 'bg-red-500/10 text-red-400 border-red-500/20';
      case 'high':
        return 'bg-orange-500/10 text-orange-400 border-orange-500/20';
      case 'medium':
        return 'bg-yellow-500/10 text-yellow-400 border-yellow-500/20';
      case 'low':
        return 'bg-blue-500/10 text-blue-400 border-blue-500/20';
      default:
        return 'bg-zinc-500/10 text-zinc-400 border-zinc-500/20';
    }
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'open':
        return 'bg-red-500/10 text-red-400';
      case 'investigating':
        return 'bg-yellow-500/10 text-yellow-400';
      case 'resolved':
        return 'bg-green-500/10 text-green-400';
      default:
        return 'bg-zinc-500/10 text-zinc-400';
    }
  };

  return (
    <div className="max-w-7xl mx-auto p-6">
      {/* Header */}
      <div className="mb-6">
        <h2 className="text-2xl font-bold mb-2">Incidents</h2>
        <p className="text-zinc-400">Track and manage security incidents across your infrastructure</p>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4 mb-6">
        <div className="bg-surface border border-border rounded-lg p-4">
          <div className="text-zinc-400 text-sm mb-2">Total Incidents</div>
          <div className="text-3xl font-bold">{total}</div>
        </div>
        <div className="bg-surface border border-border rounded-lg p-4">
          <div className="text-zinc-400 text-sm mb-2">Critical</div>
          <div className="text-3xl font-bold text-red-400">{incidents.filter((i) => i.severity === 'critical').length}</div>
        </div>
        <div className="bg-surface border border-border rounded-lg p-4">
          <div className="text-zinc-400 text-sm mb-2">Open</div>
          <div className="text-3xl font-bold text-orange-400">{incidents.filter((i) => i.status === 'open').length}</div>
        </div>
        <div className="bg-surface border border-border rounded-lg p-4">
          <div className="text-zinc-400 text-sm mb-2">Resolved</div>
          <div className="text-3xl font-bold text-green-400">{incidents.filter((i) => i.status === 'resolved').length}</div>
        </div>
      </div>

      {/* Filters & Search */}
      <div className="flex flex-col gap-4 mb-6">
        <div className="flex items-center gap-2 bg-surface border border-white/5 px-3 py-2 rounded-lg flex-1">
          <Search size={16} className="text-zinc-500" />
          <input
            type="text"
            placeholder="Search incidents..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="bg-transparent border-none outline-none text-sm w-full placeholder:text-zinc-600"
          />
        </div>
        <div className="flex gap-4">
          <div className="flex items-center gap-2">
            <Filter size={16} className="text-zinc-500" />
            <select
              value={filterSeverity}
              onChange={(e) => setFilterSeverity(e.target.value)}
              className="bg-surface border border-border px-3 py-2 rounded-lg text-sm"
            >
              <option value="all">All Severities</option>
              <option value="critical">Critical</option>
              <option value="high">High</option>
              <option value="medium">Medium</option>
              <option value="low">Low</option>
            </select>
          </div>
          <select
            value={filterStatus}
            onChange={(e) => setFilterStatus(e.target.value)}
            className="bg-surface border border-border px-3 py-2 rounded-lg text-sm"
          >
            <option value="all">All Status</option>
            <option value="open">Open</option>
            <option value="investigating">Investigating</option>
            <option value="resolved">Resolved</option>
          </select>
        </div>
      </div>

      {/* Incidents List */}
      <div className="space-y-3">
        {isLoading && (
          <div className="text-center py-12">
            <p className="text-zinc-400">Loading incidents...</p>
          </div>
        )}
        {error && (
          <div className="text-center py-12">
            <p className="text-red-400">Error: {error}</p>
          </div>
        )}
        {!isLoading && filteredIncidents.length === 0 ? (
          <div className="text-center py-12">
            <AlertTriangle size={48} className="mx-auto text-zinc-600 mb-4" />
            <p className="text-zinc-400">No incidents found</p>
          </div>
        ) : (
          filteredIncidents.map((incident, index) => (
            <motion.div
              key={incident.id}
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.3, delay: index * 0.05 }}
              className="bg-surface border border-border rounded-lg p-4 hover:border-primary/50 transition-colors cursor-pointer hover:bg-surface/80"
            >
              <div className="flex items-start justify-between">
                <div className="flex-1">
                  <div className="flex items-center gap-3 mb-2">
                    <h3 className="font-semibold text-white">{incident.title}</h3>
                    <span className={`text-xs font-bold px-2 py-1 rounded border ${getSeverityColor(incident.severity)}`}>
                      {incident.severity.toUpperCase()}
                    </span>
                    <span className={`text-xs font-bold px-2 py-1 rounded ${getStatusColor(incident.status)}`}>
                      {incident.status.toUpperCase()}
                    </span>
                  </div>
                  <p className="text-sm text-zinc-400 mb-3">{incident.description}</p>
                  <div className="flex items-center gap-6 text-xs text-zinc-500">
                    <div className="flex items-center gap-2">
                      <Clock size={14} />
                      {new Date(incident.date).toLocaleDateString()}
                    </div>
                    <div className="flex items-center gap-2">
                      <User size={14} />
                      {incident.assignee}
                    </div>
                  </div>
                </div>
                <Button variant="ghost" className="ml-4">View Details</Button>
              </div>
            </motion.div>
          ))
        )}
      </div>
    </div>
  );
};
