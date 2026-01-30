import { useEffect, useState } from 'react';
import { motion } from 'framer-motion';
import { AlertTriangle, Clock, User, Filter, Search, MapPin, Users } from 'lucide-react';
import { Button } from '../components/ui/Button';
import { ViewToggle } from '../components/ViewToggle';
import { useIncidentStore, type Incident } from '../hooks/useIncidentStore';

export const Incidents = () => {
  const { incidents, total, page, pageSize, isLoading, error, fetchIncidents } = useIncidentStore();
  const [filterSeverity, setFilterSeverity] = useState<string>('all');
  const [filterStatus, setFilterStatus] = useState<string>('all');
  const [searchTerm, setSearchTerm] = useState('');
  const [view, setView] = useState<'table' | 'card'>(() => {
    const saved = localStorage.getItem('incidentView');
    return (saved as 'table' | 'card') || 'table';
  });

  useEffect(() => {
    localStorage.setItem('incidentView', view);
  }, [view]);

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
        return 'bg-red-/ text-red- border-red-/';
      case 'high':
        return 'bg-orange-/ text-orange- border-orange-/';
      case 'medium':
        return 'bg-yellow-/ text-yellow- border-yellow-/';
      case 'low':
        return 'bg-blue-/ text-blue- border-blue-/';
      default:
        return 'bg-zinc-/ text-zinc- border-zinc-/';
    }
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'open':
        return 'bg-red-/ text-red-';
      case 'investigating':
        return 'bg-yellow-/ text-yellow-';
      case 'resolved':
        return 'bg-green-/ text-green-';
      default:
        return 'bg-zinc-/ text-zinc-';
    }
  };

  return (
    <div className="max-w-xl mx-auto p-">
      {/ Header /}
      <div className="mb- flex justify-between items-start md:items-center gap-">
        <div>
          <h className="text-xl font-bold mb-">Incidents</h>
          <p className="text-zinc-">Track and manage security incidents across your infrastructure</p>
        </div>
        <ViewToggle view={view} onViewChange={setView} />
      </div>

      {/ Stats /}
      <div className="grid grid-cols- md:grid-cols- gap- mb-">
        <div className="bg-surface border border-border rounded-lg p-">
          <div className="text-zinc- text-sm mb-">Total Incidents</div>
          <div className="text-xl font-bold">{total}</div>
        </div>
        <div className="bg-surface border border-border rounded-lg p-">
          <div className="text-zinc- text-sm mb-">Critical</div>
          <div className="text-xl font-bold text-red-">{incidents.filter((i) => i.severity === 'critical').length}</div>
        </div>
        <div className="bg-surface border border-border rounded-lg p-">
          <div className="text-zinc- text-sm mb-">Open</div>
          <div className="text-xl font-bold text-orange-">{incidents.filter((i) => i.status === 'open').length}</div>
        </div>
        <div className="bg-surface border border-border rounded-lg p-">
          <div className="text-zinc- text-sm mb-">Resolved</div>
          <div className="text-xl font-bold text-green-">{incidents.filter((i) => i.status === 'resolved').length}</div>
        </div>
      </div>

      {/ Filters & Search /}
      <div className="flex flex-col gap- mb-">
        <div className="flex items-center gap- bg-surface border border-white/ px- py- rounded-lg flex-">
          <Search size={} className="text-zinc-" />
          <input
            type="text"
            placeholder="Search incidents..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="bg-transparent border-none outline-none text-sm w-full placeholder:text-zinc-"
          />
        </div>
        <div className="flex gap-">
          <div className="flex items-center gap-">
            <Filter size={} className="text-zinc-" />
            <select
              value={filterSeverity}
              onChange={(e) => setFilterSeverity(e.target.value)}
              className="bg-surface border border-border px- py- rounded-lg text-sm"
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
            className="bg-surface border border-border px- py- rounded-lg text-sm"
          >
            <option value="all">All Status</option>
            <option value="open">Open</option>
            <option value="investigating">Investigating</option>
            <option value="resolved">Resolved</option>
          </select>
        </div>
      </div>

      {/ Incidents List / Grid /}
      {view === 'table' && (
        <div className="space-y-">
          {isLoading && (
            <div className="text-center py-">
              <p className="text-zinc-">Loading incidents...</p>
            </div>
          )}
          {error && (
            <div className="text-center py-">
              <p className="text-red-">Error: {error}</p>
            </div>
          )}
          {!isLoading && filteredIncidents.length ===  ? (
            <div className="text-center py-">
              <AlertTriangle size={} className="mx-auto text-zinc- mb-" />
              <p className="text-zinc-">No incidents found</p>
            </div>
          ) : (
            filteredIncidents.map((incident, index) => (
              <motion.div
                key={incident.id}
                initial={{ opacity: , y:  }}
                animate={{ opacity: , y:  }}
                transition={{ duration: ., delay: index  . }}
                className="bg-surface border border-border rounded-lg p- hover:border-primary/ transition-colors cursor-pointer hover:bg-surface/"
              >
                <div className="flex items-start justify-between">
                  <div className="flex-">
                    <div className="flex items-center gap- mb-">
                      <h className="font-semibold text-white">{incident.title}</h>
                      <span className={text-xs font-bold px- py- rounded border ${getSeverityColor(incident.severity)}}>
                        {incident.severity.toUpperCase()}
                      </span>
                      <span className={text-xs font-bold px- py- rounded ${getStatusColor(incident.status)}}>
                        {incident.status.toUpperCase()}
                      </span>
                    </div>
                    <p className="text-sm text-zinc- mb-">{incident.description}</p>
                    <div className="flex items-center gap- text-xs text-zinc-">
                      <div className="flex items-center gap-">
                        <Clock size={} />
                        {new Date(incident.date).toLocaleDateString()}
                      </div>
                      <div className="flex items-center gap-">
                        <User size={} />
                        {incident.assignee}
                      </div>
                    </div>
                  </div>
                  <Button variant="ghost" className="ml-">View Details</Button>
                </div>
              </motion.div>
            ))
          )}
        </div>
      )}

      {view === 'card' && (
        <div className="grid grid-cols- md:grid-cols- lg:grid-cols- gap-">
          {isLoading && (
            <div className="col-span-full text-center py-">
              <p className="text-zinc-">Loading incidents...</p>
            </div>
          )}
          {error && (
            <div className="col-span-full text-center py-">
              <p className="text-red-">Error: {error}</p>
            </div>
          )}
          {!isLoading && filteredIncidents.length ===  ? (
            <div className="col-span-full text-center py-">
              <AlertTriangle size={} className="mx-auto text-zinc- mb-" />
              <p className="text-zinc-">No incidents found</p>
            </div>
          ) : (
            filteredIncidents.map((incident) => (
              <motion.div
                key={incident.id}
                initial={{ opacity: , y:  }}
                animate={{ opacity: , y:  }}
                whileHover={{ y: - }}
                className="bg-surface border border-border rounded-lg p- hover:border-primary/ transition-all cursor-pointer group"
              >
                <div className="flex items-start justify-between mb-">
                  <div className="flex-">
                    <h className="font-semibold text-white group-hover:text-primary transition-colors mb-">
                      {incident.title}
                    </h>
                    <p className="text-xs text-zinc-">{incident.description?.slice(, )}</p>
                  </div>
                  <span className={text-xs font-bold px- py- rounded ml- flex-shrink- ${getSeverityColor(incident.severity)}}>
                    {incident.severity.charAt().toUpperCase()}
                  </span>
                </div>

                <div className="space-y- mb- border-t border-border pt-">
                  <div className="flex items-center justify-between">
                    <span className="text-xs text-zinc- flex items-center gap-">
                      <AlertTriangle size={} /> Severity
                    </span>
                    <span className="text-sm font-medium capitalize">{incident.severity}</span>
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-xs text-zinc- flex items-center gap-">
                      <Users size={} /> Status
                    </span>
                    <span className={text-xs px- py- rounded-full font-medium ${getStatusColor(incident.status)}}>
                      {incident.status.toUpperCase()}
                    </span>
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-xs text-zinc- flex items-center gap-">
                      <User size={} /> Assignee
                    </span>
                    <span className="text-sm">{incident.assignee}</span>
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-xs text-zinc- flex items-center gap-">
                      <Clock size={} /> Date
                    </span>
                    <span className="text-sm">{new Date(incident.date).toLocaleDateString()}</span>
                  </div>
                </div>

                <Button className="w-full mt-" variant="ghost">View Details</Button>
              </motion.div>
            ))
          )}
        </div>
      )}
    </div>
  );
};
