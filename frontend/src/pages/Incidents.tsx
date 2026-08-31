// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { useEffect, useState } from 'react';
import { motion } from 'framer-motion';
import { AlertTriangle, Clock, User, Filter, Search, Users } from 'lucide-react';
import { Button } from '../shared/ds';
import { ViewToggle } from '../components/ViewToggle';
import { useIncidentStore } from '../hooks/useIncidentStore';

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
        return 'bg-danger/10 text-danger-text border-danger/20';
      case 'high':
        return 'bg-warning/10 text-warning-text border-warning/20';
      case 'medium':
        return 'bg-warning/10 text-warning-text border-warning/20';
      case 'low':
        return 'bg-accent-soft text-info-text border-accent-line';
      default:
        return 'bg-surface-3/10 text-fg-secondary border-border-strong/20';
    }
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'open':
        return 'bg-danger/10 text-danger-text';
      case 'investigating':
        return 'bg-warning/10 text-warning-text';
      case 'resolved':
        return 'bg-success/10 text-success-text';
      default:
        return 'bg-surface-3/10 text-fg-secondary';
    }
  };

  return (
    <div className="max-w-7xl mx-auto p-6">
      {/* Header */}
      <div className="mb-6 flex justify-between items-start md:items-center gap-4">
        <div>
          <h2 className="text-2xl font-bold mb-2">Incidents</h2>
          <p className="text-fg-secondary">Track and manage security incidents across your infrastructure</p>
        </div>
        <ViewToggle view={view} onViewChange={setView} />
      </div>

      {/* Stats */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4 mb-6">
        <div className="bg-surface border border-border rounded-lg p-4">
          <div className="text-fg-secondary text-sm mb-2">Total Incidents</div>
          <div className="text-3xl font-bold">{total}</div>
        </div>
        <div className="bg-surface border border-border rounded-lg p-4">
          <div className="text-fg-secondary text-sm mb-2">Critical</div>
          <div className="text-3xl font-bold text-danger-text">{incidents.filter((i) => i.severity === 'critical').length}</div>
        </div>
        <div className="bg-surface border border-border rounded-lg p-4">
          <div className="text-fg-secondary text-sm mb-2">Open</div>
          <div className="text-3xl font-bold text-warning-text">{incidents.filter((i) => i.status === 'open').length}</div>
        </div>
        <div className="bg-surface border border-border rounded-lg p-4">
          <div className="text-fg-secondary text-sm mb-2">Resolved</div>
          <div className="text-3xl font-bold text-success-text">{incidents.filter((i) => i.status === 'resolved').length}</div>
        </div>
      </div>

      {/* Filters & Search */}
      <div className="flex flex-col gap-4 mb-6">
        <div className="flex items-center gap-2 bg-surface border border-border-strong/5 px-3 py-2 rounded-lg flex-1">
          <Search size={16} className="text-fg-muted" />
          <input
            type="text"
            placeholder="Search incidents..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="bg-transparent border-none outline-none text-sm w-full placeholder:text-fg-muted"
          />
        </div>
        <div className="flex gap-4">
          <div className="flex items-center gap-2">
            <Filter size={16} className="text-fg-muted" />
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

      {/* Incidents List / Grid */}
      {view === 'table' && (
        <div className="space-y-3">
          {isLoading && (
            <div className="text-center py-12">
              <p className="text-fg-secondary">Loading incidents...</p>
            </div>
          )}
          {error && (
            <div className="text-center py-12">
              <p className="text-danger-text">Error: {error}</p>
            </div>
          )}
          {!isLoading && filteredIncidents.length === 0 ? (
            <div className="text-center py-12">
              <AlertTriangle size={48} className="mx-auto text-fg-muted mb-4" />
              <p className="text-fg-secondary">No incidents found</p>
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
                      <h3 className="font-semibold text-fg-primary">{incident.title}</h3>
                      <span className={`text-xs font-bold px-2 py-1 rounded border ${getSeverityColor(incident.severity)}`}>
                        {incident.severity.toUpperCase()}
                      </span>
                      <span className={`text-xs font-bold px-2 py-1 rounded ${getStatusColor(incident.status)}`}>
                        {incident.status.toUpperCase()}
                      </span>
                    </div>
                    <p className="text-sm text-fg-secondary mb-3">{incident.description}</p>
                    <div className="flex items-center gap-6 text-xs text-fg-muted">
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
      )}

      {view === 'card' && (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {isLoading && (
            <div className="col-span-full text-center py-12">
              <p className="text-fg-secondary">Loading incidents...</p>
            </div>
          )}
          {error && (
            <div className="col-span-full text-center py-12">
              <p className="text-danger-text">Error: {error}</p>
            </div>
          )}
          {!isLoading && filteredIncidents.length === 0 ? (
            <div className="col-span-full text-center py-12">
              <AlertTriangle size={48} className="mx-auto text-fg-muted mb-4" />
              <p className="text-fg-secondary">No incidents found</p>
            </div>
          ) : (
            filteredIncidents.map((incident) => (
              <motion.div
                key={incident.id}
                initial={{ opacity: 0, y: 20 }}
                animate={{ opacity: 1, y: 0 }}
                whileHover={{ y: -4 }}
                className="bg-surface border border-border rounded-lg p-6 hover:border-primary/50 transition-all cursor-pointer group"
              >
                <div className="flex items-start justify-between mb-4">
                  <div className="flex-1">
                    <h3 className="font-semibold text-fg-primary group-hover:text-primary transition-colors mb-2">
                      {incident.title}
                    </h3>
                    <p className="text-xs text-fg-muted">{incident.description?.slice(0, 100)}</p>
                  </div>
                  <span className={`text-xs font-bold px-2 py-1 rounded ml-2 shrink-0 ${getSeverityColor(incident.severity)}`}>
                    {incident.severity.charAt(0).toUpperCase()}
                  </span>
                </div>

                <div className="space-y-3 mb-4 border-t border-border pt-4">
                  <div className="flex items-center justify-between">
                    <span className="text-xs text-fg-secondary flex items-center gap-1">
                      <AlertTriangle size={14} /> Severity
                    </span>
                    <span className="text-sm font-medium capitalize">{incident.severity}</span>
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-xs text-fg-secondary flex items-center gap-1">
                      <Users size={14} /> Status
                    </span>
                    <span className={`text-xs px-2 py-1 rounded-full font-medium ${getStatusColor(incident.status)}`}>
                      {incident.status.toUpperCase()}
                    </span>
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-xs text-fg-secondary flex items-center gap-1">
                      <User size={14} /> Assignee
                    </span>
                    <span className="text-sm">{incident.assignee}</span>
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-xs text-fg-secondary flex items-center gap-1">
                      <Clock size={14} /> Date
                    </span>
                    <span className="text-sm">{new Date(incident.date).toLocaleDateString()}</span>
                  </div>
                </div>

                <Button className="w-full mt-4" variant="ghost">View Details</Button>
              </motion.div>
            ))
          )}
        </div>
      )}
    </div>
  );
};
